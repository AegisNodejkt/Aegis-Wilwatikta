package benchmark_test

import (
	"context"
	"testing"
	"time"

	"github.com/aegis-wilwatikta/ai-reviewer/internal/agents"
	"github.com/aegis-wilwatikta/ai-reviewer/internal/domain"
)

type mockDipProvider struct{}

func (m *mockDipProvider) SendMessage(ctx context.Context, systemPrompt, userPrompt, model string) (string, error) {
	return `{
		"verdict": "COMMENT",
		"summary": "## Review Summary\n\nThe code changes require attention.",
		"reviews": [
			{
				"file": "internal/auth.go",
				"line": 15,
				"severity": "MAJOR",
				"issue": "Missing error handling",
				"suggestion": "Add proper error handling"
			}
		]
	}`, nil
}

func (m *mockDipProvider) Name() string {
	return "mock-diplomat"
}

var _ interface {
	SendMessage(ctx context.Context, systemPrompt, userPrompt, model string) (string, error)
	Name() string
} = (*mockDipProvider)(nil)

func BenchmarkDiplomatFormatReview(b *testing.B) {
	provider := &mockDipProvider{}
	diplomat := agents.NewDiplomat(provider, "gemini-1.5-flash")

	smallReview := `[{"file_path": "auth.go", "line_number": 10, "severity": "MINOR", "issue_description": "Minor issue", "refactor_suggestion": "Fix it"}]`

	mediumReview := ""
	for i := 0; i < 10; i++ {
		mediumReview += `{"file_path": "file` + string(rune('A'+i%26)) + `.go", "line_number": ` + string(rune('0'+i%10)) + `, "severity": "MAJOR", "issue_description": "Issue ` + string(rune('A'+i%26)) + `", "refactor_suggestion": "Fix it"},`
	}
	mediumReview = "[" + mediumReview[:len(mediumReview)-1] + "]"

	largeReview := ""
	for i := 0; i < 50; i++ {
		largeReview += `{"file_path": "large` + string(rune('A'+i%26)) + `.go", "line_number": ` + string(rune('0'+i%10)) + `, "severity": "CRITICAL", "issue_description": "Critical issue ` + string(rune('A'+i%26)) + ` with detailed explanation that spans multiple lines", "refactor_suggestion": "Fix it completely"},`
	}
	largeReview = "[" + largeReview[:len(largeReview)-1] + "]"

	aggregated := make(map[agents.ImpactTier][]agents.AggregatedImpact)
	aggregated[agents.TierBreaking] = []agents.AggregatedImpact{
		{
			Tier: agents.TierBreaking,
			AffectedNode: domain.CodeNode{
				Name: "AuthService",
				Path: "internal/auth/service.go",
			},
			Reason: "Direct dependency on modified interface",
		},
	}

	ctx := context.Background()

	b.Run("SmallReview", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := diplomat.FormatReview(ctx, smallReview, nil, 85)
			if err != nil {
				b.Fatalf("FormatReview failed: %v", err)
			}
		}
	})

	b.Run("MediumReview", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := diplomat.FormatReview(ctx, mediumReview, nil, 75)
			if err != nil {
				b.Fatalf("FormatReview failed: %v", err)
			}
		}
	})

	b.Run("LargeReview", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := diplomat.FormatReview(ctx, largeReview, nil, 60)
			if err != nil {
				b.Fatalf("FormatReview failed: %v", err)
			}
		}
	})

	b.Run("WithImpactAnalysis", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := diplomat.FormatReview(ctx, mediumReview, aggregated, 80)
			if err != nil {
				b.Fatalf("FormatReview failed: %v", err)
			}
		}
	})
}

func TestDiplomatSLACompliance(t *testing.T) {
	provider := &mockDipProvider{}
	diplomat := agents.NewDiplomat(provider, "gemini-1.5-flash")

	review := `[{"file_path": "auth.go", "line_number": 10, "severity": "MINOR", "issue_description": "Minor issue", "refactor_suggestion": "Fix it"}]`

	ctx := context.Background()

	targetSLA := 15 * time.Second

	for i := 0; i < 10; i++ {
		start := time.Now()
		_, err := diplomat.FormatReview(ctx, review, nil, 85)
		duration := time.Since(start)

		if err != nil {
			t.Errorf("Iteration %d: FormatReview failed: %v", i, err)
			continue
		}

		if duration > targetSLA {
			t.Errorf("Iteration %d: Diplomat exceeded SLA: %v > %v", i, duration, targetSLA)
		}
	}
}
