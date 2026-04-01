package provider

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var (
	ErrAllProvidersFailed  = errors.New("all providers failed")
	ErrRateLimitExceeded   = errors.New("rate limit exceeded")
	ErrBudgetExceeded      = errors.New("budget exceeded")
	ErrNoProviderAvailable = errors.New("no provider available")
)

type ProviderConfig struct {
	Name      string
	Model     string
	IsPrimary bool
	Priority  int
	RateLimit int
}

type LLMRequest struct {
	SystemPrompt string
	UserPrompt   string
	Model        string
	ReqID        string
	Metadata     map[string]string
}

type LLMResponse struct {
	Content      string
	Provider     string
	Model        string
	InputTokens  int
	OutputTokens int
	CostCents    int
	CacheHit     bool
	LatencyMs    int64
}

type ProviderWithLimiter struct {
	Provider    AIProvider
	Limiter     RateLimiter
	Config      ProviderConfig
	IsAvailable bool
}

type ProviderManager struct {
	providers      []*ProviderWithLimiter
	cache          Cache
	costTracker    CostTracker
	budgetEnforcer BudgetEnforcer
	tokenCounter   TokenCounter
	fallbackOrder  []int
}

func NewProviderManager(
	providers []*ProviderWithLimiter,
	cache Cache,
	costTracker CostTracker,
	budgetEnforcer BudgetEnforcer,
) *ProviderManager {
	pm := &ProviderManager{
		providers:      providers,
		cache:          cache,
		costTracker:    costTracker,
		budgetEnforcer: budgetEnforcer,
		tokenCounter:   NewSimpleTokenCounter(),
	}

	pm.fallbackOrder = make([]int, len(providers))
	for i := range providers {
		pm.fallbackOrder[i] = i
		for j := 0; j < i; j++ {
			if providers[i].Config.Priority < providers[pm.fallbackOrder[j]].Config.Priority {
				pm.fallbackOrder[i], pm.fallbackOrder[j] = pm.fallbackOrder[j], pm.fallbackOrder[i]
			}
		}
	}

	if cache == nil {
		pm.cache = NewInMemoryCache()
	}
	if costTracker == nil {
		pm.costTracker = NewInMemoryCostTracker()
	}
	if budgetEnforcer == nil {
		pm.budgetEnforcer = NewInMemoryBudgetEnforcer(BudgetConfig{
			MaxTokensPerRequest: 100000,
			MaxTokensPerDay:     1000000,
			MaxTokensPerMonth:   10000000,
		})
	}

	return pm
}

func (pm *ProviderManager) SendMessage(ctx context.Context, systemPrompt, userPrompt, model string) (string, error) {
	resp, err := pm.SendMessageFull(ctx, LLMRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Model:        model,
	})
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

func (pm *ProviderManager) SendMessageFull(ctx context.Context, req LLMRequest) (*LLMResponse, error) {
	var lastErr error

	estimatedTokens := pm.tokenCounter.CountMessages(req.SystemPrompt, req.UserPrompt)
	canProceed, err := pm.budgetEnforcer.CanProceed(ctx, estimatedTokens)
	if err != nil {
		return nil, fmt.Errorf("budget check failed: %w", err)
	}
	if !canProceed {
		return nil, ErrBudgetExceeded
	}

	cacheKey := GenerateCacheKey(req.SystemPrompt, req.UserPrompt, req.Model)
	if pm.cache != nil {
		cached, err := pm.cache.Get(ctx, cacheKey)
		if err == nil && cached != nil {
			return &LLMResponse{
				Content:  cached.Response,
				Provider: cached.Provider,
				Model:    cached.Model,
				CacheHit: true,
			}, nil
		}
	}

	for _, idx := range pm.fallbackOrder {
		pw := pm.providers[idx]
		if !pw.IsAvailable {
			continue
		}

		if pw.Limiter != nil {
			if !pw.Limiter.Allow() {
				lastErr = ErrRateLimitExceeded
				continue
			}
		}

		start := time.Now()
		content, err := pw.Provider.SendMessage(ctx, req.SystemPrompt, req.UserPrompt, req.Model)
		latency := time.Since(start).Milliseconds()

		if err != nil {
			pw.IsAvailable = false
			lastErr = err
			continue
		}

		outputTokens := pm.tokenCounter.Count(content)
		costCents := CalculateCost(pw.Config.Name, req.Model, estimatedTokens, outputTokens)

		costRecord := CostRecord{
			Provider:     pw.Config.Name,
			Model:        req.Model,
			Timestamp:    time.Now(),
			InputTokens:  estimatedTokens,
			OutputTokens: outputTokens,
			CostCents:    costCents,
			PromptHash:   cacheKey[:16],
			ReqID:        req.ReqID,
			CacheHit:     false,
		}
		pm.costTracker.Record(ctx, costRecord)

		pm.budgetEnforcer.RecordUsage(ctx, estimatedTokens+outputTokens)

		if pm.cache != nil {
			pm.cache.Set(ctx, cacheKey, &CacheEntry{
				Response:   content,
				TokensUsed: estimatedTokens + outputTokens,
				CachedAt:   time.Now(),
				Model:      req.Model,
				Provider:   pw.Config.Name,
			}, time.Hour)
		}

		return &LLMResponse{
			Content:      content,
			Provider:     pw.Config.Name,
			Model:        req.Model,
			InputTokens:  estimatedTokens,
			OutputTokens: outputTokens,
			CostCents:    costCents,
			CacheHit:     false,
			LatencyMs:    latency,
		}, nil
	}

	if lastErr != nil {
		return nil, fmt.Errorf("%w: %v", ErrAllProvidersFailed, lastErr)
	}
	return nil, ErrNoProviderAvailable
}

func (pm *ProviderManager) GetStats() map[string]interface{} {
	stats := make(map[string]interface{})
	for _, pw := range pm.providers {
		stats[pw.Config.Name] = map[string]interface{}{
			"model":       pw.Config.Model,
			"isPrimary":   pw.Config.IsPrimary,
			"priority":    pw.Config.Priority,
			"isAvailable": pw.IsAvailable,
		}
	}
	return stats
}

func (pm *ProviderManager) MarkProviderAvailable(name string, available bool) {
	for _, pw := range pm.providers {
		if pw.Config.Name == name {
			pw.IsAvailable = available
			return
		}
	}
}

func (pm *ProviderManager) Name() string {
	return "provider_manager"
}
