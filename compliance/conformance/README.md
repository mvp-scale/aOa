# Generative Session Conformance

A repeatable harness that **generates** controlled Claude Code sessions and
validates what aOa extracts from them — instead of waiting for organic sessions
to happen to emit a surface. This is the new-version onboarding test: install a
new Claude Code, run `./run.sh`, get a per-surface pass/fail in minutes, from a
fresh session, by design.

It complements the observational suite in `../claude-session/`:

| | `../claude-session/` (observe) | `conformance/` (generate) |
|---|---|---|
| Input | whatever the latest real session happened to contain | sessions we generate to order |
| Coverage | only surfaces that organically fired | every surface we can script a trigger for |
| Token check | arithmetic over historical files | closed loop: generate → known ground truth → assert |
| Trigger | manual `go test` | manual `./run.sh` (CI-able) |

## How it works

`claude -p "<prompt>" --output-format json` does two things at once:

1. Writes a real session JSONL to `~/.claude/projects/{encoded-cwd}/{id}.jsonl`
   — the **exact file the tailer reads**. Each scenario runs in its own `/tmp`
   sandbox cwd, so it gets an isolated project dir.
2. Returns a `result` block containing `session_id` and an authoritative
   `usage` block — **ground truth** the harness asserts the in-file numbers
   against.

For each scenario in `scenarios.json`, `run.sh`:
- generates the session,
- runs the contract drift check (`TestPass3_Schema`, via
  `AOA_COMPLIANCE_SESSION_DIR`) against the fresh JSONL,
- runs the scenario's `checks`,
- accumulates a coverage matrix and a pass/fail summary,
- flags installed-version ≠ contract-baseline.

## Usage

```bash
cd compliance/conformance
./run.sh                       # auto, low-cost scenarios
./run.sh --all                 # include high-cost (subagent) scenarios
./run.sh --scenario bash-tool  # one scenario
./run.sh --keep                # keep generated sessions for inspection
AOA_CONF_MODEL=claude-opus-4-8 ./run.sh   # pin a model
```

Requires `claude`, `jq`, `go` in PATH. Auto scenarios run in `/tmp` with
`bypassPermissions` — keep prompts trivial and self-contained.

## Checks

| Check | Asserts |
|-------|---------|
| `schema` | no `DRIFT (new …)` from `TestPass3_Schema` (new unrecognized type/field/block) |
| `usage_match` | in-file `message.usage` (last turn) == the `-p` result-block `usage` |
| `surface:a,b` | top-level event types `a`,`b` present |
| `block:a,b` | message-content block types `a`,`b` present |
| `toolresult:Bash\|Edit\|Agent` | a `toolUseResult` of that tool's signature appears |
| `iterations_populated` | some `usage.iterations` is non-empty (informational) |
| `agent_tokens` | closed loop — Σ subagent usage vs `toolUseResult.totalTokens` vs aOa's `chars/4` estimate; WARN if ≥10× under |

A scenario is `PASS` (all checks ok), `WARN` (a measurement flagged a known
problem, e.g. token understatement), `FAIL` (schema drift or a missing surface),
or `SKIP` (manual or high-cost without `--all`).

## Manual scenarios (can't be driven headless)

Two surfaces need an interactive session; the catalog marks them `trigger:manual`:

- **`away-summary`** (`system.subtype=away_summary`): start interactive
  `claude` in a scratch dir, send a prompt, pause/detach, then resume — the
  reattach emits the resume-summary. Point the harness at that project dir:
  `AOA_COMPLIANCE_SESSION_DIR=<dir> go test -tags compliance -run TestPass3_Schema`.
- **`mode-transitions`** (`permission-mode`, `mode`): in an interactive session,
  toggle permission mode (shift-tab) and output/fast mode; each toggle emits an
  event. Validate the same way.

## Adding a scenario

Append to `scenarios.json`:

```json
{
  "id": "my-surface",
  "surface": "human description",
  "trigger": "auto",          // or "manual"
  "cost": "low",              // or "high" (needs --all)
  "permission_mode": "bypassPermissions",
  "prompt": "a prompt that forces the surface to appear",
  "checks": ["schema", "surface:...", "block:...", "toolresult:..."]
}
```

Then `./run.sh --scenario my-surface`. If it needs a check the harness doesn't
have yet, add a `case` arm in `run.sh`'s check loop.

## What it does and doesn't prove

- **Proves:** the installed version's schema matches the contract (or names the
  drift); the numbers aOa reads from disk match Claude's authoritative numbers;
  the Agent-row token attribution is wrong by orders of magnitude, on data
  generated right now.
- **Doesn't prove:** semantic correctness of fields no scenario asserts a ground
  truth for. The pattern to close that is the same as `agent_tokens` — generate
  a scenario where the right answer is computable, then assert aOa produces it.
  Extend the catalog as new entities get ground-truth oracles.
