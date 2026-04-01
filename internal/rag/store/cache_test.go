package store

import (
	"context"
	"testing"
	"time"

	"github.com/aegis-wilwatikta/ai-reviewer/internal/domain"
)

func TestInMemoryGraphCache_GetSet(t *testing.T) {
	cache := NewInMemoryGraphCache()
	ctx := context.Background()

	key := "test-key"
	value := &domain.CodeNode{
		ID:   "test-id",
		Name: "TestFunction",
		Kind: domain.KindFunction,
		Path: "/path/to/file.go",
	}

	err := cache.Set(ctx, key, value, 5*time.Minute)
	if err != nil {
		t.Fatalf("Failed to set cache: %v", err)
	}

	retrieved, ok := cache.Get(ctx, key)
	if !ok {
		t.Fatal("Expected to find cached value")
	}

	cachedNode, ok := retrieved.(*domain.CodeNode)
	if !ok {
		t.Fatal("Expected CodeNode type")
	}

	if cachedNode.ID != value.ID {
		t.Errorf("Expected ID %s, got %s", value.ID, cachedNode.ID)
	}
}

func TestInMemoryGraphCache_Delete(t *testing.T) {
	cache := NewInMemoryGraphCache()
	ctx := context.Background()

	key := "test-key"
	value := &domain.CodeNode{ID: "test-id"}

	_ = cache.Set(ctx, key, value, 5*time.Minute)

	_, ok := cache.Get(ctx, key)
	if !ok {
		t.Fatal("Expected to find cached value before delete")
	}

	_ = cache.Delete(ctx, key)

	_, ok = cache.Get(ctx, key)
	if ok {
		t.Fatal("Expected cached value to be deleted")
	}
}

func TestInMemoryGraphCache_DeleteByPrefix(t *testing.T) {
	cache := NewInMemoryGraphCache()
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		key := "project:test:" + string(rune('a'+i))
		_ = cache.Set(ctx, key, &domain.CodeNode{ID: key}, 5*time.Minute)
	}

	_ = cache.Set(ctx, "other:key", &domain.CodeNode{ID: "other"}, 5*time.Minute)

	_ = cache.DeleteByPrefix(ctx, "project:test:")

	stats := cache.GetStats()
	if stats.Size != 1 {
		t.Errorf("Expected 1 item in cache, got %d", stats.Size)
	}
}

func TestInMemoryGraphCache_TTL(t *testing.T) {
	cache := NewInMemoryGraphCache()
	ctx := context.Background()

	key := "test-key"
	value := &domain.CodeNode{ID: "test-id"}

	_ = cache.Set(ctx, key, value, 50*time.Millisecond)

	_, ok := cache.Get(ctx, key)
	if !ok {
		t.Fatal("Expected to find cached value before expiry")
	}

	time.Sleep(100 * time.Millisecond)

	_, ok = cache.Get(ctx, key)
	if ok {
		t.Fatal("Expected cached value to be expired")
	}
}

func TestGenerateCacheKey(t *testing.T) {
	key1 := GenerateCacheKey("GetImpactContext", map[string]interface{}{
		"project_id": "proj1",
		"file_path":  "/path/to/file.go",
	})

	key2 := GenerateCacheKey("GetImpactContext", map[string]interface{}{
		"project_id": "proj1",
		"file_path":  "/path/to/file.go",
	})

	key3 := GenerateCacheKey("GetImpactContext", map[string]interface{}{
		"project_id": "proj2",
		"file_path":  "/path/to/file.go",
	})

	if key1 != key2 {
		t.Error("Expected same keys for same parameters")
	}

	if key1 == key3 {
		t.Error("Expected different keys for different parameters")
	}
}

func TestCacheStats(t *testing.T) {
	cache := NewInMemoryGraphCache()
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		_ = cache.Set(ctx, "key"+string(rune('a'+i)), &domain.CodeNode{ID: string(rune('a' + i))}, 5*time.Minute)
	}

	for i := 0; i < 3; i++ {
		_, _ = cache.Get(ctx, "key"+string(rune('a'+i)))
	}

	_, _ = cache.Get(ctx, "nonexistent")

	stats := cache.GetStats()
	if stats.Hits != 3 {
		t.Errorf("Expected 3 cache hits, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("Expected 1 cache miss, got %d", stats.Misses)
	}
	if stats.Size != 3 {
		t.Errorf("Expected cache size 3, got %d", stats.Size)
	}
}

func TestNewMemoryGraphCache(t *testing.T) {
	cache := NewInMemoryGraphCache()
	if cache == nil {
		t.Fatal("Expected non-nil cache")
	}
}
