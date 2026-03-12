package agents

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/aegis-wilwatikta/ai-reviewer/internal/domain"
	"github.com/aegis-wilwatikta/ai-reviewer/internal/platform"
	"github.com/aegis-wilwatikta/ai-reviewer/internal/provider"
	"github.com/aegis-wilwatikta/ai-reviewer/internal/rag/store"
)

type Scout struct {
	provider   provider.AIProvider
	platform   platform.Platform
	graphStore store.GraphStore
	model      string
}

func NewScout(p provider.AIProvider, plat platform.Platform, gs store.GraphStore, model string) *Scout {
	return &Scout{
		provider:   p,
		platform:   plat,
		graphStore: gs,
		model:      model,
	}
}

func (s *Scout) GatherContext(ctx context.Context, owner, repo string, pr *domain.PullRequest) (string, []*domain.ImpactReport, error) {
	var relatedFiles []string
	var reports []*domain.ImpactReport

	// 1. RAG-based context gathering (Primary)
	if s.graphStore != nil {
		for _, diff := range pr.Diffs {
			impact, err := s.graphStore.GetImpactContext(ctx, diff.Path)
			if err == nil && impact != nil {
				reports = append(reports, impact)
				for _, affected := range impact.AffectedNodes {
					relatedFiles = append(relatedFiles, affected.Node.Path)
				}
			}
		}
	}

	// 2. Heuristic context gathering (Fallback/Secondary)
	heuristicFiles := s.findRelatedFiles(pr)
	relatedFiles = append(relatedFiles, heuristicFiles...)

	// Remove duplicates
	relatedFiles = s.uniqueStrings(relatedFiles)

	// 2. LLM decision on which files are actually relevant
	systemPrompt := `You are "The Scout", a context optimization agent.
Your goal is to identify which related files are necessary to understand the logic changes in a Pull Request.
You will be given a list of files mentioned in the diff and you must decide which ones to fetch to provide better context for a senior engineer review.
Respond ONLY with a comma-separated list of filenames that are absolutely necessary.
Example output: internal/auth.go, internal/models.go`

	userPrompt := fmt.Sprintf("PR Title: %s\nPR Description: %s\nPotential related files: %v\n\nWhich of these should I fetch for more context?", pr.Title, pr.Description, relatedFiles)

	if len(relatedFiles) == 0 {
		return "", reports, nil
	}

	response, err := s.provider.SendMessage(ctx, systemPrompt, userPrompt, s.model)
	if err != nil {
		return "", reports, err
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

	return additionalContext.String(), reports, nil
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

func (s *Scout) uniqueStrings(input []string) []string {
	keys := make(map[string]bool)
	var list []string
	for _, entry := range input {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
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
