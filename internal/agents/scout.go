package agents

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/aegis-wilwatikta/ai-reviewer/internal/domain"
	"github.com/aegis-wilwatikta/ai-reviewer/internal/platform"
	"github.com/aegis-wilwatikta/ai-reviewer/internal/provider"
)

type Scout struct {
	provider provider.AIProvider
	platform platform.Platform
	model    string
}

func NewScout(p provider.AIProvider, plat platform.Platform, model string) *Scout {
	return &Scout{provider: p, platform: plat, model: model}
}

func (s *Scout) GatherContext(ctx context.Context, owner, repo string, pr *domain.PullRequest) (string, error) {
	// 1. Heuristic context gathering (simple regex for imports/mentions)
	relatedFiles := s.findRelatedFiles(pr)

	// 2. LLM decision on which files are actually relevant
	systemPrompt := `You are "The Scout", a context optimization agent.
Your goal is to identify which related files are necessary to understand the logic changes in a Pull Request.
You will be given a list of files mentioned in the diff and you must decide which ones to fetch to provide better context for a senior engineer review.
Respond ONLY with a comma-separated list of filenames that are absolutely necessary.
Example output: internal/auth.go, internal/models.go`

	userPrompt := fmt.Sprintf("PR Title: %s\nPR Description: %s\nPotential related files: %v\n\nWhich of these should I fetch for more context?", pr.Title, pr.Description, relatedFiles)

	if len(relatedFiles) == 0 {
		return "", nil
	}

	response, err := s.provider.SendMessage(ctx, systemPrompt, userPrompt, s.model)
	if err != nil {
		return "", err
	}

	// Extract filenames using regex to be more robust against conversational filler
	filesToFetch := s.extractFilenames(response)
	var additionalContext strings.Builder
	for _, f := range filesToFetch {
		content, err := s.platform.GetFileContent(ctx, owner, repo, f, pr.BaseBranch)
		if err == nil {
			additionalContext.WriteString(fmt.Sprintf("\n--- File: %s ---\n%s\n", f, content))
		}
	}

	return additionalContext.String(), nil
}

func (s *Scout) findRelatedFiles(pr *domain.PullRequest) []string {
	relatedMap := make(map[string]bool)
	// Very simple heuristic: look for things that look like filenames or types in the diff
	re := regexp.MustCompile(`[a-zA-Z0-9_/]+\.(go|rs|py|js|ts)`)
	for _, diff := range pr.Diffs {
		matches := re.FindAllString(diff.Content, -1)
		for _, m := range matches {
			if m != diff.Path {
				relatedMap[m] = true
			}
		}
	}

	var related []string
	for k := range relatedMap {
		related = append(related, k)
	}
	return related
}

func (s *Scout) extractFilenames(response string) []string {
	// Simple comma split but then trim each part and only keep things that look like filenames
	parts := strings.Split(response, ",")
	var files []string
	re := regexp.MustCompile(`[a-zA-Z0-9_/]+\.[a-z]+`)
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		match := re.FindString(trimmed)
		if match != "" {
			files = append(files, match)
		}
	}
	return files
}
