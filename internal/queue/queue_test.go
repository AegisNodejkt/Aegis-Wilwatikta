package queue

import (
	"context"
	"errors"
	"testing"

	"github.com/aegis-wilwatikta/ai-reviewer/internal/domain"
)

// newTestQueue creates a MemoryAdapter for unit testing.
func newTestQueue() *MemoryAdapter {
	return NewMemoryAdapter()
}

func TestEnqueue_SetsJobFields(t *testing.T) {
	q := newTestQueue()
	payload := []byte(`{"pr": 42}`)

	job, err := q.Enqueue(context.Background(), "reviews", payload)
	if err != nil {
		t.Fatalf("Enqueue returned error: %v", err)
	}

	if job.ID == "" {
		t.Error("expected non-empty job ID")
	}
	if job.QueueName != "reviews" {
		t.Errorf("expected queue name 'reviews', got %q", job.QueueName)
	}
	if string(job.Payload) != string(payload) {
		t.Errorf("expected payload %q, got %q", payload, job.Payload)
	}
	if job.Status != domain.JobStatusPending {
		t.Errorf("expected status %q, got %q", domain.JobStatusPending, job.Status)
	}
	if job.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
	if job.UpdatedAt.IsZero() {
		t.Error("expected non-zero UpdatedAt")
	}
}

func TestDequeue_ReturnsFirstJob(t *testing.T) {
	q := newTestQueue()

	job1, _ := q.Enqueue(context.Background(), "reviews", []byte("first"))
	job2, _ := q.Enqueue(context.Background(), "reviews", []byte("second"))

	dequeued, err := q.Dequeue(context.Background(), "reviews")
	if err != nil {
		t.Fatalf("Dequeue returned error: %v", err)
	}

	if dequeued.ID != job1.ID {
		t.Errorf("expected first job ID %q, got %q", job1.ID, dequeued.ID)
	}
	if dequeued.Status != domain.JobStatusRunning {
		t.Errorf("expected status running after dequeue, got %q", dequeued.Status)
	}

	// Second dequeue should return the second job
	dequeued2, _ := q.Dequeue(context.Background(), "reviews")
	if dequeued2.ID != job2.ID {
		t.Errorf("expected second job ID %q, got %q", job2.ID, dequeued2.ID)
	}
}

func TestDequeue_EmptyQueue(t *testing.T) {
	q := newTestQueue()

	_, err := q.Dequeue(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for empty queue, got nil")
	}

	var qerr *QueueError
	if !errors.As(err, &qerr) || qerr.Code != "QUEUE_EMPTY" {
		t.Errorf("expected QUEUE_EMPTY error, got %v", err)
	}
}

func TestAck_CompletesJob(t *testing.T) {
	q := newTestQueue()

	job, _ := q.Enqueue(context.Background(), "reviews", []byte("test"))
	q.Dequeue(context.Background(), "reviews")

	err := q.Ack(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("Ack returned error: %v", err)
	}

	// Verify job status is completed
	q.mu.Lock()
	stored := q.jobs[job.ID]
	q.mu.Unlock()

	if stored.Status != domain.JobStatusCompleted {
		t.Errorf("expected status completed, got %q", stored.Status)
	}
}

func TestNack_MarksJobFailed(t *testing.T) {
	q := newTestQueue()

	job, _ := q.Enqueue(context.Background(), "reviews", []byte("test"))
	q.Dequeue(context.Background(), "reviews")

	err := q.Nack(context.Background(), job.ID, "processing error")
	if err != nil {
		t.Fatalf("Nack returned error: %v", err)
	}

	q.mu.Lock()
	stored := q.jobs[job.ID]
	q.mu.Unlock()

	if stored.Status != domain.JobStatusFailed {
		t.Errorf("expected status failed, got %q", stored.Status)
	}
}

func TestFullLifecycle_EnqueueDequeueAck(t *testing.T) {
	q := newTestQueue()
	ctx := context.Background()

	// Enqueue 3 jobs
	j1, _ := q.Enqueue(ctx, "ingestion", []byte("job-1"))
	j2, _ := q.Enqueue(ctx, "ingestion", []byte("job-2"))
	j3, _ := q.Enqueue(ctx, "ingestion", []byte("job-3"))

	// Dequeue and ack first
	d1, _ := q.Dequeue(ctx, "ingestion")
	if d1.ID != j1.ID {
		t.Fatalf("expected job1, got %s", d1.ID)
	}
	q.Ack(ctx, d1.ID)

	// Dequeue and nack second
	d2, _ := q.Dequeue(ctx, "ingestion")
	if d2.ID != j2.ID {
		t.Fatalf("expected job2, got %s", d2.ID)
	}
	q.Nack(ctx, d2.ID, "transient error")

	// Dequeue third
	d3, _ := q.Dequeue(ctx, "ingestion")
	if d3.ID != j3.ID {
		t.Fatalf("expected job3, got %s", d3.ID)
	}

	// Queue should now be empty
	_, err := q.Dequeue(ctx, "ingestion")
	if err == nil {
		t.Error("expected empty queue error")
	}
}

func TestMultipleQueues(t *testing.T) {
	q := newTestQueue()
	ctx := context.Background()

	q.Enqueue(ctx, "reviews", []byte("review-job"))
	q.Enqueue(ctx, "ingestion", []byte("ingest-job"))

	rJob, _ := q.Dequeue(ctx, "reviews")
	if string(rJob.Payload) != "review-job" {
		t.Errorf("expected review-job, got %s", rJob.Payload)
	}

	iJob, _ := q.Dequeue(ctx, "ingestion")
	if string(iJob.Payload) != "ingest-job" {
		t.Errorf("expected ingest-job, got %s", iJob.Payload)
	}
}

func TestAck_UnknownJob(t *testing.T) {
	q := newTestQueue()

	err := q.Ack(context.Background(), "nonexistent-id")
	if err == nil {
		t.Fatal("expected error for unknown job, got nil")
	}

	var qerr *QueueError
	if !errors.As(err, &qerr) || qerr.Code != "ACK_NOT_FOUND" {
		t.Errorf("expected ACK_NOT_FOUND error, got %v", err)
	}
}

func TestNack_UnknownJob(t *testing.T) {
	q := newTestQueue()

	err := q.Nack(context.Background(), "nonexistent-id", "reason")
	if err == nil {
		t.Fatal("expected error for unknown job, got nil")
	}

	var qerr *QueueError
	if !errors.As(err, &qerr) || qerr.Code != "NACK_NOT_FOUND" {
		t.Errorf("expected NACK_NOT_FOUND error, got %v", err)
	}
}

func TestClose_PreventsOperations(t *testing.T) {
	q := newTestQueue()
	q.Close()

	_, err := q.Enqueue(context.Background(), "reviews", []byte("test"))
	if err == nil {
		t.Fatal("expected error after close, got nil")
	}
}

func TestQueueError_Unwrap(t *testing.T) {
	inner := errors.New("inner error")
	qerr := &QueueError{Code: "TEST", Message: "msg", Cause: inner}

	if !errors.Is(qerr, inner) {
		t.Error("expected errors.Is to match inner error")
	}

	unwrapped := errors.Unwrap(qerr)
	if unwrapped != inner {
		t.Error("expected Unwrap to return inner error")
	}
}

func TestNewQueue_MemoryBackend(t *testing.T) {
	q, err := NewQueue(Config{Backend: "memory"})
	if err != nil {
		t.Fatalf("NewQueue returned error: %v", err)
	}
	if q == nil {
		t.Fatal("expected non-nil queue")
	}
}

func TestNewQueue_UnknownBackend(t *testing.T) {
	_, err := NewQueue(Config{Backend: "kafka"})
	if err == nil {
		t.Fatal("expected error for unknown backend")
	}
}

func TestNewQueue_RedisMissingURL(t *testing.T) {
	_, err := NewQueue(Config{Backend: "redis"})
	if err == nil {
		t.Fatal("expected error for missing Redis URL")
	}
}
