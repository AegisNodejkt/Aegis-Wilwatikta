package benchmark_test

import (
	"context"
	"testing"
	"time"

	"github.com/aegis-wilwatikta/ai-reviewer/internal/agents"
	"github.com/aegis-wilwatikta/ai-reviewer/internal/domain"
	"github.com/aegis-wilwatikta/ai-reviewer/internal/rag/embedding"
	"github.com/aegis-wilwatikta/ai-reviewer/internal/rag/store"
)

type mockScoutProvider struct {
	delay time.Duration
}

func (m *mockScoutProvider) SendMessage(ctx context.Context, systemPrompt, userPrompt, model string) (string, error) {
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	return "internal/auth.go, internal/models.go", nil
}

func (m *mockScoutProvider) Name() string {
	return "mock-scout"
}

type mockScoutPlatform struct{}

func (m *mockScoutPlatform) GetFileContent(ctx context.Context, owner, repo, path, ref string) (string, error) {
	time.Sleep(5 * time.Millisecond)
	return `package auth

import "fmt"

func Login() error {
	return nil
}
`, nil
}

func (m *mockScoutPlatform) GetPullRequest(ctx context.Context, owner, repo string, prNumber int) (*domain.PullRequest, error) {
	return &domain.PullRequest{
		ID:             prNumber,
		Title:          "Benchmark Test PR",
		Description:    "PR for benchmarking Scout agent",
		BaseBranch:     "main",
		HeadBranch:     "feature-benchmark",
		HeadSHA:        "abc123",
		Diffs:          nil,
		PreviousReview: nil,
	}, nil
}

func (m *mockScoutPlatform) PostReview(ctx context.Context, owner, repo string, pr *domain.PullRequest, review *domain.ReviewResult) error {
	return nil
}

func (m *mockScoutPlatform) GetLastReview(ctx context.Context, owner, repo string, prNumber int) (*domain.ReviewResult, error) {
	return nil, nil
}

type mockScoutGraphStore struct{}

func (m *mockScoutGraphStore) UpsertNode(ctx context.Context, node domain.CodeNode) error {
	return nil
}

func (m *mockScoutGraphStore) UpsertRelation(ctx context.Context, rel domain.CodeRelation) error {
	return nil
}

func (m *mockScoutGraphStore) GetImpactContext(ctx context.Context, projectID, filePath string) (*domain.ImpactReport, error) {
	return nil, nil
}

func (m *mockScoutGraphStore) QueryContext(ctx context.Context, projectID, filePath string) ([]domain.CodeNode, error) {
	return nil, nil
}

func (m *mockScoutGraphStore) FindRelatedByEmbedding(ctx context.Context, projectID string, embedding []float32, limit int) ([]domain.CodeNode, error) {
	return nil, nil
}

func (m *mockScoutGraphStore) GetFileHash(ctx context.Context, projectID, path string) (string, error) {
	return "", nil
}

func (m *mockScoutGraphStore) DeleteNodesByFile(ctx context.Context, projectID, path string) error {
	return nil
}

func (m *mockScoutGraphStore) DeleteNodesByProject(ctx context.Context, projectID string) error {
	return nil
}

var _ store.GraphStore = (*mockScoutGraphStore)(nil)

type mockScoutEmbedder struct{}

func (m *mockScoutEmbedder) EmbedText(ctx context.Context, text string) ([]float32, error) {
	embedding := make([]float32, 768)
	for i := range embedding {
		embedding[i] = float32(i%100) / 100.0
	}
	return embedding, nil
}

func (m *mockScoutEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i := range result {
		emb, _ := m.EmbedText(ctx, texts[i])
		result[i] = emb
	}
	return result, nil
}

func (m *mockScoutEmbedder) Dimension() int {
	return 768
}

var _ embedding.EmbeddingProvider = (*mockScoutEmbedder)(nil)

func BenchmarkScoutGatherContext(b *testing.B) {
	provider := &mockScoutProvider{}
	platform := &mockScoutPlatform{}
	graphStore := &mockScoutGraphStore{}
	embedder := &mockScoutEmbedder{}

	scout := agents.NewScout(provider, platform, graphStore, embedder, "gemini-1.5-flash", "test-project")

	pr := &domain.PullRequest{
		ID:          1,
		Title:       "Benchmark Test PR",
		Description: "Testing Scout performance",
		Diffs: []domain.FileDiff{
			{Path: "internal/auth.go", Content: "+func Login() error { return nil }"},
			{Path: "internal/models.go", Content: "+type User struct { ID string }"},
		},
		BaseBranch: "main",
		HeadBranch: "feature-test",
		HeadSHA:    "abc123",
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := scout.GatherContext(ctx, "testowner", "test-repo", pr)
		if err != nil {
			b.Fatalf("GatherContext failed: %v", err)
		}
	}
}

func BenchmarkScoutWithLargePR(b *testing.B) {
	provider := &mockScoutProvider{delay: 20 * time.Millisecond}
	platform := &mockScoutPlatform{}
	graphStore := &mockScoutGraphStore{}
	embedder := &mockScoutEmbedder{}

	scout := agents.NewScout(provider, platform, graphStore, embedder, "gemini-1.5-flash", "test-project")

	diffs := make([]domain.FileDiff, 100)
	for i := range diffs {
		diffs[i] = domain.FileDiff{
			Path:    "internal/file_" + string(rune('A'+i%26)) + ".go",
			Content: "+func Func" + string(rune('A'+i%26)) + "() error { return nil }",
		}
	}

	pr := &domain.PullRequest{
		ID:          1,
		Title:       "Large PR Benchmark",
		Description: "Testing Scout with large PR",
		Diffs:       diffs,
		BaseBranch:  "main",
		HeadBranch:  "feature-large",
		HeadSHA:     "def456",
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := scout.GatherContext(ctx, "testowner", "test-repo", pr)
		if err != nil {
			b.Fatalf("GatherContext failed: %v", err)
		}
	}
}
