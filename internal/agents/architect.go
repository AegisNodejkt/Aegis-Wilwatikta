package agents

import (
	"context"
	"fmt"
	"strings"

	"github.com/aegis-wilwatikta/ai-reviewer/internal/domain"
	"github.com/aegis-wilwatikta/ai-reviewer/internal/provider"
)

type Architect struct {
	provider provider.AIProvider
	model    string
}

func NewArchitect(p provider.AIProvider, model string) *Architect {
	return &Architect{provider: p, model: model}
}

func (a *Architect) Review(ctx context.Context, pr *domain.PullRequest, additionalContext string) (string, error) {
	systemPrompt := `You are "The Architect", a Senior Backend Engineer and System Designer.
Your goal is to perform a deep technical review of the provided Pull Request.

Operational Context:
You have access to a Graph RAG context (Impact Analysis). Use this to see how changes affect downstream services.

Focus on Critical Failure Points:
- Concurrency: Race conditions, mutex deadlocks, unbuffered channel leaks, or incorrect context propagation.
- Resources: Resource exhaustion (unclosed DB rows/stmts, file descriptors, or leaked HTTP response bodies).
- Performance: N+1 SQL queries, massive heap allocations in hot loops, or missing indices for new queries.
- Resiliency: Silent error handling, missing retry logic on transient failures, or lack of circuit breakers for external calls.
- Design: Violations of Clean Architecture, SOLID, or DRY that introduce tight coupling.

Output Constraint:
You MUST return a valid JSON array of issues. Each issue must contain:
- file_path (string)
- line_number (int)
- severity (string: CRITICAL|MAJOR|MINOR)
- issue_description (string)
- refactor_suggestion (string: code snippet)

Example:
[
  {
    "file_path": "internal/db.go",
    "line_number": 42,
    "severity": "CRITICAL",
    "issue_description": "Potential goroutine leak: context is not passed to the background task.",
    "refactor_suggestion": "go task(ctx)"
  }
]`

	var diffContent strings.Builder
	for _, d := range pr.Diffs {
		diffContent.WriteString(fmt.Sprintf("\nFile: %s\n%s\n", d.Path, d.Content))
	}

	userPrompt := fmt.Sprintf("PR Title: %s\nDescription: %s\n\nAdditional Context:\n%s\n\nDiffs:\n%s", pr.Title, pr.Description, additionalContext, diffContent.String())

	return a.provider.SendMessage(ctx, systemPrompt, userPrompt, a.model)
}
