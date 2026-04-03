package store

import (
	"sync"
	"sync/atomic"
	"time"
)

type QueryMetric struct {
	Query         string
	Duration      time.Duration
	OperationType string
	ProjectID     string
	IsCacheHit    bool
	Error         error
	Timestamp     time.Time
}

type MetricsCollector struct {
	mu                 sync.RWMutex
	slowQueries        []QueryMetric
	totalQueries       atomic.Int64
	cacheHits          atomic.Int64
	cacheMisses        atomic.Int64
	slowQueryThreshold time.Duration
	enabled            bool
}

func NewMetricsCollector(slowQueryThreshold time.Duration) *MetricsCollector {
	return &MetricsCollector{
		slowQueries:        make([]QueryMetric, 0),
		slowQueryThreshold: slowQueryThreshold,
		enabled:            true,
	}
}

func (m *MetricsCollector) RecordQuery(query string, duration time.Duration, opType, projectID string, isCacheHit bool, err error) {
	if !m.enabled {
		return
	}

	m.totalQueries.Add(1)

	if isCacheHit {
		m.cacheHits.Add(1)
	} else {
		m.cacheMisses.Add(1)
	}

	if duration > m.slowQueryThreshold {
		m.mu.Lock()
		m.slowQueries = append(m.slowQueries, QueryMetric{
			Query:         query,
			Duration:      duration,
			OperationType: opType,
			ProjectID:     projectID,
			IsCacheHit:    isCacheHit,
			Error:         err,
			Timestamp:     time.Now(),
		})
		if len(m.slowQueries) > 100 {
			m.slowQueries = m.slowQueries[len(m.slowQueries)-100:]
		}
		m.mu.Unlock()
	}
}

func (m *MetricsCollector) IncrementCacheHit() {
	m.cacheHits.Add(1)
}

func (m *MetricsCollector) IncrementCacheMiss() {
	m.cacheMisses.Add(1)
}

func (m *MetricsCollector) GetSlowQueries() []QueryMetric {
	m.mu.RLock()
	defer m.mu.RUnlock()

	queries := make([]QueryMetric, len(m.slowQueries))
	copy(queries, m.slowQueries)
	return queries
}

func (m *MetricsCollector) GetStats() MetricsStats {
	return MetricsStats{
		TotalQueries:   m.totalQueries.Load(),
		CacheHits:      m.cacheHits.Load(),
		CacheMisses:    m.cacheMisses.Load(),
		SlowQueryCount: int64(len(m.slowQueries)),
	}
}

func (m *MetricsCollector) ClearSlowQueries() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.slowQueries = make([]QueryMetric, 0)
}

func (m *MetricsCollector) SetEnabled(enabled bool) {
	m.enabled = enabled
}

type MetricsStats struct {
	TotalQueries   int64
	CacheHits      int64
	CacheMisses    int64
	SlowQueryCount int64
}

func (s MetricsStats) CacheHitRate() float64 {
	if s.TotalQueries == 0 {
		return 0
	}
	return float64(s.CacheHits) / float64(s.TotalQueries)
}
