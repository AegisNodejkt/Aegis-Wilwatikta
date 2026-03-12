package engine

import (
	"context"
	"testing"

	"github.com/aegis-wilwatikta/ai-reviewer/internal/agents"
	"github.com/aegis-wilwatikta/ai-reviewer/internal/domain"
)

type MockPlatform struct{}

func (m *MockPlatform) GetPullRequest(ctx context.Context, owner, repo string, prNumber int) (*domain.PullRequest, error) {
	return &domain.PullRequest{
		ID:    prNumber,
		Title: "Test PR",
		Diffs: []domain.FileDiff{{Path: "main.go", Content: "diff content"}},
	}, nil
}
func (m *MockPlatform) PostReview(ctx context.Context, owner, repo string, pr *domain.PullRequest, review *domain.ReviewResult) error {
	return nil
}
func (m *MockPlatform) GetFileContent(ctx context.Context, owner, repo, path, ref string) (string, error) {
	return "mock content", nil
}
func (m *MockPlatform) GetLastReview(ctx context.Context, owner, repo string, prNumber int) (*domain.ReviewResult, error) {
	return nil, nil
}

type MockAIProvider struct {
	Response string
}

func (m *MockAIProvider) SendMessage(ctx context.Context, systemPrompt string, userPrompt string, model string) (string, error) {
	return m.Response, nil
}
func (m *MockAIProvider) Name() string { return "mock" }

func TestReviewerEngine_RunReview(t *testing.T) {
	mockPlat := &MockPlatform{}
	mockAI := &MockAIProvider{
		Response: `{
			"verdict": "REQUEST_CHANGES",
			"summary": "Found some issues",
			"reviews": [{"file": "main.go", "line": 10, "severity": "HIGH", "issue": "test issue", "suggestion": "fix it"}]
		}`,
	}

	scout := agents.NewScout(mockAI, mockPlat, nil, nil, "test-model", "test-project")
	arch := agents.NewArchitect(mockAI, "test-model")
	dip := agents.NewDiplomat(mockAI, "test-model")

	e := NewReviewerEngine(mockPlat, scout, arch, dip)

	err := e.RunReview(context.Background(), "owner", "repo", 1)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}
