package store

import (
	"testing"
	"time"
)

func TestDefaultNeo4jConfig(t *testing.T) {
	config := DefaultNeo4jConfig()

	if config.MaxConnectionPoolSize != 50 {
		t.Errorf("Expected MaxConnectionPoolSize 50, got %d", config.MaxConnectionPoolSize)
	}
	if config.SlowQueryThreshold != 100*time.Millisecond {
		t.Errorf("Expected SlowQueryThreshold 100ms, got %v", config.SlowQueryThreshold)
	}
	if !config.EnableCache {
		t.Error("Expected EnableCache to be true by default")
	}
	if config.CacheTTL != 5*time.Minute {
		t.Errorf("Expected CacheTTL 5m, got %v", config.CacheTTL)
	}
	if !config.FallbackEnabled {
		t.Error("Expected FallbackEnabled to be true by default")
	}
}

func TestNeo4jConfigOptions(t *testing.T) {
	tests := []struct {
		name  string
		opts  []Neo4jConfigOption
		check func(*testing.T, *Neo4jConfig)
	}{
		{
			name: "WithConnectionPoolSize",
			opts: []Neo4jConfigOption{WithConnectionPoolSize(100)},
			check: func(t *testing.T, c *Neo4jConfig) {
				if c.MaxConnectionPoolSize != 100 {
					t.Errorf("Expected pool size 100, got %d", c.MaxConnectionPoolSize)
				}
			},
		},
		{
			name: "WithSlowQueryThreshold",
			opts: []Neo4jConfigOption{WithSlowQueryThreshold(200 * time.Millisecond)},
			check: func(t *testing.T, c *Neo4jConfig) {
				if c.SlowQueryThreshold != 200*time.Millisecond {
					t.Errorf("Expected threshold 200ms, got %v", c.SlowQueryThreshold)
				}
			},
		},
		{
			name: "WithCache enabled",
			opts: []Neo4jConfigOption{WithCache(true, 10*time.Minute)},
			check: func(t *testing.T, c *Neo4jConfig) {
				if !c.EnableCache {
					t.Error("Expected EnableCache true")
				}
				if c.CacheTTL != 10*time.Minute {
					t.Errorf("Expected TTL 10m, got %v", c.CacheTTL)
				}
			},
		},
		{
			name: "WithCache disabled",
			opts: []Neo4jConfigOption{WithCache(false, 0)},
			check: func(t *testing.T, c *Neo4jConfig) {
				if c.EnableCache {
					t.Error("Expected EnableCache false")
				}
			},
		},
		{
			name: "WithFallback disabled",
			opts: []Neo4jConfigOption{WithFallback(false)},
			check: func(t *testing.T, c *Neo4jConfig) {
				if c.FallbackEnabled {
					t.Error("Expected FallbackEnabled false")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultNeo4jConfig()
			for _, opt := range tt.opts {
				opt(config)
			}
			tt.check(t, config)
		})
	}
}

func TestDefaultTierLimits(t *testing.T) {
	communityLimits, ok := DefaultTierLimits[TierCommunity]
	if !ok {
		t.Fatal("Expected Community tier limits")
	}
	if communityLimits.MaxQueryDepth != 2 {
		t.Errorf("Expected Community MaxQueryDepth 2, got %d", communityLimits.MaxQueryDepth)
	}
	if communityLimits.MaxResultLimit != 50 {
		t.Errorf("Expected Community MaxResultLimit 50, got %d", communityLimits.MaxResultLimit)
	}

	proLimits, ok := DefaultTierLimits[TierPro]
	if !ok {
		t.Fatal("Expected Pro tier limits")
	}
	customLimits, ok := DefaultTierLimits[TierCustom]
	if !ok {
		t.Fatal("Expected Custom tier limits")
	}

	if proLimits.MaxQueryDepth <= communityLimits.MaxQueryDepth {
		t.Error("Expected Pro tier to have higher query depth than Community")
	}

	if customLimits.MaxQueryDepth != -1 {
		t.Errorf("Expected Custom tier unlimited depth (-1), got %d", customLimits.MaxQueryDepth)
	}
}

func TestRateLimiter(t *testing.T) {
	limiter := NewRateLimiter(2, time.Minute)

	if !limiter.Allow() {
		t.Error("Expected first request to be allowed")
	}
	if !limiter.Allow() {
		t.Error("Expected second request to be allowed")
	}
	if limiter.Allow() {
		t.Error("Expected third request to be rate limited")
	}
}

func TestRateLimiter_Refill(t *testing.T) {
	limiter := NewRateLimiter(1, 10*time.Millisecond)

	if !limiter.Allow() {
		t.Error("Expected first request to be allowed")
	}

	if limiter.Allow() {
		t.Error("Expected immediate second request to be rate limited")
	}

	time.Sleep(20 * time.Millisecond)

	if !limiter.Allow() {
		t.Error("Expected request after refill to be allowed")
	}
}
