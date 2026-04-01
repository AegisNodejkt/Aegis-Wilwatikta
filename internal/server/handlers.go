package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/aegis-wilwatikta/ai-reviewer/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type TriggerRequest struct {
	TenantID  string `json:"tenant_id"`
	ProjectID string `json:"project_id"`
	Owner     string `json:"owner"`
	Repo      string `json:"repo"`
	PRNumber  int    `json:"pr_number"`
}

type ReviewResponse struct {
	ID           string         `json:"id"`
	TenantID     string         `json:"tenant_id"`
	ProjectID    string         `json:"project_id"`
	Owner        string         `json:"owner"`
	Repo         string         `json:"repo"`
	PRNumber     int            `json:"pr_number"`
	Status       string         `json:"status"`
	Verdict      string         `json:"verdict,omitempty"`
	Summary      string         `json:"summary,omitempty"`
	HealthScore  int            `json:"health_score,omitempty"`
	ErrorMessage string         `json:"error_message,omitempty"`
	Violations   []ViolationDTO `json:"violations,omitempty"`
	CreatedAt    string         `json:"created_at"`
	UpdatedAt    string         `json:"updated_at,omitempty"`
	CompletedAt  string         `json:"completed_at,omitempty"`
}

type ViolationDTO struct {
	ID         string `json:"id"`
	File       string `json:"file"`
	Line       int    `json:"line"`
	Severity   string `json:"severity"`
	Issue      string `json:"issue"`
	Suggestion string `json:"suggestion,omitempty"`
	RuleID     string `json:"rule_id,omitempty"`
	RuleName   string `json:"rule_name,omitempty"`
}

func toReviewResponse(r *store.ReviewRecord) ReviewResponse {
	resp := ReviewResponse{
		ID:           r.ID.String(),
		TenantID:     r.TenantID.String(),
		ProjectID:    r.ProjectID.String(),
		Owner:        r.Owner,
		Repo:         r.Repo,
		PRNumber:     r.PRNumber,
		Status:       r.Status,
		Verdict:      string(r.Verdict),
		Summary:      r.Summary,
		HealthScore:  r.HealthScore,
		ErrorMessage: r.ErrorMessage,
		CreatedAt:    r.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:    r.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if r.CompletedAt != nil {
		resp.CompletedAt = r.CompletedAt.Format("2006-01-02T15:04:05Z07:00")
	}
	if len(r.Violations) > 0 {
		resp.Violations = make([]ViolationDTO, len(r.Violations))
		for i, v := range r.Violations {
			resp.Violations[i] = ViolationDTO{
				ID:         v.ID.String(),
				File:       v.File,
				Line:       v.Line,
				Severity:   v.Severity,
				Issue:      v.Issue,
				Suggestion: v.Suggestion,
				RuleID:     v.RuleID,
				RuleName:   v.RuleName,
			}
		}
	}
	return resp
}

func (s *ReviewService) HandleTriggerReview(w http.ResponseWriter, r *http.Request) {
	var req TriggerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Owner == "" || req.Repo == "" || req.PRNumber <= 0 {
		http.Error(w, "owner, repo, and pr_number are required", http.StatusBadRequest)
		return
	}

	var tenantID, projectID uuid.UUID
	var err error
	if req.TenantID != "" {
		tenantID, err = uuid.Parse(req.TenantID)
		if err != nil {
			http.Error(w, "invalid tenant_id", http.StatusBadRequest)
			return
		}
	}
	if req.ProjectID != "" {
		projectID, err = uuid.Parse(req.ProjectID)
		if err != nil {
			http.Error(w, "invalid project_id", http.StatusBadRequest)
			return
		}
	}

	record, err := s.EnqueueReview(r.Context(), &EnqueueRequest{
		TenantID:  tenantID,
		ProjectID: projectID,
		Owner:     req.Owner,
		Repo:      req.Repo,
		PRNumber:  req.PRNumber,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(toReviewResponse(record))
}

func (s *ReviewService) HandleGetReview(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		http.Error(w, "review id required", http.StatusBadRequest)
		return
	}

	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "invalid review id", http.StatusBadRequest)
		return
	}

	record, err := s.GetReview(r.Context(), id)
	if err != nil {
		http.Error(w, "review not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toReviewResponse(record))
}

func (s *ReviewService) HandleListReviews(w http.ResponseWriter, r *http.Request) {
	tenantIDStr := chi.URLParam(r, "tenantId")
	projectIDStr := chi.URLParam(r, "projectId")

	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		http.Error(w, "invalid tenant_id", http.StatusBadRequest)
		return
	}
	projectID, err := uuid.Parse(projectIDStr)
	if err != nil {
		http.Error(w, "invalid project_id", http.StatusBadRequest)
		return
	}

	limit := parseInt(r.URL.Query().Get("limit"), 20)
	offset := parseInt(r.URL.Query().Get("offset"), 0)

	records, total, err := s.ListReviews(r.Context(), tenantID, projectID, limit, offset)
	if err != nil {
		http.Error(w, "failed to list reviews", http.StatusInternalServerError)
		return
	}

	responses := make([]ReviewResponse, len(records))
	for i, rec := range records {
		responses[i] = toReviewResponse(rec)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"reviews": responses,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}

func HandleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func parseInt(s string, defaultVal int) int {
	if v, err := strconv.Atoi(s); err == nil {
		return v
	}
	return defaultVal
}
