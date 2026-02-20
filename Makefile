# aOa-go — local quality gates
# Run `make check` before committing. That's the CI.

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X github.com/corey/aoa/internal/version.Version=$(VERSION) \
           -X github.com/corey/aoa/internal/version.BuildDate=$(BUILD_DATE)

.PHONY: build build-lean test lint bench coverage check vet

# Build the binary with version info (all grammars compiled in, ~80 MB)
build:
	go build -ldflags "$(LDFLAGS)" -o aoa ./cmd/aoa/

# Build lean binary (no grammars compiled in, ~12 MB)
# Grammars loaded dynamically from .aoa/grammars/*.so at runtime
build-lean:
	go build -tags lean -ldflags "-s -w $(LDFLAGS)" -o aoa ./cmd/aoa/

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
	go test ./... -bench=. -benchmem -run=^$

# Test coverage report
coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out
	@rm -f coverage.out

# The local CI: vet + lint + test
check: vet lint test
	@echo ""
	@echo "✓ All checks passed"

# Count test status
status:
	@echo "=== Test Status ==="
	@go test ./... -v 2>&1 | grep -c "SKIP" | xargs -I{} echo "  Skipped: {}"
	@go test ./... -v 2>&1 | grep -c "PASS" | xargs -I{} echo "  Passing: {}"
	@go test ./... -v 2>&1 | grep -c "FAIL" | xargs -I{} echo "  Failing: {}"
