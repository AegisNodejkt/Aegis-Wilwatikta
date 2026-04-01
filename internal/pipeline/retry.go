package pipeline

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"
)

var (
	ErrMaxRetriesExceeded = errors.New("max retries exceeded")
)

type RetryableError struct {
	Err error
}

func (e *RetryableError) Error() string {
	return e.Err.Error()
}

func IsRetryableError(err error, patterns []string) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	for _, pattern := range patterns {
		if strings.Contains(strings.ToLower(errStr), strings.ToLower(pattern)) {
			return true
		}
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "429") || strings.Contains(errStr, "rate limit") {
		return true
	}

	return false
}

type Retrier struct {
	config RetryConfig
}

func NewRetrier(config RetryConfig) *Retrier {
	if config.BaseWait == 0 {
		config.BaseWait = time.Second
	}
	if config.MaxWait == 0 {
		config.MaxWait = time.Minute
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	return &Retrier{config: config}
}

func (r *Retrier) WithBackoff(ctx context.Context, fn func() error) error {
	var lastErr error

	for attempt := 0; attempt <= r.config.MaxRetries; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		if attempt == r.config.MaxRetries {
			break
		}

		isRetryable := IsRetryableError(err, r.config.RetryableErrors)
		if !isRetryable {
			return err
		}

		wait := r.calculateBackoff(attempt)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}

	if lastErr != nil {
		return fmt.Errorf("%w: %v", ErrMaxRetriesExceeded, lastErr)
	}
	return ErrMaxRetriesExceeded
}

func (r *Retrier) calculateBackoff(attempt int) time.Duration {
	base := r.config.BaseWait
	max := r.config.MaxWait

	// Exponential backoff: base * 2^attempt
	wait := base * time.Duration(1<<uint(attempt))

	// Add jitter: random between 0-25% of wait time
	jitter := time.Duration(rand.Int63n(int64(wait / 4)))
	wait = wait + jitter

	if wait > max {
		wait = max
	}

	return wait
}

func DefaultRetryableErrors() []string {
	return []string{
		"timeout",
		"429",
		"rate limit",
		"temporary",
		"unavailable",
		"deadline exceeded",
		"context deadline",
		"connection reset",
		"connection refused",
		"too many requests",
	}
}
