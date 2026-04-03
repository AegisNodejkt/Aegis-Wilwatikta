package provider

import (
	"context"
	"sync"
	"time"
)

type CostRecord struct {
	ID           string
	Provider     string
	Model        string
	Timestamp    time.Time
	InputTokens  int
	OutputTokens int
	CostCents    int
	PromptHash   string
	CacheHit     bool
	ReqID        string
}

type CostTracker interface {
	Record(ctx context.Context, record CostRecord) error
	GetTotalCost(ctx context.Context, provider string, since time.Time) (int, error)
	GetUsage(ctx context.Context, provider string, since time.Time) (int, int, error)
}

type InMemoryCostTracker struct {
	records []CostRecord
	mu      sync.RWMutex
}

func NewInMemoryCostTracker() *InMemoryCostTracker {
	return &InMemoryCostTracker{
		records: make([]CostRecord, 0),
	}
}

func (t *InMemoryCostTracker) Record(ctx context.Context, record CostRecord) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.records = append(t.records, record)
	return nil
}

func (t *InMemoryCostTracker) GetTotalCost(ctx context.Context, provider string, since time.Time) (int, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	total := 0
	for _, r := range t.records {
		if (provider == "" || r.Provider == provider) && r.Timestamp.After(since) {
			total += r.CostCents
		}
	}
	return total, nil
}

func (t *InMemoryCostTracker) GetUsage(ctx context.Context, provider string, since time.Time) (int, int, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	input, output := 0, 0
	for _, r := range t.records {
		if (provider == "" || r.Provider == provider) && r.Timestamp.After(since) {
			input += r.InputTokens
			output += r.OutputTokens
		}
	}
	return input, output, nil
}

type PricingConfig struct {
	InputPricePer1k  float64
	OutputPricePer1k float64
}

var DefaultPricing = map[string]PricingConfig{
	"gemini-1.5-flash": {InputPricePer1k: 0.00001875, OutputPricePer1k: 0.000075},
	"gemini-1.5-pro":   {InputPricePer1k: 0.00125, OutputPricePer1k: 0.005},
	"gpt-4o":           {InputPricePer1k: 0.0025, OutputPricePer1k: 0.01},
	"gpt-4o-mini":      {InputPricePer1k: 0.00015, OutputPricePer1k: 0.0006},
}

func CalculateCost(provider, model string, inputTokens, outputTokens int) int {
	priceKey := model
	if pricing, ok := DefaultPricing[priceKey]; ok {
		inputCost := float64(inputTokens) / 1000 * pricing.InputPricePer1k
		outputCost := float64(outputTokens) / 1000 * pricing.OutputPricePer1k
		return int((inputCost + outputCost) * 10000)
	}
	return 0
}
