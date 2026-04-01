# Load Testing Makefile

.PHONY: test bench profile k6-all k6-load k6-stress k6-queue bench-all bench-scout bench-architect bench-diplomat bench-pipeline profile-cpu profile-mem check-goroutines

# Go benchmarks
bench-all:
	go test -bench=. -benchmem -benchtime=5s ./benchmark/

bench-scout:
	go test -bench=BenchmarkScout -benchmem -benchtime=5s ./benchmark/

bench-architect:
	go test -bench=BenchmarkArchitect -benchmem -benchtime=5s ./benchmark/

bench-diplomat:
	go test -bench=BenchmarkDiplomat -benchmem -benchtime=5s ./benchmark/

bench-pipeline:
	go test -bench=BenchmarkPipeline -benchmem -benchtime=5s ./benchmark/

# SLA compliance tests
test-sla:
	go test -v -run 'Test.*SLA' ./benchmark/

# All tests including unit tests
test:
	go test -v ./...

# CPU Profiling
profile-cpu:
	go test -bench=. -cpuprofile=cpu.prof ./benchmark/
	@echo "CPU profile saved to cpu.prof"
	@echo "View with: go tool pprof -http=:8080 cpu.prof"

# Memory Profiling
profile-mem:
	go test -bench=. -memprofile=mem.prof ./benchmark/
	@echo "Memory profile saved to mem.prof"
	@echo "View with: go tool pprof -http=:8080 mem.prof"

# Check for goroutine leaks
check-goroutines:
	go test -run TestPipelineSLACompliance -v ./benchmark/
	@echo "Goroutine test completed"

# k6 load tests (requires k6 installed)
k6-all: k6-load k6-stress k6-queue

k6-load:
	@echo "Running webhook load test..."
	k6 run loadtest/k6/webhook_load.js

k6-stress:
	@echo "Running stress test..."
	k6 run loadtest/k6/stress_test.js

k6-queue:
	@echo "Running queue depth test..."
	k6 run loadtest/k6/queue_depth.js

# k6 with custom target
k6-load-custom:
	@echo "Usage: BASE_URL=http://your-server:port make k6-load-custom"
	k6 run loadtest/k6/webhook_load.js

# Start test server for local testing
test-server:
	go run ./cmd/webhook/main.go

# Full benchmark suite
full-benchmark: bench-all test-sla profile-cpu profile-mem
	@echo "Full benchmark suite completed"
	@echo "Results in: cpu.prof, mem.prof"

# Clean up profiles
clean:
	rm -f cpu.prof mem.prof