package queue

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aegis-wilwatikta/ai-reviewer/internal/domain"
	"github.com/google/uuid"
)

// MemoryAdapter implements the Queue interface with in-memory storage.
// Primarily intended for testing and development.
type MemoryAdapter struct {
	mu        sync.Mutex
	jobs      map[string]*domain.Job    // jobID → Job
	queues    map[string][]string       // queueName → []jobID (ordered)
	closed    bool
}

// NewMemoryAdapter creates a new in-memory queue adapter.
func NewMemoryAdapter() *MemoryAdapter {
	return &MemoryAdapter{
		jobs:   make(map[string]*domain.Job),
		queues: make(map[string][]string),
	}
}

func (m *MemoryAdapter) Enqueue(ctx context.Context, queueName string, payload []byte) (*domain.Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil, &QueueError{Code: "QUEUE_CLOSED", Message: "queue is closed"}
	}

	now := time.Now().UTC()
	job := &domain.Job{
		ID:        uuid.New().String(),
		QueueName: queueName,
		Payload:   payload,
		Status:    domain.JobStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}

	m.jobs[job.ID] = job
	m.queues[queueName] = append(m.queues[queueName], job.ID)
	return job, nil
}

func (m *MemoryAdapter) Dequeue(ctx context.Context, queueName string) (*domain.Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil, &QueueError{Code: "QUEUE_CLOSED", Message: "queue is closed"}
	}

	queue := m.queues[queueName]
	if len(queue) == 0 {
		return nil, ErrQueueEmpty
	}

	jobID := queue[0]
	m.queues[queueName] = queue[1:]

	job, ok := m.jobs[jobID]
	if !ok {
		return nil, &QueueError{
			Code:    "DEQUEUE_NOT_FOUND",
			Message: fmt.Sprintf("job %s not found", jobID),
		}
	}

	now := time.Now().UTC()
	job.Status = domain.JobStatusRunning
	job.UpdatedAt = now
	return job, nil
}

func (m *MemoryAdapter) Ack(ctx context.Context, jobID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, ok := m.jobs[jobID]
	if !ok {
		return &QueueError{
			Code:    "ACK_NOT_FOUND",
			Message: fmt.Sprintf("job %s not found", jobID),
		}
	}

	now := time.Now().UTC()
	job.Status = domain.JobStatusCompleted
	job.UpdatedAt = now
	return nil
}

func (m *MemoryAdapter) Nack(ctx context.Context, jobID string, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, ok := m.jobs[jobID]
	if !ok {
		return &QueueError{
			Code:    "NACK_NOT_FOUND",
			Message: fmt.Sprintf("job %s not found", jobID),
		}
	}

	now := time.Now().UTC()
	job.Status = domain.JobStatusFailed
	job.UpdatedAt = now
	_ = reason
	return nil
}

func (m *MemoryAdapter) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

// Suppress unused import for json in memory adapter (used for future serialization)
