package queue

import (
	"context"
	"time"

	"github.com/aegis-wilwatikta/ai-reviewer/internal/domain"
)

// Queue defines the abstract interface for job queue operations.
// Implementations must be safe for concurrent use.
type Queue interface {
	// Enqueue pushes a new job onto the named queue. Returns the created Job
	// with its assigned ID and timestamps populated.
	Enqueue(ctx context.Context, queueName string, payload []byte) (*domain.Job, error)

	// Dequeue pops the next available job from the named queue for a worker
	// to process. Blocks until a job is available or the context is cancelled.
	// Returns ErrQueueEmpty if no jobs are available.
	Dequeue(ctx context.Context, queueName string) (*domain.Job, error)

	// Ack marks a job as successfully completed after processing.
	Ack(ctx context.Context, jobID string) error

	// Nack marks a job as failed with an optional reason. The job may be
	// retried depending on the backend implementation.
	Nack(ctx context.Context, jobID string, reason string) error

	// Close releases any resources held by the queue backend.
	Close() error
}

// ErrQueueEmpty is returned by Dequeue when no jobs are available.
var ErrQueueEmpty = &QueueError{Code: "QUEUE_EMPTY", Message: "no jobs available in queue"}

// QueueError represents a queue-specific error.
type QueueError struct {
	Code    string
	Message string
	Cause   error
}

func (e *QueueError) Error() string {
	if e.Cause != nil {
		return e.Code + ": " + e.Message + ": " + e.Cause.Error()
	}
	return e.Code + ": " + e.Message
}

func (e *QueueError) Unwrap() error {
	return e.Cause
}

// Config holds configuration for creating a Queue backend.
type Config struct {
	// Backend selects the queue implementation: "redis" (default), "memory".
	Backend string

	// RedisURL is the Redis connection URL (e.g., "redis://localhost:6379/0").
	// Required when Backend is "redis".
	RedisURL string

	// PollInterval is the duration between dequeue polling attempts.
	// Defaults to 1 second if zero.
	PollInterval time.Duration

	// VisibilityTimeout is how long a dequeued job is invisible to other
	// workers before it becomes available again. Defaults to 30 seconds.
	VisibilityTimeout time.Duration
}
