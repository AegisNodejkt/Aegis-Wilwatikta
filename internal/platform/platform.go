package platform

import (
	"context"
	"github.com/aegis-wilwatikta/ai-reviewer/internal/domain"
)

type Platform interface {
	GetPullRequest(ctx context.Context, owner, repo string, prNumber int) (*domain.PullRequest, error)
	PostReview(ctx context.Context, owner, repo string, prNumber int, review *domain.ReviewResult) error
	GetFileContent(ctx context.Context, owner, repo, path, ref string) (string, error)
}
