# aOa ↔ Claude Code — Alignment Certificate

> The one-page answer to **"are we aligned, and does the data we consume fit?"**
> For per-field coverage detail, see [`REPORT.md`](REPORT.md). For how to
> re-verify, see [`RUNBOOK.md`](RUNBOOK.md) and the commands at the bottom.

**Installed Claude Code:** `2.1.181` &nbsp;·&nbsp; **As of:** 2026-06-18 &nbsp;·&nbsp; **Overall: 🟡 PARTIAL**

Claude Code is an unpublished, fast-drifting external contract — it moved
`2.1.178 → 2.1.181` *during the session that produced this file*. "Aligned"
is therefore never permanent; it is a checklist re-run against whatever version
is installed today.

---

## The checklist (what "success" means)

A surface is **aligned** only when all six are ✓ **at the installed version**:

| # | Check | Means |
|---|-------|-------|
| 1 | **Current** | installed version == certified baseline (no version gap) |
| 2 | **Recognized** | every event type / field is consumed or explicitly catalogued `[DROPPED]` — no uncatalogued drift |
| 3 | **Unbroken** | every field we *consume* is present and correctly typed — nothing we depend on vanished |
| 4 | **Fits** | consumed mapping points resolve to real values and render/parse correctly against a live sample |
| 5 | **Watched** | drift is detectable — live sentinel (session) + a captured `sample.json` (status-line) |
| 6 | **Green** | the runnable compliance suite passes |

Legend: ✅ pass · 🟡 partial / in-progress · ❌ not yet · ⚪ out of scope

---

## Scorecard

### Surface 1 — Session JSONL (`~/.claude/projects/…`)  → 🟡

| Check | State | Note |
|-------|-------|------|
| 1 Current | 🟡 | baseline `v2.1.178`; installed moved to `2.1.181` — **re-observe due** |
| 2 Recognized | ✅ | all types + envelope fields catalogued (incl. `interruptedMessageId`) |
| 3 Unbroken | ✅ | additive drift only; verified green vs a real `2.1.178` session |
| 4 Fits | 🟡 | main-turn tokens **exact** (closed-loop); subagent tokens fixed to ~2.8× (was 414×) — full reconciliation is **L18** |
| 5 Watched | 🟡 | drift sentinel built (PR #1, branch `drift-sentinel-l20`) — **not yet on main**; compliance suite covers out-of-band |
| 6 Green | ✅ | Pass0/1/2/3/5 green vs a matching-version session |

### Surface 2 — Status line (hook stdin)  → ❌ remap in progress

| Check | State | Note |
|-------|-------|------|
| 1 Current | ❌ | baseline `v2.1.126`; installed `2.1.181` |
| 2 Recognized | ❌ | ~8 new fields uncatalogued (`effort`, `thinking`, `exceeds_200k_tokens`, `output_style`, `transcript_path`, `workspace.*`, `current_usage.output_tokens`, `fast_mode`) |
| 3 Unbroken | ✅ | **22/22 consumed paths present** at `2.1.181` |
| 4 Fits | ✅ | all 22 consumed values resolve correctly and render (validated via CLI-driven PTY capture, 2026-06-18) |
| 5 Watched | ❌ | no `sample.json` committed → Pass4 (live-shape detector) is dark |
| 6 Green | 🟡 | Pass2/Pass3 pass; Pass4 skips for lack of a sample |

### Surface 3 — `claude -p` response block  → ⚪ no contract

Used only as a token oracle inside `conformance/run.sh`. Not a formalized
surface; `total_cost_usd` / `num_turns` / `modelUsage` are unmapped by design.

---

## Bottom line

- **Session surface:** structurally aligned, but the baseline is **one+ version
  behind** the installed binary (it auto-updated past us), and the drift
  sentinel isn't merged to main yet. Re-observe to `2.1.181` to clear check 1.
- **Status-line surface:** the **largest open gap** — still at `2.1.126`. The
  good news: the consumed mapping points were just validated **22/22 correct at
  2.1.181**; what remains is authoring the snapshot + capturing `sample.json`.
- **We are NOT fully aligned.** Two checks are red/amber on each active surface.
  Nothing is *broken* (no consumed field vanished), but "aligned to latest" is
  not yet true.

---

## How to re-verify (regenerate this verdict)

```bash
# 1. Version gap (check 1) — installed vs certified baseline
claude --version
ls -d compliance/claude-session/versions/v*-observed | sort -t. -k3 -n | tail -1

# 2-3, 6. Schema + unbroken + green — session surface
go test -tags compliance ./compliance/claude-session/...
#   (against a session matching the installed version; Pass0 flags any gap)

# 4. Fits — session token economics, generated on demand
cd compliance/conformance && ./run.sh --all

# Status-line capture (CLI-driven, since `claude -p` renders no status line):
touch .aoa/.capture-statusline   # arm (requires the capture line in the hook)
#   then drive an ephemeral interactive claude in a PTY:
#   timeout 60 script -qfc "claude 'reply ok'" /dev/null
#   → version-gated capture lands in .aoa/, flatten + diff vs consumed paths
go test -tags compliance ./compliance/claude-statusline/...
```

When every surface shows ✅ across all six checks at the installed version,
**that is the certificate** — and the matching `versions/v<installed>-observed/`
snapshot + green suite is its committed proof.
