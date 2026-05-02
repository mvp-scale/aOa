# Claude Code Session — Integration Compliance

This folder formalizes aOa's integration contract with Claude Code's on-disk
session format. It exists because that format is **undocumented and
unstable**: Anthropic doesn't publish it, and it changes across Claude Code
releases. When it changes silently, our parser silently drops signal — the
events fall into an `UnknownTypes` bucket, fields read as empty, tool
invocations go uncounted.

This compliance suite catches that drift.

## Why this is here, not in `.context/`

`.context/` is project-internal planning that's gitignored. The integration
contract is product-relevant: it describes a real external dependency, must
travel with the code, and must be reproducible by anyone running the test.
So it lives at repo root, under git control.

## What's in here

```
compliance/claude-session/
  README.md            ← you are here
  CONTRACT.md          ← the spec: every field marked [CONSUMED] / [DROPPED] / [IGNORED]
  compliance_test.go   ← three-pass runnable check (build tag: compliance)
  versions/
    HISTORY.md         ← narrative changelog: what changed between versions and why
    v2.1.70-inferred/   ← what aOa's parser was DESIGNED for (derived from code)
      README.md
      assumed-types.md
      assumed-fields.md
    v2.1.126-observed/ ← what Claude Code 2.1.126 actually emits (validated live)
      README.md
      manifest.json
      observations.md  ← drift report + ranked follow-up list
```

## Two version flavors, deliberately

- **`vX-inferred/`** — reconstructed from `parser.go`/`reader.go`/`tailer.go`.
  Says: "this is what we built against." Cannot be validated, since we
  don't have a saved live session at that version.
- **`vX-observed/`** — captured from a real session at version X. This is
  the source of truth. The compliance test validates against the *latest*
  observed version.

The diff between them is the integration debt list — see
`versions/v2.1.126-observed/observations.md`.

## How the compliance check works

Three independent passes, each runnable as a separate `go test`:

| Pass | Test                | Validates                                                               | Failure mode |
|------|---------------------|-------------------------------------------------------------------------|--------------|
| 1    | `TestPass1_Topology` | Session dir exists; `{S}.jsonl` + `{S}/{subagents,tool-results}` shape | FAIL on shape change |
| 2    | `TestPass2_FileFormat` | Files are newline-delimited JSON, valid objects, ≤512KB lines        | FAIL on bad JSON |
| 3    | `TestPass3_Schema`     | Required fields present; new types/fields → drift signal             | FAIL on missing-required, INFO on additive drift |

Additive drift (new types, new fields) is logged but does NOT fail. That
keeps the test green when Claude Code adds capabilities, while still
surfacing them as work-to-do via `t.Logf`. To raise the bar (treat unknown
types as failures), edit `knownUnhandledTypes` and `knownUnconsumedEnvelopeFields`
in `compliance_test.go`.

## Run it

```bash
# default: validates ~/.claude/projects/{this-repo-encoded}/
go test -tags compliance ./compliance/claude-session/... -v

# override the target session directory
AOA_COMPLIANCE_SESSION_DIR=~/.claude/projects/-home-corey-other-repo \
  go test -tags compliance ./compliance/claude-session/... -v
```

Without the `-tags compliance` build tag, this test is invisible to the
default `go test ./...` so it never blocks normal CI.

## When to refresh

Refresh when **any** of these is true:

1. Claude Code ships a new minor or major version.
2. The compliance test starts logging `DRIFT (new type)` or `DRIFT (new envelope field)` (these become FAILures).
3. The parser is updated to consume previously-dropped fields — move them out of `knownUnconsumedEnvelopeFields` into `consumedEnvelopeFields`, and remove from `versions/v{old}-observed/observations.md` recommended-followup.

Refresh process:

1. Run the test. Capture observed types, fields, and content blocks (the test logs them).
2. Create `versions/v{NEW}-observed/` with `README.md`, `manifest.json`, `observations.md`.
3. Update `CONTRACT.md` §3 only if the parser was updated to consume the new shape — never update §3 ahead of code.
4. Update the `knownUnhandledTypes` and `knownUnconsumedEnvelopeFields` maps in `compliance_test.go` to acknowledge new shapes.

## Out of scope

- This folder does NOT modify production code. It only documents and tests the contract.
- It does not validate `~/.claude/sessions/`, `~/.claude/tasks/`, `~/.claude/todos/`, or any other peer directories under `~/.claude/`. aOa does not depend on them. If integration scope expands, this README and `CONTRACT.md` §1.3 must be updated.
