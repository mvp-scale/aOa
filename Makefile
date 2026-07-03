# aOa-go — local quality gates
# Run `make check` before committing. That's the CI.
#
# IMPORTANT: All builds go through build.sh. Never run "go build" directly.
# Two build modes:
#   ./build.sh         — core (tree-sitter + dynamic grammars)
#   ./build.sh --light — lean (pure Go, no tree-sitter)

.PHONY: build build-light test test-v test-active lint lint-check lint-baseline vet bench bench-gauntlet bench-baseline bench-compare coverage check status

# Core build — tree-sitter + dynamic grammar loading
build:
	./build.sh

# Lean build — pure Go, no tree-sitter, no CGo
build-light:
	./build.sh --light

# Run all tests (skipped tests are expected during development)
test:
	go test ./...

# Run tests with verbose output (see skip reasons)
test-v:
	go test ./... -v

# Run only non-skipped tests (useful to see what's actually passing)
test-active:
	go test ./... -v 2>&1 | grep -E "^(=== RUN|--- PASS|--- FAIL|FAIL|ok)" || true

# Lint (raw) — runs golangci-lint and exits 1 on any finding.
# Use lint-check (below) for the gate-passing baseline-diff mode.
lint:
	golangci-lint run ./...

# Lint gate — zero findings introduced in F1 (--new-from-rev against F0 base).
# P6 ruling (checkpoint-F1 PC2): ~91 pre-existing findings grandfathered at F0
# (commit cd33a5c); any finding in F1-changed lines is a gate failure.
# Audit record: scripts/lint-baseline.txt (human-readable, ~91 grandfathered).
# GOLANGCI_LINT can be overridden: make lint-check GOLANGCI_LINT=/path/to/bin
# Note: golangci-lint's --new-from-rev uses git diff, so it requires a git repo.
GOLANGCI_LINT ?= $(shell command -v golangci-lint 2>/dev/null || ls $$HOME/go/bin/golangci-lint 2>/dev/null || ls $$HOME/.local/bin/golangci-lint 2>/dev/null || echo golangci-lint)
F0_COMMIT ?= cd33a5c
lint-check:
	@echo "=== Lint gate: F1-changed code only (baseline: F0 @ $(F0_COMMIT)) ==="
	@echo "  (pre-existing ~91 findings grandfathered — see scripts/lint-baseline.txt)"
	@if $(GOLANGCI_LINT) run --new-from-rev=$(F0_COMMIT) ./... 2>&1; then \
	  echo "  OK — 0 new findings in F1-changed code"; \
	else \
	  echo ""; \
	  echo "FAIL: new lint findings introduced in F1-changed code (see above)"; \
	  echo "  Fix the findings, or if pre-existing, update the F0 base commit:"; \
	  echo "    make lint-check F0_COMMIT=<new-base-sha>"; \
	  exit 1; \
	fi

# Regenerate the baseline audit file from the current golangci-lint output.
# Does not affect the gate — gate always uses --new-from-rev=$(F0_COMMIT).
lint-baseline:
	@$(GOLANGCI_LINT) run ./... 2>&1 | \
	  grep -E '^(internal|cmd|test|atlas)' | sort > scripts/lint-baseline.txt
	@echo "Baseline updated: $$(wc -l < scripts/lint-baseline.txt) findings in scripts/lint-baseline.txt"

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

# The local CI: vet + lint gate (no new findings) + test + standard build + size gate
check: vet lint-check test build
	@SIZE=$$(stat --format=%s aoa 2>/dev/null || stat -f%z aoa); \
	 SIZE_MB=$$((SIZE / 1048576)); \
	 if [ "$$SIZE" -gt 20971520 ]; then \
	   echo ""; \
	   echo "FAIL: binary is $${SIZE_MB} MB — max 20 MB"; \
	   echo "  Something dragged in unexpected dependencies."; \
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
