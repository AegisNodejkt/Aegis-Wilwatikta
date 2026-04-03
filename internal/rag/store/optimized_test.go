package store

import (
	"context"
	"testing"
	"time"

	"github.com/aegis-wilwatikta/ai-reviewer/internal/domain"
)

type MockGraphStore struct {
	Nodes     map[string]domain.CodeNode
	Relations []domain.CodeRelation
	CallCount int
}

func NewMockGraphStore() *MockGraphStore {
	return &MockGraphStore{
		Nodes:     make(map[string]domain.CodeNode),
		Relations: make([]domain.CodeRelation, 0),
	}
}

func (m *MockGraphStore) UpsertNode(ctx context.Context, node domain.CodeNode) error {
	m.CallCount++
	m.Nodes[node.ID] = node
	return nil
}

func (m *MockGraphStore) UpsertRelation(ctx context.Context, rel domain.CodeRelation) error {
	m.CallCount++
	m.Relations = append(m.Relations, rel)
	return nil
}

func (m *MockGraphStore) GetImpactContext(ctx context.Context, projectID, filePath string) (*domain.ImpactReport, error) {
	m.CallCount++
	return &domain.ImpactReport{
		TargetNode:       domain.CodeNode{ID: "test", Path: filePath},
		AffectedNodes:    []domain.AffectedNode{},
		BlastRadiusScore: 5,
	}, nil
}

func (m *MockGraphStore) QueryContext(ctx context.Context, projectID, filePath string) ([]domain.CodeNode, error) {
	m.CallCount++
	return []domain.CodeNode{{ID: "test", Path: filePath}}, nil
}

func (m *MockGraphStore) FindRelatedByEmbedding(ctx context.Context, projectID string, embedding []float32, limit int) ([]domain.CodeNode, error) {
	m.CallCount++
	return []domain.CodeNode{{ID: "related", Name: "RelatedFunc"}}, nil
}

func (m *MockGraphStore) GetFileHash(ctx context.Context, projectID, path string) (string, error) {
	m.CallCount++
	return "hash123", nil
}

func (m *MockGraphStore) DeleteNodesByFile(ctx context.Context, projectID, path string) error {
	m.CallCount++
	return nil
}

func (m *MockGraphStore) DeleteNodesByProject(ctx context.Context, projectID string) error {
	m.CallCount++
	return nil
}

func TestRateLimiter_TierIntegration(t *testing.T) {
	communityLimiter := NewRateLimiter(5, time.Minute)
	proLimiter := NewRateLimiter(20, time.Minute)

	communityLimits := DefaultTierLimits[TierCommunity]
	proLimits := DefaultTierLimits[TierPro]

	if communityLimits.RateLimitPerMinute != 30 {
		t.Errorf("Expected Community rate limit 30, got %d", communityLimits.RateLimitPerMinute)
	}
	if proLimits.RateLimitPerMinute != 100 {
		t.Errorf("Expected Pro rate limit 100, got %d", proLimits.RateLimitPerMinute)
	}

	for i := 0; i < 5; i++ {
		if !communityLimiter.Allow() {
			t.Errorf("Expected request %d to be allowed for community tier", i)
		}
	}
	if communityLimiter.Allow() {
		t.Error("Expected community tier to be rate limited after 5 requests")
	}

	for i := 0; i < 20; i++ {
		if !proLimiter.Allow() {
			t.Errorf("Expected request %d to be allowed for pro tier", i)
		}
	}
	if proLimiter.Allow() {
		t.Error("Expected pro tier to be rate limited after 20 requests")
	}
}

func TestCacheIntegration(t *testing.T) {
	cache := NewInMemoryGraphCache()
	ctx := context.Background()

	testNodes := []domain.CodeNode{
		{ID: "node1", Name: "Func1", Kind: domain.KindFunction, Path: "/path1.go"},
		{ID: "node2", Name: "Func2", Kind: domain.KindFunction, Path: "/path2.go"},
	}

	key1 := GenerateCacheKey("QueryContext", map[string]interface{}{
		"project_id": "proj1",
		"file_path":  "/path1.go",
	})

	err := cache.Set(ctx, key1, testNodes, 5*time.Minute)
	if err != nil {
		t.Fatalf("Failed to set cache: %v", err)
	}

	retrieved, ok := cache.Get(ctx, key1)
	if !ok {
		t.Fatal("Expected to find cached value")
	}

	cachedNodes, ok := retrieved.([]domain.CodeNode)
	if !ok {
		t.Fatal("Expected []domain.CodeNode type")
	}

	if len(cachedNodes) != 2 {
		t.Errorf("Expected 2 nodes, got %d", len(cachedNodes))
	}

	key2 := GenerateCacheKey("QueryContext", map[string]interface{}{
		"project_id": "proj1",
		"file_path":  "/path2.go",
	})

	_, ok = cache.Get(ctx, key2)
	if ok {
		t.Error("Expected cache miss for different key")
	}
}

func TestFallbackStore(t *testing.T) {
	cache := NewInMemoryGraphCache()
	fallback := NewFallbackStore(cache)
	ctx := context.Background()

	report := &domain.ImpactReport{
		TargetNode:       domain.CodeNode{ID: "test"},
		BlastRadiusScore: 10,
	}

	err := fallback.StoreImpactReport(ctx, "proj1", "/path/file.go", report)
	if err != nil {
		t.Fatalf("Failed to store impact report: %v", err)
	}

	retrieved, err := fallback.Get(ctx, GenerateCacheKey("ImpactContext", map[string]interface{}{
		"project_id": "proj1",
		"file_path":  "/path/file.go",
	}))
	if err != nil {
		t.Fatalf("Failed to get from fallback: %v", err)
	}

	if retrieved == nil {
		t.Error("Expected non-nil result from fallback")
	}
}

func TestBatchQueue_Basic(t *testing.T) {
	batchQueue := NewBatchQueue(100)

	if batchQueue == nil {
		t.Fatal("Expected non-nil BatchQueue")
	}

	if batchQueue.batchSize != 100 {
		t.Errorf("Expected batch size 100, got %d", batchQueue.batchSize)
	}
}

func TestIndexManager_RequiredIndexes(t *testing.T) {
	if len(RequiredIndexes) == 0 {
		t.Error("Expected at least one required index")
	}

	requiredIndexNames := make(map[string]bool)
	for _, idx := range RequiredIndexes {
		requiredIndexNames[idx.Name] = true
	}

	requiredProperties := []string{"id", "project_id", "path", "name", "kind", "signature_hash", "content_hash"}
	for _, prop := range requiredProperties {
		found := false
		for _, idx := range RequiredIndexes {
			if idx.Property == prop {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected index on property %s", prop)
		}
	}
}

func TestTierLimits(t *testing.T) {
	tests := []struct {
		tier          Tier
		expectedDepth int
		expectedLimit int
		unlimited     bool
	}{
		{TierCommunity, 2, 50, false},
		{TierPro, 4, 200, false},
		{TierCustom, -1, -1, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.tier), func(t *testing.T) {
			limits, ok := DefaultTierLimits[tt.tier]
			if !ok {
				t.Fatalf("Expected tier limits for %s", tt.tier)
			}

			if tt.unlimited {
				if limits.MaxQueryDepth != -1 {
					t.Errorf("Expected unlimited depth for %s, got %d", tt.tier, limits.MaxQueryDepth)
				}
				if limits.MaxResultLimit != -1 {
					t.Errorf("Expected unlimited results for %s, got %d", tt.tier, limits.MaxResultLimit)
				}
			} else {
				if limits.MaxQueryDepth != tt.expectedDepth {
					t.Errorf("Expected depth %d for %s, got %d", tt.expectedDepth, tt.tier, limits.MaxQueryDepth)
				}
				if limits.MaxResultLimit != tt.expectedLimit {
					t.Errorf("Expected limit %d for %s, got %d", tt.expectedLimit, tt.tier, limits.MaxResultLimit)
				}
			}
		})
	}
}
