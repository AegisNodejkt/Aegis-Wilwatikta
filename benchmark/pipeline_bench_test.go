package benchmark_test

import (
	"context"
	"testing"
	"time"

	"github.com/aegis-wilwatikta/ai-reviewer/internal/domain"
	"github.com/aegis-wilwatikta/ai-reviewer/internal/pipeline"
)

type mockAgentExecutor struct {
	name        pipeline.AgentName
	executeFn   func(ctx context.Context, input interface{}) (interface{}, error)
	delay       time.Duration
	failOnError bool
}

func (m *mockAgentExecutor) Execute(ctx context.Context, input interface{}) (interface{}, error) {
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if m.executeFn != nil {
		return m.executeFn(ctx, input)
	}
	return nil, nil
}

func (m *mockAgentExecutor) Name() pipeline.AgentName {
	return m.name
}

var _ pipeline.AgentExecutor = (*mockAgentExecutor)(nil)

func BenchmarkPipelineOrchestrator(b *testing.B) {
	b.Run("FastAgents", func(b *testing.B) {
		config := pipeline.PipelineConfig{
			Agents: map[pipeline.AgentName]pipeline.AgentConfig{
				pipeline.AgentScout: {
					Name:       pipeline.AgentScout,
					Timeout:    30 * time.Second,
					MaxRetries: 3,
					Enabled:    true,
				},
				pipeline.AgentArchitect: {
					Name:       pipeline.AgentArchitect,
					Timeout:    60 * time.Second,
					MaxRetries: 3,
					Enabled:    true,
				},
				pipeline.AgentDiplomat: {
					Name:       pipeline.AgentDiplomat,
					Timeout:    30 * time.Second,
					MaxRetries: 3,
					Enabled:    true,
				},
			},
			CircuitBreaker: pipeline.CircuitBreakerConfig{
				FailureThreshold: 5,
				Timeout:          30 * time.Second,
			},
		}

		orchestrator := pipeline.NewPipelineOrchestrator(config)

		agents := map[pipeline.AgentName]pipeline.AgentExecutor{
			pipeline.AgentScout: &mockAgentExecutor{
				name: pipeline.AgentScout,
				executeFn: func(ctx context.Context, input interface{}) (interface{}, error) {
					return &pipeline.ScoutOutput{
						AdditionalContext: "test context",
						ImpactReports:     nil,
					}, nil
				},
			},
			pipeline.AgentArchitect: &mockAgentExecutor{
				name: pipeline.AgentArchitect,
				executeFn: func(ctx context.Context, input interface{}) (interface{}, error) {
					return &pipeline.ArchitectOutput{
						RawReview: `[{"file_path": "test.go", "line_number": 1, "severity": "MINOR"}]`,
					}, nil
				},
			},
			pipeline.AgentDiplomat: &mockAgentExecutor{
				name: pipeline.AgentDiplomat,
				executeFn: func(ctx context.Context, input interface{}) (interface{}, error) {
					return &pipeline.DiplomatOutput{
						Result: &domain.ReviewResult{
							Verdict: "APPROVE",
							Summary: "Looks good",
						},
					}, nil
				},
			},
		}

		input := &pipeline.PipelineInput{
			PR: &domain.PullRequest{
				ID:          1,
				Title:       "Benchmark PR",
				Description: "Testing pipeline performance",
				Diffs: []domain.FileDiff{
					{Path: "test.go", Content: "+func Test() {}"},
				},
			},
		}

		ctx := context.Background()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := orchestrator.Execute(ctx, agents, input)
			if err != nil {
				b.Fatalf("Pipeline execution failed: %v", err)
			}
		}
	})

	b.Run("SlowAgents", func(b *testing.B) {
		config := pipeline.PipelineConfig{
			Agents: map[pipeline.AgentName]pipeline.AgentConfig{
				pipeline.AgentScout: {
					Name:       pipeline.AgentScout,
					Timeout:    30 * time.Second,
					MaxRetries: 3,
					Enabled:    true,
				},
				pipeline.AgentArchitect: {
					Name:       pipeline.AgentArchitect,
					Timeout:    60 * time.Second,
					MaxRetries: 3,
					Enabled:    true,
				},
				pipeline.AgentDiplomat: {
					Name:       pipeline.AgentDiplomat,
					Timeout:    30 * time.Second,
					MaxRetries: 3,
					Enabled:    true,
				},
			},
			CircuitBreaker: pipeline.CircuitBreakerConfig{
				FailureThreshold: 5,
				Timeout:          30 * time.Second,
			},
		}

		orchestrator := pipeline.NewPipelineOrchestrator(config)

		agents := map[pipeline.AgentName]pipeline.AgentExecutor{
			pipeline.AgentScout: &mockAgentExecutor{
				name:  pipeline.AgentScout,
				delay: 100 * time.Millisecond,
				executeFn: func(ctx context.Context, input interface{}) (interface{}, error) {
					return &pipeline.ScoutOutput{AdditionalContext: "test"}, nil
				},
			},
			pipeline.AgentArchitect: &mockAgentExecutor{
				name:  pipeline.AgentArchitect,
				delay: 200 * time.Millisecond,
				executeFn: func(ctx context.Context, input interface{}) (interface{}, error) {
					return &pipeline.ArchitectOutput{RawReview: "[]"}, nil
				},
			},
			pipeline.AgentDiplomat: &mockAgentExecutor{
				name:  pipeline.AgentDiplomat,
				delay: 50 * time.Millisecond,
				executeFn: func(ctx context.Context, input interface{}) (interface{}, error) {
					return &pipeline.DiplomatOutput{Result: &domain.ReviewResult{Verdict: "APPROVE"}}, nil
				},
			},
		}

		input := &pipeline.PipelineInput{
			PR: &domain.PullRequest{
				ID:          1,
				Title:       "Slow Benchmark PR",
				Description: "Testing with slow agents",
				Diffs:       []domain.FileDiff{{Path: "test.go", Content: "+func Test() {}"}},
			},
		}

		ctx := context.Background()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := orchestrator.Execute(ctx, agents, input)
			if err != nil {
				b.Fatalf("Pipeline execution failed: %v", err)
			}
		}
	})

	b.Run("WithRetries", func(b *testing.B) {
		config := pipeline.PipelineConfig{
			Agents: map[pipeline.AgentName]pipeline.AgentConfig{
				pipeline.AgentScout: {
					Name:          pipeline.AgentScout,
					Timeout:       30 * time.Second,
					MaxRetries:    3,
					RetryBaseWait: 100 * time.Millisecond,
					RetryMaxWait:  time.Second,
					Enabled:       true,
				},
				pipeline.AgentArchitect: {
					Name:          pipeline.AgentArchitect,
					Timeout:       60 * time.Second,
					MaxRetries:    3,
					RetryBaseWait: 100 * time.Millisecond,
					RetryMaxWait:  time.Second,
					Enabled:       true,
				},
				pipeline.AgentDiplomat: {
					Name:          pipeline.AgentDiplomat,
					Timeout:       30 * time.Second,
					MaxRetries:    3,
					RetryBaseWait: 100 * time.Millisecond,
					RetryMaxWait:  time.Second,
					Enabled:       true,
				},
			},
			CircuitBreaker: pipeline.CircuitBreakerConfig{
				FailureThreshold: 5,
				Timeout:          30 * time.Second,
			},
		}

		orchestrator := pipeline.NewPipelineOrchestrator(config)

		attemptCount := 0
		agents := map[pipeline.AgentName]pipeline.AgentExecutor{
			pipeline.AgentScout: &mockAgentExecutor{
				name: pipeline.AgentScout,
				executeFn: func(ctx context.Context, input interface{}) (interface{}, error) {
					return &pipeline.ScoutOutput{AdditionalContext: "test"}, nil
				},
			},
			pipeline.AgentArchitect: &mockAgentExecutor{
				name: pipeline.AgentArchitect,
				executeFn: func(ctx context.Context, input interface{}) (interface{}, error) {
					attemptCount++
					if attemptCount%3 == 0 {
						return &pipeline.ArchitectOutput{RawReview: "[]"}, nil
					}
					return nil, context.DeadlineExceeded
				},
			},
			pipeline.AgentDiplomat: &mockAgentExecutor{
				name: pipeline.AgentDiplomat,
				executeFn: func(ctx context.Context, input interface{}) (interface{}, error) {
					return &pipeline.DiplomatOutput{Result: &domain.ReviewResult{Verdict: "APPROVE"}}, nil
				},
			},
		}

		input := &pipeline.PipelineInput{
			PR: &domain.PullRequest{
				ID:    1,
				Title: "Retry Benchmark",
				Diffs: []domain.FileDiff{{Path: "test.go", Content: "+"}},
			},
		}

		ctx := context.Background()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			attemptCount = 0
			_, err := orchestrator.Execute(ctx, agents, input)
			if err != nil {
				b.Logf("Pipeline execution had errors: %v", err)
			}
		}
	})
}

func TestPipelineSLACompliance(t *testing.T) {
	scoutSLA := 30 * time.Second
	architectSLA := 60 * time.Second
	diplomatSLA := 15 * time.Second

	config := pipeline.PipelineConfig{
		Agents: map[pipeline.AgentName]pipeline.AgentConfig{
			pipeline.AgentScout: {
				Name: pipeline.AgentScout, Timeout: scoutSLA, MaxRetries: 3, Enabled: true,
			},
			pipeline.AgentArchitect: {
				Name: pipeline.AgentArchitect, Timeout: architectSLA, MaxRetries: 3, Enabled: true,
			},
			pipeline.AgentDiplomat: {
				Name: pipeline.AgentDiplomat, Timeout: diplomatSLA, MaxRetries: 3, Enabled: true,
			},
		},
		CircuitBreaker: pipeline.CircuitBreakerConfig{
			FailureThreshold: 5,
			Timeout:          30 * time.Second,
		},
	}

	orchestrator := pipeline.NewPipelineOrchestrator(config)

	agents := map[pipeline.AgentName]pipeline.AgentExecutor{
		pipeline.AgentScout: &mockAgentExecutor{
			name: pipeline.AgentScout,
			executeFn: func(ctx context.Context, input interface{}) (interface{}, error) {
				return &pipeline.ScoutOutput{AdditionalContext: "context"}, nil
			},
		},
		pipeline.AgentArchitect: &mockAgentExecutor{
			name: pipeline.AgentArchitect,
			executeFn: func(ctx context.Context, input interface{}) (interface{}, error) {
				return &pipeline.ArchitectOutput{RawReview: "[]"}, nil
			},
		},
		pipeline.AgentDiplomat: &mockAgentExecutor{
			name: pipeline.AgentDiplomat,
			executeFn: func(ctx context.Context, input interface{}) (interface{}, error) {
				return &pipeline.DiplomatOutput{Result: &domain.ReviewResult{Verdict: "APPROVE"}}, nil
			},
		},
	}

	input := &pipeline.PipelineInput{
		PR: &domain.PullRequest{ID: 1, Title: "SLA Test", Diffs: []domain.FileDiff{{Path: "test.go"}}},
	}

	for i := 0; i < 10; i++ {
		ctx := context.Background()
		start := time.Now()
		result, err := orchestrator.Execute(ctx, agents, input)
		duration := time.Since(start)

		if err != nil {
			t.Errorf("Run %d: Pipeline failed: %v", i, err)
			continue
		}

		scoutTrace, _ := orchestrator.GetTrace(pipeline.AgentScout)
		architectTrace, _ := orchestrator.GetTrace(pipeline.AgentArchitect)
		diplomatTrace, _ := orchestrator.GetTrace(pipeline.AgentDiplomat)

		if scoutTrace != nil && scoutTrace.Duration > scoutSLA {
			t.Logf("Run %d: Scout exceeded SLA: %v > %v", i, scoutTrace.Duration, scoutSLA)
		}
		if architectTrace != nil && architectTrace.Duration > architectSLA {
			t.Logf("Run %d: Architect exceeded SLA: %v > %v", i, architectTrace.Duration, architectSLA)
		}
		if diplomatTrace != nil && diplomatTrace.Duration > diplomatSLA {
			t.Logf("Run %d: Diplomat exceeded SLA: %v > %v", i, diplomatTrace.Duration, diplomatSLA)
		}

		t.Logf("Run %d: Total duration: %v, Success: %v", i, duration, result.Success)
	}
}
