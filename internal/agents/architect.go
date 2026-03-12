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
	systemPrompt := `You are "The Architect", a Senior Backend Engineer.
Your goal is to perform a deep technical review of the provided Pull Request.
Focus on:
1. Concurrency issues (race conditions, mutex leaks, goroutine leaks).
2. Resource leaks (unclosed DB connections, files, or bodies).
3. Performance (N+1 queries, inefficient memory allocations).
4. Error handling (ensuring errors are not silently ignored).
5. Architectural integrity (Clean Architecture, SOLID, DRY).

You must provide your review in a structured format that "The Diplomat" can understand.
Include file paths, line numbers, severity, the issue found, and a suggestion for improvement.`

	var diffContent strings.Builder
	for _, d := range pr.Diffs {
		diffContent.WriteString(fmt.Sprintf("\nFile: %s\n%s\n", d.Path, d.Content))
	}

	userPrompt := fmt.Sprintf("PR Title: %s\nDescription: %s\n\nAdditional Context:\n%s\n\nDiffs:\n%s", pr.Title, pr.Description, additionalContext, diffContent.String())

	return a.provider.SendMessage(ctx, systemPrompt, userPrompt, a.model)
}
