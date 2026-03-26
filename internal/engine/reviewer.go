package engine

import (
	"context"
	"fmt"

	"github.com/aegis-wilwatikta/ai-reviewer/internal/agents"
	"github.com/aegis-wilwatikta/ai-reviewer/internal/platform"
)

type ReviewerEngine struct {
	Platform  platform.Platform
	Scout     *agents.Scout
	Architect *agents.Architect
	Diplomat  *agents.Diplomat
}

func NewReviewerEngine(plat platform.Platform, scout *agents.Scout, arch *agents.Architect, dip *agents.Diplomat) *ReviewerEngine {
	return &ReviewerEngine{
		Platform:  plat,
		Scout:     scout,
		Architect: arch,
		Diplomat:  dip,
	}
}

func (e *ReviewerEngine) RunReview(ctx context.Context, owner, repo string, prNumber int) error {
	fmt.Printf("Starting review for %s/%s PR #%d\n", owner, repo, prNumber)

	// 1. Fetch PR Data
	pr, err := e.Platform.GetPullRequest(ctx, owner, repo, prNumber)
	if err != nil {
		return fmt.Errorf("failed to fetch PR: %w", err)
	}

	// 1.5 Fetch Previous Review
	fmt.Println("Fetching last review context...")
	lastReview, err := e.Platform.GetLastReview(ctx, owner, repo, prNumber)
	if err != nil {
		fmt.Printf("Warning: failed to fetch last review: %v\n", err)
	} else {
		pr.PreviousReview = lastReview
	}

	// 2. Scout Phase: Gather Context
	fmt.Println("Scout is gathering context...")
	additionalContext, reports, err := e.Scout.GatherContext(ctx, owner, repo, pr)
	if err != nil {
		fmt.Printf("Warning: Scout context gathering failed: %v\n", err)
		// We can still proceed with just the diff
	}

	// 3. Architect Phase: Deep Review
	fmt.Println("Architect is reviewing changes...")
	rawReview, err := e.Architect.Review(ctx, pr, additionalContext)
	if err != nil {
		return fmt.Errorf("architect review failed: %w", err)
	}

	// 4. Diplomat Phase: Formatting
	fmt.Println("Diplomat is formatting feedback...")

	aggregated := agents.AggregateImpacts(reports)
	healthScore := agents.CalculateHealthScore(aggregated)

	reviewResult, err := e.Diplomat.FormatReview(ctx, rawReview, aggregated, healthScore)
	if err != nil {
		return fmt.Errorf("diplomat formatting failed: %w", err)
	}

	// 5. Post Review back to Platform
	fmt.Println("Posting review to platform...")
	err = e.Platform.PostReview(ctx, owner, repo, pr, reviewResult)
	if err != nil {
		return fmt.Errorf("failed to post review: %w", err)
	}

	// 6. Sync Review to Dashboard (Diplomat)
	fmt.Println("Diplomat is syncing review to dashboard...")
	if err := e.Diplomat.SubmitReviewToDashboard(ctx, owner, repo, pr, reviewResult, healthScore); err != nil {
		fmt.Printf("Warning: Diplomat dashboard sync failed: %v\n", err)
	}

	fmt.Println("Review completed successfully.")
	return nil
}
