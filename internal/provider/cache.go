package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"
)

type CacheEntry struct {
	Response   string
	TokensUsed int
	CachedAt   time.Time
	Model      string
	Provider   string
}

type Cache interface {
	Get(ctx context.Context, key string) (*CacheEntry, error)
	Set(ctx context.Context, key string, entry *CacheEntry, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Clear(ctx context.Context) error
}

type InMemoryCache struct {
	store map[string]*CacheEntry
	ttls  map[string]time.Time
}

func NewInMemoryCache() *InMemoryCache {
	return &InMemoryCache{
		store: make(map[string]*CacheEntry),
		ttls:  make(map[string]time.Time),
	}
}

func (c *InMemoryCache) Get(ctx context.Context, key string) (*CacheEntry, error) {
	entry, ok := c.store[key]
	if !ok {
		return nil, nil
	}
	if expiry, hasExpiry := c.ttls[key]; hasExpiry && time.Now().After(expiry) {
		delete(c.store, key)
		delete(c.ttls, key)
		return nil, nil
	}
	return entry, nil
}

func (c *InMemoryCache) Set(ctx context.Context, key string, entry *CacheEntry, ttl time.Duration) error {
	c.store[key] = entry
	if ttl > 0 {
		c.ttls[key] = time.Now().Add(ttl)
	}
	return nil
}

func (c *InMemoryCache) Delete(ctx context.Context, key string) error {
	delete(c.store, key)
	delete(c.ttls, key)
	return nil
}

func (c *InMemoryCache) Clear(ctx context.Context) error {
	c.store = make(map[string]*CacheEntry)
	c.ttls = make(map[string]time.Time)
	return nil
}

func GenerateCacheKey(systemPrompt, userPrompt, model string) string {
	h := sha256.New()
	h.Write([]byte(systemPrompt))
	h.Write([]byte(userPrompt))
	h.Write([]byte(model))
	return hex.EncodeToString(h.Sum(nil))
}
