package agents

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/aegis-wilwatikta/ai-reviewer/internal/domain"
	"github.com/aegis-wilwatikta/ai-reviewer/internal/platform"
	"github.com/aegis-wilwatikta/ai-reviewer/internal/provider"
	"github.com/aegis-wilwatikta/ai-reviewer/internal/rag/embedding"
	"github.com/aegis-wilwatikta/ai-reviewer/internal/rag/store"
)

type Scout struct {
	provider   provider.AIProvider
	platform   platform.Platform
	graphStore store.GraphStore
	embedder   embedding.EmbeddingProvider
	model      string
	projectID  string
}

func NewScout(p provider.AIProvider, plat platform.Platform, gs store.GraphStore, emb embedding.EmbeddingProvider, model, projectID string) *Scout {
	return &Scout{
		provider:   p,
		platform:   plat,
		graphStore: gs,
		embedder:   emb,
		model:      model,
		projectID:  projectID,
	}
}

func (s *Scout) GatherContext(ctx context.Context, owner, repo string, pr *domain.PullRequest) (string, []*domain.ImpactReport, error) {
	var relatedFiles []string
	var reports []*domain.ImpactReport
	var fragileNodes []string

	// 0. Historical Context (Resolution Tracking)
	historicalContext := ""
	if pr.PreviousReview != nil {
		historicalContext = "\n--- PREVIOUS REVIEW UNRESOLVED ISSUES ---\n"
		for _, r := range pr.PreviousReview.Reviews {
			if r.Severity == domain.SeverityMedium || r.Severity == domain.SeverityHigh || r.Severity == domain.SeverityCritical {
				historicalContext += fmt.Sprintf("- [%s] %s in %s\n", r.Severity, r.Issue, r.File)
				relatedFiles = append(relatedFiles, r.File)
			}
		}
	}

	// 1. RAG-based context gathering (Primary)
	if s.graphStore != nil {
		for _, diff := range pr.Diffs {
			impact, err := s.graphStore.GetImpactContext(ctx, s.projectID, diff.Path)
			if err == nil && impact != nil {
				reports = append(reports, impact)
				if impact.BlastRadiusScore > 10 { // Threshold for "Fragile Nodes"
					fragileNodes = append(fragileNodes, impact.TargetNode.Name)
				}
				for _, affected := range impact.AffectedNodes {
					relatedFiles = append(relatedFiles, affected.Node.Path)
				}
			}
		}
	}

	// 2. Semantic Search context gathering (Secondary)
	if s.graphStore != nil && s.embedder != nil {
		for _, diff := range pr.Diffs {
			// Generate embedding for the diff content
			emb, err := s.embedder.EmbedText(ctx, diff.Content)
			if err == nil {
				relatedNodes, err := s.graphStore.FindRelatedByEmbedding(ctx, s.projectID, emb, 5)
				if err == nil {
					for _, node := range relatedNodes {
						relatedFiles = append(relatedFiles, node.Path)
					}
				}
			}
		}
	}

	// 3. Heuristic context gathering (Fallback/Tertiary)
	heuristicFiles := s.findRelatedFiles(pr)
	relatedFiles = append(relatedFiles, heuristicFiles...)

	// Remove duplicates
	relatedFiles = s.uniqueStrings(relatedFiles)

	// 2. LLM decision on which files are actually relevant
	systemPrompt := fmt.Sprintf(`You are "The Scout", a Context Optimization Expert. Your mission is to provide "The Architect" with the perfect amount of information.
Project ID: %s

Your Strategy:
- Identify not just what changed, but what might break.
- For every changed Interface, fetch its Implementations. For every changed Struct, fetch its Consumers.
- Prune boilerplate noise (generated code, mocks).
- Focus on "Fragile Nodes" (nodes with high downstream impact).
- You are provided with a list of UNRESOLVED issues from the previous review iteration. Focus your context gathering on the files where CRITICAL and WARNING issues were previously flagged. Verify if the changes in the current diff address these specific issues.

Respond ONLY with a comma-separated list of filenames that are absolutely necessary to understand the impact of this PR.
Example output: internal/auth.go, internal/models.go`, s.projectID)

	// Inject project awareness (go.mod / package.json)
	projectContext := ""
	for _, f := range []string{"go.mod", "package.json"} {
		content, err := s.platform.GetFileContent(ctx, owner, repo, f, pr.BaseBranch)
		if err == nil {
			projectContext += fmt.Sprintf("\n--- Project File: %s ---\n%s\n", f, content)
		}
	}

	userPrompt := fmt.Sprintf("PR Title: %s\nPR Description: %s\nFragile Nodes: %v\nPotential related files: %v\n%s\n\nProject Environment:%s\n\nWhich of these should I fetch for more context?",
		pr.Title, pr.Description, fragileNodes, relatedFiles, historicalContext, projectContext)

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
