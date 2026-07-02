# 16 — Gaps in Test Strategy

**Status:** the test-coverage companion to the integration board (`00-INTEGRATION-BOARD.md`).
No code changes — this names *where the existing test strategy does not yet cover the
rollout*, so each gap becomes a base exit-gate criterion rather than a surprise later.

**The structural reason there are gaps:** aOa's current test contract is **behavioral
parity against Python** (fixtures in `test/fixtures/` are the source of truth, per
CLAUDE.md). The entire arch layer (E1–E7, P1–P6) is **net-new — there is no Python to
parity against.** So the contract shifts from *parity* to **determinism + golden fixtures
+ invariants + benchmarks + one safety test**. Every gap below is an instance of that shift.

**Gap type legend:** 🟥 blocker (a base cannot pass its gate without it) · 🟧 important ·
🟦 hardening. **Mapped to the base it gates.**

---

## The gap ledger

| # | Gap | Type | Base | Goal at risk | What the test must do |
|---|-----|------|------|-------------|----------------------|
| T1 | **G0 keystone benchmark** — the ≤+3% build budget is *asserted, never measured*. No build-time benchmark of parse-with-edges vs without exists. | 🟥 | 1 | G0 | Benchmark `BuildIndex` before/after the import-extraction concern on a large real repo (not a no-op baseline). **This is the single make-or-break test** — it decides whether E1 rides the always-on parse or falls back to opt-in/async. |
| T2 | **Edge-store per-file invalidation** — keyed-by-file delete (`Delete(fileIDKey)`) must drop exactly one file's edges with no orphans, in O(edges-for-file). No `store_test` covers edges. | 🟥 | 1 | G0/G4 | Save edges for a 3-file repo; delete one file's key; assert the other two intact, the deleted file's gone, and the layout never full-scans. JSON round-trip parity beside `store_test.go`. |
| T3 | **Freshness / ETag regression** — today a save reindexes but does **not** bump the ETag (`bumpRevision` absent from `onFileChanged`/`Reindex`); the viewer serves a stale 304. No test guards this. | 🟥 | 1 | G0/G6 | Edit a file (incl. one yielding **zero symbols** — the path the prior draft missed) → `revision` increments same tick → next `GET` with old `If-None-Match` returns **200 + new ETag**, not stale 304. Pattern on the existing ETag test. |
| T4 | **Determinism / golden shards** — no Python parity exists for renditions; nothing asserts "same repo → same edges/shards." | 🟥 | 2 | G1 (reframed) | Byte-stable golden fixtures: each renderer on a fixed edge-set emits deterministic shard JSON with a **stable content hash**; re-run yields byte-identical hash (so the ETag/304 short-circuit holds). Replaces parity for this layer. |
| T5 | **Detector correctness fixtures** — Tarjan/god/orphan have no test fixtures. | 🟧 | 2 | G0 | On a fixture with a known 3-module cycle + one god-node + one orphan: assert Tarjan returns exactly that SCC, degree detector flags the god-node, orphan detector flags the isolated file — all `file:line`-stamped, all deterministic. |
| T6 | **graph→peek handoff + import-edge asymmetry** — the load-bearing claim that a graph result resolves to a peekable body (and that import-edge *targets* resolve to package, not body) is unverified by any test. | 🟧 | 2 | G3 | Assert a resolved intra-repo edge peeks **both** ends to a method body via `TokenRef`→`Metadata`→`StartLine-EndLine`; assert an external import edge peeks the **source site** but resolves the target to package grain (honestly stamped), not a phantom body. (Design in `13`.) |
| T7 | **MCP-vs-CLI contract parity** — the 4-tool MCP surface must return results identical to the matching `aoa arch` CLI call; no contract test. | 🟧 | 2 | G3 | Assert the MCP server enumerates **exactly 4** tools; each returns a `file:line:commit` result byte-equal to its CLI sibling; blast-radius reflects a freshly git-changed file with no stale graph. |
| T8 | **`aoa arch` three-mode parity** — like `grep`/`peek`, must give consistent output/exit codes in direct/pipe/daemon modes. No test. | 🟦 | 2 | G3 | Run each `aoa arch *` subcommand in all three modes; assert identical output + exit codes; the unknown-method fallback (`server.go:228`) stays untouched for non-arch methods. |
| T9 | **Blind-judge gate on DERIVED data** — the gate exists but runs against the **mock/proxy** generator, not real derived shards. | 🟥 | 3 | G6 | Port the blind judge (`MODEL-STANDARD.md:43-53`) to screenshot the **daemon-served** `/arch/` URL fed by real derived shards; a view passes only if the judge answers its question from the image alone. (Currently mock-only.) |
| T10 | **Touch-one-package proof (the integration exit gate)** — no end-to-end test proves incremental correctness on a stranger repo. | 🟥 | 3 | G0/G6 | Clone a stranger repo → `aoa init` → 4 views render REAL-stamped → edit **one** package → assert **only the affected shards change** (hashes of untouched scopes stable). This is the L19.5 exit gate. |
| T11 | **The self-test invariant as CI regression** — aOa's own `domain → adapters` edge set should be **empty** (hexagonal). Proposed, never wired. | 🟧 | 2/3 | G4 | A CI test that runs the cycle/boundary detector on aOa itself and asserts the domain→adapters violation set is empty — arch eating its own dog food as a standing regression. |
| T12 | **Leash safety test (CRITICAL)** — nothing asserts the service **never** initiates an LLM call on a derive path, or that the overlay loader rejects invented ids. | 🟥 | 4 | scope-law/G0 | Assert: (a) no derive path makes a network/LLM call (the model runs only in the outside CLI/MCP layer); (b) the overlay loader **rejects any id not in the facts** with a warning fact and drops the view to **MIXED**, never REAL. The single most important *safety* test in the plan. |
| T13 | **Diff renderer golden fixtures** — Mode A/B before/after diffs have no fixtures. | 🟧 | 4 | G1(reframed) | Golden before/after edge-sets → deterministic delta (new cycles / blast / new findings) from pure set arithmetic; assert no LLM in the path; conformance edge classes (convergent/divergent/**absent**) render with the existing `e.tag` machinery. |
| T14 | **Conformance classification** — declared-vs-derived diff (`arch.yaml`) is untested. | 🟦 | 4 | G6 | On a declared pattern + a known-divergent repo, assert correct convergent/divergent/absent edge classification and baseline/freeze "report only NEW drift" semantics. |
| T15 | **Scale / perf regression for arch** — `scale_gauntlet` covers search, not arch. Max-RSS-during-compact and sub-ms arch reads on a 30k-file fixture are ungated. | 🟧 | 2/3 | G0 | Extend the gauntlet: assert arch reads stay sub-ms and **max-RSS-during-compact** stays within budget on the 30k-file / 1.2M-symbol fixture; edge memory grows ~linearly. |
| T16 | **Vendored-bundle fork-guard** — the viewer extracted to `static/arch/` must not drift from the source generator, and the offline ESM bundle must stay in budget. | 🟦 | 3 | G0/maintenance | Hash-compare test between the extracted `viewer.js/html/css` and the generator source (fork-guard); assert the vendored esbuild bundle drops the esm.sh CDN imports and stays ≤2.2MB raw / ≤650KB gz (elkjs dominates ~1.5MB). |

**Added by the integration-pattern validation swarm (2026-06-21 — see board §0):**

| # | Gap | Type | Base | Goal at risk | What the test must do |
|---|-----|------|------|-------------|----------------------|
| T17 | **Lock-ordering assertion** — no `db.Update` ever runs while `App.mu` is held (arch OR index path). The existing `markIndexDirty`→`SaveIndex`-under-mutex pattern (`app.go:838-846`) must be **fixed, not copied**. | 🟥 | 1 | G0/G4 | Instrument or wrap the store so any `db.Update` entered with `a.mu` held fails the test; run the watcher + search paths through it. |
| T18 | **Burst write-pressure test** — combined write wall during a multi-file event storm is unbudgeted (each writer only budgeted in isolation; bbolt is single-writer). | 🟥 | 1 | G0 | Simulate a branch switch (~200 file events <1s): assert total write-tx wall bounded, per-window coalescing works (one tx per debounce window), and reads stay sub-ms *during* the burst. |
| T19 | **Both-direction schema migration** — new buckets (`edges`, `arch_shards`, baselines) have no `_version` byte; old-binary-opens-new-DB after rollback is undefined (index-wipe risk). | 🟥 | 1 | G4/data-safety | Open (i) a pre-arch DB with the new binary and (ii) a post-arch DB with the prior binary: both degrade gracefully (missing bucket = empty; version mismatch = drop-and-re-derive), **never** trip index self-recovery. |
| T20 | **`-race` soak** — new shared mutable state (shard cache, adjacency cache) read lock-free by socket/web while the watcher invalidates and compact rewrites; zero race coverage today. | 🟧 | 3 | G0 correctness | N seconds of concurrent socket reads + watcher writes + forced compacts under `go test -race`: zero races; every served shard is a consistent snapshot (no torn read across a revision bump). |
| T21 | **Fuzz the trust boundary** — the overlay loader and JSONL fact reader parse *untrusted model-generated JSON* inside the live daemon; security-relevant, unfuzzed. | 🟧 | 4 | leash/security | Go fuzz targets for the overlay parser + fact reader: no panics, invented ids always rejected, malformed input never mutates state. |
| T22 | **Property-based graph invariants** — golden fixtures catch regressions, not classes of bugs. | 🟦 | 2 | G1(reframed) | Properties: every edge endpoint resolves to a live node; cycles ⊆ SCCs; DSM = adjacency of the edge set; N single-file edits ≡ one full rebuild (incremental-equivalence). |

**Added by the L21 phase checkpoint (2026-07-02 — see `checkpoint-L21.md`).** L21 (daemon
resilience) is operational hardening outside the arch matrix: **none of T1–T22 applied**;
its exit gate was the phase-specific E-1…E-9 set (kickoff-L21 §6) — all 9 PASS on live
evidence, verdict **Conditional** pending T23. The checkpoint panel exposed these NEW gaps
("Base" = the L21 surface, not a rollout base):

| # | Gap | Type | Base | Goal at risk | What the test must do |
|---|-----|------|------|-------------|----------------------|
| T23 | **E-2 flock single-spawn not locked by the suite** — `TestDaemonEnsure_RevivesOnce` computes the pgrep count but asserts only health-exit-0 (a double-spawn passes); it also calls `runAOA` (`t.Fatalf`) from spawned goroutines. Live storm proved one spawn; the suite doesn't. | 🟥 | L21 | G0/G4 | Assert exactly one spawn deterministically (count "daemon starting" lines in `.aoa/log/daemon.log`, not pgrep); route goroutine errors via channel to the test goroutine. **Punch P2 — gates L21 Tested.** |
| T24 | **Revive-failure path untested** — `reviveDaemon` takes `LOCK_EX` with no `LOCK_NB`/timeout/backoff; if `.aoa` exists but the daemon can't start (corrupt DB, port conflict), callers queue serially at ~15s each while the status-line hook enqueues ~1/s → unbounded process pileup. Only the happy path has evidence. | 🟧 | L21 | G0/G6 | Simulate an unstartable daemon; assert ensure exits fast when another reviver holds the lock (`LOCK_NB`) and repeated failures back off — no queue growth over N hook ticks. |
| T25 | **Version-skew guard has zero coverage** — no test feeds a `DaemonOK`-less (old-daemon) health response through `runHealth`; the derive predicate (`ok\|\|recovered`) is duplicated server-side (`server.go:309`) and client-side (`health.go:50-57`) and can silently drift. | 🟧 | L21 | G3 | Feed an old-daemon JSON payload through the CLI health path; assert tri-state derives from `Status` both directions; extract one shared helper so the predicate exists once. |
| T26 | **Live-daemon Web probe is a startup snapshot** — `webOK := a.WebServer.Port() > 0` (`app.go:497`) is true forever after bind; a died HTTP listener still reports `Web: up`. Only the dead-daemon path actually dials. | 🟧 | L21 | G6 | Kill the web listener under a live daemon → `aoa health` reports `Web: down` (probe closure performs a real 200ms localhost dial, like the dead path). |
| T27 | **L21 assertion-tightening grab-bag** — `TestGrep_DeadDaemon_Revives` never asserts semantic stdout (revive-then-empty passes); stale-status tested only at 3h, not the 60s boundary; `aoa remove` vs hook-backgrounded `ensure` race can resurrect `.aoa` mid-removal (no intentional-stop marker); wedged-tailer staleness under a live daemon unflagged. | 🟦 | L21 | G3/G6 | Add `strings.Contains(stdout, "hello")` to the revive test; test the 60s threshold edge both sides; test that `remove` completes with the hook firing (no `.aoa` resurrection); consider a stop-marker. |

---

## Reading the gaps

- **The blockers cluster at the base gates, exactly where they should.** Base 1 cannot
  pass without T1–T3; Base 2 without T4; Base 3 without T9–T10; Base 4 without T12. Fold
  each into its base's `Tested` gate in `00-INTEGRATION-BOARD.md` §4.
- **T1 is the keystone of the keystone.** If the G0 benchmark fails, the whole sequence
  re-plans (E1 moves off the always-on pass). Run it *first*, before committing build effort.
- **T12 is the one safety test that protects the brand promise** — "the agent suggests,
  never mutates." It is cheap and non-negotiable for any Base-4 feature.
- **The parity→determinism shift (T4, T13) is a strategy change, not just a gap:** the
  arch layer's correctness contract is golden-fixture byte-stability, since there is no
  Python oracle. State this in the test docs so a future contributor doesn't look for
  parity fixtures that will never exist.

**Maintenance-free-for-the-user goal:** T3 (freshness), T10 (incremental correctness),
and T16 (no CDN / offline bundle) are the tests that keep the product *self-maintaining* —
diagrams that never go stale, never need a manual rebuild, and never phone home. They are
the ones to never let regress.
