package pipeline

import (
	"context"
	"errors"
	"time"

	"github.com/aegis-wilwatikta/ai-reviewer/internal/domain"
)

var (
	ErrInvalidInput = errors.New("invalid input type")
)

type AgentName string

const (
	AgentScout     AgentName = "scout"
	AgentArchitect AgentName = "architect"
	AgentDiplomat  AgentName = "diplomat"
)

type AgentResult struct {
	AgentName  AgentName
	Output     interface{}
	Error      error
	Duration   time.Duration
	Attempts   int
	Skipped    bool
	SkipReason string
}

type ScoutOutput struct {
	AdditionalContext string
	ImpactReports     []*domain.ImpactReport
}

type ArchitectOutput struct {
	RawReview string
}

type DiplomatOutput struct {
	Result *domain.ReviewResult
}

type AgentConfig struct {
	Name          AgentName
	Timeout       time.Duration
	MaxRetries    int
	RetryBaseWait time.Duration
	RetryMaxWait  time.Duration
	Enabled       bool
}

type PipelineConfig struct {
	Agents          map[AgentName]AgentConfig
	CircuitBreaker  CircuitBreakerConfig
	ParallelEnabled bool
}

type CircuitBreakerConfig struct {
	FailureThreshold    int
	SuccessThreshold    int
	Timeout             time.Duration
	MaxHalfOpenRequests int
}

type CircuitBreakerState string

const (
	StateClosed   CircuitBreakerState = "closed"
	StateOpen     CircuitBreakerState = "open"
	StateHalfOpen CircuitBreakerState = "half-open"
)

type Agent interface {
	Name() AgentName
	Execute(ctx context.Context, input interface{}) (interface{}, error)
}

type PipelineResult struct {
	ScoutResult     *AgentResult
	ArchitectResult *AgentResult
	DiplomatResult  *AgentResult
	FinalReview     *domain.ReviewResult
	Success         bool
	PartialOutput   bool
	Errors          []error
}

type RetryConfig struct {
	MaxRetries      int
	BaseWait        time.Duration
	MaxWait         time.Duration
	RetryableErrors []string
}
