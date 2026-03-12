package platform

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/aegis-wilwatikta/ai-reviewer/internal/domain"
)

type LocalPlatform struct {
	BaseBranch string
}

func NewLocalPlatform(baseBranch string) *LocalPlatform {
	if baseBranch == "" {
		baseBranch = "main"
	}
	return &LocalPlatform{BaseBranch: baseBranch}
}

func (l *LocalPlatform) GetPullRequest(ctx context.Context, owner, repo string, prNumber int) (*domain.PullRequest, error) {
	// For local, we assume current branch is head
	headBranchCmd := exec.CommandContext(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD")
	headBranchBytes, err := headBranchCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get current branch: %w", err)
	}
	headBranch := strings.TrimSpace(string(headBranchBytes))

	// Get diff between base and head
	diffCmd := exec.CommandContext(ctx, "git", "diff", l.BaseBranch+"..."+headBranch)
	diffBytes, err := diffCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get git diff: %w", err)
	}

	// Parse diff into FileDiffs (simplified parsing)
	diffs := parseGitDiff(string(diffBytes))

	return &domain.PullRequest{
		ID:          0,
		Title:       "Local Review",
		Description: "Reviewing local changes against " + l.BaseBranch,
		BaseBranch:  l.BaseBranch,
		HeadBranch:  headBranch,
		Diffs:       diffs,
	}, nil
}

func (l *LocalPlatform) PostReview(ctx context.Context, owner, repo string, prNumber int, review *domain.ReviewResult) error {
	fmt.Printf("\n--- LOCAL REVIEW RESULT ---\n")
	fmt.Printf("Verdict: %s\n", review.Verdict)
	fmt.Printf("Summary: %s\n", review.Summary)
	fmt.Printf("Comments:\n")
	for _, c := range review.Reviews {
		fmt.Printf("- %s:%d [%s] %s\n  Suggestion: %s\n", c.File, c.Line, c.Severity, c.Issue, c.Suggestion)
	}
	fmt.Printf("---------------------------\n")
	return nil
}

func (l *LocalPlatform) GetFileContent(ctx context.Context, owner, repo, path, ref string) (string, error) {
	catCmd := exec.CommandContext(ctx, "git", "show", ref+":"+path)
	contentBytes, err := catCmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get file content: %w", err)
	}
	return string(contentBytes), nil
}

func (l *LocalPlatform) GetLastReview(ctx context.Context, owner, repo string, prNumber int) (*domain.ReviewResult, error) {
	return nil, nil
}

func parseGitDiff(diffStr string) []domain.FileDiff {
	var diffs []domain.FileDiff
	files := strings.Split(diffStr, "diff --git ")
	for _, fileDiff := range files {
		if fileDiff == "" {
			continue
		}
		lines := strings.SplitN(fileDiff, "\n", 2)
		if len(lines) < 2 {
			continue
		}
		// Extract filename from "a/filename b/filename"
		parts := strings.Fields(lines[0])
		if len(parts) < 2 {
			continue
		}
		filename := strings.TrimPrefix(parts[1], "b/")
		diffs = append(diffs, domain.FileDiff{
			Path:    filename,
			Content: fileDiff,
		})
	}
	return diffs
}
