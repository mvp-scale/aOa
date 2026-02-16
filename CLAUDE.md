# aOa-go Rules

## Isolation Rule

**aOa-go is a standalone project.** Do NOT import, reference, copy, or depend on anything from outside the `aOa-go/` directory.

- No imports from the parent `aOa/` codebase (Python services, CLI, plugins, hooks, etc.)
- No reading files outside `aOa-go/` to inform implementation
- No shared config, scripts, or utilities from the parent project
- All dependencies come from Go modules (`go.mod`) or are written fresh within `aOa-go/`

The Go port is a clean-room rewrite guided by the behavioral specs and test fixtures in `aOa-go/test/fixtures/`, not by copying Python code.

## GO-BOARD

When referencing "the go board" or "GO-BOARD", this means `aOa-go/GO-BOARD.md` — the project board for the Go port. Not the parent aOa board.
