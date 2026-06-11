# Pre-Build Review — Three Architect Verdicts (2026-06-11)

Three independent reviews of specs 01–03 before any code: performance (G0),
over-engineering, and contract correctness. Full reports in session transcript;
this is the actionable record. **Status: amendments pending discussion.**

## Verdicts

| Lens | Grade | One-liner |
|---|---|---|
| Performance (G0) | **AMBER** | Read path is G0-clean; the write/compact path has 3 fixable design faults |
| Over-engineering | **STUFFED** (engineering layer, not features) | Phase ② as specced ≈ 10 engineer-weeks vs the claimed 1–2; cut to a 3-week slice |
| Contract correctness | 9 contradictions, 7 gaps | All resolvable on paper; biggest gap: Phase ① mock was never specced |

## Performance: required before Phase ② (all verified against code)

1. **Two-tier incremental compaction + memory gate.** Per-save whole-graph
   re-resolution = ~1s CPU + 60–100MB transient heap at 200k deps (breaks the
   50MB ceiling). Default to per-file delta resolution; whole-graph only on
   manifest change (go.mod/package.json/tsconfig). Benchmark gate must measure
   max-RSS-during-compact on a 30k-file fixture.
2. **Locking law: no facts/arch bbolt write under App.mu.** The proposed watcher
   hook sits inside `onFileChanged`'s critical section (watcher.go:43); a 1s
   compact tx would convoy SaveLearnerState → freeze every App.mu path.
   Enqueue to one background facts-writer outside the lock.
3. **Daemon-coexistence fix.** bbolt flock semantics (verified v1.4.3): a
   read-only open CANNOT coexist with the running daemon — spec 02's direct
   path is broken as written. Everything goes daemon-first; direct open only
   when daemon is down. Plus: arch ETag needs a facts-scoped revision (the
   global revision bumps on every search → polling never 304s while working).
4. Also: fixed struct fields for common fact attrs (map-based Attrs ≈ double
   the alloc gate), one timestamp per pass, hand-rolled JSONL encoder.

**What's right:** pre-marshaled shard reads (<1ms, real), emission riding the
parse pass (hook points verified accurate), detectors at compact never render,
lazy boot (+0ms startup verified), search hot path untouched (lock-free read
path confirmed).

## Over-engineering: the cut list (ranked)

1. **Phase ② does NOT need the universal Fact substrate.** Merge into the
   existing Index: `FileMeta.Imports []ImportRef` + one resolved `DepAdjacency`
   (in-memory, rebuilt at debounce). Kills 3 new ports, 7+ buckets, the JSONL
   staging protocol, the second watcher path — incremental comes free because
   FileMeta is already swapped per file. Introduce the generic Fact model in
   Phase ③ when a second fact kind actually exists. (This also dissolves most
   of perf findings F1/F2 — the write pressure largely disappears.)
2. **All visualization tasks defer to ③.** The existing viewer is fully
   data-driven (`?model=`); `aoa arch view component > shard.json` demos in it
   today with zero web-adapter work.
3. **Drop byte-parity with the Python generator.** Downgrade to "loads in the
   viewer + schema-validates." Keep byte-stability of Go's OWN output for
   golden fixtures.
4. **Phase ② views: 7 → 3+1.** component/dsm/cycles (one dataset, three
   renditions) + domains (enricher exists). Cut sbom (hidden sub-project),
   code (undefined derivation), shrink techportfolio.
5. **Journeys/Yen's defer; `derive` ships later as single BFS + the
   first-class empty-path answer.**
6. **Evidence packs: delete implementation detail; revisit post-③ with a buyer.**
7. **Conformance stays ④ at sketch grade** — stop designing it to
   implementation grade two phases early.
8. Viewer bundle: build-tag out of `--light` (infra exists: `!lean` tags);
   +2.2MB embedded bytes on the full binary is fine, lazy-download is not.

**Minimum lovable Phase ② (~3 weeks honest):** keystone extraction (Go/Py/TS)
riding the parse pass ≤+3% · `aoa arch view component|dsm|cycles` + `facts
<unit>` (the audit trail = the moat) · 3 compact detectors + the self-test
invariant (aOa's own domain→adapters edge set is empty) · exit gate: stranger
repo → views load in the existing viewer + CLAUDE.md guidance block ships.

## Contract fixes (apply when amending specs)

- C1: one type — `ports.FactSource` (02 adopts).
- C2: one detector home — graph-shape detectors in domain/facts emitting
  finding-facts; presentation/declaration detectors in domain/arch reading them.
  One severity/threshold table.
- C3: two distinct features, one name each: `baseline save` (facts/delta) and
  `findings --freeze` (fingerprints); delete `baseline set` from 03.
- C4: adopt 03's layout — viewer page at `/arch/`, data at `/api/arch/*`.
- C5: one cache bucket (`arch_shards`), revision-suffixed keys.
- C6: findings JSON = 02's struct (`message/subjects/sources`); 03 renames.
- C7: manifest carries renderable views only in ②; viewer catalog constants
  handle "planned" (no viewer change).
- C8: journeys served no-cache+ETag (no hash exists in their manifest entries).
- C9: 03's trigger list adds debounced watch compaction (the touch-one-package
  demo depends on it).
- Gaps to write: facts revision definition (NOT the global revision), bulk
  graph read on the store interface, light-build "dep facts unavailable"
  surfacing, `view --all --out` for the lint CI.

## The Phase ① question (decision needed)

Checkpoint #4 promised a playbook mock (facts JSONL → substrate → renditions +
touch-one-package demo). No spec exists for it, and the trimmed Phase ② is now
small enough that the mock would mostly duplicate it in Python. Recommendation:
**fold checkpoint #4's proof into the trimmed Phase ② exit gate** — the
touch-one-package demo runs against real aOa (edit a file → only affected
shards change), which is a stronger proof than a mock. Derive-a-journey moves
to Phase ③ with the substrate generalization.
