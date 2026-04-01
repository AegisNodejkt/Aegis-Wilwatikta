package pipeline

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/aegis-wilwatikta/ai-reviewer/internal/domain"
)

type Logger interface {
	Debug(msg string, fields ...interface{})
	Info(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
}

type defaultLogger struct{}

func (l *defaultLogger) Debug(msg string, fields ...interface{}) {
	log.Printf("[DEBUG] "+msg, fields...)
}

func (l *defaultLogger) Info(msg string, fields ...interface{}) {
	log.Printf("[INFO] "+msg, fields...)
}

func (l *defaultLogger) Warn(msg string, fields ...interface{}) {
	log.Printf("[WARN] "+msg, fields...)
}

func (l *defaultLogger) Error(msg string, fields ...interface{}) {
	log.Printf("[ERROR] "+msg, fields...)
}

type AgentExecutor interface {
	Execute(ctx context.Context, input interface{}) (interface{}, error)
	Name() AgentName
}

type PipelineOrchestrator struct {
	config         PipelineConfig
	circuitBreaker *CircuitBreaker
	logger         Logger
	traces         sync.Map
}

type AgentTrace struct {
	AgentName  AgentName
	StartTime  time.Time
	EndTime    time.Time
	Duration   time.Duration
	Input      interface{}
	Output     interface{}
	Error      error
	Attempts   int
	Skipped    bool
	SkipReason string
}

func NewPipelineOrchestrator(config PipelineConfig) *PipelineOrchestrator {
	return &PipelineOrchestrator{
		config:         config,
		circuitBreaker: NewCircuitBreaker(config.CircuitBreaker),
		logger:         &defaultLogger{}}
}

func (o *PipelineOrchestrator) WithLogger(logger Logger) *PipelineOrchestrator {
	o.logger = logger
	return o
}

func (o *PipelineOrchestrator) Execute(ctx context.Context, agents map[AgentName]AgentExecutor, input *PipelineInput) (*PipelineResult, error) {
	result := &PipelineResult{
		Success: false,
	}

	startTime := time.Now()
	o.logger.Info("Pipeline execution started")

	// Execute Scout phase
	scoutResult := o.executeAgent(ctx, AgentScout, agents[AgentScout], input, nil)
	result.ScoutResult = scoutResult

	if scoutResult.Error != nil && !scoutResult.Skipped {
		o.logger.Warn("Scout agent failed: %v", scoutResult.Error)
		result.Errors = append(result.Errors, fmt.Errorf("scout: %w", scoutResult.Error))
	}

	// Prepare Architect input
	architectInput := &ArchitectInput{
		PR:                input.PR,
		AdditionalContext: "",
		ImpactReports:     nil,
	}

	if scoutResult.Output != nil {
		if scoutOutput, ok := scoutResult.Output.(*ScoutOutput); ok {
			architectInput.AdditionalContext = scoutOutput.AdditionalContext
			architectInput.ImpactReports = scoutOutput.ImpactReports
		}
	}

	// Execute Architect phase
	architectResult := o.executeAgent(ctx, AgentArchitect, agents[AgentArchitect], architectInput, nil)
	result.ArchitectResult = architectResult

	if architectResult.Error != nil && !architectResult.Skipped {
		o.logger.Error("Architect agent failed: %v", architectResult.Error)
		result.Errors = append(result.Errors, fmt.Errorf("architect: %w", architectResult.Error))
		// Architect failure is critical - cannot proceed without review
		return result, fmt.Errorf("architect review failed: %w", architectResult.Error)
	}

	// Prepare Diplomat input
	diplomatInput := &DiplomatInput{}
	if architectResult.Output != nil {
		if architectOutput, ok := architectResult.Output.(*ArchitectOutput); ok {
			diplomatInput.RawReview = architectOutput.RawReview
		}
	}
	if architectInput.ImpactReports != nil {
		diplomatInput.ImpactReports = architectInput.ImpactReports
	}

	// Execute Diplomat phase
	diplomatResult := o.executeAgent(ctx, AgentDiplomat, agents[AgentDiplomat], diplomatInput, nil)
	result.DiplomatResult = diplomatResult

	if diplomatResult.Error != nil && !diplomatResult.Skipped {
		o.logger.Error("Diplomat agent failed: %v", diplomatResult.Error)
		result.Errors = append(result.Errors, fmt.Errorf("diplomat: %w", diplomatResult.Error))
		return result, fmt.Errorf("diplomat formatting failed: %w", diplomatResult.Error)
	}

	if diplomatResult.Output != nil {
		if diplomatOutput, ok := diplomatResult.Output.(*DiplomatOutput); ok {
			result.FinalReview = diplomatOutput.Result
		}
	}

	result.Success = true
	if len(result.Errors) > 0 {
		result.PartialOutput = true
	}

	o.logger.Info("Pipeline execution completed in %v", time.Since(startTime))
	return result, nil
}

func (o *PipelineOrchestrator) executeAgent(ctx context.Context, name AgentName, agent AgentExecutor, input interface{}, preReqResult *AgentResult) *AgentResult {
	startTime := time.Now()
	trace := &AgentTrace{
		AgentName: name,
		StartTime: startTime,
		Input:     input,
	}

	// Check circuit breaker
	if !o.circuitBreaker.Allow() {
		trace.EndTime = time.Now()
		trace.Duration = trace.EndTime.Sub(trace.StartTime)
		trace.Skipped = true
		trace.SkipReason = "circuit breaker open"
		o.traces.Store(string(name), trace)
		o.logger.Warn("Agent %s skipped: circuit breaker open", name)
		return &AgentResult{
			AgentName:  name,
			Skipped:    true,
			SkipReason: "circuit breaker open",
			Duration:   trace.Duration,
		}
	}

	// Get agent config
	agentConfig, exists := o.config.Agents[name]
	if !exists {
		agentConfig = AgentConfig{
			Name:          name,
			Timeout:       60 * time.Second,
			MaxRetries:    3,
			RetryBaseWait: time.Second,
			RetryMaxWait:  time.Minute,
			Enabled:       true,
		}
	}

	if !agentConfig.Enabled {
		trace.EndTime = time.Now()
		trace.Duration = trace.EndTime.Sub(trace.StartTime)
		trace.Skipped = true
		trace.SkipReason = "agent disabled"
		o.traces.Store(string(name), trace)
		return &AgentResult{
			AgentName:  name,
			Skipped:    true,
			SkipReason: "agent disabled",
			Duration:   trace.Duration,
		}
	}

	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, agentConfig.Timeout)
	defer cancel()

	// Create retrier
	retrier := NewRetrier(RetryConfig{
		MaxRetries:      agentConfig.MaxRetries,
		BaseWait:        agentConfig.RetryBaseWait,
		MaxWait:         agentConfig.RetryMaxWait,
		RetryableErrors: DefaultRetryableErrors(),
	})

	var output interface{}
	var err error
	attempts := 0

	// Execute with retry
	execErr := retrier.WithBackoff(timeoutCtx, func() error {
		attempts++
		output, err = agent.Execute(timeoutCtx, input)
		return err
	})

	trace.EndTime = time.Now()
	trace.Duration = trace.EndTime.Sub(trace.StartTime)
	trace.Output = output
	trace.Attempts = attempts

	if execErr != nil {
		trace.Error = execErr
		o.circuitBreaker.RecordFailure()
		o.traces.Store(string(name), trace)
		return &AgentResult{
			AgentName: name,
			Output:    output,
			Error:     execErr,
			Duration:  trace.Duration,
			Attempts:  attempts,
		}
	}

	o.circuitBreaker.RecordSuccess()
	o.traces.Store(string(name), trace)

	return &AgentResult{
		AgentName: name,
		Output:    output,
		Duration:  trace.Duration,
		Attempts:  attempts,
	}
}

func (o *PipelineOrchestrator) GetTrace(name AgentName) (*AgentTrace, bool) {
	if v, ok := o.traces.Load(string(name)); ok {
		return v.(*AgentTrace), true
	}
	return nil, false
}

func (o *PipelineOrchestrator) GetAllTraces() map[AgentName]*AgentTrace {
	traces := make(map[AgentName]*AgentTrace)
	o.traces.Range(func(key, value interface{}) bool {
		traces[AgentName(key.(string))] = value.(*AgentTrace)
		return true
	})
	return traces
}

// Pipeline input structs
type PipelineInput struct {
	PR *domain.PullRequest
}

type ArchitectInput struct {
	PR                *domain.PullRequest
	AdditionalContext string
	ImpactReports     []*domain.ImpactReport
}

type DiplomatInput struct {
	RawReview     string
	ImpactReports []*domain.ImpactReport
}
