package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aegis-wilwatikta/ai-reviewer/internal/db"
	"github.com/aegis-wilwatikta/ai-reviewer/internal/domain"
	"github.com/google/uuid"
)

type ReviewStore interface {
	Save(ctx context.Context, r *ReviewRecord) error
	Get(ctx context.Context, id uuid.UUID) (*ReviewRecord, error)
	GetLatest(ctx context.Context, owner, repo string, prNumber int) (*ReviewRecord, error)
	List(ctx context.Context, tenantID, projectID uuid.UUID, limit, offset int) ([]*ReviewRecord, int, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status, errorMsg string) error
	SetResult(ctx context.Context, id uuid.UUID, verdict domain.Verdict, summary string, healthScore int, rawReview string, violations []domain.ReviewComment) error
	GetViolations(ctx context.Context, reviewID uuid.UUID) ([]*ViolationRecord, error)
}

type ReviewRecord struct {
	ID                  uuid.UUID               `json:"id"`
	TenantID            uuid.UUID               `json:"tenant_id"`
	ProjectID           uuid.UUID               `json:"project_id"`
	Owner               string                  `json:"owner"`
	Repo                string                  `json:"repo"`
	PRNumber            int                     `json:"pr_number"`
	Status              string                  `json:"status"`
	Verdict             domain.Verdict          `json:"verdict"`
	Summary             string                  `json:"summary"`
	HealthScore         int                     `json:"health_score"`
	GuardrailViolations []*domain.ReviewComment `json:"guardrail_violations"`
	RawReview           string                  `json:"raw_review"`
	ErrorMessage        string                  `json:"error_message"`
	CreatedAt           time.Time               `json:"created_at"`
	UpdatedAt           time.Time               `json:"updated_at"`
	CompletedAt         *time.Time              `json:"completed_at"`
	Violations          []*ViolationRecord      `json:"violations"`
}

type ViolationRecord struct {
	ID         uuid.UUID `json:"id"`
	ReviewID   uuid.UUID `json:"review_id"`
	File       string    `json:"file"`
	Line       int       `json:"line"`
	Severity   string    `json:"severity"`
	Issue      string    `json:"issue"`
	Suggestion string    `json:"suggestion"`
	RuleID     string    `json:"rule_id"`
	RuleName   string    `json:"rule_name"`
	CreatedAt  time.Time `json:"created_at"`
}

type PostgresReviewStore struct {
	db *db.DB
}

func NewPostgresReviewStore(database *db.DB) *PostgresReviewStore {
	return &PostgresReviewStore{db: database}
}

func (s *PostgresReviewStore) Save(ctx context.Context, r *ReviewRecord) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	violationsJSON, _ := json.Marshal(r.GuardrailViolations)
	_, err := s.db.Pool.Exec(ctx, `
		INSERT INTO backend_reviews (id, tenant_id, project_id, owner, repo, pr_number, status, verdict, summary, health_score, guardrail_violations, raw_review, error_message, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $14)
		ON CONFLICT (id) DO UPDATE SET
			status = EXCLUDED.status,
			updated_at = NOW()
	`, r.ID, r.TenantID, r.ProjectID, r.Owner, r.Repo, r.PRNumber, r.Status, r.Verdict, r.Summary, r.HealthScore, violationsJSON, r.RawReview, r.ErrorMessage, time.Now())
	return err
}

func (s *PostgresReviewStore) Get(ctx context.Context, id uuid.UUID) (*ReviewRecord, error) {
	row := s.db.Pool.QueryRow(ctx, `
		SELECT id, tenant_id, project_id, owner, repo, pr_number, status, verdict, summary, health_score, guardrail_violations, raw_review, error_message, created_at, updated_at, completed_at
		FROM backend_reviews WHERE id = $1
	`, id)
	return s.scanReview(row)
}

func (s *PostgresReviewStore) GetLatest(ctx context.Context, owner, repo string, prNumber int) (*ReviewRecord, error) {
	row := s.db.Pool.QueryRow(ctx, `
		SELECT id, tenant_id, project_id, owner, repo, pr_number, status, verdict, summary, health_score, guardrail_violations, raw_review, error_message, created_at, updated_at, completed_at
		FROM backend_reviews
		WHERE owner = $1 AND repo = $2 AND pr_number = $3 AND status = 'completed'
		ORDER BY created_at DESC
		LIMIT 1
	`, owner, repo, prNumber)
	return s.scanReview(row)
}

func (s *PostgresReviewStore) List(ctx context.Context, tenantID, projectID uuid.UUID, limit, offset int) ([]*ReviewRecord, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	var total int
	countQuery := `SELECT COUNT(*) FROM backend_reviews WHERE tenant_id = $1 AND project_id = $2`
	if err := s.db.Pool.QueryRow(ctx, countQuery, tenantID, projectID).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.db.Pool.Query(ctx, `
		SELECT id, tenant_id, project_id, owner, repo, pr_number, status, verdict, summary, health_score, guardrail_violations, raw_review, error_message, created_at, updated_at, completed_at
		FROM backend_reviews
		WHERE tenant_id = $1 AND project_id = $2
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`, tenantID, projectID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var records []*ReviewRecord
	for rows.Next() {
		rec, err := s.scanReviewRows(rows)
		if err != nil {
			return nil, 0, err
		}
		records = append(records, rec)
	}

	return records, total, nil
}

func (s *PostgresReviewStore) UpdateStatus(ctx context.Context, id uuid.UUID, status, errorMsg string) error {
	query := `UPDATE backend_reviews SET status = $2, error_message = $3, updated_at = NOW() WHERE id = $1`
	if status == "completed" || status == "failed" {
		query = `UPDATE backend_reviews SET status = $2, error_message = $3, completed_at = NOW(), updated_at = NOW() WHERE id = $1`
	}
	_, err := s.db.Pool.Exec(ctx, query, id, status, errorMsg)
	return err
}

func (s *PostgresReviewStore) SetResult(ctx context.Context, id uuid.UUID, verdict domain.Verdict, summary string, healthScore int, rawReview string, violations []domain.ReviewComment) error {
	tx, err := s.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		UPDATE backend_reviews
		SET verdict = $2, summary = $3, health_score = $4, raw_review = $5, status = 'completed', completed_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`, id, verdict, summary, healthScore, rawReview)
	if err != nil {
		return fmt.Errorf("failed to update review: %w", err)
	}

	if len(violations) > 0 {
		for _, v := range violations {
			_, err = tx.Exec(ctx, `
				INSERT INTO backend_review_violations (id, review_id, file, line, severity, issue, suggestion)
				VALUES ($1, $2, $3, $4, $5, $6, $7)
			`, uuid.New(), id, v.File, v.Line, string(v.Severity), v.Issue, v.Suggestion)
			if err != nil {
				return fmt.Errorf("failed to insert violation: %w", err)
			}
		}
	}

	return tx.Commit(ctx)
}

type scanner interface {
	Scan(dest ...any) error
}

func (s *PostgresReviewStore) scanReview(row scanner) (*ReviewRecord, error) {
	var r ReviewRecord
	var violationsJSON []byte
	var completedAt *time.Time

	err := row.Scan(&r.ID, &r.TenantID, &r.ProjectID, &r.Owner, &r.Repo, &r.PRNumber, &r.Status, &r.Verdict, &r.Summary, &r.HealthScore, &violationsJSON, &r.RawReview, &r.ErrorMessage, &r.CreatedAt, &r.UpdatedAt, &completedAt)
	if err != nil {
		return nil, err
	}
	r.CompletedAt = completedAt

	if len(violationsJSON) > 0 {
		json.Unmarshal(violationsJSON, &r.GuardrailViolations)
	}

	return &r, nil
}

func (s *PostgresReviewStore) scanReviewRows(rows interface {
	Next() bool
	Scan(...any) error
}) (*ReviewRecord, error) {
	return s.scanReview(rows)
}

func (s *PostgresReviewStore) GetViolations(ctx context.Context, reviewID uuid.UUID) ([]*ViolationRecord, error) {
	rows, err := s.db.Pool.Query(ctx, `
		SELECT id, review_id, file, line, severity, issue, suggestion, COALESCE(rule_id, ''), COALESCE(rule_name, ''), created_at
		FROM backend_review_violations
		WHERE review_id = $1
		ORDER BY created_at ASC
	`, reviewID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []*ViolationRecord
	for rows.Next() {
		var v ViolationRecord
		if err := rows.Scan(&v.ID, &v.ReviewID, &v.File, &v.Line, &v.Severity, &v.Issue, &v.Suggestion, &v.RuleID, &v.RuleName, &v.CreatedAt); err != nil {
			return nil, err
		}
		records = append(records, &v)
	}

	return records, nil
}
