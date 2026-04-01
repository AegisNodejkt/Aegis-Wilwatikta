package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/aegis-wilwatikta/ai-reviewer/internal/agents"
	"github.com/aegis-wilwatikta/ai-reviewer/internal/domain"
	"github.com/aegis-wilwatikta/ai-reviewer/internal/pipeline"
	"github.com/aegis-wilwatikta/ai-reviewer/internal/platform"
)

type ReviewerEngine struct {
	Platform     platform.Platform
	Scout        *agents.Scout
	Architect    *agents.Architect
	Diplomat     *agents.Diplomat
	orchestrator *pipeline.PipelineOrchestrator
}

func NewReviewerEngine(plat platform.Platform, scout *agents.Scout, arch *agents.Architect, dip *agents.Diplomat) *ReviewerEngine {
	return &ReviewerEngine{
		Platform:  plat,
		Scout:     scout,
		Architect: arch,
		Diplomat:  dip,
	}
}

func NewPipelinedReviewerEngine(plat platform.Platform, scout *agents.Scout, arch *agents.Architect, dip *agents.Diplomat, config pipeline.PipelineConfig) *ReviewerEngine {
	return &ReviewerEngine{
		Platform:     plat,
		Scout:        scout,
		Architect:    arch,
		Diplomat:     dip,
		orchestrator: pipeline.NewPipelineOrchestrator(config),
	}
}

func DefaultPipelineConfig() pipeline.PipelineConfig {
	return pipeline.PipelineConfig{
		Agents: map[pipeline.AgentName]pipeline.AgentConfig{
			pipeline.AgentScout: {
				Name:          pipeline.AgentScout,
				Timeout:       30 * time.Second,
				MaxRetries:    3,
				RetryBaseWait: time.Second,
				RetryMaxWait:  30 * time.Second,
				Enabled:       true,
			},
			pipeline.AgentArchitect: {
				Name:          pipeline.AgentArchitect,
				Timeout:       60 * time.Second,
				MaxRetries:    3,
				RetryBaseWait: time.Second,
				RetryMaxWait:  60 * time.Second,
				Enabled:       true,
			},
			pipeline.AgentDiplomat: {
				Name:          pipeline.AgentDiplomat,
				Timeout:       15 * time.Second,
				MaxRetries:    3,
				RetryBaseWait: time.Second,
				RetryMaxWait:  15 * time.Second,
				Enabled:       true,
			},
		},
		CircuitBreaker: pipeline.CircuitBreakerConfig{
			FailureThreshold:    5,
			SuccessThreshold:    3,
			Timeout:             2 * time.Minute,
			MaxHalfOpenRequests: 2,
		},
		ParallelEnabled: true,
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

	fmt.Println("Review completed successfully.")
	return nil
}

func (e *ReviewerEngine) RunPipelinedReview(ctx context.Context, owner, repo string, prNumber int) (*PipelineResult, error) {
	if e.orchestrator == nil {
		return nil, fmt.Errorf("pipelined review not configured, use NewPipelinedReviewerEngine")
	}

	fmt.Printf("[Pipeline] Starting review for %s/%s PR #%d\n", owner, repo, prNumber)

	// 1. Fetch PR Data
	pr, err := e.Platform.GetPullRequest(ctx, owner, repo, prNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch PR: %w", err)
	}

	// 1.5 Fetch Previous Review
	fmt.Println("[Pipeline] Fetching last review context...")
	lastReview, err := e.Platform.GetLastReview(ctx, owner, repo, prNumber)
	if err != nil {
		fmt.Printf("[Pipeline] Warning: failed to fetch last review: %v\n", err)
	} else {
		pr.PreviousReview = lastReview
	}

	// Create agent adapters
	agentAdapters := map[pipeline.AgentName]pipeline.AgentExecutor{
		pipeline.AgentScout:     pipeline.NewScoutAdapter(e.Scout, owner, repo),
		pipeline.AgentArchitect: pipeline.NewArchitectAdapter(e.Architect),
		pipeline.AgentDiplomat:  pipeline.NewDiplomatAdapter(e.Diplomat),
	}

	// Execute pipeline
	result, err := e.orchestrator.Execute(ctx, agentAdapters, &pipeline.PipelineInput{PR: pr})
	if err != nil {
		return nil, err
	}

	if result.FinalReview == nil {
		return nil, fmt.Errorf("no review result produced")
	}

	// Post review to platform
	fmt.Println("[Pipeline] Posting review to platform...")
	if err := e.Platform.PostReview(ctx, owner, repo, pr, result.FinalReview); err != nil {
		return nil, fmt.Errorf("failed to post review: %w", err)
	}

	// Log traces
	e.logTraces()

	fmt.Println("[Pipeline] Review completed successfully.")
	return &PipelineResult{
		PipelineResult: result,
	}, nil
}

func (e *ReviewerEngine) RunReviewWithGracefulDegradation(ctx context.Context, owner, repo string, prNumber int) (*PipelineResult, error) {
	result, err := e.RunPipelinedReview(ctx, owner, repo, prNumber)
	if err != nil {
		fmt.Printf("[Pipeline] Error during review: %v. Attempting graceful degradation...\n", err)

		// Try legacy approach as fallback
		legacyErr := e.RunReview(ctx, owner, repo, prNumber)
		if legacyErr != nil {
			return nil, fmt.Errorf("both pipelined and legacy reviews failed: pipeline=%w, legacy=%w", err, legacyErr)
		}

		return &PipelineResult{
			Success:       true,
			PartialOutput: true,
			Errors:        []error{err},
		}, nil
	}
	return result, nil
}

func (e *ReviewerEngine) logTraces() {
	if e.orchestrator == nil {
		return
	}
	traces := e.orchestrator.GetAllTraces()
	for name, trace := range traces {
		fmt.Printf("[Trace] Agent %s: duration=%v, attempts=%d, error=%v, skipped=%v\n",
			name, trace.Duration, trace.Attempts, trace.Error, trace.Skipped)
	}
}

type PipelineResult struct {
	PipelineResult *pipeline.PipelineResult
	Success        bool
	PartialOutput  bool
	Errors         []error
}

func (r *PipelineResult) GetFinalReview() *domain.ReviewResult {
	if r.PipelineResult == nil {
		return nil
	}
	return r.PipelineResult.FinalReview
}
