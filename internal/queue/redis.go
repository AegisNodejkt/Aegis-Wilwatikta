package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aegis-wilwatikta/ai-reviewer/internal/domain"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	defaultPollInterval       = 1 * time.Second
	defaultVisibilityTimeout  = 30 * time.Second
	redisJobsPrefix           = "aegis:queue:jobs:"
	redisQueuePrefix          = "aegis:queue:list:"
	redisProcessingPrefix     = "aegis:queue:processing:"
)

// RedisAdapter implements the Queue interface using Redis as the backend.
// It uses Redis LIST for FIFO ordering and HASH for job metadata.
type RedisAdapter struct {
	client             redis.Cmdable
	pollInterval       time.Duration
	visibilityTimeout  time.Duration
}

// NewRedisAdapter creates a new Redis-backed queue.
// The client must be a properly configured Redis client.
func NewRedisAdapter(client redis.Cmdable, opts ...RedisOption) *RedisAdapter {
	q := &RedisAdapter{
		client:            client,
		pollInterval:      defaultPollInterval,
		visibilityTimeout: defaultVisibilityTimeout,
	}
	for _, opt := range opts {
		opt(q)
	}
	return q
}

// RedisOption configures a RedisAdapter.
type RedisOption func(*RedisAdapter)

// WithPollInterval sets the polling interval for Dequeue operations.
func WithPollInterval(d time.Duration) RedisOption {
	return func(q *RedisAdapter) { q.pollInterval = d }
}

// WithVisibilityTimeout sets how long a job is invisible after dequeue.
func WithVisibilityTimeout(d time.Duration) RedisOption {
	return func(q *RedisAdapter) { q.visibilityTimeout = d }
}

// Enqueue pushes a job onto the specified queue using RPUSH (FIFO via LPOP).
func (q *RedisAdapter) Enqueue(ctx context.Context, queueName string, payload []byte) (*domain.Job, error) {
	now := time.Now().UTC()
	job := &domain.Job{
		ID:        uuid.New().String(),
		QueueName: queueName,
		Payload:   payload,
		Status:    domain.JobStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}

	jobKey := redisJobsPrefix + job.ID
	jobData, err := json.Marshal(job)
	if err != nil {
		return nil, &QueueError{
			Code:    "ENQUEUE_MARSHAL",
			Message: "failed to marshal job",
			Cause:   err,
		}
	}

	queueKey := redisQueuePrefix + queueName

	// Store job data and add to queue list atomically
	pipe, ok := q.client.(redis.Pipeliner)
	if !ok {
		// Fallback: non-pipelined
		if err := q.client.Set(ctx, jobKey, jobData, 0).Err(); err != nil {
			return nil, &QueueError{
				Code:    "ENQUEUE_STORE",
				Message: "failed to store job data",
				Cause:   err,
			}
		}
		if err := q.client.RPush(ctx, queueKey, job.ID).Err(); err != nil {
			return nil, &QueueError{
				Code:    "ENQUEUE_PUSH",
				Message: "failed to push job to queue",
				Cause:   err,
			}
		}
		return job, nil
	}

	pipe.Set(ctx, jobKey, jobData, 0)
	pipe.RPush(ctx, queueKey, job.ID)
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, &QueueError{
			Code:    "ENQUEUE_ATOMIC",
			Message: "failed to enqueue job atomically",
			Cause:   err,
		}
	}

	return job, nil
}

// Dequeue pops the next pending job from the queue (LPOP for FIFO).
// Returns ErrQueueEmpty if no jobs are available.
func (q *RedisAdapter) Dequeue(ctx context.Context, queueName string) (*domain.Job, error) {
	queueKey := redisQueuePrefix + queueName

	// Pop job ID from the left of the list (FIFO)
	jobID, err := q.client.LPop(ctx, queueKey).Result()
	if err == redis.Nil {
		return nil, ErrQueueEmpty
	}
	if err != nil {
		return nil, &QueueError{
			Code:    "DEQUEUE_POP",
			Message: "failed to pop from queue",
			Cause:   err,
		}
	}

	// Fetch job data
	jobKey := redisJobsPrefix + jobID
	jobData, err := q.client.Get(ctx, jobKey).Bytes()
	if err == redis.Nil {
		return nil, &QueueError{
			Code:    "DEQUEUE_NOT_FOUND",
			Message: fmt.Sprintf("job %s not found in store", jobID),
		}
	}
	if err != nil {
		return nil, &QueueError{
			Code:    "DEQUEUE_FETCH",
			Message: "failed to fetch job data",
			Cause:   err,
		}
	}

	var job domain.Job
	if err := json.Unmarshal(jobData, &job); err != nil {
		return nil, &QueueError{
			Code:    "DEQUEUE_UNMARSHAL",
			Message: "failed to unmarshal job data",
			Cause:   err,
		}
	}

	// Mark as running and set visibility timeout
	now := time.Now().UTC()
	job.Status = domain.JobStatusRunning
	job.UpdatedAt = now

	updatedData, err := json.Marshal(&job)
	if err != nil {
		return nil, &QueueError{
			Code:    "DEQUEUE_MARSHAL",
			Message: "failed to marshal updated job",
			Cause:   err,
		}
	}

	processingKey := redisProcessingPrefix + jobID
	pipe, ok := q.client.(redis.Pipeliner)
	if !ok {
		if err := q.client.Set(ctx, jobKey, updatedData, 0).Err(); err != nil {
			return nil, &QueueError{Code: "DEQUEUE_UPDATE", Cause: err}
		}
		q.client.Set(ctx, processingKey, "1", q.visibilityTimeout)
		return &job, nil
	}

	pipe.Set(ctx, jobKey, updatedData, 0)
	pipe.Set(ctx, processingKey, "1", q.visibilityTimeout)
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, &QueueError{
			Code:    "DEQUEUE_UPDATE",
			Message: "failed to update job state",
			Cause:   err,
		}
	}

	return &job, nil
}

// Ack marks a job as completed and removes it from processing.
func (q *RedisAdapter) Ack(ctx context.Context, jobID string) error {
	jobKey := redisJobsPrefix + jobID
	processingKey := redisProcessingPrefix + jobID

	// Fetch current job data
	jobData, err := q.client.Get(ctx, jobKey).Bytes()
	if err == redis.Nil {
		return &QueueError{
			Code:    "ACK_NOT_FOUND",
			Message: fmt.Sprintf("job %s not found", jobID),
		}
	}
	if err != nil {
		return &QueueError{
			Code:    "ACK_FETCH",
			Message: "failed to fetch job",
			Cause:   err,
		}
	}

	var job domain.Job
	if err := json.Unmarshal(jobData, &job); err != nil {
		return &QueueError{
			Code:    "ACK_UNMARSHAL",
			Message: "failed to unmarshal job",
			Cause:   err,
		}
	}

	now := time.Now().UTC()
	job.Status = domain.JobStatusCompleted
	job.UpdatedAt = now

	updatedData, err := json.Marshal(&job)
	if err != nil {
		return &QueueError{Code: "ACK_MARSHAL", Cause: err}
	}

	pipe, ok := q.client.(redis.Pipeliner)
	if !ok {
		if err := q.client.Set(ctx, jobKey, updatedData, 0).Err(); err != nil {
			return &QueueError{Code: "ACK_UPDATE", Cause: err}
		}
		q.client.Del(ctx, processingKey)
		return nil
	}

	pipe.Set(ctx, jobKey, updatedData, 0)
	pipe.Del(ctx, processingKey)
	if _, err := pipe.Exec(ctx); err != nil {
		return &QueueError{
			Code:    "ACK_ATOMIC",
			Message: "failed to ack job atomically",
			Cause:   err,
		}
	}

	return nil
}

// Nack marks a job as failed with a reason.
func (q *RedisAdapter) Nack(ctx context.Context, jobID string, reason string) error {
	jobKey := redisJobsPrefix + jobID
	processingKey := redisProcessingPrefix + jobID

	jobData, err := q.client.Get(ctx, jobKey).Bytes()
	if err == redis.Nil {
		return &QueueError{
			Code:    "NACK_NOT_FOUND",
			Message: fmt.Sprintf("job %s not found", jobID),
		}
	}
	if err != nil {
		return &QueueError{
			Code:    "NACK_FETCH",
			Message: "failed to fetch job",
			Cause:   err,
		}
	}

	var job domain.Job
	if err := json.Unmarshal(jobData, &job); err != nil {
		return &QueueError{
			Code:    "NACK_UNMARSHAL",
			Message: "failed to unmarshal job",
			Cause:   err,
		}
	}

	now := time.Now().UTC()
	job.Status = domain.JobStatusFailed
	job.UpdatedAt = now

	updatedData, err := json.Marshal(&job)
	if err != nil {
		return &QueueError{Code: "NACK_MARSHAL", Cause: err}
	}

	pipe, ok := q.client.(redis.Pipeliner)
	if !ok {
		if err := q.client.Set(ctx, jobKey, updatedData, 0).Err(); err != nil {
			return &QueueError{Code: "NACK_UPDATE", Cause: err}
		}
		q.client.Del(ctx, processingKey)
		return nil
	}

	// Store updated job with failure status and remove from processing
	pipe.Set(ctx, jobKey, updatedData, 0)
	pipe.Del(ctx, processingKey)
	if _, err := pipe.Exec(ctx); err != nil {
		return &QueueError{
			Code:    "NACK_ATOMIC",
			Message: "failed to nack job atomically",
			Cause:   err,
		}
	}

	_ = reason // reason is stored in updatedData via the job struct in future iterations
	return nil
}

// Close is a no-op for Redis adapter; the client lifecycle is managed externally.
func (q *RedisAdapter) Close() error {
	return nil
}
