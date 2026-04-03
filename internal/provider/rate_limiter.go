package provider

import (
	"context"
	"sync"
	"time"
)

type RateLimiter interface {
	Wait(ctx context.Context) error
	Allow() bool
	Reset()
}

type TokenBucketLimiter struct {
	tokens     int
	maxTokens  int
	refillRate int
	mu         sync.Mutex
	stopCh     chan struct{}
}

func NewTokenBucketLimiter(maxTokens, refillRate int) *TokenBucketLimiter {
	limiter := &TokenBucketLimiter{
		tokens:     maxTokens,
		maxTokens:  maxTokens,
		refillRate: refillRate,
		stopCh:     make(chan struct{}),
	}
	go limiter.refill()
	return limiter
}

func (l *TokenBucketLimiter) refill() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			l.mu.Lock()
			l.tokens = min(l.tokens+l.refillRate, l.maxTokens)
			l.mu.Unlock()
		case <-l.stopCh:
			return
		}
	}
}

func (l *TokenBucketLimiter) Wait(ctx context.Context) error {
	for {
		if l.Allow() {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func (l *TokenBucketLimiter) Allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.tokens > 0 {
		l.tokens--
		return true
	}
	return false
}

func (l *TokenBucketLimiter) Reset() {
	l.mu.Lock()
	l.tokens = l.maxTokens
	l.mu.Unlock()
}

func (l *TokenBucketLimiter) Stop() {
	close(l.stopCh)
}

type SlidingWindowLimiter struct {
	requests   []time.Time
	windowSize time.Duration
	maxReqs    int
	mu         sync.Mutex
}

func NewSlidingWindowLimiter(maxReqs int, windowSize time.Duration) *SlidingWindowLimiter {
	return &SlidingWindowLimiter{
		requests:   make([]time.Time, 0),
		windowSize: windowSize,
		maxReqs:    maxReqs,
	}
}

func (l *SlidingWindowLimiter) Wait(ctx context.Context) error {
	for {
		if l.Allow() {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func (l *SlidingWindowLimiter) Allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-l.windowSize)

	validIdx := 0
	for i, t := range l.requests {
		if t.After(windowStart) {
			validIdx = i
			break
		}
	}
	l.requests = l.requests[validIdx:]

	if len(l.requests) < l.maxReqs {
		l.requests = append(l.requests, now)
		return true
	}
	return false
}

func (l *SlidingWindowLimiter) Reset() {
	l.mu.Lock()
	l.requests = make([]time.Time, 0)
	l.mu.Unlock()
}

func NewGeminiRateLimiter() RateLimiter {
	return NewSlidingWindowLimiter(60, time.Minute)
}

func NewOpenAIRateLimiter() RateLimiter {
	return NewSlidingWindowLimiter(500, time.Minute)
}
