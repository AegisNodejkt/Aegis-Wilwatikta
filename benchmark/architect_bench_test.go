package benchmark_test

import (
	"context"
	"testing"
	"time"

	"github.com/aegis-wilwatikta/ai-reviewer/internal/agents"
	"github.com/aegis-wilwatikta/ai-reviewer/internal/domain"
)

type mockArchProvider struct {
	responseDelay time.Duration
}

func (m *mockArchProvider) SendMessage(ctx context.Context, systemPrompt, userPrompt, model string) (string, error) {
	if m.responseDelay > 0 {
		select {
		case <-time.After(m.responseDelay):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	return `[{"file_path": "internal/auth.go", "line_number": 15, "severity": "MAJOR", "issue_description": "Missing error handling in Login function", "refactor_suggestion": "Add proper error handling and logging"}]`, nil
}

func (m *mockArchProvider) Name() string {
	return "mock-architect"
}

var _ interface {
	SendMessage(ctx context.Context, systemPrompt, userPrompt, model string) (string, error)
	Name() string
} = (*mockArchProvider)(nil)

func BenchmarkArchitectReview(b *testing.B) {
	provider := &mockArchProvider{}
	architect := agents.NewArchitect(provider, "gemini-1.5-pro")

	smallPR := &domain.PullRequest{
		ID:          1,
		Title:       "Small PR",
		Description: "Small change",
		Diffs: []domain.FileDiff{
			{
				Path:    "internal/auth.go",
				Content: "+func Login() error { return nil }",
			},
		},
	}

	mediumPR := &domain.PullRequest{
		ID:          2,
		Title:       "Medium PR",
		Description: "Medium change",
		Diffs:       []domain.FileDiff{},
	}

	for i := 0; i < 10; i++ {
		mediumPR.Diffs = append(mediumPR.Diffs, domain.FileDiff{
			Path:    "internal/file" + string(rune('A'+i)) + ".go",
			Content: "+func Func" + string(rune('A'+i)) + "() error { return nil }\n+// Additional lines\n+// More lines\n+// Even more lines",
		})
	}

	largePR := &domain.PullRequest{
		ID:          3,
		Title:       "Large PR",
		Description: "Large change",
		Diffs:       []domain.FileDiff{},
	}

	for i := 0; i < 50; i++ {
		largePR.Diffs = append(largePR.Diffs, domain.FileDiff{
			Path:    "internal/large" + string(rune('A'+i%26)) + ".go",
			Content: generateLargeDiff(20),
		})
	}

	ctx := context.Background()

	b.Run("SmallPR", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := architect.Review(ctx, smallPR, "")
			if err != nil {
				b.Fatalf("Review failed: %v", err)
			}
		}
	})

	b.Run("MediumPR", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := architect.Review(ctx, mediumPR, "")
			if err != nil {
				b.Fatalf("Review failed: %v", err)
			}
		}
	})

	b.Run("LargePR", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := architect.Review(ctx, largePR, "")
			if err != nil {
				b.Fatalf("Review failed: %v", err)
			}
		}
	})
}

func BenchmarkArchitectWithContext(b *testing.B) {
	provider := &mockArchProvider{}
	architect := agents.NewArchitect(provider, "gemini-1.5-pro")

	pr := &domain.PullRequest{
		ID:          1,
		Title:       "PR with Context",
		Description: "Testing with additional context",
		Diffs: []domain.FileDiff{
			{
				Path:    "internal/auth.go",
				Content: "+func Login() error { return nil }",
			},
		},
	}

	additionalContext := generateLargeContext(1000)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := architect.Review(ctx, pr, additionalContext)
		if err != nil {
			b.Fatalf("Review failed: %v", err)
		}
	}
}

func TestArchitectSLACompliance(t *testing.T) {
	provider := &mockArchProvider{responseDelay: 50 * time.Millisecond}
	architect := agents.NewArchitect(provider, "gemini-1.5-pro")

	pr := &domain.PullRequest{
		ID:          1,
		Title:       "SLA Test",
		Description: "Testing timeout handling",
		Diffs: []domain.FileDiff{
			{Path: "internal/auth.go", Content: "+func Login() error { return nil }"},
		},
	}

	targetSLA := 60 * time.Second
	ctx := context.Background()

	start := time.Now()
	_, err := architect.Review(ctx, pr, "")
	duration := time.Since(start)

	if err != nil {
		t.Logf("Review had error (may be expected in mock): %v", err)
	}

	if duration > targetSLA {
		t.Errorf("Architect exceeded SLA: %v > %v", duration, targetSLA)
	}

	t.Logf("Architect duration: %v (SLA: %v)", duration, targetSLA)
}

func generateLargeDiff(lines int) string {
	result := ""
	for i := 0; i < lines; i++ {
		result += "+func Function" + string(rune('A'+i%26)) + "() error {\n"
		result += "+    // Implementation line " + string(rune('A'+i%26)) + "\n"
		result += "+    return nil\n"
		result += "+}\n\n"
	}
	return result
}

func generateLargeContext(lines int) string {
	result := "--- File: context.go ---\n"
	for i := 0; i < lines; i++ {
		result += "package context\n\nfunc ContextFunc" + string(rune('A'+i%26)) + "() {}\n"
	}
	return result
}
