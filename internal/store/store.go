package store

import (
	"context"

	"github.com/aegis-wilwatikta/ai-reviewer/internal/domain"
)

// IntegrationStore provides CRUD operations for git integrations,
// always scoped by tenant_id and project_id.
type IntegrationStore interface {
	// Create inserts a new git integration.
	Create(ctx context.Context, integration *domain.ProjectGitIntegration) error

	// GetByID fetches a single integration by ID, scoped to the given tenant.
	GetByID(ctx context.Context, tenantID, id string) (*domain.ProjectGitIntegration, error)

	// ListByProject returns all integrations for a project, scoped by tenant.
	ListByProject(ctx context.Context, tenantID, projectID string) ([]*domain.ProjectGitIntegration, error)

	// Update modifies an existing integration.
	Update(ctx context.Context, integration *domain.ProjectGitIntegration) error

	// Delete soft-deletes by setting is_active = false.
	Delete(ctx context.Context, tenantID, id string) error
}

// WebhookEventStore provides operations for webhook event persistence
// with idempotency guarantees.
type WebhookEventStore interface {
	// Insert stores a new webhook event. Returns ErrDuplicateEvent if
	// an event with the same idempotency_key already exists.
	Insert(ctx context.Context, event *domain.GitWebhookEvent) error

	// FindByIdempotencyKey looks up an event by its idempotency key.
	FindByIdempotencyKey(ctx context.Context, tenantID, key string) (*domain.GitWebhookEvent, error)

	// UpdateStatus changes the processing status of an event.
	UpdateStatus(ctx context.Context, id string, status domain.WebhookEventStatus, errMsg string) error
}

// IngestionJobStore manages PR ingestion job lifecycle and state transitions.
type IngestionJobStore interface {
	// Create inserts a new ingestion job with status "queued".
	Create(ctx context.Context, job *domain.PRIngestionJob) error

	// GetByID fetches a job by ID, scoped by tenant.
	GetByID(ctx context.Context, tenantID, id string) (*domain.PRIngestionJob, error)

	// ClaimNextQueued atomically picks the next "queued" job and sets it to "running".
	// Returns nil if no jobs are available.
	ClaimNextQueued(ctx context.Context) (*domain.PRIngestionJob, error)

	// MarkCompleted transitions a job to "completed".
	MarkCompleted(ctx context.Context, id string) error

	// MarkFailed transitions a job to "failed" with an error message.
	// Increments retry_count. If retry_count < max_retries, resets status to "queued".
	MarkFailed(ctx context.Context, id string, errMsg string) error

	// MarkCancelled transitions a job to "cancelled".
	MarkCancelled(ctx context.Context, id string) error
}

// Store aggregates all repository interfaces.
type Store interface {
	Integrations() IntegrationStore
	WebhookEvents() WebhookEventStore
	IngestionJobs() IngestionJobStore
}
