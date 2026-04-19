package store

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aegis-wilwatikta/ai-reviewer/internal/domain"
	"github.com/google/uuid"
)

// MemoryStore implements the Store interface with in-memory storage.
// Intended for testing and development without a real database.
type MemoryStore struct {
	integrations  *memoryIntegrationStore
	webhookEvents *memoryWebhookEventStore
	ingestionJobs *memoryIngestionJobStore
}

// NewMemoryStore creates a new in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		integrations:  &memoryIntegrationStore{data: make(map[string]*domain.ProjectGitIntegration)},
		webhookEvents: &memoryWebhookEventStore{data: make(map[string]*domain.GitWebhookEvent), byKey: make(map[string]string)},
		ingestionJobs: &memoryIngestionJobStore{data: make(map[string]*domain.PRIngestionJob)},
	}
}

func (s *MemoryStore) Integrations() IntegrationStore   { return s.integrations }
func (s *MemoryStore) WebhookEvents() WebhookEventStore  { return s.webhookEvents }
func (s *MemoryStore) IngestionJobs() IngestionJobStore  { return s.ingestionJobs }

// compile-time check
var _ Store = (*MemoryStore)(nil)

// ============================================================================
// IntegrationStore (memory)
// ============================================================================

type memoryIntegrationStore struct {
	mu   sync.Mutex
	data map[string]*domain.ProjectGitIntegration // id → integration
}

func (s *memoryIntegrationStore) Create(_ context.Context, i *domain.ProjectGitIntegration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if i.ID == "" {
		i.ID = uuid.New().String()
	}
	now := time.Now().UTC()
	i.CreatedAt = now
	i.UpdatedAt = now

	cp := *i
	s.data[i.ID] = &cp
	return nil
}

func (s *memoryIntegrationStore) GetByID(_ context.Context, tenantID, id string) (*domain.ProjectGitIntegration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	i, ok := s.data[id]
	if !ok || i.TenantID != tenantID {
		return nil, ErrNotFound
	}
	cp := *i
	return &cp, nil
}

func (s *memoryIntegrationStore) ListByProject(_ context.Context, tenantID, projectID string) ([]*domain.ProjectGitIntegration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var result []*domain.ProjectGitIntegration
	for _, i := range s.data {
		if i.TenantID == tenantID && i.ProjectID == projectID {
			cp := *i
			result = append(result, &cp)
		}
	}
	return result, nil
}

func (s *memoryIntegrationStore) Update(_ context.Context, i *domain.ProjectGitIntegration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.data[i.ID]
	if !ok || existing.TenantID != i.TenantID {
		return ErrNotFound
	}

	i.UpdatedAt = time.Now().UTC()
	cp := *i
	s.data[i.ID] = &cp
	return nil
}

func (s *memoryIntegrationStore) Delete(_ context.Context, tenantID, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	i, ok := s.data[id]
	if !ok || i.TenantID != tenantID {
		return ErrNotFound
	}
	i.IsActive = false
	i.UpdatedAt = time.Now().UTC()
	return nil
}

// ============================================================================
// WebhookEventStore (memory)
// ============================================================================

type memoryWebhookEventStore struct {
	mu    sync.Mutex
	data  map[string]*domain.GitWebhookEvent // id → event
	byKey map[string]string                  // idempotency_key → id
}

func (s *memoryWebhookEventStore) Insert(_ context.Context, e *domain.GitWebhookEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.byKey[e.IdempotencyKey]; exists {
		return ErrDuplicateEvent
	}

	if e.ID == "" {
		e.ID = uuid.New().String()
	}
	e.CreatedAt = time.Now().UTC()

	cp := *e
	s.data[e.ID] = &cp
	s.byKey[e.IdempotencyKey] = e.ID
	return nil
}

func (s *memoryWebhookEventStore) FindByIdempotencyKey(_ context.Context, tenantID, key string) (*domain.GitWebhookEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id, ok := s.byKey[key]
	if !ok {
		return nil, ErrNotFound
	}
	e, ok := s.data[id]
	if !ok || e.TenantID != tenantID {
		return nil, ErrNotFound
	}
	cp := *e
	return &cp, nil
}

func (s *memoryWebhookEventStore) UpdateStatus(_ context.Context, id string, status domain.WebhookEventStatus, errMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.data[id]
	if !ok {
		return ErrNotFound
	}
	e.Status = status
	e.Error = errMsg
	if status == domain.WebhookEventProcessed {
		now := time.Now().UTC()
		e.ProcessedAt = &now
	}
	return nil
}

// ============================================================================
// IngestionJobStore (memory)
// ============================================================================

type memoryIngestionJobStore struct {
	mu   sync.Mutex
	data map[string]*domain.PRIngestionJob // id → job
}

func (s *memoryIngestionJobStore) Create(_ context.Context, j *domain.PRIngestionJob) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if j.ID == "" {
		j.ID = uuid.New().String()
	}
	now := time.Now().UTC()
	j.CreatedAt = now
	j.UpdatedAt = now
	if j.Status == "" {
		j.Status = domain.IngestionJobQueued
	}

	cp := *j
	s.data[j.ID] = &cp
	return nil
}

func (s *memoryIngestionJobStore) GetByID(_ context.Context, tenantID, id string) (*domain.PRIngestionJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	j, ok := s.data[id]
	if !ok || j.TenantID != tenantID {
		return nil, ErrNotFound
	}
	cp := *j
	return &cp, nil
}

func (s *memoryIngestionJobStore) ClaimNextQueued(_ context.Context) (*domain.PRIngestionJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var oldest *domain.PRIngestionJob
	for _, j := range s.data {
		if j.Status == domain.IngestionJobQueued {
			if oldest == nil || j.CreatedAt.Before(oldest.CreatedAt) {
				cp := *j
				oldest = &cp
			}
		}
	}
	if oldest == nil {
		return nil, nil
	}

	oldest.Status = domain.IngestionJobRunning
	oldest.UpdatedAt = time.Now().UTC()
	s.data[oldest.ID] = oldest
	return oldest, nil
}

func (s *memoryIngestionJobStore) MarkCompleted(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	j, ok := s.data[id]
	if !ok {
		return ErrInvalidTransition
	}
	if j.Status != domain.IngestionJobRunning {
		return ErrInvalidTransition
	}
	j.Status = domain.IngestionJobCompleted
	j.UpdatedAt = time.Now().UTC()
	return nil
}

func (s *memoryIngestionJobStore) MarkFailed(_ context.Context, id string, errMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	j, ok := s.data[id]
	if !ok {
		return ErrInvalidTransition
	}
	if j.Status != domain.IngestionJobRunning && j.Status != domain.IngestionJobQueued {
		return ErrInvalidTransition
	}

	j.RetryCount++
	j.Error = errMsg
	j.UpdatedAt = time.Now().UTC()

	if j.RetryCount >= j.MaxRetries {
		j.Status = domain.IngestionJobFailed
	} else {
		j.Status = domain.IngestionJobQueued
	}
	return nil
}

func (s *memoryIngestionJobStore) MarkCancelled(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	j, ok := s.data[id]
	if !ok {
		return ErrInvalidTransition
	}
	if j.Status != domain.IngestionJobQueued && j.Status != domain.IngestionJobRunning {
		return ErrInvalidTransition
	}
	j.Status = domain.IngestionJobCancelled
	j.UpdatedAt = time.Now().UTC()
	return nil
}

// GetJobCount returns the number of jobs for testing purposes.
func (s *memoryIngestionJobStore) GetJobCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.data)
}

// Helper to get the internal data for assertions
func (s *memoryIngestionJobStore) GetJob(id string) *domain.PRIngestionJob {
	s.mu.Lock()
	defer s.mu.Unlock()
	j, ok := s.data[id]
	if !ok {
		return nil
	}
	cp := *j
	return &cp
}

// Suppress unused warning
var _ = fmt.Sprintf
