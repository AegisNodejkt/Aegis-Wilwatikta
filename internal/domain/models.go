package domain

type Verdict string

const (
	VerdictApprove        Verdict = "APPROVE"
	VerdictComment        Verdict = "COMMENT"
	VerdictRequestChanges Verdict = "REQUEST_CHANGES"
)

type Severity string

const (
	SeverityLow      Severity = "LOW"
	SeverityMedium   Severity = "MEDIUM"
	SeverityHigh     Severity = "HIGH"
	SeverityCritical Severity = "CRITICAL"
)

type FileDiff struct {
	Path     string
	Content  string // The actual diff content for this file
}

type PullRequest struct {
	ID             int
	Title          string
	Description    string
	Author         string
	BaseBranch     string
	HeadBranch     string
	HeadSHA        string
	Diffs          []FileDiff
	PreviousReview *ReviewResult
}

type ReviewComment struct {
	File       string   `json:"file"`
	Line       int      `json:"line"`
	Severity   Severity `json:"severity"`
	Issue      string   `json:"issue"`
	Suggestion string   `json:"suggestion"`
}

type ReviewResult struct {
	Verdict Verdict         `json:"verdict"`
	Summary string          `json:"summary"`
	Reviews []ReviewComment `json:"reviews"`
}

type AgentMetadata struct {
	ID    string `json:"id"`
	Model string `json:"model"`
}

type DashboardViolation struct {
	File       string   `json:"file"`
	LineNumber int      `json:"line_number"`
	Severity   Severity `json:"severity"`
	RuleCode   string   `json:"rule_code"`
	Message    string   `json:"message"`
	Suggestion string   `json:"suggestion"`
}

type DashboardReviewPayload struct {
	PRID             string               `json:"pr_id"`
	PRURL            string               `json:"pr_url"`
	Author           string               `json:"author"`
	Branch           string               `json:"branch"`
	Verdict          Verdict              `json:"verdict"`
	Summary          string               `json:"summary"`
	Violations       []DashboardViolation `json:"violations"`
	HealthScoreDelta int                  `json:"health_score_delta"`
	ReviewedAt       string               `json:"reviewed_at"`
}
