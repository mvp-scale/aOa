# aOa-go — local quality gates
# Run `make check` before committing. That's the CI.
#
# IMPORTANT: All builds go through build.sh. Never run "go build" directly.
# Standard build = no recon, no CGo, no compiled grammars.
# Recon is opt-in only: make build-recon

.PHONY: build build-recon build-recon-bin test test-v test-active lint vet bench bench-gauntlet bench-baseline bench-compare coverage check status

# Standard build — no recon, no CGo, pure Go (~12 MB)
build:
	./build.sh

# Build with recon/dimensional analysis (opt-in only)
build-recon:
	./build.sh --recon

# Build standalone aoa-recon binary
build-recon-bin:
	./build.sh --recon-bin

# Run all tests (skipped tests are expected during development)
test:
	go test ./...

# Run tests with verbose output (see skip reasons)
test-v:
	go test ./... -v

# Run only non-skipped tests (useful to see what's actually passing)
test-active:
	go test ./... -v 2>&1 | grep -E "^(=== RUN|--- PASS|--- FAIL|FAIL|ok)" || true

# Lint with golangci-lint
lint:
	golangci-lint run ./...

# Go vet (built-in, no install needed)
vet:
	go vet ./...

# Benchmarks (skipped until implementations exist)
bench:
	go test ./... -bench=. -benchmem -run=^$$

# Test coverage report
coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out
	@rm -f coverage.out

# The local CI: vet + lint + test + standard build + size gate
check: vet lint test build
	@SIZE=$$(stat --format=%s aoa 2>/dev/null || stat -f%z aoa); \
	 SIZE_MB=$$((SIZE / 1048576)); \
	 if [ "$$SIZE" -gt 20971520 ]; then \
	   echo ""; \
	   echo "FAIL: binary is $${SIZE_MB} MB — max 20 MB"; \
	   echo "  Something dragged in CGo/treesitter/recon."; \
	   exit 1; \
	 fi
	@echo ""
	@echo "All checks passed (standard build + size gate)"

# Search performance gauntlet (22-shape query matrix, benchstat-compatible)
bench-gauntlet:
	go test ./test/ -bench=BenchmarkSearchGauntlet -benchmem -run=^$$ -count=6

# Generate benchstat baseline for the search gauntlet
bench-baseline:
	@mkdir -p test/testdata/benchmarks
	go test ./test/ -bench=BenchmarkSearchGauntlet -benchmem -run=^$$ -count=6 > test/testdata/benchmarks/baseline.txt

# Compare current performance against baseline (requires benchstat)
bench-compare:
	go test ./test/ -bench=BenchmarkSearchGauntlet -benchmem -run=^$$ -count=6 > /tmp/aoa-bench-current.txt
	benchstat test/testdata/benchmarks/baseline.txt /tmp/aoa-bench-current.txt

# Count test status
status:
	@echo "=== Test Status ==="
	@go test ./... -v 2>&1 | grep -c "SKIP" | xargs -I{} echo "  Skipped: {}"
	@go test ./... -v 2>&1 | grep -c "PASS" | xargs -I{} echo "  Passing: {}"
	@go test ./... -v 2>&1 | grep -c "FAIL" | xargs -I{} echo "  Failing: {}"
