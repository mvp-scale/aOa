# Contributing to aOa

Thanks for your interest in contributing to aOa. This document covers the basics of building, testing, and submitting changes.

## Setup

```bash
git clone https://github.com/MVP-Scale/aOa.git
cd aOa
./build.sh
```

**Never run `go build ./cmd/aoa/` directly.** A compile-time build guard enforces this — it will panic at startup. All builds go through `./build.sh`.

Build variants:
- `./build.sh` — standard build (tree-sitter + dynamic grammars)
- `./build.sh --light` — light build (no tree-sitter, pure Go)

## Before You Submit

Run the full local CI gate:

```bash
make check
```

This runs `go vet`, linting, and the full test suite. Your PR must pass this gate.

You can also run individual checks:

```bash
go vet ./...
go test ./...
go test ./internal/domain/learner/ -run TestSpecificTest -v
```

## Architecture Constraints

aOa uses hexagonal (ports/adapters) architecture. All domain logic lives in `internal/domain/` and has zero external dependencies. External concerns are behind interfaces in `internal/ports/`.

When contributing:

- **No outbound network calls.** aOa makes zero network connections to external services. This is a security guarantee, not a suggestion. CI enforces it.
- **No new runtime dependencies** without discussion. The goal is a single self-contained binary.
- **Domain logic stays pure.** `internal/domain/` must not import anything from `internal/adapters/`.
- **Project-scoped data only.** All data lives under `{projectRoot}/.aoa/`. Nothing goes in home directories or system paths.

## What We'll Merge

- Bug fixes with tests
- Performance improvements with benchmarks
- New language support (tree-sitter grammars or tokenizer mappings)
- Documentation corrections
- Test coverage improvements

## What We Won't Merge

- Features that require network access
- Changes that break behavioral parity with test fixtures
- Dependencies on external services (databases, APIs, containers)
- Rewrites of working subsystems without prior discussion

## Pull Request Expectations

1. **One concern per PR.** Don't mix a bug fix with a refactor.
2. **Tests required.** Bug fixes need a regression test. New features need coverage.
3. **`make check` passes.** No exceptions.
4. **No `go build` in instructions or scripts.** Always `./build.sh`.
5. **Keep it focused.** Don't "improve" surrounding code unless it's part of the fix.

## Project Structure

```
cmd/aoa/           CLI entrypoint (cobra)
internal/
  ports/           Interfaces and shared types
  domain/          Pure business logic (index, learner, enricher, status)
  adapters/        External integrations (bbolt, socket, web, tailer, treesitter)
  app/             Wiring and configuration
atlas/v1/          134 semantic domains (embedded)
test/fixtures/     Behavioral parity data
```

See [README.md](README.md) for full architecture details.

## Security Issues

Do not open public issues for security vulnerabilities. See [SECURITY.md](SECURITY.md) for reporting instructions.

## License

By contributing, you agree that your contributions will be licensed under the [Apache License 2.0](LICENSE).
