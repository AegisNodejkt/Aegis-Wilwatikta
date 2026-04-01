package server

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/aegis-wilwatikta/ai-reviewer/internal/domain"
	"github.com/aegis-wilwatikta/ai-reviewer/internal/engine"
	"github.com/aegis-wilwatikta/ai-reviewer/internal/platform"
	"github.com/aegis-wilwatikta/ai-reviewer/internal/store"
	"github.com/google/uuid"
)

type ReviewService struct {
	plat    platform.Platform
	engine  *engine.ReviewerEngine
	store   store.ReviewStore
	running sync.Map
}

func NewReviewService(plat platform.Platform, eng *engine.ReviewerEngine, s store.ReviewStore) *ReviewService {
	return &ReviewService{
		plat:   plat,
		engine: eng,
		store:  s,
	}
}

type EnqueueRequest struct {
	TenantID  uuid.UUID
	ProjectID uuid.UUID
	Owner     string
	Repo      string
	PRNumber  int
}

func (s *ReviewService) EnqueueReview(ctx context.Context, req *EnqueueRequest) (*store.ReviewRecord, error) {
	if s.engine == nil {
		return nil, &ServiceUnavailableError{Message: "engine not configured"}
	}

	record := &store.ReviewRecord{
		ID:        uuid.New(),
		TenantID:  req.TenantID,
		ProjectID: req.ProjectID,
		Owner:     req.Owner,
		Repo:      req.Repo,
		PRNumber:  req.PRNumber,
		Status:    "pending",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.store.Save(ctx, record); err != nil {
		return nil, fmt.Errorf("failed to save review record: %w", err)
	}

	go func() {
		s.runReview(context.Background(), record.ID, req.TenantID, req.ProjectID, req.Owner, req.Repo, req.PRNumber)
	}()

	return record, nil
}

func (s *ReviewService) GetReview(ctx context.Context, id uuid.UUID) (*store.ReviewRecord, error) {
	record, err := s.store.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	violations, err := s.store.GetViolations(ctx, id)
	if err != nil {
		log.Printf("Warning: failed to fetch violations for review %s: %v", id, err)
	} else {
		record.Violations = violations
	}
	return record, nil
}

func (s *ReviewService) ListReviews(ctx context.Context, tenantID, projectID uuid.UUID, limit, offset int) ([]*store.ReviewRecord, int, error) {
	return s.store.List(ctx, tenantID, projectID, limit, offset)
}

func (s *ReviewService) UpdateStatus(ctx context.Context, id uuid.UUID, status, errorMsg string) error {
	return s.store.UpdateStatus(ctx, id, status, errorMsg)
}

func (s *ReviewService) runReview(ctx context.Context, recordID, tenantID, projectID uuid.UUID, owner, repo string, prNumber int) {
	key := fmt.Sprintf("%s-%d-%s-%s", owner, prNumber, tenantID.String(), projectID.String())
	if _, loaded := s.running.LoadOrStore(key, true); loaded {
		return
	}
	defer s.running.Delete(key)

	s.store.UpdateStatus(ctx, recordID, "processing", "")

	var lastReview *domain.ReviewResult
	if s.store != nil {
		last, err := s.store.GetLatest(ctx, owner, repo, prNumber)
		if err != nil {
			log.Printf("Warning: failed to fetch last review: %v", err)
		} else if last != nil {
			lastReview = &domain.ReviewResult{
				Verdict: last.Verdict,
				Summary: last.Summary,
			}
		}
	}

	phases, err := s.engine.RunReviewPhases(ctx, owner, repo, prNumber, lastReview)
	if err != nil {
		s.store.UpdateStatus(ctx, recordID, "failed", err.Error())
		return
	}

	if s.plat != nil {
		if err := s.plat.PostReview(ctx, owner, repo, phases.PR, phases.ReviewResult); err != nil {
			log.Printf("Warning: failed to post review to platform: %v", err)
		}
	}

	if err := s.store.SetResult(ctx, recordID, phases.ReviewResult.Verdict, phases.ReviewResult.Summary, phases.HealthScore, phases.RawReview, phases.ReviewResult.Reviews); err != nil {
		log.Printf("Warning: failed to save review result: %v", err)
	}
}

type ServiceUnavailableError struct {
	Message string
}

func (e *ServiceUnavailableError) Error() string {
	return e.Message
}
