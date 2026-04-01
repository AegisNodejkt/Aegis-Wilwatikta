package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"

	"github.com/aegis-wilwatikta/ai-reviewer/internal/domain"
)

type GraphCacheEntry struct {
	Key       string
	Result    interface{}
	Timestamp time.Time
	TTL       time.Duration
}

type GraphCache interface {
	Get(ctx context.Context, key string) (interface{}, bool)
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	DeleteByPrefix(ctx context.Context, prefix string) error
	Clear(ctx context.Context) error
	GetStats() CacheStats
}

type CacheStats struct {
	Hits        int64
	Misses      int64
	Size        int
	Evictions   int64
	LastResetAt time.Time
}

type InMemoryGraphCache struct {
	mu      sync.RWMutex
	store   map[string]*GraphCacheEntry
	ttls    map[string]time.Time
	stats   CacheStats
	metrics *MetricsCollector
}

func NewInMemoryGraphCache() *InMemoryGraphCache {
	return &InMemoryGraphCache{
		store: make(map[string]*GraphCacheEntry),
		ttls:  make(map[string]time.Time),
		stats: CacheStats{LastResetAt: time.Now()},
	}
}

func (c *InMemoryGraphCache) SetMetrics(m *MetricsCollector) {
	c.metrics = m
}

func (c *InMemoryGraphCache) Get(ctx context.Context, key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.store[key]
	if !ok {
		c.stats.Misses++
		if c.metrics != nil {
			c.metrics.IncrementCacheMiss()
		}
		return nil, false
	}

	if expiry, hasExpiry := c.ttls[key]; hasExpiry && time.Now().After(expiry) {
		c.mu.RUnlock()
		c.mu.Lock()
		delete(c.store, key)
		delete(c.ttls, key)
		c.stats.Evictions++
		c.mu.Unlock()
		c.mu.RLock()
		c.stats.Misses++
		if c.metrics != nil {
			c.metrics.IncrementCacheMiss()
		}
		return nil, false
	}

	c.stats.Hits++
	if c.metrics != nil {
		c.metrics.IncrementCacheHit()
	}
	return entry.Result, true
}

func (c *InMemoryGraphCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.store[key] = &GraphCacheEntry{
		Key:       key,
		Result:    value,
		Timestamp: time.Now(),
		TTL:       ttl,
	}

	if ttl > 0 {
		c.ttls[key] = time.Now().Add(ttl)
	}

	return nil
}

func (c *InMemoryGraphCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.store, key)
	delete(c.ttls, key)
	return nil
}

func (c *InMemoryGraphCache) DeleteByPrefix(ctx context.Context, prefix string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for key := range c.store {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			delete(c.store, key)
			delete(c.ttls, key)
		}
	}
	return nil
}

func (c *InMemoryGraphCache) Clear(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.store = make(map[string]*GraphCacheEntry)
	c.ttls = make(map[string]time.Time)
	c.stats.LastResetAt = time.Now()
	return nil
}

func (c *InMemoryGraphCache) GetStats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	stats := c.stats
	stats.Size = len(c.store)
	return stats
}

func GenerateCacheKey(operation string, params map[string]interface{}) string {
	data, _ := json.Marshal(params)
	h := sha256.New()
	h.Write([]byte(operation))
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

func GenerateProjectCachePrefix(projectID string) string {
	return "project:" + projectID + ":"
}

type CachedResult struct {
	Nodes         []domain.CodeNode
	ImpactReport  *domain.ImpactReport
	FileHash      string
	CachedAt      time.Time
	QueryDuration time.Duration
}
