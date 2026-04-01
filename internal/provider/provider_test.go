package provider

import (
	"context"
	"errors"
	"testing"
	"time"
)

type mockProvider struct {
	name      string
	response  string
	err       error
	callCount int
}

func (m *mockProvider) SendMessage(ctx context.Context, systemPrompt, userPrompt, model string) (string, error) {
	m.callCount++
	if m.err != nil {
		return "", m.err
	}
	return m.response, nil
}

func (m *mockProvider) Name() string {
	return m.name
}

func TestRateLimiter_Allow(t *testing.T) {
	limiter := NewTokenBucketLimiter(2, 1)
	defer limiter.Stop()

	if !limiter.Allow() {
		t.Error("Expected first request to be allowed")
	}
	if !limiter.Allow() {
		t.Error("Expected second request to be allowed")
	}
	if limiter.Allow() {
		t.Error("Expected third request to be denied")
	}
}

func TestSlidingWindowLimiter(t *testing.T) {
	limiter := NewSlidingWindowLimiter(2, time.Minute)

	if !limiter.Allow() {
		t.Error("Expected first request to be allowed")
	}
	if !limiter.Allow() {
		t.Error("Expected second request to be allowed")
	}
	if limiter.Allow() {
		t.Error("Expected third request to be denied")
	}
}

func TestInMemoryCache_GetSet(t *testing.T) {
	cache := NewInMemoryCache()
	ctx := context.Background()

	key := GenerateCacheKey("system", "user", "model")
	entry := &CacheEntry{
		Response: "test response",
		Model:    "model",
		Provider: "test",
	}

	err := cache.Set(ctx, key, entry, time.Hour)
	if err != nil {
		t.Fatalf("Failed to set cache: %v", err)
	}

	retrieved, err := cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("Failed to get cache: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Expected cached entry, got nil")
	}
	if retrieved.Response != "test response" {
		t.Errorf("Expected response 'test response', got '%s'", retrieved.Response)
	}
}

func TestInMemoryCache_TTL(t *testing.T) {
	cache := NewInMemoryCache()
	ctx := context.Background()

	key := GenerateCacheKey("system", "user", "model")
	entry := &CacheEntry{
		Response: "test response",
		Model:    "model",
	}

	err := cache.Set(ctx, key, entry, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to set cache: %v", err)
	}

	time.Sleep(20 * time.Millisecond)

	retrieved, err := cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("Failed to get cache: %v", err)
	}
	if retrieved != nil {
		t.Error("Expected expired entry to return nil")
	}
}

func TestInMemoryCostTracker(t *testing.T) {
	tracker := NewInMemoryCostTracker()
	ctx := context.Background()

	record := CostRecord{
		Provider:     "gemini",
		Model:        "gemini-1.5-pro",
		InputTokens:  100,
		OutputTokens: 50,
		CostCents:    10,
		Timestamp:    time.Now(),
	}

	err := tracker.Record(ctx, record)
	if err != nil {
		t.Fatalf("Failed to record cost: %v", err)
	}

	cost, err := tracker.GetTotalCost(ctx, "gemini", time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("Failed to get cost: %v", err)
	}
	if cost != 10 {
		t.Errorf("Expected cost 10, got %d", cost)
	}

	input, output, err := tracker.GetUsage(ctx, "gemini", time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("Failed to get usage: %v", err)
	}
	if input != 100 || output != 50 {
		t.Errorf("Expected input=100, output=50, got input=%d, output=%d", input, output)
	}
}

func TestCalculateCost(t *testing.T) {
	cost := CalculateCost("gemini", "gemini-1.5-pro", 1000, 1000)
	if cost == 0 {
		t.Error("Expected non-zero cost for gemini-1.5-pro")
	}

	cost = CalculateCost("openai", "gpt-4o", 1000, 1000)
	if cost == 0 {
		t.Error("Expected non-zero cost for gpt-4o")
	}

	cost = CalculateCost("unknown", "unknown-model", 1000, 1000)
	if cost != 0 {
		t.Error("Expected zero cost for unknown model")
	}
}

func TestInMemoryPromptManager_Get(t *testing.T) {
	mgr := NewInMemoryPromptManager()
	ctx := context.Background()

	template := PromptTemplate{
		ID:        "test-1",
		Name:      "scout-context",
		Version:   "1.0",
		Content:   "Hello {{name}}",
		Variables: []string{"name"},
		CreatedAt: time.Now(),
	}

	err := mgr.Save(ctx, template)
	if err != nil {
		t.Fatalf("Failed to save template: %v", err)
	}

	retrieved, err := mgr.Get(ctx, "scout-context")
	if err != nil {
		t.Fatalf("Failed to get template: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Expected template, got nil")
	}
	if retrieved.Content != "Hello {{name}}" {
		t.Errorf("Expected content 'Hello {{name}}', got '%s'", retrieved.Content)
	}
}

func TestInMemoryPromptManager_Render(t *testing.T) {
	mgr := NewInMemoryPromptManager()
	ctx := context.Background()

	template := PromptTemplate{
		ID:        "test-1",
		Name:      "test",
		Version:   "1.0",
		Content:   "Hello {{name}}, you are {{role}}",
		Variables: []string{"name", "role"},
		CreatedAt: time.Now(),
	}

	_ = mgr.Save(ctx, template)

	rendered, err := mgr.Render(ctx, "test", map[string]string{"name": "Alice", "role": "developer"})
	if err != nil {
		t.Fatalf("Failed to render template: %v", err)
	}
	if rendered != "Hello Alice, you are developer" {
		t.Errorf("Expected 'Hello Alice, you are developer', got '%s'", rendered)
	}
}

func TestSimpleTokenCounter(t *testing.T) {
	counter := NewSimpleTokenCounter()

	count := counter.Count("Hello world")
	if count < 1 {
		t.Errorf("Count('Hello world') = %d, want >= 1", count)
	}

	count = counter.Count("")
	if count != 0 {
		t.Errorf("Count('') = %d, want 0", count)
	}

	count = counter.Count("a")
	if count < 0 {
		t.Errorf("Count('a') = %d, want >= 0", count)
	}
}

func TestBudgetEnforcer(t *testing.T) {
	config := BudgetConfig{
		MaxTokensPerRequest: 100,
		MaxTokensPerDay:     1000,
	}
	enforcer := NewInMemoryBudgetEnforcer(config)
	ctx := context.Background()

	canProceed, err := enforcer.CanProceed(ctx, 50)
	if err != nil {
		t.Fatalf("CanProceed failed: %v", err)
	}
	if !canProceed {
		t.Error("Expected CanProceed to return true for 50 tokens")
	}

	canProceed, err = enforcer.CanProceed(ctx, 150)
	if err != nil {
		t.Fatalf("CanProceed failed: %v", err)
	}
	if canProceed {
		t.Error("Expected CanProceed to return false for 150 tokens")
	}

	_ = enforcer.RecordUsage(ctx, 50)
	remaining, err := enforcer.GetRemainingBudget(ctx)
	if err != nil {
		t.Fatalf("GetRemainingBudget failed: %v", err)
	}
	expectedRemaining := 1000 - 50
	if remaining != expectedRemaining {
		t.Errorf("Expected remaining budget %d, got %d", expectedRemaining, remaining)
	}
}

func TestProviderManager_CacheHit(t *testing.T) {
	ctx := context.Background()

	mock := &mockProvider{name: "mock", response: "response from api"}

	cache := NewInMemoryCache()
	cacheKey := GenerateCacheKey("system", "user", "model")
	_ = cache.Set(ctx, cacheKey, &CacheEntry{
		Response: "cached response",
		Model:    "model",
		Provider: "cached",
	}, time.Hour)

	pm := NewProviderManager(
		[]*ProviderWithLimiter{
			{Provider: mock, Config: ProviderConfig{Name: "mock", Priority: 1}, IsAvailable: true},
		},
		cache,
		NewInMemoryCostTracker(),
		NewInMemoryBudgetEnforcer(BudgetConfig{MaxTokensPerRequest: 100000}),
	)

	resp, err := pm.SendMessageFull(ctx, LLMRequest{
		SystemPrompt: "system",
		UserPrompt:   "user",
		Model:        "model",
	})

	if err != nil {
		t.Fatalf("SendMessageFull failed: %v", err)
	}

	if !resp.CacheHit {
		t.Error("Expected cache hit")
	}
	if resp.Content != "cached response" {
		t.Errorf("Expected 'cached response', got '%s'", resp.Content)
	}
	if mock.callCount != 0 {
		t.Errorf("Expected provider not to be called, but it was called %d times", mock.callCount)
	}
}

func TestProviderManager_Fallback(t *testing.T) {
	ctx := context.Background()

	failingProvider := &mockProvider{name: "failing", err: errors.New("provider failed")}
	workingProvider := &mockProvider{name: "working", response: "success response"}

	pm := NewProviderManager(
		[]*ProviderWithLimiter{
			{Provider: failingProvider, Config: ProviderConfig{Name: "failing", Priority: 1}, IsAvailable: true},
			{Provider: workingProvider, Config: ProviderConfig{Name: "working", Priority: 2}, IsAvailable: true},
		},
		NewInMemoryCache(),
		NewInMemoryCostTracker(),
		NewInMemoryBudgetEnforcer(BudgetConfig{MaxTokensPerRequest: 100000}),
	)

	resp, err := pm.SendMessageFull(ctx, LLMRequest{
		SystemPrompt: "system",
		UserPrompt:   "user",
		Model:        "model",
	})

	if err != nil {
		t.Fatalf("SendMessageFull failed: %v", err)
	}

	if resp.Provider != "working" {
		t.Errorf("Expected provider 'working', got '%s'", resp.Provider)
	}
	if resp.Content != "success response" {
		t.Errorf("Expected 'success response', got '%s'", resp.Content)
	}
}

func TestProviderManager_AllProvidersFail(t *testing.T) {
	ctx := context.Background()

	failingProvider1 := &mockProvider{name: "failing1", err: errors.New("provider 1 failed")}
	failingProvider2 := &mockProvider{name: "failing2", err: errors.New("provider 2 failed")}

	pm := NewProviderManager(
		[]*ProviderWithLimiter{
			{Provider: failingProvider1, Config: ProviderConfig{Name: "failing1", Priority: 1}, IsAvailable: true},
			{Provider: failingProvider2, Config: ProviderConfig{Name: "failing2", Priority: 2}, IsAvailable: true},
		},
		NewInMemoryCache(),
		NewInMemoryCostTracker(),
		NewInMemoryBudgetEnforcer(BudgetConfig{MaxTokensPerRequest: 100000}),
	)

	_, err := pm.SendMessageFull(ctx, LLMRequest{
		SystemPrompt: "system",
		UserPrompt:   "user",
		Model:        "model",
	})

	if err == nil {
		t.Fatal("Expected error when all providers fail")
	}
	if !errors.Is(err, ErrAllProvidersFailed) {
		t.Errorf("Expected ErrAllProvidersFailed, got %v", err)
	}
}

func TestGenerateCacheKey(t *testing.T) {
	key1 := GenerateCacheKey("system", "user", "model")
	key2 := GenerateCacheKey("system", "user", "model")
	key3 := GenerateCacheKey("system", "different", "model")

	if key1 != key2 {
		t.Error("Same inputs should produce same cache keys")
	}
	if key1 == key3 {
		t.Error("Different inputs should produce different cache keys")
	}
}

func TestHashPrompt(t *testing.T) {
	hash1 := HashPrompt("test content")
	hash2 := HashPrompt("test content")
	hash3 := HashPrompt("different content")

	if hash1 != hash2 {
		t.Error("Same content should produce same hash")
	}
	if hash1 == hash3 {
		t.Error("Different content should produce different hash")
	}
	if len(hash1) != 16 {
		t.Errorf("Expected hash length 16, got %d", len(hash1))
	}
}
