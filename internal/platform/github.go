package platform

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/aegis-wilwatikta/ai-reviewer/internal/domain"
	"github.com/google/go-github/v60/github"
	"golang.org/x/oauth2"
)

type GitHubPlatform struct {
	client *github.Client
}

func NewGitHubPlatform(token string) *GitHubPlatform {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(context.Background(), ts)
	return &GitHubPlatform{
		client: github.NewClient(tc),
	}
}

func (g *GitHubPlatform) GetPullRequest(ctx context.Context, owner, repo string, prNumber int) (*domain.PullRequest, error) {
	pr, _, err := g.client.PullRequests.Get(ctx, owner, repo, prNumber)
	if err != nil {
		return nil, err
	}

	files, _, err := g.client.PullRequests.ListFiles(ctx, owner, repo, prNumber, nil)
	if err != nil {
		return nil, err
	}

	var diffs []domain.FileDiff
	for _, file := range files {
		diffs = append(diffs, domain.FileDiff{
			Path:    file.GetFilename(),
			Content: file.GetPatch(),
		})
	}

	return &domain.PullRequest{
		ID:          pr.GetNumber(),
		Title:       pr.GetTitle(),
		Description: pr.GetBody(),
		Author:      pr.GetUser().GetLogin(),
		BaseBranch:  pr.GetBase().GetRef(),
		HeadBranch:  pr.GetHead().GetRef(),
		HeadSHA:     pr.GetHead().GetSHA(),
		Diffs:       diffs,
	}, nil
}

func (g *GitHubPlatform) PostReview(ctx context.Context, owner, repo string, pr *domain.PullRequest, review *domain.ReviewResult) error {
	var validComments []*github.DraftReviewComment
	var failedComments strings.Builder

	// Map of patches per file for line validation
	patches := make(map[string]string)
	for _, d := range pr.Diffs {
		patches[d.Path] = d.Content
	}

	for _, r := range review.Reviews {
		if r.Line <= 0 {
			failedComments.WriteString(fmt.Sprintf("- **[%s]** %s (File: %s, Line: %d): %s\n", r.Severity, r.Issue, r.File, r.Line, r.Suggestion))
			continue
		}

		patch, ok := patches[r.File]
		if !ok || !g.validateLineInDiff(patch, r.Line) {
			failedComments.WriteString(fmt.Sprintf("- **[%s]** %s (File: %s, Line: %d): %s\n", r.Severity, r.Issue, r.File, r.Line, r.Suggestion))
			continue
		}

		validComments = append(validComments, &github.DraftReviewComment{
			Path: github.String(r.File),
			Body: github.String(fmt.Sprintf("**[%s]** %s\n\n> %s", r.Severity, r.Issue, r.Suggestion)),
			Line: github.Int(r.Line),
			Side: github.String("RIGHT"),
		})
	}

	summary := review.Summary
	if failedComments.Len() > 0 {
		summary += "\n\n### ⚠️ Other Findings (Not in Diff or Invalid Line)\n" + failedComments.String()
	}

	event := string(review.Verdict)
	if review.Verdict == domain.VerdictComment {
		event = "COMMENT"
	} else if review.Verdict == domain.VerdictRequestChanges {
		event = "REQUEST_CHANGES"
	} else if review.Verdict == domain.VerdictApprove {
		event = "APPROVE"
	}

	reviewRequest := &github.PullRequestReviewRequest{
		Body:     github.String(summary),
		Event:    github.String(event),
		Comments: validComments,
		CommitID: github.String(pr.HeadSHA),
	}

	_, _, err := g.client.PullRequests.CreateReview(ctx, owner, repo, pr.ID, reviewRequest)
	if err != nil && strings.Contains(err.Error(), "422") {
		// Second-level fallback: Post everything in the summary if CreateReview fails
		var allComments strings.Builder
		for _, r := range review.Reviews {
			allComments.WriteString(fmt.Sprintf("- **[%s]** %s (File: %s, Line: %d): %s\n", r.Severity, r.Issue, r.File, r.Line, r.Suggestion))
		}
		fallbackSummary := review.Summary + "\n\n### 🔍 Detailed Findings\n" + allComments.String()
		fallbackRequest := &github.PullRequestReviewRequest{
			Body:     github.String(fallbackSummary),
			Event:    github.String(event),
			CommitID: github.String(pr.HeadSHA),
		}
		_, _, err = g.client.PullRequests.CreateReview(ctx, owner, repo, pr.ID, fallbackRequest)
	}
	return err
}

func (g *GitHubPlatform) validateLineInDiff(patch string, line int) bool {
	// Basic parsing of diff hunks to check if 'line' is an added line.
	// Hunk header format: @@ -start,len +start,len @@
	lines := strings.Split(patch, "\n")
	currentLine := 0
	for _, l := range lines {
		if strings.HasPrefix(l, "@@") {
			parts := strings.Split(l, " ")
			if len(parts) < 3 {
				continue
			}
			newRange := strings.Split(parts[2], ",")
			start, _ := strconv.Atoi(strings.TrimPrefix(newRange[0], "+"))
			currentLine = start
			continue
		}
		if currentLine == 0 {
			continue
		}
		// If it's an added line or context line in the new file, currentLine increments
		if !strings.HasPrefix(l, "-") {
			if strings.HasPrefix(l, "+") && currentLine == line {
				return true
			}
			currentLine++
		}
	}
	return false
}

func (g *GitHubPlatform) GetFileContent(ctx context.Context, owner, repo, path, ref string) (string, error) {
	fileContent, _, _, err := g.client.Repositories.GetContents(ctx, owner, repo, path, &github.RepositoryContentGetOptions{Ref: ref})
	if err != nil {
		return "", err
	}
	content, err := fileContent.GetContent()
	if err != nil {
		return "", err
	}
	return content, nil
}

func (g *GitHubPlatform) GetLastReview(ctx context.Context, owner, repo string, prNumber int) (*domain.ReviewResult, error) {
	// In a real implementation, we would fetch the last review from GitHub
	// and parse it back into domain.ReviewResult.
	// For now, this is a placeholder as the actual implementation would involve
	// complex parsing of Markdown comments.
	return nil, nil
}
