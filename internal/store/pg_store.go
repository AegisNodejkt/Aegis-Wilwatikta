package store

import (
	"context"
	"database/sql"
	"fmt"
	"sync"

	"github.com/aegis-wilwatikta/ai-reviewer/internal/domain"
)

// PGStore implements the Store interface backed by PostgreSQL.
type PGStore struct {
	integrations  IntegrationStore
	webhookEvents WebhookEventStore
	ingestionJobs IngestionJobStore
}

// NewPGStore creates a new PostgreSQL-backed store.
func NewPGStore(db *sql.DB) *PGStore {
	return &PGStore{
		integrations:  &pgIntegrationStore{db: db},
		webhookEvents: &pgWebhookEventStore{db: db},
		ingestionJobs: &pgIngestionJobStore{db: db},
	}
}

func (s *PGStore) Integrations() IntegrationStore     { return s.integrations }
func (s *PGStore) WebhookEvents() WebhookEventStore    { return s.webhookEvents }
func (s *PGStore) IngestionJobs() IngestionJobStore    { return s.ingestionJobs }

// compile-time check
var _ Store = (*PGStore)(nil)

// ============================================================================
// IntegrationStore
// ============================================================================

type pgIntegrationStore struct {
	db *sql.DB
}

func (s *pgIntegrationStore) Create(ctx context.Context, integration *domain.ProjectGitIntegration) error {
	query := `
		INSERT INTO project_git_integrations
			(id, tenant_id, project_id, provider, repository_url, webhook_secret,
			 install_id, access_token, refresh_token, default_branch, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err := s.db.Exec(
		query,
		integration.ID, integration.TenantID, integration.ProjectID,
		integration.Provider, integration.RepositoryURL, integration.WebhookSecret,
		integration.InstallID, integration.AccessToken, integration.RefreshToken,
		integration.DefaultBranch, integration.IsActive,
	)
	return err
}

func (s *pgIntegrationStore) GetByID(ctx context.Context, tenantID, id string) (*domain.ProjectGitIntegration, error) {
	query := `
		SELECT id, tenant_id, project_id, provider, repository_url, webhook_secret,
		       install_id, access_token, refresh_token, default_branch, is_active,
		       created_at, updated_at
		FROM project_git_integrations
		WHERE id = $1 AND tenant_id = $2
	`
	row := s.db.QueryRow(query, id, tenantID)
	return scanIntegration(row)
}

func (s *pgIntegrationStore) ListByProject(ctx context.Context, tenantID, projectID string) ([]*domain.ProjectGitIntegration, error) {
	query := `
		SELECT id, tenant_id, project_id, provider, repository_url, webhook_secret,
		       install_id, access_token, refresh_token, default_branch, is_active,
		       created_at, updated_at
		FROM project_git_integrations
		WHERE tenant_id = $1 AND project_id = $2
		ORDER BY created_at DESC
	`
	rows, err := s.db.Query(query, tenantID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var integrations []*domain.ProjectGitIntegration
	for rows.Next() {
		i, err := scanIntegration(rows)
		if err != nil {
			return nil, err
		}
		integrations = append(integrations, i)
	}
	return integrations, rows.Err()
}

func (s *pgIntegrationStore) Update(ctx context.Context, integration *domain.ProjectGitIntegration) error {
	query := `
		UPDATE project_git_integrations
		SET provider = $3, repository_url = $4, webhook_secret = $5,
		    install_id = $6, access_token = $7, refresh_token = $8,
		    default_branch = $9, is_active = $10, updated_at = NOW()
		WHERE id = $1 AND tenant_id = $2
	`
	res, err := s.db.Exec(
		query,
		integration.ID, integration.TenantID,
		integration.Provider, integration.RepositoryURL, integration.WebhookSecret,
		integration.InstallID, integration.AccessToken, integration.RefreshToken,
		integration.DefaultBranch, integration.IsActive,
	)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *pgIntegrationStore) Delete(ctx context.Context, tenantID, id string) error {
	query := `
		UPDATE project_git_integrations
		SET is_active = FALSE, updated_at = NOW()
		WHERE id = $1 AND tenant_id = $2
	`
	res, err := s.db.Exec(query, id, tenantID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func scanIntegration(row interface{ Scan(dest ...interface{}) error }) (*domain.ProjectGitIntegration, error) {
	i := &domain.ProjectGitIntegration{}
	err := row.Scan(
		&i.ID, &i.TenantID, &i.ProjectID, &i.Provider, &i.RepositoryURL,
		&i.WebhookSecret, &i.InstallID, &i.AccessToken, &i.RefreshToken,
		&i.DefaultBranch, &i.IsActive, &i.CreatedAt, &i.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return i, err
}

// ============================================================================
// WebhookEventStore
// ============================================================================

type pgWebhookEventStore struct {
	db *sql.DB
}

func (s *pgWebhookEventStore) Insert(ctx context.Context, event *domain.GitWebhookEvent) error {
	query := `
		INSERT INTO git_webhook_events
			(id, tenant_id, integration_id, idempotency_key, event_type, payload, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (idempotency_key) DO NOTHING
	`
	res, err := s.db.Exec(
		query,
		event.ID, event.TenantID, event.IntegrationID,
		event.IdempotencyKey, event.EventType, event.Payload, event.Status,
	)
	if err != nil {
		return fmt.Errorf("insert webhook event: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrDuplicateEvent
	}
	return nil
}

func (s *pgWebhookEventStore) FindByIdempotencyKey(ctx context.Context, tenantID, key string) (*domain.GitWebhookEvent, error) {
	query := `
		SELECT id, tenant_id, integration_id, idempotency_key, event_type,
		       payload, status, processed_at, error, created_at
		FROM git_webhook_events
		WHERE idempotency_key = $1 AND tenant_id = $2
	`
	row := s.db.QueryRow(query, key, tenantID)
	return scanWebhookEvent(row)
}

func (s *pgWebhookEventStore) UpdateStatus(ctx context.Context, id string, status domain.WebhookEventStatus, errMsg string) error {
	query := `
		UPDATE git_webhook_events
		SET status = $2, error = $3,
		    processed_at = CASE WHEN $2 = 'processed' THEN NOW() ELSE processed_at END
		WHERE id = $1
	`
	res, err := s.db.Exec(query, id, status, errMsg)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func scanWebhookEvent(row interface{ Scan(dest ...interface{}) error }) (*domain.GitWebhookEvent, error) {
	e := &domain.GitWebhookEvent{}
	err := row.Scan(
		&e.ID, &e.TenantID, &e.IntegrationID, &e.IdempotencyKey,
		&e.EventType, &e.Payload, &e.Status, &e.ProcessedAt, &e.Error,
		&e.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return e, err
}

// ============================================================================
// IngestionJobStore
// ============================================================================

type pgIngestionJobStore struct {
	db *sql.DB
	mu  sync.Mutex // protects ClaimNextQueued atomicity
}

func (s *pgIngestionJobStore) Create(ctx context.Context, job *domain.PRIngestionJob) error {
	query := `
		INSERT INTO pr_ingestion_jobs
			(id, tenant_id, integration_id, event_id, repository, pr_number,
			 status, retry_count, max_retries)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := s.db.Exec(
		query,
		job.ID, job.TenantID, job.IntegrationID, job.EventID,
		job.Repository, job.PRNumber, job.Status, job.RetryCount, job.MaxRetries,
	)
	return err
}

func (s *pgIngestionJobStore) GetByID(ctx context.Context, tenantID, id string) (*domain.PRIngestionJob, error) {
	query := `
		SELECT id, tenant_id, integration_id, event_id, repository, pr_number,
		       status, error, retry_count, max_retries, created_at, updated_at
		FROM pr_ingestion_jobs
		WHERE id = $1 AND tenant_id = $2
	`
	row := s.db.QueryRow(query, id, tenantID)
	return scanIngestionJob(row)
}

func (s *pgIngestionJobStore) ClaimNextQueued(ctx context.Context) (*domain.PRIngestionJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Find the oldest queued job
	query := `
		SELECT id, tenant_id, integration_id, event_id, repository, pr_number,
		       status, error, retry_count, max_retries, created_at, updated_at
		FROM pr_ingestion_jobs
		WHERE status = 'queued'
		ORDER BY created_at ASC
		LIMIT 1
		FOR UPDATE SKIP LOCKED
	`
	row := s.db.QueryRow(query)
	job, err := scanIngestionJob(row)
	if err != nil {
		if err == ErrNotFound {
			return nil, nil // no jobs available
		}
		return nil, err
	}

	// Update to running
	updateQuery := `
		UPDATE pr_ingestion_jobs
		SET status = 'running', updated_at = NOW()
		WHERE id = $1 AND status = 'queued'
	`
	_, err = s.db.Exec(updateQuery, job.ID)
	if err != nil {
		return nil, fmt.Errorf("claim job %s: %w", job.ID, err)
	}

	job.Status = domain.IngestionJobRunning
	return job, nil
}

func (s *pgIngestionJobStore) MarkCompleted(ctx context.Context, id string) error {
	query := `
		UPDATE pr_ingestion_jobs
		SET status = 'completed', updated_at = NOW()
		WHERE id = $1 AND status = 'running'
	`
	res, err := s.db.Exec(query, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrInvalidTransition
	}
	return nil
}

func (s *pgIngestionJobStore) MarkFailed(ctx context.Context, id string, errMsg string) error {
	// If retries remain, reset to queued; otherwise mark as failed
	query := `
		UPDATE pr_ingestion_jobs
		SET status = CASE
		               WHEN retry_count + 1 >= max_retries THEN 'failed'
		               ELSE 'queued'
		             END,
		    error = $2,
		    retry_count = retry_count + 1,
		    updated_at = NOW()
		WHERE id = $1 AND status IN ('running', 'queued')
	`
	res, err := s.db.Exec(query, id, errMsg)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrInvalidTransition
	}
	return nil
}

func (s *pgIngestionJobStore) MarkCancelled(ctx context.Context, id string) error {
	query := `
		UPDATE pr_ingestion_jobs
		SET status = 'cancelled', updated_at = NOW()
		WHERE id = $1 AND status IN ('queued', 'running')
	`
	res, err := s.db.Exec(query, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrInvalidTransition
	}
	return nil
}

func scanIngestionJob(row interface{ Scan(dest ...interface{}) error }) (*domain.PRIngestionJob, error) {
	j := &domain.PRIngestionJob{}
	err := row.Scan(
		&j.ID, &j.TenantID, &j.IntegrationID, &j.EventID,
		&j.Repository, &j.PRNumber, &j.Status, &j.Error,
		&j.RetryCount, &j.MaxRetries, &j.CreatedAt, &j.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return j, err
}
