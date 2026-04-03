package pipeline

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCircuitBreaker_StateTransitions(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold:    3,
		SuccessThreshold:    2,
		Timeout:             time.Second * 5,
		MaxHalfOpenRequests: 1,
	}

	cb := NewCircuitBreaker(config)

	if cb.State() != StateClosed {
		t.Errorf("expected initial state to be closed, got %s", cb.State())
	}

	for i := 0; i < config.FailureThreshold; i++ {
		cb.RecordFailure()
	}

	if cb.State() != StateOpen {
		t.Errorf("expected state to be open after %d failures, got %s", config.FailureThreshold, cb.State())
	}

	if cb.Allow() {
		t.Error("expected Allow() to return false when circuit is open")
	}
}

func TestCircuitBreaker_HalfOpenTransition(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold:    2,
		SuccessThreshold:    2,
		Timeout:             time.Millisecond * 100,
		MaxHalfOpenRequests: 1,
	}

	cb := NewCircuitBreaker(config)

	cb.RecordFailure()
	cb.RecordFailure()

	if cb.State() != StateOpen {
		t.Errorf("expected state to be open, got %s", cb.State())
	}

	time.Sleep(config.Timeout + time.Millisecond*10)

	if !cb.Allow() {
		t.Error("expected Allow() to return true after timeout in open state")
	}

	if cb.State() != StateHalfOpen {
		t.Errorf("expected state to be half-open, got %s", cb.State())
	}

	cb.RecordSuccess()
	cb.RecordSuccess()

	if cb.State() != StateClosed {
		t.Errorf("expected state to be closed after %d successes, got %s", config.SuccessThreshold, cb.State())
	}
}

func TestCircuitBreaker_Execute(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold:    2,
		SuccessThreshold:    1,
		Timeout:             time.Second * 5,
		MaxHalfOpenRequests: 1,
	}

	cb := NewCircuitBreaker(config)

	err := cb.Execute(context.Background(), func() error {
		return nil
	})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	err = cb.Execute(context.Background(), func() error {
		return errors.New("test error")
	})
	if err == nil {
		t.Error("expected error, got nil")
	}

	if cb.State() != StateClosed {
		t.Errorf("expected state to be closed, got %s", cb.State())
	}
}

func TestRetrier_WithBackoff_Success(t *testing.T) {
	config := RetryConfig{
		MaxRetries:      3,
		BaseWait:        time.Millisecond,
		MaxWait:         time.Millisecond * 100,
		RetryableErrors: []string{"timeout"},
	}

	retrier := NewRetrier(config)

	attempts := 0
	err := retrier.WithBackoff(context.Background(), func() error {
		attempts++
		if attempts < 2 {
			return errors.New("connection timeout")
		}
		return nil
	})

	if err != nil {
		t.Errorf("expected no error after retry, got %v", err)
	}
	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

func TestRetrier_WithBackoff_MaxRetriesExceeded(t *testing.T) {
	config := RetryConfig{
		MaxRetries:      2,
		BaseWait:        time.Millisecond,
		MaxWait:         time.Millisecond * 10,
		RetryableErrors: []string{"timeout"},
	}

	retrier := NewRetrier(config)

	attempts := 0
	err := retrier.WithBackoff(context.Background(), func() error {
		attempts++
		return errors.New("connection timeout")
	})

	if err == nil {
		t.Error("expected error after max retries")
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestRetrier_WithContextCancellation(t *testing.T) {
	config := RetryConfig{
		MaxRetries:      5,
		BaseWait:        time.Second,
		MaxWait:         time.Second * 5,
		RetryableErrors: []string{"timeout"},
	}

	retrier := NewRetrier(config)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := retrier.WithBackoff(ctx, func() error {
		return errors.New("connection timeout")
	})

	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		patterns  []string
		retryable bool
	}{
		{
			name:      "timeout error",
			err:       errors.New("connection timeout"),
			patterns:  DefaultRetryableErrors(),
			retryable: true,
		},
		{
			name:      "rate limit error",
			err:       errors.New("429 rate limit exceeded"),
			patterns:  DefaultRetryableErrors(),
			retryable: true,
		},
		{
			name:      "non-retryable error",
			err:       errors.New("invalid input"),
			patterns:  DefaultRetryableErrors(),
			retryable: false,
		},
		{
			name:      "nil error",
			err:       nil,
			patterns:  DefaultRetryableErrors(),
			retryable: false,
		},
		{
			name:      "context deadline exceeded",
			err:       context.DeadlineExceeded,
			patterns:  DefaultRetryableErrors(),
			retryable: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRetryableError(tt.err, tt.patterns)
			if result != tt.retryable {
				t.Errorf("expected %v, got %v", tt.retryable, result)
			}
		})
	}
}

func TestPipelineOrchestrator_Execute(t *testing.T) {
	config := PipelineConfig{
		Agents: map[AgentName]AgentConfig{
			AgentScout: {
				Name:          AgentScout,
				Timeout:       time.Second * 10,
				MaxRetries:    1,
				RetryBaseWait: time.Millisecond,
				RetryMaxWait:  time.Second,
				Enabled:       true,
			},
			AgentArchitect: {
				Name:          AgentArchitect,
				Timeout:       time.Second * 10,
				MaxRetries:    1,
				RetryBaseWait: time.Millisecond,
				RetryMaxWait:  time.Second,
				Enabled:       true,
			},
			AgentDiplomat: {
				Name:          AgentDiplomat,
				Timeout:       time.Second * 10,
				MaxRetries:    1,
				RetryBaseWait: time.Millisecond,
				RetryMaxWait:  time.Second,
				Enabled:       true,
			},
		},
		CircuitBreaker: CircuitBreakerConfig{
			FailureThreshold:    5,
			SuccessThreshold:    3,
			Timeout:             time.Minute,
			MaxHalfOpenRequests: 2,
		},
		ParallelEnabled: false,
	}

	orchestrator := NewPipelineOrchestrator(config)

	mockAgents := map[AgentName]AgentExecutor{
		AgentScout:     &mockAgent{name: AgentScout},
		AgentArchitect: &mockAgent{name: AgentArchitect},
		AgentDiplomat:  &mockAgent{name: AgentDiplomat},
	}

	result, err := orchestrator.Execute(context.Background(), mockAgents, &PipelineInput{})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if !result.Success {
		t.Error("expected successful result")
	}
}

func TestPipelineOrchestrator_AgentTimeout(t *testing.T) {
	config := PipelineConfig{
		Agents: map[AgentName]AgentConfig{
			AgentScout: {
				Name:          AgentScout,
				Timeout:       time.Millisecond * 50,
				MaxRetries:    1,
				RetryBaseWait: time.Millisecond,
				RetryMaxWait:  time.Millisecond * 10,
				Enabled:       true,
			},
			AgentArchitect: {
				Name:          AgentArchitect,
				Timeout:       time.Second,
				MaxRetries:    1,
				RetryBaseWait: time.Millisecond,
				RetryMaxWait:  time.Millisecond * 10,
				Enabled:       true,
			},
			AgentDiplomat: {
				Name:          AgentDiplomat,
				Timeout:       time.Second,
				MaxRetries:    1,
				RetryBaseWait: time.Millisecond,
				RetryMaxWait:  time.Millisecond * 10,
				Enabled:       true,
			},
		},
		CircuitBreaker: CircuitBreakerConfig{
			FailureThreshold:    5,
			SuccessThreshold:    3,
			Timeout:             time.Minute,
			MaxHalfOpenRequests: 2,
		},
	}

	orchestrator := NewPipelineOrchestrator(config)

	mockAgents := map[AgentName]AgentExecutor{
		AgentScout:     &slowMockAgent{name: AgentScout, delay: time.Second},
		AgentArchitect: &mockAgent{name: AgentArchitect},
		AgentDiplomat:  &mockAgent{name: AgentDiplomat},
	}

	result, err := orchestrator.Execute(context.Background(), mockAgents, &PipelineInput{})

	// Scout timeout should be handled gracefully - pipeline continues but scout result shows error
	if result == nil {
		t.Error("expected result even with timeout")
		return
	}
	if result.ScoutResult == nil {
		t.Error("expected scout result")
		return
	}
	if result.ScoutResult.Error == nil {
		t.Error("expected scout result to have timeout error")
	}
	// Pipeline should still continue with other agents despite scout failure
	_ = err // Pipeline handles scout failures gracefully
}

func TestPipelineOrchestrator_CircuitBreaker(t *testing.T) {
	config := PipelineConfig{
		Agents: map[AgentName]AgentConfig{
			AgentScout: {
				Name:       AgentScout,
				Timeout:    time.Second,
				MaxRetries: 1,
				Enabled:    true,
			},
		},
		CircuitBreaker: CircuitBreakerConfig{
			FailureThreshold:    1,
			SuccessThreshold:    1,
			Timeout:             time.Minute,
			MaxHalfOpenRequests: 1,
		},
	}

	orchestrator := NewPipelineOrchestrator(config)

	failingAgent := &failingMockAgent{name: AgentScout}
	mockAgents := map[AgentName]AgentExecutor{
		AgentScout: failingAgent,
	}

	_, _ = orchestrator.Execute(context.Background(), mockAgents, &PipelineInput{})

	if orchestrator.circuitBreaker.State() != StateOpen {
		t.Errorf("expected circuit breaker to be open, got %s", orchestrator.circuitBreaker.State())
	}
}

func TestPipelineOrchestrator_Tracing(t *testing.T) {
	config := DefaultTestPipelineConfig()
	orchestrator := NewPipelineOrchestrator(config)

	mockAgents := map[AgentName]AgentExecutor{
		AgentScout:     &mockAgent{name: AgentScout},
		AgentArchitect: &mockAgent{name: AgentArchitect},
		AgentDiplomat:  &mockAgent{name: AgentDiplomat},
	}

	_, err := orchestrator.Execute(context.Background(), mockAgents, &PipelineInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, agentName := range []AgentName{AgentScout, AgentArchitect, AgentDiplomat} {
		trace, ok := orchestrator.GetTrace(agentName)
		if !ok {
			t.Errorf("expected trace for %s", agentName)
			continue
		}
		if trace.Duration == 0 {
			t.Errorf("expected non-zero duration for %s", agentName)
		}
		if trace.Error != nil {
			t.Errorf("unexpected error in trace for %s: %v", agentName, trace.Error)
		}
	}
}

func DefaultTestPipelineConfig() PipelineConfig {
	return PipelineConfig{
		Agents: map[AgentName]AgentConfig{
			AgentScout: {
				Name:          AgentScout,
				Timeout:       time.Second * 10,
				MaxRetries:    1,
				RetryBaseWait: time.Millisecond,
				RetryMaxWait:  time.Second,
				Enabled:       true,
			},
			AgentArchitect: {
				Name:          AgentArchitect,
				Timeout:       time.Second * 10,
				MaxRetries:    1,
				RetryBaseWait: time.Millisecond,
				RetryMaxWait:  time.Second,
				Enabled:       true,
			},
			AgentDiplomat: {
				Name:          AgentDiplomat,
				Timeout:       time.Second * 10,
				MaxRetries:    1,
				RetryBaseWait: time.Millisecond,
				RetryMaxWait:  time.Second,
				Enabled:       true,
			},
		},
		CircuitBreaker: CircuitBreakerConfig{
			FailureThreshold:    5,
			SuccessThreshold:    3,
			Timeout:             time.Minute,
			MaxHalfOpenRequests: 2,
		},
	}
}

type mockAgent struct {
	name AgentName
}

func (m *mockAgent) Execute(ctx context.Context, input interface{}) (interface{}, error) {
	switch m.name {
	case AgentScout:
		return &ScoutOutput{}, nil
	case AgentArchitect:
		return &ArchitectOutput{}, nil
	case AgentDiplomat:
		return &DiplomatOutput{}, nil
	default:
		return nil, nil
	}
}

func (m *mockAgent) Name() AgentName {
	return m.name
}

type slowMockAgent struct {
	name  AgentName
	delay time.Duration
}

func (m *slowMockAgent) Execute(ctx context.Context, input interface{}) (interface{}, error) {
	select {
	case <-time.After(m.delay):
		return nil, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (m *slowMockAgent) Name() AgentName {
	return m.name
}

type failingMockAgent struct {
	name AgentName
}

func (m *failingMockAgent) Execute(ctx context.Context, input interface{}) (interface{}, error) {
	return nil, errors.New("intentional failure")
}

func (m *failingMockAgent) Name() AgentName {
	return m.name
}
