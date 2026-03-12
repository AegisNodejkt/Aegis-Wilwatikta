package platform

import (
	"context"
	"fmt"

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
		BaseBranch:  pr.GetBase().GetRef(),
		HeadBranch:  pr.GetHead().GetRef(),
		Diffs:       diffs,
	}, nil
}

func (g *GitHubPlatform) PostReview(ctx context.Context, owner, repo string, prNumber int, review *domain.ReviewResult) error {
	var comments []*github.DraftReviewComment
	for _, r := range review.Reviews {
		comments = append(comments, &github.DraftReviewComment{
			Path: github.String(r.File),
			Body: github.String(fmt.Sprintf("**[%s]** %s\n\n> %s", r.Severity, r.Issue, r.Suggestion)),
			Line: github.Int(r.Line),
			Side: github.String("RIGHT"),
		})
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
		Body:     github.String(review.Summary),
		Event:    github.String(event),
		Comments: comments,
	}

	_, _, err := g.client.PullRequests.CreateReview(ctx, owner, repo, prNumber, reviewRequest)
	return err
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
