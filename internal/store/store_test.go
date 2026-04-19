package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aegis-wilwatikta/ai-reviewer/internal/domain"
	"github.com/google/uuid"
)

// ============================================================================
// IntegrationStore Tests
// ============================================================================

func TestIntegrationStore_CreateAndGet(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	integration := &domain.ProjectGitIntegration{
		TenantID:      "tenant-1",
		ProjectID:     "project-1",
		Provider:      domain.GitProviderGitHub,
		RepositoryURL: "https://github.com/org/repo",
		WebhookSecret: "secret123",
		InstallID:     "install-1",
		DefaultBranch: "main",
		IsActive:      true,
	}

	err := s.Integrations().Create(ctx, integration)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if integration.ID == "" {
		t.Error("expected ID to be set after Create")
	}

	got, err := s.Integrations().GetByID(ctx, "tenant-1", integration.ID)
	if err != nil {
		t.Fatalf("GetByID returned error: %v", err)
	}

	if got.RepositoryURL != "https://github.com/org/repo" {
		t.Errorf("expected RepositoryURL 'https://github.com/org/repo', got %q", got.RepositoryURL)
	}
	if got.Provider != domain.GitProviderGitHub {
		t.Errorf("expected provider github, got %q", got.Provider)
	}
}

func TestIntegrationStore_GetByID_WrongTenant(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	integration := &domain.ProjectGitIntegration{
		TenantID:      "tenant-1",
		ProjectID:     "project-1",
		Provider:      domain.GitProviderGitHub,
		RepositoryURL: "https://github.com/org/repo",
	}
	s.Integrations().Create(ctx, integration)

	_, err := s.Integrations().GetByID(ctx, "tenant-2", integration.ID)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestIntegrationStore_ListByProject(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		s.Integrations().Create(ctx, &domain.ProjectGitIntegration{
			TenantID:      "tenant-1",
			ProjectID:     "project-1",
			Provider:      domain.GitProviderGitHub,
			RepositoryURL: "https://github.com/org/repo" + string(rune('a'+i)),
		})
	}

	// Different project
	s.Integrations().Create(ctx, &domain.ProjectGitIntegration{
		TenantID:      "tenant-1",
		ProjectID:     "project-2",
		Provider:      domain.GitProviderGitHub,
		RepositoryURL: "https://github.com/org/other",
	})

	list, err := s.Integrations().ListByProject(ctx, "tenant-1", "project-1")
	if err != nil {
		t.Fatalf("ListByProject returned error: %v", err)
	}

	if len(list) != 3 {
		t.Errorf("expected 3 integrations, got %d", len(list))
	}
}

func TestIntegrationStore_Update(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	integration := &domain.ProjectGitIntegration{
		TenantID:      "tenant-1",
		ProjectID:     "project-1",
		Provider:      domain.GitProviderGitHub,
		RepositoryURL: "https://github.com/org/repo",
		DefaultBranch: "main",
	}
	s.Integrations().Create(ctx, integration)

	integration.DefaultBranch = "develop"
	err := s.Integrations().Update(ctx, integration)
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}

	got, _ := s.Integrations().GetByID(ctx, "tenant-1", integration.ID)
	if got.DefaultBranch != "develop" {
		t.Errorf("expected default branch 'develop', got %q", got.DefaultBranch)
	}
}

func TestIntegrationStore_Delete_SoftDelete(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	integration := &domain.ProjectGitIntegration{
		TenantID:      "tenant-1",
		ProjectID:     "project-1",
		Provider:      domain.GitProviderGitHub,
		RepositoryURL: "https://github.com/org/repo",
		IsActive:      true,
	}
	s.Integrations().Create(ctx, integration)

	err := s.Integrations().Delete(ctx, "tenant-1", integration.ID)
	if err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	got, _ := s.Integrations().GetByID(ctx, "tenant-1", integration.ID)
	if got.IsActive {
		t.Error("expected is_active = false after delete")
	}
}

func TestIntegrationStore_Delete_NotFound(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	err := s.Integrations().Delete(ctx, "tenant-1", "nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// ============================================================================
// WebhookEventStore Tests
// ============================================================================

func TestWebhookEventStore_InsertAndFind(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	event := &domain.GitWebhookEvent{
		TenantID:       "tenant-1",
		IntegrationID:  "integration-1",
		IdempotencyKey: "delivery-uuid-123",
		EventType:      "pull_request",
		Payload:        []byte(`{"action": "opened"}`),
		Status:         domain.WebhookEventReceived,
	}

	err := s.WebhookEvents().Insert(ctx, event)
	if err != nil {
		t.Fatalf("Insert returned error: %v", err)
	}

	if event.ID == "" {
		t.Error("expected ID to be set")
	}

	got, err := s.WebhookEvents().FindByIdempotencyKey(ctx, "tenant-1", "delivery-uuid-123")
	if err != nil {
		t.Fatalf("FindByIdempotencyKey returned error: %v", err)
	}

	if got.EventType != "pull_request" {
		t.Errorf("expected event_type 'pull_request', got %q", got.EventType)
	}
	if string(got.Payload) != `{"action": "opened"}` {
		t.Errorf("unexpected payload: %s", got.Payload)
	}
}

func TestWebhookEventStore_InsertDuplicate(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	event := &domain.GitWebhookEvent{
		TenantID:       "tenant-1",
		IntegrationID:  "integration-1",
		IdempotencyKey: "dup-key",
		EventType:      "push",
		Payload:        []byte(`{}`),
		Status:         domain.WebhookEventReceived,
	}
	s.WebhookEvents().Insert(ctx, event)

	dup := &domain.GitWebhookEvent{
		TenantID:       "tenant-1",
		IntegrationID:  "integration-1",
		IdempotencyKey: "dup-key",
		EventType:      "push",
		Payload:        []byte(`{"dup": true}`),
		Status:         domain.WebhookEventReceived,
	}

	err := s.WebhookEvents().Insert(ctx, dup)
	if !errors.Is(err, ErrDuplicateEvent) {
		t.Errorf("expected ErrDuplicateEvent, got %v", err)
	}
}

func TestWebhookEventStore_UpdateStatus(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	event := &domain.GitWebhookEvent{
		TenantID:       "tenant-1",
		IntegrationID:  "integration-1",
		IdempotencyKey: "key-1",
		EventType:      "push",
		Payload:        []byte(`{}`),
		Status:         domain.WebhookEventReceived,
	}
	s.WebhookEvents().Insert(ctx, event)

	err := s.WebhookEvents().UpdateStatus(ctx, event.ID, domain.WebhookEventProcessed, "")
	if err != nil {
		t.Fatalf("UpdateStatus returned error: %v", err)
	}

	got, _ := s.WebhookEvents().FindByIdempotencyKey(ctx, "tenant-1", "key-1")
	if got.Status != domain.WebhookEventProcessed {
		t.Errorf("expected status processed, got %q", got.Status)
	}
	if got.ProcessedAt == nil {
		t.Error("expected ProcessedAt to be set when status = processed")
	}
}

func TestWebhookEventStore_UpdateStatus_Failed(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	event := &domain.GitWebhookEvent{
		TenantID:       "tenant-1",
		IntegrationID:  "integration-1",
		IdempotencyKey: "key-fail",
		EventType:      "push",
		Payload:        []byte(`{}`),
		Status:         domain.WebhookEventReceived,
	}
	s.WebhookEvents().Insert(ctx, event)

	err := s.WebhookEvents().UpdateStatus(ctx, event.ID, domain.WebhookEventFailed, "timeout")
	if err != nil {
		t.Fatalf("UpdateStatus returned error: %v", err)
	}

	got, _ := s.WebhookEvents().FindByIdempotencyKey(ctx, "tenant-1", "key-fail")
	if got.Status != domain.WebhookEventFailed {
		t.Errorf("expected status failed, got %q", got.Status)
	}
	if got.Error != "timeout" {
		t.Errorf("expected error 'timeout', got %q", got.Error)
	}
}

// ============================================================================
// IngestionJobStore Tests
// ============================================================================

func newTestJob() *domain.PRIngestionJob {
	return &domain.PRIngestionJob{
		TenantID:      "tenant-1",
		IntegrationID: "integration-1",
		EventID:       "event-1",
		Repository:    "org/repo",
		PRNumber:      42,
		Status:        domain.IngestionJobQueued,
		RetryCount:    0,
		MaxRetries:    3,
	}
}

func TestIngestionJobStore_CreateAndGet(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	job := newTestJob()
	err := s.IngestionJobs().Create(ctx, job)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if job.ID == "" {
		t.Error("expected ID to be set")
	}

	got, err := s.IngestionJobs().GetByID(ctx, "tenant-1", job.ID)
	if err != nil {
		t.Fatalf("GetByID returned error: %v", err)
	}

	if got.Repository != "org/repo" {
		t.Errorf("expected repository 'org/repo', got %q", got.Repository)
	}
	if got.PRNumber != 42 {
		t.Errorf("expected PR number 42, got %d", got.PRNumber)
	}
}

func TestIngestionJobStore_ClaimNextQueued(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	job1 := newTestJob()
	job1.ID = uuid.New().String()
	s.IngestionJobs().Create(ctx, job1)

	// Small delay to ensure ordering
	time.Sleep(1 * time.Millisecond)

	job2 := newTestJob()
	job2.ID = uuid.New().String()
	s.IngestionJobs().Create(ctx, job2)

	claimed, err := s.IngestionJobs().ClaimNextQueued(ctx)
	if err != nil {
		t.Fatalf("ClaimNextQueued returned error: %v", err)
	}

	if claimed.ID != job1.ID {
		t.Errorf("expected oldest job %s, got %s", job1.ID, claimed.ID)
	}
	if claimed.Status != domain.IngestionJobRunning {
		t.Errorf("expected status running, got %q", claimed.Status)
	}
}

func TestIngestionJobStore_ClaimNextQueued_Empty(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	claimed, err := s.IngestionJobs().ClaimNextQueued(ctx)
	if err != nil {
		t.Fatalf("ClaimNextQueued returned error: %v", err)
	}
	if claimed != nil {
		t.Error("expected nil when no jobs available")
	}
}

func TestIngestionJobStore_MarkCompleted(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	job := newTestJob()
	s.IngestionJobs().Create(ctx, job)

	claimed, _ := s.IngestionJobs().ClaimNextQueued(ctx)

	err := s.IngestionJobs().MarkCompleted(ctx, claimed.ID)
	if err != nil {
		t.Fatalf("MarkCompleted returned error: %v", err)
	}

	inner := s.IngestionJobs().(*memoryIngestionJobStore)
	got := inner.GetJob(claimed.ID)
	if got.Status != domain.IngestionJobCompleted {
		t.Errorf("expected status completed, got %q", got.Status)
	}
}

func TestIngestionJobStore_MarkFailed_Retry(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	job := newTestJob()
	job.MaxRetries = 3
	s.IngestionJobs().Create(ctx, job)

	claimed, _ := s.IngestionJobs().ClaimNextQueued(ctx)

	err := s.IngestionJobs().MarkFailed(ctx, claimed.ID, "transient error")
	if err != nil {
		t.Fatalf("MarkFailed returned error: %v", err)
	}

	inner := s.IngestionJobs().(*memoryIngestionJobStore)
	got := inner.GetJob(claimed.ID)

	// Should be re-queued since retry_count (1) < max_retries (3)
	if got.Status != domain.IngestionJobQueued {
		t.Errorf("expected status queued for retry, got %q", got.Status)
	}
	if got.RetryCount != 1 {
		t.Errorf("expected retry_count 1, got %d", got.RetryCount)
	}
	if got.Error != "transient error" {
		t.Errorf("expected error message, got %q", got.Error)
	}
}

func TestIngestionJobStore_MarkFailed_Exhausted(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	job := newTestJob()
	job.MaxRetries = 1
	s.IngestionJobs().Create(ctx, job)

	claimed, _ := s.IngestionJobs().ClaimNextQueued(ctx)

	err := s.IngestionJobs().MarkFailed(ctx, claimed.ID, "permanent failure")
	if err != nil {
		t.Fatalf("MarkFailed returned error: %v", err)
	}

	inner := s.IngestionJobs().(*memoryIngestionJobStore)
	got := inner.GetJob(claimed.ID)

	// Should be failed since retry_count (1) >= max_retries (1)
	if got.Status != domain.IngestionJobFailed {
		t.Errorf("expected status failed, got %q", got.Status)
	}
}

func TestIngestionJobStore_MarkCancelled(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	job := newTestJob()
	s.IngestionJobs().Create(ctx, job)

	err := s.IngestionJobs().MarkCancelled(ctx, job.ID)
	if err != nil {
		t.Fatalf("MarkCancelled returned error: %v", err)
	}

	inner := s.IngestionJobs().(*memoryIngestionJobStore)
	got := inner.GetJob(job.ID)
	if got.Status != domain.IngestionJobCancelled {
		t.Errorf("expected status cancelled, got %q", got.Status)
	}
}

func TestIngestionJobStore_MarkCompleted_InvalidTransition(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	job := newTestJob()
	s.IngestionJobs().Create(ctx, job)

	// Try to complete a queued job without claiming it first
	err := s.IngestionJobs().MarkCompleted(ctx, job.ID)
	if !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("expected ErrInvalidTransition, got %v", err)
	}
}

func TestIngestionJobStore_GetByID_WrongTenant(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	job := newTestJob()
	s.IngestionJobs().Create(ctx, job)

	_, err := s.IngestionJobs().GetByID(ctx, "wrong-tenant", job.ID)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestIngestionJobStore_FullLifecycle(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	// Create
	job := newTestJob()
	s.IngestionJobs().Create(ctx, job)

	// Claim
	claimed, _ := s.IngestionJobs().ClaimNextQueued(ctx)
	if claimed.Status != domain.IngestionJobRunning {
		t.Fatalf("expected running, got %q", claimed.Status)
	}

	// Complete
	s.IngestionJobs().MarkCompleted(ctx, claimed.ID)

	inner := s.IngestionJobs().(*memoryIngestionJobStore)
	got := inner.GetJob(claimed.ID)
	if got.Status != domain.IngestionJobCompleted {
		t.Errorf("expected completed, got %q", got.Status)
	}
}

// ============================================================================
// Store Interface Tests
// ============================================================================

func TestMemoryStore_ImplementsStore(t *testing.T) {
	var _ Store = NewMemoryStore()
}

func TestPGStore_ImplementsStore(t *testing.T) {
	var _ Store = &PGStore{}
}
