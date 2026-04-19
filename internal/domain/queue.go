package domain

import "time"

// JobStatus represents the lifecycle state of a queued job.
type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
)

// Job represents a unit of work in the queue.
type Job struct {
	ID        string    // Unique job identifier
	QueueName string    // Name of the queue this job belongs to
	Payload   []byte    // Serialized job payload
	Status    JobStatus // Current job status
	CreatedAt time.Time // When the job was enqueued
	UpdatedAt time.Time // When the job was last updated
}
