package store

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/aegis-wilwatikta/ai-reviewer/internal/domain"
)

type OptimizedNeo4jStore struct {
	baseStore     *Neo4jStore
	config        *Neo4jConfig
	cache         GraphCache
	metrics       *MetricsCollector
	fallbackStore *FallbackStore
	indexManager  *IndexManager
	batchQueue    *BatchQueue
	tierLimits    map[Tier]TierLimits
	rateLimiters  map[Tier]*RateLimiter
	mu            sync.RWMutex
}

type RateLimiter struct {
	mu         sync.Mutex
	tokens     int
	maxTokens  int
	refillRate time.Duration
	lastRefill time.Time
}

func NewRateLimiter(maxTokens int, refillRate time.Duration) *RateLimiter {
	return &RateLimiter{
		tokens:     maxTokens,
		maxTokens:  maxTokens,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

func (r *RateLimiter) Allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(r.lastRefill)
	tokensToAdd := int(elapsed / r.refillRate)

	if tokensToAdd > 0 {
		r.tokens = min(r.tokens+tokensToAdd, r.maxTokens)
		r.lastRefill = now
	}

	if r.tokens > 0 {
		r.tokens--
		return true
	}
	return false
}

func NewOptimizedNeo4jStore(uri, username, password, databaseName string, opts ...Neo4jConfigOption) (*OptimizedNeo4jStore, error) {
	config := DefaultNeo4jConfig()
	config.URI = uri
	config.Username = username
	config.Password = password
	config.DatabaseName = databaseName

	for _, opt := range opts {
		opt(config)
	}

	baseStore, err := NewNeo4jStore(uri, username, password, databaseName)
	if err != nil {
		return nil, fmt.Errorf("failed to create base neo4j store: %w", err)
	}

	store := &OptimizedNeo4jStore{
		baseStore:    baseStore,
		config:       config,
		tierLimits:   DefaultTierLimits,
		rateLimiters: make(map[Tier]*RateLimiter),
	}

	if config.EnableCache {
		store.cache = NewInMemoryGraphCache()
	}

	store.metrics = NewMetricsCollector(config.SlowQueryThreshold)

	if cache, ok := store.cache.(*InMemoryGraphCache); ok {
		cache.SetMetrics(store.metrics)
	}

	if config.FallbackEnabled {
		store.fallbackStore = NewFallbackStore(store.cache)
	}

	store.indexManager = NewIndexManager(baseStore, databaseName)

	if config.MaxConnectionPoolSize > 0 {
		store.batchQueue = NewBatchQueue(1000)
	}

	for tier, limits := range DefaultTierLimits {
		if limits.RateLimitPerMinute > 0 {
			store.rateLimiters[tier] = NewRateLimiter(
				limits.RateLimitPerMinute,
				time.Minute/time.Duration(limits.RateLimitPerMinute),
			)
		}
	}

	return store, nil
}

func (s *OptimizedNeo4jStore) Initialize(ctx context.Context) error {
	if s.indexManager != nil {
		if err := s.indexManager.CreateIndexes(ctx); err != nil {
			log.Printf("Warning: failed to create indexes: %v", err)
		}
	}
	return nil
}

func (s *OptimizedNeo4jStore) checkRateLimit(tier Tier) error {
	if limiter, ok := s.rateLimiters[tier]; ok {
		if !limiter.Allow() {
			return fmt.Errorf("rate limit exceeded for tier %s", tier)
		}
	}
	return nil
}

func (s *OptimizedNeo4jStore) executeWithFallback(ctx context.Context, op string, fn func() (interface{}, error)) (interface{}, error) {
	start := time.Now()
	result, err := fn()
	duration := time.Since(start)

	s.metrics.RecordQuery(op, duration, op, "", false, err)

	if err != nil {
		if s.fallbackStore != nil {
			log.Printf("Neo4j query failed, attempting fallback: %v", err)
			if fallbackResult, fallbackErr := s.fallbackStore.Get(ctx, op); fallbackErr == nil && fallbackResult != nil {
				return fallbackResult, nil
			}
		}
		return nil, err
	}

	return result, nil
}

func (s *OptimizedNeo4jStore) UpsertNode(ctx context.Context, node domain.CodeNode) error {
	return s.baseStore.UpsertNode(ctx, node)
}

func (s *OptimizedNeo4jStore) UpsertRelation(ctx context.Context, rel domain.CodeRelation) error {
	return s.baseStore.UpsertRelation(ctx, rel)
}

func (s *OptimizedNeo4jStore) GetImpactContext(ctx context.Context, projectID, filePath string) (*domain.ImpactReport, error) {
	cacheKey := GenerateCacheKey("ImpactContext", map[string]interface{}{
		"project_id": projectID,
		"file_path":  filePath,
	})

	if s.cache != nil {
		if cached, ok := s.cache.Get(ctx, cacheKey); ok {
			if report, ok := cached.(*domain.ImpactReport); ok {
				return report, nil
			}
		}
	}

	result, err := s.executeWithFallback(ctx, "GetImpactContext", func() (interface{}, error) {
		return s.baseStore.GetImpactContext(ctx, projectID, filePath)
	})

	if err != nil {
		return nil, err
	}

	report := result.(*domain.ImpactReport)

	if s.cache != nil && report != nil {
		_ = s.cache.Set(ctx, cacheKey, report, s.config.CacheTTL)
	}

	return report, nil
}

func (s *OptimizedNeo4jStore) QueryContext(ctx context.Context, projectID, filePath string, tier Tier) ([]domain.CodeNode, error) {
	if err := s.checkRateLimit(tier); err != nil {
		return nil, err
	}

	limits, ok := s.tierLimits[tier]
	if !ok {
		limits = DefaultTierLimits[TierPro]
	}

	cacheKey := GenerateCacheKey("QueryContext", map[string]interface{}{
		"project_id": projectID,
		"file_path":  filePath,
		"tier":       string(tier),
	})

	if s.cache != nil {
		if cached, ok := s.cache.Get(ctx, cacheKey); ok {
			if nodes, ok := cached.([]domain.CodeNode); ok {
				return nodes, nil
			}
		}
	}

	result, err := s.executeWithFallback(ctx, "QueryContext", func() (interface{}, error) {
		nodes, err := s.baseStore.QueryContext(ctx, projectID, filePath)
		if err != nil {
			return nil, err
		}
		if limits.MaxResultLimit > 0 && len(nodes) > limits.MaxResultLimit {
			nodes = nodes[:limits.MaxResultLimit]
		}
		return nodes, nil
	})

	if err != nil {
		return nil, err
	}

	nodes := result.([]domain.CodeNode)

	if s.cache != nil {
		_ = s.cache.Set(ctx, cacheKey, nodes, s.config.CacheTTL)
	}

	return nodes, nil
}

func (s *OptimizedNeo4jStore) FindRelatedByEmbedding(ctx context.Context, projectID string, embedding []float32, limit int, tier Tier) ([]domain.CodeNode, error) {
	if err := s.checkRateLimit(tier); err != nil {
		return nil, err
	}

	limits, ok := s.tierLimits[tier]
	if !ok {
		limits = DefaultTierLimits[TierPro]
	}

	effectiveLimit := limit
	if limits.MaxResultLimit > 0 && limit > limits.MaxResultLimit {
		effectiveLimit = limits.MaxResultLimit
	}

	cacheKey := GenerateCacheKey("FindRelatedByEmbedding", map[string]interface{}{
		"project_id":     projectID,
		"embedding_hash": fmt.Sprintf("%x", embedding[:min(8, len(embedding))]),
		"limit":          effectiveLimit,
	})

	if s.cache != nil {
		if cached, ok := s.cache.Get(ctx, cacheKey); ok {
			if nodes, ok := cached.([]domain.CodeNode); ok {
				return nodes, nil
			}
		}
	}

	result, err := s.executeWithFallback(ctx, "FindRelatedByEmbedding", func() (interface{}, error) {
		return s.baseStore.FindRelatedByEmbedding(ctx, projectID, embedding, effectiveLimit)
	})

	if err != nil {
		return nil, err
	}

	nodes := result.([]domain.CodeNode)

	if s.cache != nil {
		_ = s.cache.Set(ctx, cacheKey, nodes, s.config.CacheTTL)
	}

	return nodes, nil
}

func (s *OptimizedNeo4jStore) GetFileHash(ctx context.Context, projectID, path string) (string, error) {
	cacheKey := GenerateCacheKey("FileHash", map[string]interface{}{
		"project_id": projectID,
		"path":       path,
	})

	if s.cache != nil {
		if cached, ok := s.cache.Get(ctx, cacheKey); ok {
			if hash, ok := cached.(string); ok {
				return hash, nil
			}
		}
	}

	result, err := s.executeWithFallback(ctx, "GetFileHash", func() (interface{}, error) {
		return s.baseStore.GetFileHash(ctx, projectID, path)
	})

	if err != nil {
		return "", err
	}

	hash := result.(string)

	if s.cache != nil {
		_ = s.cache.Set(ctx, cacheKey, hash, s.config.CacheTTL)
	}

	return hash, nil
}

func (s *OptimizedNeo4jStore) DeleteNodesByFile(ctx context.Context, projectID, path string) error {
	err := s.baseStore.DeleteNodesByFile(ctx, projectID, path)
	if err != nil {
		return err
	}

	if s.cache != nil {
		prefix := GenerateProjectCachePrefix(projectID)
		_ = s.cache.DeleteByPrefix(ctx, prefix)
	}

	return nil
}

func (s *OptimizedNeo4jStore) DeleteNodesByProject(ctx context.Context, projectID string) error {
	err := s.baseStore.DeleteNodesByProject(ctx, projectID)
	if err != nil {
		return err
	}

	if s.cache != nil {
		prefix := GenerateProjectCachePrefix(projectID)
		_ = s.cache.DeleteByPrefix(ctx, prefix)
	}

	return nil
}

func (s *OptimizedNeo4jStore) BatchUpsertNodes(ctx context.Context, nodes []domain.CodeNode) error {
	return s.batchQueue.SubmitNodes(ctx, nodes, s.baseStore)
}

func (s *OptimizedNeo4jStore) BatchUpsertRelations(ctx context.Context, relations []domain.CodeRelation) error {
	return s.batchQueue.SubmitRelations(ctx, relations, s.baseStore)
}

func (s *OptimizedNeo4jStore) GetSlowQueries() []QueryMetric {
	return s.metrics.GetSlowQueries()
}

func (s *OptimizedNeo4jStore) GetCacheStats() CacheStats {
	if s.cache == nil {
		return CacheStats{}
	}
	return s.cache.GetStats()
}

func (s *OptimizedNeo4jStore) GetMetrics() MetricsStats {
	return s.metrics.GetStats()
}

func (s *OptimizedNeo4jStore) ClearCache(ctx context.Context) error {
	if s.cache == nil {
		return nil
	}
	return s.cache.Clear(ctx)
}

func (s *OptimizedNeo4jStore) Close(ctx context.Context) error {
	var errs []error

	if err := s.baseStore.Close(ctx); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing store: %v", errs)
	}
	return nil
}

func (s *OptimizedNeo4jStore) QueryContextWithTier(ctx context.Context, projectID, filePath string, tier Tier) ([]domain.CodeNode, error) {
	return s.QueryContext(ctx, projectID, filePath, tier)
}

func (s *OptimizedNeo4jStore) FindRelatedByEmbeddingWithTier(ctx context.Context, projectID string, embedding []float32, limit int, tier Tier) ([]domain.CodeNode, error) {
	return s.FindRelatedByEmbedding(ctx, projectID, embedding, limit, tier)
}
