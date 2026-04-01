package store

import (
	"time"
)

type Tier string

const (
	TierCommunity Tier = "community"
	TierPro       Tier = "pro"
	TierCustom    Tier = "custom"
)

type TierLimits struct {
	MaxQueryDepth      int
	MaxResultLimit     int
	AllowedQueryTypes  []string
	RateLimitPerMinute int
}

var DefaultTierLimits = map[Tier]TierLimits{
	TierCommunity: {
		MaxQueryDepth:      2,
		MaxResultLimit:     50,
		AllowedQueryTypes:  []string{"simple", "lookup"},
		RateLimitPerMinute: 30,
	},
	TierPro: {
		MaxQueryDepth:      4,
		MaxResultLimit:     200,
		AllowedQueryTypes:  []string{"simple", "lookup", "complex", "traversal"},
		RateLimitPerMinute: 100,
	},
	TierCustom: {
		MaxQueryDepth:      -1,
		MaxResultLimit:     -1,
		AllowedQueryTypes:  []string{"simple", "lookup", "complex", "traversal", "analytics"},
		RateLimitPerMinute: -1,
	},
}

type Neo4jConfig struct {
	URI                          string
	Username                     string
	Password                     string
	DatabaseName                 string
	MaxConnectionPoolSize        int
	ConnectionAcquisitionTimeout time.Duration
	MaxTransactionRetryTime      time.Duration
	SlowQueryThreshold           time.Duration
	EnableCache                  bool
	CacheTTL                     time.Duration
	FallbackEnabled              bool
}

func DefaultNeo4jConfig() *Neo4jConfig {
	return &Neo4jConfig{
		MaxConnectionPoolSize:        50,
		ConnectionAcquisitionTimeout: 30 * time.Second,
		MaxTransactionRetryTime:      30 * time.Second,
		SlowQueryThreshold:           100 * time.Millisecond,
		EnableCache:                  true,
		CacheTTL:                     5 * time.Minute,
		FallbackEnabled:              true,
	}
}

type Neo4jConfigOption func(*Neo4jConfig)

func WithConnectionPoolSize(size int) Neo4jConfigOption {
	return func(c *Neo4jConfig) {
		c.MaxConnectionPoolSize = size
	}
}

func WithSlowQueryThreshold(threshold time.Duration) Neo4jConfigOption {
	return func(c *Neo4jConfig) {
		c.SlowQueryThreshold = threshold
	}
}

func WithCache(enabled bool, ttl time.Duration) Neo4jConfigOption {
	return func(c *Neo4jConfig) {
		c.EnableCache = enabled
		c.CacheTTL = ttl
	}
}

func WithFallback(enabled bool) Neo4jConfigOption {
	return func(c *Neo4jConfig) {
		c.FallbackEnabled = enabled
	}
}
