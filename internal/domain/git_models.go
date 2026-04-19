package domain

import "time"

// GitProvider enumerates supported git hosting platforms.
type GitProvider string

const (
	GitProviderGitHub GitProvider = "github"
	GitProviderGitLab GitProvider = "gitlab"
)

// ProjectGitIntegration represents a git integration configured for a project.
// Maps to the project_git_integrations table (tech doc §4.1).
type ProjectGitIntegration struct {
	ID             string      // Primary key (UUID)
	TenantID       string      // Tenant scope
	ProjectID      string      // Project scope
	Provider       GitProvider // github or gitlab
	RepositoryURL  string      // Full repository URL
	WebhookSecret  string      // Secret for validating webhook payloads
	InstallID      string      // GitHub App installation ID or equivalent
	AccessToken    string      // OAuth or personal access token (encrypted at rest)
	RefreshToken   string      // OAuth refresh token (encrypted at rest)
	DefaultBranch  string      // Default branch name (e.g. "main")
	IsActive       bool        // Whether the integration is active
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// WebhookEventStatus represents the processing state of a webhook event.
type WebhookEventStatus string

const (
	WebhookEventReceived   WebhookEventStatus = "received"
	WebhookEventProcessing WebhookEventStatus = "processing"
	WebhookEventProcessed  WebhookEventStatus = "processed"
	WebhookEventFailed     WebhookEventStatus = "failed"
)

// GitWebhookEvent represents a received git webhook event.
// Maps to the git_webhook_events table (tech doc §4.1).
type GitWebhookEvent struct {
	ID              string              // Primary key (UUID)
	TenantID        string              // Tenant scope
	IntegrationID   string              // FK → project_git_integrations
	IdempotencyKey  string              // Unique key for dedup (e.g., delivery GUID)
	EventType       string              // e.g., "pull_request", "push"
	Payload         []byte              // Raw webhook payload (JSON)
	Status          WebhookEventStatus  // Processing status
	ProcessedAt     *time.Time          // When the event was fully processed
	Error           string              // Error message if processing failed
	CreatedAt       time.Time
}

// IngestionJobStatus represents the state of a PR ingestion job.
type IngestionJobStatus string

const (
	IngestionJobQueued    IngestionJobStatus = "queued"
	IngestionJobRunning   IngestionJobStatus = "running"
	IngestionJobCompleted IngestionJobStatus = "completed"
	IngestionJobFailed    IngestionJobStatus = "failed"
	IngestionJobCancelled IngestionJobStatus = "cancelled"
)

// PRIngestionJob represents a job to ingest and process a pull request.
// Maps to the pr_ingestion_jobs table (tech doc §4.1).
type PRIngestionJob struct {
	ID            string               // Primary key (UUID)
	TenantID      string               // Tenant scope
	IntegrationID string               // FK → project_git_integrations
	EventID       string               // FK → git_webhook_events
	Repository    string               // "owner/repo" format
	PRNumber      int                  // Pull request number
	Status        IngestionJobStatus   // Current job state
	Error         string               // Error message on failure
	RetryCount    int                  // Number of retry attempts
	MaxRetries    int                  // Maximum allowed retries
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
