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
	ID          int
	Title       string
	Description string
	BaseBranch  string
	HeadBranch  string
	Diffs       []FileDiff
}

type ReviewComment struct {
	File      string   `json:"file"`
	Position  int      `json:"position"`
	Severity  Severity `json:"severity"`
	Issue     string   `json:"issue"`
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

type StandardizedReviewJSON struct {
	AgentMetadata AgentMetadata `json:"agent_metadata"`
	Summary       struct {
		Verdict           Verdict `json:"verdict"`
		OverallLogicScore int     `json:"overall_logic_score"`
	} `json:"summary"`
	Reviews []ReviewComment `json:"reviews"`
}
