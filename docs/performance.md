# Performance Benchmarking and Load Testing

This document defines the performance baselines, SLAs, and load testing infrastructure for the Aegis AI Code Reviewer.

## SLA Targets

| Metric | Target | Description |
|--------|--------|-------------|
| Total Review Time (p95) | <60s | End-to-end review completion |
| Scout Agent | <30s | Context gathering and impact analysis |
| Architect Agent | <60s | Code review and issue detection |
| Diplomat Agent | <15s | Review formatting and feedback generation |
| 100 Concurrent Reviews | 100% success | All reviews complete within SLA |
| 500 Concurrent Reviews | 95% success | 95% complete within 120s |
| 1000 Concurrent Reviews | Stable | Queue processes all within 5min |
| Memory per Agent | <2GB | Peak memory usage per agent instance |
| CPU Utilization | <80% | Under normal load conditions |

## Load Test Infrastructure

### k6 Load Test Suite

Located in `loadtest/k6/`:

1. **webhook_load.js** - Main load test with scenarios:
   - Normal Load: Ramp from 0→10→100→0 VUs over 2 minutes
   - Spike Test: Sudden jump to 500 VUs sustained for 30s
   - Sustained High: 200 VUs for 60 seconds
   - Concurrent Reviews: 100 VUs × 10 iterations

2. **stress_test.js** - Stress testing with large payloads:
   - Tests system behavior under extreme load
   - Large diffs simulating big PRs
   - Measures system degradation limits

3. **queue_depth.js** - Queue behavior testing:
   - Tests queue depth under sustained load
   - Measures queue drain time
   - Validates queue capacity (default: 1000)

### Running k6 Tests

```bash
# Install k6
brew install k6  # macOS
# or
sudo apt install k6  # Ubuntu

# Run all scenarios
cd loadtest/k6
k6 run webhook_load.js

# Run with custom target
BASE_URL=http://localhost:8080 k6 run webhook_load.js

# Run stress test
k6 run stress_test.js

# Run queue depth test
k6 run queue_depth.js
```

## Go Benchmarks

### Agent Benchmarks

Located in `benchmark/`:

```bash
# Run all benchmarks
go test -bench=. -benchmem ./benchmark/

# Run specific benchmark
go test -bench=BenchmarkScout -benchmem ./benchmark/
go test -bench=BenchmarkArchitect -benchmem ./benchmark/
go test -bench=BenchmarkDiplomat -benchmem ./benchmark/
go test -bench=BenchmarkPipelineOrchestrator -benchmem ./benchmark/

# Run with CPU profiling
go test -bench=. -cpuprofile=cpu.prof ./benchmark/
go tool pprof -http=:8080 cpu.prof

# Run with memory profiling
go test -bench=. -memprofile=mem.prof ./benchmark/
go tool pprof -http=:8080 mem.prof

# Run SLA compliance tests
go test -v -run Test.*SLA ./benchmark/
```

### Expected Benchmark Results

| Benchmark | Target Time | Notes |
|-----------|-------------|-------|
| ScoutGatherContext | <100ms | Mock provider, no real LLM calls |
| ScoutWithLargePR | <500ms | With 100 file diffs |
| ArchitectReview/SmallPR | <50ms | Single file diff |
| ArchitectReview/MediumPR | <200ms | 10 file diffs |
| ArchitectReview/LargePR | <1s | 50 file diffs |
| DiplomatFormatReview/SmallReview | <50ms | Single issue |
| DiplomatFormatReview/LargeReview | <500ms | 50 issues |
| PipelineOrchestrator/FastAgents | <350ms | Sum of agent times |
| PipelineOrchestrator/SlowAgents | <350ms | With simulated delays |

## Profiling

### CPU Profiling

```bash
# Profile running server
go tool pprof http://localhost:8080/debug/pprof/profile?seconds=30

# Profile benchmarks
go test -bench=. -cpuprofile=cpu.prof ./benchmark/
go tool pprof -top cpu.prof
```

### Memory Profiling

```bash
# Profile running server
go tool pprof http://localhost:8080/debug/pprof/heap

# Profile benchmarks
go test -bench=. -memprofile=mem.prof ./benchmark/
go tool pprof -top mem.prof
```

### Goroutine Analysis

```bash
# Check for goroutine leaks
go tool pprof http://localhost:8080/debug/pprof/goroutine

# After load test
go test -run TestPipelineSLACompliance -v ./benchmark/
```

## Database Connection Pool Tuning

### Neo4j Connection Pool

For high concurrency, tune Neo4j connection pool settings:

```go
// In store/neo4j.go
config := neo4j.Config{
    MaxConnectionPoolSize:    100,  // Default: 100
    MaxConnectionLifetime:    1 * time.Hour,
    MaxConnectionIdleTimeout: 30 * time.Second,
    ConnectionAcquisitionTimeout: 60 * time.Second,
}
```

### Recommended Pool Sizes

| Scenario | Pool Size | Rationale |
|----------|-----------|-----------|
| 100 concurrent | 50 | 0.5 connections per concurrent review |
| 500 concurrent | 200 | Reduced ratio for efficiency |
| 1000 concurrent | 400 | Queue absorbs burst traffic |

## Hardware Requirements

### Development Environment

| Component | Minimum | Recommended |
|-----------|---------|-------------|
| CPU | 4 cores | 8 cores |
| RAM | 8 GB | 16 GB |
| Disk | 20 GB SSD | 50 GB SSD |
| Network | 100 Mbps | 1 Gbps |

### Production Environment

| Component | Small Deployment | Large Deployment |
|-----------|-----------------|------------------|
| CPU | 8 cores | 32 cores |
| RAM | 32 GB | 128 GB |
| Disk | 100 GB NVMe SSD | 1 TB NVMe SSD |
| Network | 1 Gbps dedicated | 10 Gbps dedicated |

### Container Resources

```yaml
# Kubernetes resource limits
resources:
  requests:
    memory: "2Gi"
    cpu: "1000m"
  limits:
    memory: "4Gi"
    cpu: "4000m"
```

## Performance Monitoring

### Key Metrics to Monitor

1. **Queue Depth**: `queue.Enqueue()` vs `queue.Dequeue()` rate
2. **Agent Latency**: Per-agent response times
3. **LLM API Calls**: Rate, latency, error rate
4. **Neo4j Queries**: Query latency, connection pool usage
5. **Memory**: Heap allocation, GC pressure
6. **Goroutines**: Active goroutines, goroutine leaks

### Prometheus Metrics (Recommended)

```go
// Add to internal/webhook/metrics.go
var (
    ReviewsStarted = promauto.NewCounter(prometheus.CounterOpts{
        Name: "aegis_reviews_started_total",
        Help: "Total number of reviews started",
    })
    
    ReviewsCompleted = promauto.NewCounter(prometheus.CounterOpts{
        Name: "aegis_reviews_completed_total",
        Help: "Total number of reviews completed",
    })
    
    ReviewDuration = promauto.NewHistogram(prometheus.HistogramOpts{
        Name: "aegis_review_duration_seconds",
        Help: "Duration of review processing",
        Buckets: []float64{.1, .5, 1, 5, 10, 30, 60, 120, 300},
    })
    
    QueueDepth = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "aegis_queue_depth",
        Help: "Current queue depth",
    })
    
    AgentDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
        Name: "aegis_agent_duration_seconds",
        Help: "Duration per agent",
        Buckets: []float64{.1, .5, 1, 5, 10, 30, 60},
    }, []string{"agent"})
)
```

## Test Scenarios

### Scenario 1: Normal Load (100 concurrent)

**Expected Behavior:**
- All reviews complete within 60s
- No errors
- Memory stays under 2GB
- CPU utilization under 80%

**Validation:**
```bash
k6 run --vus 100 --duration 60s webhook_load.js
# Check: http_req_duration p95 < 500ms
# Check: errors rate < 0.01
```

### Scenario 2: Spike (5x sudden load)

**Expected Behavior:**
- System absorbs burst via queue
- Graceful degradation
- No crashes or OOM errors
- Queue drains after spike

**Validation:**
```bash
k6 run stress_test.js
# Check: No 500 errors
# Check: System recovers within 2 minutes after spike
```

### Scenario 3: Sustained High Load (500 for 1 hour)

**Expected Behavior:**
- System remains stable
- No memory leaks
- Queue processes all requests
- 95% success rate

**Validation:**
```bash
k6 run --vus 500 --duration 1h webhook_load.js
# Check: Memory profile stable
# Check: No goroutine leaks (pprof/goroutine)
```

## Baseline Metrics Document

| Component | Baseline (p50) | Target (p95) | Limit (p99) |
|-----------|----------------|--------------|-------------|
| Webhook Receive | 10ms | 50ms | 500ms |
| Queue Enqueue | 5ms | 20ms | 100ms |
| Scout Context | 2s | 30s | 45s |
| Architect Analysis | 5s | 60s | 90s |
| Diplomat Format | 1s | 15s | 30s |
| Total Pipeline | 8s | 60s | 120s |
| LLM API Call | 500ms | 10s | 30s |
| Neo4j Query | 10ms | 100ms | 500ms |

## Action Items for QA

- [ ] Run full k6 suite and capture baseline metrics
- [ ] Verify memory profile under sustained load
- [ ] Confirm no goroutine leaks after 1 hour test
- [ ] Document any performance regressions
- [ ] Provide Go benchmark results
- [ ] Submit PR with test results