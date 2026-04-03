package provider

import (
	"context"
	"unicode"
)

type TokenCounter interface {
	Count(text string) int
	CountMessages(systemPrompt, userPrompt string) int
}

type SimpleTokenCounter struct{}

func NewSimpleTokenCounter() *SimpleTokenCounter {
	return &SimpleTokenCounter{}
}

func (c *SimpleTokenCounter) Count(text string) int {
	wordCount := 0
	inWord := false
	for _, r := range text {
		if unicode.IsSpace(r) || unicode.IsPunct(r) {
			if inWord {
				wordCount++
				inWord = false
			}
		} else {
			inWord = true
		}
	}
	if inWord {
		wordCount++
	}
	chars := len(text)
	return (chars + wordCount) / 4
}

func (c *SimpleTokenCounter) CountMessages(systemPrompt, userPrompt string) int {
	return c.Count(systemPrompt) + c.Count(userPrompt)
}

type BudgetConfig struct {
	MaxTokensPerRequest int
	MaxTokensPerDay     int
	MaxTokensPerMonth   int
}

type BudgetEnforcer interface {
	CanProceed(ctx context.Context, estimatedTokens int) (bool, error)
	RecordUsage(ctx context.Context, tokensUsed int) error
	GetRemainingBudget(ctx context.Context) (int, error)
}

type InMemoryBudgetEnforcer struct {
	config      BudgetConfig
	currentDay  int
	usedToday   int
	usedMonth   int
	usedRequest int
}

func NewInMemoryBudgetEnforcer(config BudgetConfig) *InMemoryBudgetEnforcer {
	return &InMemoryBudgetEnforcer{
		config: config,
	}
}

func (e *InMemoryBudgetEnforcer) CanProceed(ctx context.Context, estimatedTokens int) (bool, error) {
	if e.config.MaxTokensPerRequest > 0 && estimatedTokens > e.config.MaxTokensPerRequest {
		return false, nil
	}
	if e.config.MaxTokensPerDay > 0 && e.usedToday+estimatedTokens > e.config.MaxTokensPerDay {
		return false, nil
	}
	if e.config.MaxTokensPerMonth > 0 && e.usedMonth+estimatedTokens > e.config.MaxTokensPerMonth {
		return false, nil
	}
	return true, nil
}

func (e *InMemoryBudgetEnforcer) RecordUsage(ctx context.Context, tokensUsed int) error {
	e.usedToday += tokensUsed
	e.usedMonth += tokensUsed
	e.usedRequest += tokensUsed
	return nil
}

func (e *InMemoryBudgetEnforcer) GetRemainingBudget(ctx context.Context) (int, error) {
	remainingDay := e.config.MaxTokensPerDay - e.usedToday
	remainingMonth := e.config.MaxTokensPerMonth - e.usedMonth

	if e.config.MaxTokensPerDay > 0 && e.config.MaxTokensPerMonth > 0 {
		if remainingDay < remainingMonth {
			return remainingDay, nil
		}
		return remainingMonth, nil
	}
	if e.config.MaxTokensPerDay > 0 {
		return remainingDay, nil
	}
	if e.config.MaxTokensPerMonth > 0 {
		return remainingMonth, nil
	}
	return -1, nil
}
