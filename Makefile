# aOa-go — local quality gates
# Run `make check` before committing. That's the CI.

.PHONY: test lint bench coverage check vet

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
