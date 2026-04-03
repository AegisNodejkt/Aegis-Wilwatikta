package store

import (
	"testing"
	"time"
)

func TestMetricsCollector_RecordQuery(t *testing.T) {
	collector := NewMetricsCollector(50 * time.Millisecond)

	collector.RecordQuery("MATCH (n) RETURN n", 100*time.Millisecond, "lookup", "proj1", false, nil)

	slowQueries := collector.GetSlowQueries()
	if len(slowQueries) != 1 {
		t.Errorf("Expected 1 slow query, got %d", len(slowQueries))
	}

	collector.RecordQuery("MATCH (n) RETURN n", 10*time.Millisecond, "lookup", "proj1", false, nil)

	slowQueries = collector.GetSlowQueries()
	if len(slowQueries) != 1 {
		t.Errorf("Expected 1 slow query (fast query should not count), got %d", len(slowQueries))
	}
}

func TestMetricsCollector_CacheStats(t *testing.T) {
	collector := NewMetricsCollector(100 * time.Millisecond)

	collector.RecordQuery("q1", 10*time.Millisecond, "lookup", "proj1", true, nil)
	collector.RecordQuery("q2", 10*time.Millisecond, "lookup", "proj1", true, nil)
	collector.RecordQuery("q3", 10*time.Millisecond, "lookup", "proj1", false, nil)

	stats := collector.GetStats()
	if stats.CacheHits != 2 {
		t.Errorf("Expected 2 cache hits, got %d", stats.CacheHits)
	}
	if stats.CacheMisses != 1 {
		t.Errorf("Expected 1 cache miss, got %d", stats.CacheMisses)
	}
	if stats.TotalQueries != 3 {
		t.Errorf("Expected 3 total queries, got %d", stats.TotalQueries)
	}
	hitRate := stats.CacheHitRate()
	expectedRate := float64(2) / float64(3)
	if hitRate != expectedRate {
		t.Errorf("Expected cache hit rate %f, got %f", expectedRate, hitRate)
	}
}

func TestMetricsCollector_ClearSlowQueries(t *testing.T) {
	collector := NewMetricsCollector(50 * time.Millisecond)

	collector.RecordQuery("slow query", 100*time.Millisecond, "complex", "proj1", false, nil)

	if len(collector.GetSlowQueries()) != 1 {
		t.Fatal("Expected 1 slow query")
	}

	collector.ClearSlowQueries()

	if len(collector.GetSlowQueries()) != 0 {
		t.Error("Expected slow queries to be cleared")
	}
}

func TestMetricsCollector_SetEnabled(t *testing.T) {
	collector := NewMetricsCollector(50 * time.Millisecond)

	collector.RecordQuery("query1", 100*time.Millisecond, "lookup", "proj1", false, nil)
	if len(collector.GetSlowQueries()) != 1 {
		t.Fatal("Expected 1 slow query when enabled")
	}

	collector.SetEnabled(false)
	collector.RecordQuery("query2", 100*time.Millisecond, "lookup", "proj1", false, nil)

	if len(collector.GetSlowQueries()) != 1 {
		t.Error("Expected no new slow queries when disabled")
	}
}

func TestMetricsStats_CacheHitRate(t *testing.T) {
	tests := []struct {
		name     string
		stats    MetricsStats
		expected float64
	}{
		{
			name:     "50 percent hit rate",
			stats:    MetricsStats{CacheHits: 5, CacheMisses: 5, TotalQueries: 10},
			expected: 0.5,
		},
		{
			name:     "no queries",
			stats:    MetricsStats{CacheHits: 0, CacheMisses: 0, TotalQueries: 0},
			expected: 0,
		},
		{
			name:     "100 percent hit rate",
			stats:    MetricsStats{CacheHits: 10, CacheMisses: 0, TotalQueries: 10},
			expected: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.stats.CacheHitRate() != tt.expected {
				t.Errorf("Expected hit rate %f, got %f", tt.expected, tt.stats.CacheHitRate())
			}
		})
	}
}
