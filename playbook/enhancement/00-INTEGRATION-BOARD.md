# 00 — Integration Board: how all of this rolls into aOa

**Status:** integration plan — **no code changes**; this determines the *order and
gates* for rolling the enhancement pool (`01`–`17`) into the product without
disrupting the existing goals. **Read this first; read the detail docs later** —
every row links to its source. Test gaps are catalogued separately in `16-gaps-in-test-strategy.md`.

**Rev 2 (2026-06-21)** — folds in: the integration-pattern validation swarm's findings
(§0 runtime contract — the two blockers + operational hardening), **daemon resilience /
lazy-start** (DR — new element from the live outage), the grep+graph+peek toolkit docs
(`13`–`15`), and doc `17`'s goal-litmus as the governing self-check.

**The principle (your layered approach, stated once):** **substrate first, features
last.** We get the infrastructure *in → integrated → working → tested* before any
feature rides on it. Each of the four bases below passes the same four maturity
gates, and **a base does not start until the prior base is `Tested`.** Nothing
user-facing ships on an unverified substrate.

```
        in ──▶ integrated ──▶ working ──▶ tested ──▶ (next base starts)
```

**Binding rule:** every element is gated by the pre-existing goals (`.context/GOALS.md`).
The goal-alignment matrix (§3) is the "do not disrupt" contract — if an element can't
pass it, it doesn't roll in.

---

## 0. Runtime Integration Contract (cross-cutting — binds every base)

The validation swarm's verdict: the board was a *build-order* plan and nearly silent on
the **runtime/operational axis** — exactly where grafting a derived-data plane onto a
live single-writer-bbolt daemon goes wrong. These four laws bind **every** element below;
the two blockers are Base-1 `Tested` criteria, not suggestions.

| # | Law | Why (code-verified) | Gate |
|---|-----|---------------------|------|
| C1 | **Lock-ordering: no `db.Update` under `App.mu`.** Snapshot state under `a.mu`, *release*, then write. | The precedent the arch plane would copy does the opposite — `markIndexDirty` runs `SaveIndex` (full `db.Update`) *holding* `a.mu` (`app.go:838-846`); the watcher holds `a.mu` across its whole handler (`watcher.go:43-44`). Copying it serializes edge writes against live searches — the exact G0 regression this plan exists to prevent. **Fix the existing pattern; don't match it.** | `16` T17 (blocker) |
| C2 | **Burst write budget + coalescing.** Per-file fact swaps within one debounce window batch into a single write tx (shared with the resolved-rewrite + findings write). | bbolt is single-writer process-wide. A branch switch / format-on-save-all fires N per-file txs + index tx + recompute + findings, all stacked behind one writer lock — precisely when the user opens the "what changed?" viewer. Budgeting writers in isolation misses the combined burst. | `16` T18 (blocker) |
| C3 | **Schema version + both-direction rollback on every new bucket** (`edges`, `arch_shards`, baselines): missing bucket reads as empty (expand phase); version-mismatch → drop-and-re-derive (shards are pure cache). | The index bucket self-heals via `_version` (`store.go:24,88-94`); the new buckets have no story. The dangerous case: an **old binary opening a post-arch DB after rollback** — undefined today, could trip self-recovery and wipe the index. | `16` T19 |
| C4 | **Kill switch / dark launch.** Edge emission behind a default-on `AOA_ARCH` flag (env + `.aoa/config`), evaluated once at parser construction; also fences `aoa arch` registration and `/api/arch/*`. | E1 rides the always-on parse in the shipped binary. If a pathological field repo blows the +3% budget, the only remedy without a flag is rebuild-and-redeploy. One construction-time check = zero hot-path cost; each base can ship dark. | Base-1 benchmark runs flag-on vs flag-off |

Hardening (fold into `16`, not blocking): `-race` soak under concurrent socket-read +
watcher-write + compact (T20) · fuzz the overlay loader + JSONL fact reader — untrusted
model JSON crossing into the live daemon (T21) · property-based graph invariants +
multi-edit incremental-equivalence (T22) · compact/derive observability on the existing
telemetry surface · MCP tool annotations/version negotiation.

*(Adjudicated false alarms — deliberately NOT adopted: walking-skeleton re-sequencing,
anti-corruption layer, branch-by-abstraction, Mikado, mutation testing, a standalone
rollback narrative — the board or existing machinery already covers them.)*

---

## 1. The four bases (layers)

| Base | Layer | What lands here | Why it's the floor for the next | Maps to L19 phase |
|---|---|---|---|---|
| **Base 1** | **Substrate Infrastructure** | E1 keystone import-edges · E2 EdgeStore · E3 freshness wiring | No relation is representable today (`Index` is three node-maps, `storage.go:59-63`). Until edges exist and persist, *everything downstream is empty*. | ① / ② keystone |
| **Base 2** | **Derivation & Access** | E4 renderers · E5 detectors · E6 `aoa arch` CLI+socket · E7 thin MCP · the grep+graph+peek toolkit (`13`/`14`) | Turns raw edges into queryable/renderable answers. The agent + CLI surface the substrate; nothing visual works without renditions. | ② GREEN views |
| **Base 3** | **Visualization Surface** | P1 viewer-in-product · P2/P3 real-time live diagram | The human face. Renders what Base 2 derives, fresh on save. The stranger-repo exit gate lives here. | ②→③ |
| **Base 4** | **Competitive Features** | P4 before/after diff · P5 interactive Claude loop · conformance · evidence packs · P6 streaming | The headline differentiators. They *compose* the lower bases — none is buildable until the substrate, access, and surface are tested. | ③ / ④ |

Engine tasks **E1** and **E3** can land in parallel (E3 is independent — one `defer` line).
Everything else is strictly dependency-ordered.

---

## 2. The master element table (scan the ratings; read details later)

**Competitive rating** — ★★★ headline differentiator (a moat) · ★★ strong edge ·
★ necessary/parity · ⚙ enabler (invisible, rated by leverage) · ✗ out of scope.
**State** — 🟢 built today · 🔵 proposed (net-new) · 🟡 partly built.

| ID | Element | Base | State | Depends on | Effort | **Compete** | Detail |
|----|---------|------|-------|-----------|--------|:----------:|--------|
| DR | **Daemon resilience — lazy-start** (dead socket → `flock`-guarded auto-start at point of use; status-line fire-and-forget revive; tri-state `aoa health` daemon/db/web) | 1 (parallel — main-line now) | 🔵 | — (reuses `spawnDaemon`) | S | ⚙ *maintenance-free* | session design 2026-06-21 |
| E1 | Import-edge extraction (always-on parse) | 1 | 🔵 | — | M | ⚙ *highest-leverage* | `12` E1 · `01` |
| E2 | `ports.EdgeStore` + bbolt `arch_shards`/`edges` (keyed-by-file) | 1 | 🔵 | E1 | S–M | ⚙ | `12` E2 · `02` |
| E3 | `bumpRevision` freshness wiring (one `defer`) | 1 | 🔵 | — | S | ⚙ → unlocks ★★ | `12` E3 |
| E4 | `internal/domain/arch` renderers (component/dsm/cycles) | 2 | 🔵 | E2 | M | ★★ | `12` E4 · `02` |
| E5 | Detectors at compact-time (Tarjan/god/orphan) | 2 | 🔵 | E2 | S–M | ★★ | `12` E5 |
| E6 | `aoa arch` CLI + socket `MethodArch*` | 2 | 🔵 | E4,E5 | M | ★★ | `12` E6 · `03` |
| TK | Unified **grep + graph + peek** toolkit + CLAUDE.md guidance | 2 | 🟡 (grep/peek 🟢) | E6 | S | ★★ | `13` · `14` |
| E7 | Thin MCP adapter (exactly 4 grep-beaters) | 2 | 🔵 | E6 | S | ★★ *diligence* | `12` E7 · `03` |
| P1 | Viewer-in-product (`/api/arch/*`, vendored bundle) | 3 | 🟡 (viewer 🟢 as mockup) | E4,E5 | M | ★★★ | `12` P1 · `03` |
| P2/P3 | Real-time live diagram (refresh-on-save, ETag poll) | 3 | 🔵 | E3,P1 | S | ★★ | `12` P2/P3 · `08` |
| P4 | **Before/after diff** (overlay loader, Mode A→B) | 4 | 🔵 | E1,E4,E5,P1 | M–L | ★★★ *the wedge* | `12` P4 · `08` |
| P5 | Interactive **click→fact-pack→Claude** loop (leash) | 4 | 🔵 | E6,P4 | M | ★★★ | `12` P5 · `08` |
| CF | Conformance (declared pattern vs derived actual) | 4 | 🔵 | E4,E5 | M | ★★★ *the money* | `07` · `09` |
| EP | Evidence packs (DD / PCI / SOC2 / "what changed") | 4 | 🔵 | CF,P4 | M | ★★ *the money* | `09` · guide §6 |
| PV | Provenance on every answer (`file:line:commit`) | all | 🔵 (cross-cutting) | E1 | — | ★★ *cleanest gap* | `07` · `10` |
| RW | Recon-weighted prominence (hotspots) | 4 | 🟡 (recon 🟢) | E4 + recon | S | ★★ *option value* | `07` · `10` |
| P6 | AG-UI / MCP-App streaming (`STATE_DELTA`, `ui://`) | 4 | 🔵 | E7,P5 | S | ★ | `12` P6 |
| ✗ | Mode C — autonomous worktree rewrite | — | ✗ | — | — | ✗ *leash/OUT* | `08` §8.10 |

**Scan view — the competitive spine, sorted:**
- **★★★ (build the company on these):** P1 living arch viewer · P4 before/after diff · P5 interactive Claude loop · CF conformance.
- **★★ (real edges, mostly diligence/retention):** E4/E5 renderers+detectors · E6/TK agent toolkit · E7 thin MCP · P2/P3 freshness · EP evidence packs · PV provenance · RW recon.
- **⚙ (invisible, but everything depends on them):** E1 · E2 · E3 · DR. *Highest leverage in the whole plan — E1 is the single gate; DR is independent and can land on main-line today.*
- **★ / ✗:** P6 streaming (last, thin) · Mode C (off the ladder).

> **Why DR is on a competitive board:** "maintenance-free for the user" is part of the
> value prop (`16` closing note). A dead daemon silently kills freshness (Moat A), the
> live viewer, and the whole "never stale" claim — graphify's core weakness becomes ours
> if the daemon can die for three days unnoticed (observed 2026-06-21: stale
> `status.json`, no revive path). Lazy-start is the portable fix; guardian/systemd-run
> are optional escalations, deliberately deferred (YAGNI).

---

## 3. Goal-alignment guardrail (the "do not disrupt" contract)

Every element passes this before it rolls in. Derived from `.context/GOALS.md`,
verified in `12-onboarding-plan.md`. **How it's applied:** the per-goal **litmus**
(doc `17` §2 — yes/no self-check, inline, zero machinery) is the default gate on every
change; the parked escalation agents (`17` §3) fire only for new-subsystem /
concurrency / release moments. Pending user decision in `17` §4: G1 refine
(parity → correctness-contract) + proposed G7 Provenance / G8 Verifiable Quality.

| Goal | The guardrail this rollout holds | Where enforced |
|---|---|---|
| **G0 Speed** | Edges emit *in the existing parse* (≤+3% build, **measured at the Base-1 gate** — not asserted); renderers/detectors run at compact-time, never the keystroke hot path; arch reads are O(1)/O(edges) bucket gets sub-ms; `bumpRevision` is an atomic `.Add(1)`. | E1/E4/E5/E3 exit checks |
| **G1 Parity** | The arch layer is **net-new — there is no Python to parity against**, so the test contract is *determinism + golden fixtures*, not byte-parity. (This is a test-strategy shift → `16`.) | `16` §determinism |
| **G2 Two-binary** | Keystone rides the **always-on parse**, never the recon walker (`WalkForDimensions`/`countImportSpecs` is `//go:build core`); `aoa arch` + MCP ship in base `aoa`; recon weighting (RW) is strictly additive-optional. | E1/E6/E7 G2 notes |
| **G3 Agent-First** | `aoa arch` works in all three modes (direct/pipe/daemon) like `grep`/`peek`; **grep+peek stay primary**, the graph adds only relational verbs that terminate in peek. | E6 · `14` |
| **G4 Hexagonal** | `ports.EdgeStore` defined first; `internal/domain/arch` is dependency-free (plain `ports` types in/out); bbolt/tree-sitter/web stay in adapters; **lock-ordering law §0-C1** (no `db.Update` under `App.mu`). | E2/E4 G4 notes · §0 |
| **G5 Self-Learning** | Recon stays its own product; its signal feeds arch only as an *additive overlay* that drops the view to MIXED — never required for a REAL answer. | RW |
| **G6 Value-Proof** | Provenance on every answer + evidence packs surface measurable, auditable value. | PV · EP |

**Leash law (scope-law, CONFIRMED clean):** every agent action SUGGESTS, never mutates
the substrate. Mode A overlay loader **rejects invented ids** and drops the view to
MIXED; the Claude loop runs the model **outside** the derive path (CLI subprocess / MCP
host), with `recon-investigate` (`recon.go:556`) as the verified live precedent. No LLM
call on any fact path → G0 intact.

---

## 4. Per-base exit gates (the in → integrated → working → tested ladder)

**Base 1 — Substrate Infrastructure.** *Tested before Base 2 starts.*
- **In:** import extractor emits `ImportEdge` for Go/Python/JS-TS-TSX; `EdgeStore` port + `edges` bucket; `bumpRevision` defer line.
- **Integrated:** edges persist keyed-by-file; `onFileChanged` invalidates only the edited file's edges; ETag bumps on every save (incl. zero-symbol).
- **Working:** `BuildIndex` on a real Go repo yields one REAL `file:line`-stamped edge per import.
- **Tested:** **G0 benchmark ≤+3%, run flag-on vs flag-off** (the make-or-break — `16` T1);
  per-file delete round-trip; ETag-200-not-stale-304 regression; **§0 contract gates:**
  no-`db.Update`-under-`App.mu` assertion (T17), burst test — ~200 file events <1s, reads
  stay sub-ms during it (T18), both-direction old/new-binary DB open (T19).

**Base 2 — Derivation & Access.** *Tested before Base 3.*
- **In:** `internal/domain/arch` renderers + detectors; `aoa arch` CLI + `MethodArch*`; 4-tool MCP; CLAUDE.md grep+graph+peek guidance block.
- **Integrated:** renderers read the edge bucket; detectors run at compact-time; CLI/MCP wrap the same domain via the socket.
- **Working:** `aoa arch cycles|view|derive` returns deterministic `file:line` results <1 ms; MCP enumerates exactly 4 tools matching the CLI.
- **Tested:** golden-fixture determinism (byte-stable shard hash); known-cycle/god/orphan detector fixtures; graph→peek handoff + import-edge asymmetry test; MCP-vs-CLI contract parity (`16`).

**Base 3 — Visualization Surface.** *Tested before Base 4.*
- **In:** viewer extracted to `static/arch/` (fork-guard); `/api/arch/*` handler; ETag-poll refresh.
- **Integrated:** viewer reads `arch_shards` bucket; refresh-on-save live via E3.
- **Working:** stranger repo → `aoa init` → component/dsm/cycles render REAL-stamped.
- **Tested:** **blind-judge gate on DERIVED data** (not mocks); **touch-one-package proof** (edit one package → only affected shards change); vendored-bundle ≤2.2MB / ≤650KB gz; `-race` soak — concurrent socket reads + watcher writes + forced compacts (T20) (`16`).

**Base 4 — Competitive Features.** *Each feature gated on the three bases below it.*
- **In:** Mode A overlay loader + diff renderer; click→fact-pack→Claude loop; conformance diff; evidence-pack export.
- **Integrated:** overlay rejects invented ids (leash); fact-pack rides `MethodArch*`; conformance diffs `arch.yaml` against derived edges.
- **Working:** side-by-side BEFORE/AFTER from a Claude-proposed plan; a DD/PCI pack exports end-to-end.
- **Tested:** **leash safety test** (service never initiates an LLM call; overlay→MIXED); diff golden fixtures; conformance convergent/divergent/absent classification (`16`).

---

## 5. Critical path & sequencing

```
DR ── (independent — main-line, can ship today)
E1 ─┬─▶ E2 ─┬─▶ E4 ─┬─▶ E6 ─▶ E7        ── Base 1 → Base 2
    │       │       └─▶ E5 ─┘
E3 ─┘  (parallel)            │
                             ▼
              P1 ─▶ P2/P3                ── Base 3
                     │
                     ▼
              P4 ─▶ P5 ─▶ {CF, EP} ─▶ P6 ── Base 4   (Mode C: OUT)
```

**The one gate that governs everything: E1 landing inside G0 ≤+3%.** It is asserted,
not yet measured — so the **first action is the Base-1 benchmark**, before committing
the sequence. If E1 can't ride the always-on parse within budget, the fallback (opt-in /
async derive) reshapes Bases 3–4. Everything else is low-risk additive wiring.

---

## 6. How this connects to the real project board

- This board is the **rollout view**; the engineering detail per element is `12-onboarding-plan.md`;
  the per-element specs are `playbook/integration/01-03`.
- Bases 1–2 = the existing **L19.1–L19.4** (keystone → renditions → detectors → CLI);
  Base 3 = **L19.5** (stranger-repo exit gate); Base 4 = the L19 Phase ③/④ tail + the
  moat/feature work in `06`–`10`.
- **Recommended next board action (Beacon):** fold Bases 1–3 into L19 sub-IDs as the
  ordered build, and open a new layer for Base 4 (competitive features) gated on L19.5.

**Detail map:** substrate `01`/`02` · access `03`/`13`/`14` · viz `08`/`15` · moats/positioning
`06`/`07`/`09`/`10` · onboarding `12` · accuracy ledger `11` · **test gaps `16`** ·
**process guardrails (litmus + escalation agents) `17`**.

**Branch discipline:** this pool is docs on `playbook`. All *code* (DR, E1, everything
below it) lands on a fresh branch off **main** — never on the docs branch.
