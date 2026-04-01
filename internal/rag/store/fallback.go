package store

import (
	"context"
	"fmt"
	"log"
	"sync"
)

type FallbackStore struct {
	cache      GraphCache
	persistent map[string]interface{}
	mu         sync.RWMutex
}

func NewFallbackStore(cache GraphCache) *FallbackStore {
	return &FallbackStore{
		cache:      cache,
		persistent: make(map[string]interface{}),
	}
}

func (f *FallbackStore) Get(ctx context.Context, key string) (interface{}, error) {
	if f.cache != nil {
		if cached, ok := f.cache.Get(ctx, key); ok {
			return cached, nil
		}
	}

	f.mu.RLock()
	defer f.mu.RUnlock()

	if val, ok := f.persistent[key]; ok {
		return val, nil
	}

	return nil, fmt.Errorf("fallback: key not found: %s", key)
}

func (f *FallbackStore) Set(ctx context.Context, key string, value interface{}) error {
	f.mu.Lock()
	f.persistent[key] = value
	f.mu.Unlock()

	if f.cache != nil {
		_ = f.cache.Set(ctx, key, value, 0)
	}
	return nil
}

func (f *FallbackStore) StoreResult(ctx context.Context, key string, result interface{}) error {
	log.Printf("FallbackStore: storing result for key %s", key)
	return f.Set(ctx, key, result)
}

func (f *FallbackStore) Clear(ctx context.Context) error {
	f.mu.Lock()
	f.persistent = make(map[string]interface{})
	f.mu.Unlock()

	if f.cache != nil {
		_ = f.cache.Clear(ctx)
	}
	return nil
}

func (f *FallbackStore) StoreImpactReport(ctx context.Context, projectID, filePath string, report interface{}) error {
	key := GenerateCacheKey("ImpactContext", map[string]interface{}{
		"project_id": projectID,
		"file_path":  filePath,
	})
	return f.Set(ctx, key, report)
}

func (f *FallbackStore) StoreCodeNodes(ctx context.Context, key string, nodes []interface{}) error {
	return f.Set(ctx, key, nodes)
}
