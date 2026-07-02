# Integration Touchpoints — The Real-Code Seams (Deep Dive)

**Status:** integration reference — falsifiable. Every load-bearing claim cites a
`file:line`. No code changes are prescribed; this is a NO-CODE-CHANGES map of where
the architecture-views feature *attaches* to the binary that already exists. If a
cited anchor is wrong, the claim built on it is void.

**Binding law (do not relitigate here — see the parent guide):**
- **Scope law** — `.context/decisions/2026-06-11-core-competence-and-scope-line.md`
- **Goals** — `.context/GOALS.md` (G0 Speed ≤+3% build · G2 Two-Binary split · G3 Agent-First · G4 Hexagonal)
- **Companion specs** — `playbook/integration/01-facts-substrate.md` (data plane), `02-arch-service.md` (service plane), `03-visualization.md` (presentation plane)
- **Parent guide** — `playbook/enhancement/01-*` / `playbook/ENHANCEMENT-GUIDE.md` (the spine, could-vs-should, access surface, language ladder)

This document answers one question precisely: **where does the feature touch real
code, and what is the constraint at each touch?** Six pre-existing extension
points. Five are additive to machinery aOa already runs — zero architectural
inversion. One is the keystone, and it gates the other five.

---

## 0. The map — six seams, one keystone

```
                          ┌─────────────────────────────────────────────┐
   AGENT  ──socket──▶  ① socket method switch  ─┐                        │
   (CLI)  ──exec────▶  ⑤ cobra `aoa arch` ──────┤                        │
   HUMAN  ──http────▶  ② web route + viewer ─────┤── read ──▶  EDGE STORE │
                                                 │             (bbolt ④)  │
                                                 │                ▲       │
   FILE EDIT ──fsnotify──▶ ⑥ onFileChanged ──────┴── write ───────┘       │
                                  │                                       │
                                  ▼                                       │
                       ③ PARSE PASS (KEYSTONE) ── emits edges in-pass ────┘
                          ParseFileToMeta → extractSymbols
```

| # | Seam | What it does today | Net-new | Hardest constraint |
|---|------|--------------------|---------|--------------------|
| 1 | Socket method switch | flat JSON method dispatch | `arch.*` case arms + handlers; MCP rides alongside | additive only; delegate, don't reach into domain |
| 2 | Web route + viewer | ETag-gated JSON API + embedded shell | `GET /api/arch/*` + embed the viewer at `GET /arch` | localhost-only; build-neutral bytes |
| 3 | **Parse pass (KEYSTONE)** | extracts symbols, never edges | **emit import edges in the always-on pass** | **the only build-cost seam; G0 + G2 gate** |
| 4 | bbolt buckets | per-project index/learner/dimensions | edge bucket/key + SHA snapshots | additive key; behind `ports` port |
| 5 | cobra surface | `grep`/`peek`/`grammar`… | `archCmd` + children | thin delegate; mirror `_cgo`/`_nocgo` |
| 6 | fsnotify reindex | re-runs `ParseFileToMeta` per edit | per-file edge upsert/delete | must be O(edges-for-file), not O(all edges) |

**The whole feature pivots on seam 3.** Storage, CLI, socket, web, and freshness
all inherit the keystone for free — *if and only if* the edge is born on the
always-on parse path (site (a) below). Get the keystone wrong and the inheritance
breaks: the edge becomes stale, recon-gated, or G0-blowing. So the keystone is
documented first and in the most detail; the other five are the easy seams that
ride it.

---

## 1. The keystone (seam 3) — pinned to the exact pass

### 1.1 Why there is no edge today

aOa's entire persisted code model is `ports.Index`
(`internal/ports/storage.go:59-63`): three maps, all **node-shaped**, keyed off a
compact `TokenRef{FileID, Line}` (`storage.go:66-69`).

| Map | Holds | Source |
|-----|-------|--------|
| `Tokens map[string][]TokenRef` | token → locations (the O(1) search spine) | storage.go:60 |
| `Metadata map[TokenRef]*SymbolMeta` | location → symbol | storage.go:61 |
| `Files map[uint32]*FileMeta` | fileID → file (carries `Language`, atlas `Domain`) | storage.go:62 |

There is **no map whose key and value are both symbols** — no relation is
representable. `SymbolMeta.Parent` (`storage.go:78`) is the lone proto-edge
(method→receiver containment), but it is an un-indexed display *string*, not a
traversable reference. The keystone adds exactly one new shape:
`Edge = {Src TokenRef, Dst TokenRef|DstPath, Kind, prov}`, reusing the existing
`TokenRef` identity as endpoints. (Logical shape only — physical layout is §1.5.)

### 1.2 Two different tree-sitter passes — and only one is the keystone's home

The prior research said "the walker visits import nodes but never emits edges."
Grounding that against code sharpens it into **two genuinely different passes**,
and the choice between them is the entire G0 question.

**Site (a) — the always-on index-build pass. The keystone's home; the
recommendation.** `ParseFileToMeta` (`parser.go:108`) calls **only**
`extractSymbols` (`parser.go:104→235`); it never invokes the walker. This is the
pass the indexer always runs (`internal/app/indexer.go:140`) and the pass the
watcher re-runs on every edit (`watcher.go:132`). `extractSymbols` dispatches to
`extractGo` (`parser.go:347`), `extractPython` (`:458`), `extractJavaScript`
(`:532`), or `extractGeneric` (`:250`). The Go extractor switches on exactly three
node kinds — `function_declaration`, `method_declaration`, `type_declaration`
(`parser.go:351-359`) — and `import_declaration` is silently skipped. **On the
always-on path, imports are never visited, so no edge is constructed.**

**Site (b) — the dimensions/recon walk. Visits imports, but NOT chosen.**
`countImportSpecs` (`walker.go:568-583`) walks `import_spec_list`/`import_spec`
children and returns an `int` — it *counts* imports and **discards the package
names**. It is reached *only* via `walkContext.walk` (`walker.go:54`), the
dimensions engine, persisted through `SaveAllDimensions` (`dim_engine.go:200`).
This is the literal "visits imports, emits no edges" site — but it is **not the
index pass**.

### 1.3 The two sites are NOT interchangeable

| | Site (a) `extractSymbols` | Site (b) `countImportSpecs` |
|---|---|---|
| Always-on? | **Yes** — every index build + every edit | No — dimensions/recon-gated |
| G0 cost | in-pass, no second walk → ≤+3% | second walk *or* forces recon to run |
| G2 (two-binary) | clean — base binary, no recon | **base binary would depend on recon (G2 violation)** |
| G4 cleanliness | clean once `EdgeStore` port exists | rides off-interface `SaveAllDimensions` (§4.3) |
| Freshness | **free** — watcher re-runs it | own recon update path (`dim_engine.go:222`), not free |

**Do not collapse these into "either way, same pass."** The "no second walk /
freshness for free" guarantee attaches to **site (a) alone**. Site (b) is the
richer-but-recon-gated option; reserve it only for a future recon-gated
*enrichment* tier, never for the always-on base-product edge.

### 1.4 What lands at site (a)

Add an `import_declaration` case to the per-language extractors (Go: the switch at
`parser.go:351-359` already iterates every top-level child) and emit
`(importerFileID → importPath)` edge records alongside the symbols already
produced at `indexer.go:142`. The edge is born **inside the existing, always-on
`ParseFileToMeta` pass** — no second walk, no second file read — which is what
keeps it G0-safe (≤+3% build, `00-OVERVIEW.md:99`) and freshness-free
(`onFileChanged` re-runs exactly this function, §6).

**Precondition (hard gate, not a caveat): define `ports.EdgeStore` (or extend
`Storage`) first; the bbolt adapter then implements it.** The port's contract must
include a per-file delete (`DeleteEdgesForFile(FileID)`) so the keyed-by-file
storage constraint (§1.5) is enforced at the interface, not left to the adapter.

### 1.5 The G0-relevant *write* number — storage must be keyed-by-file

Riding `onFileChanged` makes freshness free but inherits that callback's hot-path
shape: `onFileChanged` already does **two full-map linear scans of
`a.Index.Files` per edit** — find-existing-ID (`watcher.go:65`,
`for id, fm := range a.Index.Files`) and allocate-new-ID (`watcher.go:110`,
`for id := range a.Index.Files`). To re-emit one file's edges, the watcher must
first *delete* that file's outbound edges. If edges are stored as a flat `[]Edge`
slice (the §1.1 *logical* shape), that delete is **O(all edges)** — every
keystroke-driven reindex would scan the entire estate's edge set. That is the
number that blows the ≤+3% budget on a large estate, and it is on the
**write/invalidation** path, not the read path.

> **Constraint:** `[]Edge` is the *logical* shape (what the rendition engine and
> agent see). The *physical* layout must be keyed for per-file deletion —
> `map[FileID][]Edge` (outbound-by-src) with a separately maintained inbound index
> for reachability/blast reads — so `onFileChanged` drops and re-emits exactly one
> file's edges in **O(edges-for-that-file)**, never adding a third full-map scan to
> a callback that already does two.

### 1.6 Reconciling with the locking law — the edge is index data, not an "arch write"

A G4 red-teamer reaches for `00-OVERVIEW.md:101` (*"no arch/facts write ever holds
App.mu; daemon-first reads"*) and asks how the keystone can ride `App.mu` — it
must, because `onFileChanged` holds `a.mu.Lock` (`watcher.go:43`) and
`ParseFileToMeta` runs inside that locked section (`watcher.go:132`). The answer is
a line to draw, not elide: **two distinct write classes.**

1. **The import-edge FACT is INDEX DATA.** It is produced by `extractSymbols`
   alongside the symbols already written at `indexer.go:142`, and it rides the
   *existing* `SaveIndex` write (`bbolt/store.go:98`) — the same write that already
   holds `App.mu` for every symbol. The keystone is **not a new "arch write"**; it
   is one more field on the index write that already runs under the lock. *This is
   exactly what makes freshness free* — the edge invalidates and re-derives on the
   same `onFileChanged` tick as every token.
2. **Derived renditions and detector output are the "arch/facts writes" the law
   means** — laid-out shards, DSM matrices, cycle/SCC findings, conformance diffs,
   evidence packs. *These* never hold `App.mu`: they compute off the hot path
   (compact-time detectors) and serve daemon-first as reads.

So `00-OVERVIEW.md:101` and the keystone are not contradictory: the law governs the
**derived** layer; the import edge is not in it. (Board wording note:
`00-OVERVIEW.md:101` should read "no **derived** arch/facts write holds App.mu" to
stop a careful reader landing on a false contradiction.)

---

## 2. Seam 1 — the socket method switch (the agent's hot path)

**What exists.** The daemon answers over a unix socket with a **flat JSON method
switch** — `handleRequest` (`socket/server.go:224`) dispatches `MethodSearch`,
`MethodPeek`, etc. with no handshake, no session, no JSON-RPC envelope (cases
`:226-245`, switch closes `:248`; method constants `protocol.go:39-48`). The
default arm returns `unknown method` (`server.go:246`).

**Net-new.** Add the six spec `MethodArch*` `case` arms (`MethodArchViews/View/Findings/Journey/Derive/Facts` — `02-arch-service.md:126-131`; reach/blast are CLI-only aliases per the 2026-07-02 ADR, never protocol methods)
arms that delegate to `arch.*` handlers, exactly as `handleSearch`/`handlePeek`
already delegate. Each new arm is a one-line addition that inherits the sub-ms read
path G0 mandates — you cannot make any surface faster than this socket, so it is
the latency floor and gets built first (G3 native-first).

**MCP rides here as a sibling.** Hexagonal architecture makes the MCP adapter a
second server beside socket/web, calling the *same* `arch.*` handlers 1:1 with zero
duplicated logic. MCP's stdio + JSON-RPC handshake/session overhead sits
structurally *above* the socket, so MCP buys **reach** (MCP-only agents/IDEs),
**never speed** — it must never front a latency-sensitive query.

**Constraint (G4).** The new arms **delegate**; they must not reach into domain
logic. The handler calls an app-level `ArchQuerier`; the socket layer stays a thin
transport, identical to today's `handleSearch` shape.

---

## 3. Seam 2 — the web route table + the viewer

**What exists.** A localhost-only HTTP dashboard with an **ETag-gated JSON API**.
Routes are registered on a `mux` (`web/server.go:92-113`), each wrapped in
`withETag` (`:159`) for revision-gated polling; the root `GET /` serves an embedded
shell (`server.go:87`). The `/api/recon*` family (`server.go:107-110`) is the exact
precedent: a read-only, ETag-gated, JSON-over-HTTP feed.

**Net-new.**
- `GET /api/arch/views`, `/api/arch/view/{id}`, `/api/arch/reach`, `/api/arch/findings`
  — reuse `withETag` so the canvas auto-refreshes on the same revision-bump the
  agent's socket read sees. This is what makes the diagram and the agent's answer
  *one truth*: both read the same shard contract
  (`architecture-c4.html:32-37` manifest→shard chain), fed by the same invalidation
  event.
- Embed the mockup viewer at `GET /arch` via the existing `//go:embed` static-bytes
  pattern — build-neutral, no new runtime dependency.

**Constraint.** Localhost-only (inherits the existing dashboard's bind); the viewer
ships as vendored static bytes (no CDN, no force-directed physics — the leash and
the blind-judge gate forbid invented nodes/edges, `03-visualization.md:343`).

---

## 4. Seam 4 — bbolt buckets (edges + SHA snapshots)

**What exists.** Per-project top-level buckets, each with JSON-serialized
sub-buckets: `index`, `learner`, `sessions`, `dimensions`, `telemetry`
(`bbolt/store.go:32-37`). A `_version` key (`store.go:29`) lets old DBs
self-recover. `SaveIndex`/`LoadIndex` (`store.go:98`/`:154`) own the index bucket;
`SaveAllDimensions`/`LoadAllDimensions` (`store.go:461`/`:488`) use a
**delete-then-recreate replace-all** lifecycle (`store.go:468-469`).

**Net-new — two options.**
- **(cheapest) `keyEdges` inside the existing `index` bucket** — rides
  `SaveIndex`/`LoadIndex`, so the edge fact persists on the same write as the index
  (matches §1.6: edge is index data). Self-recovers via the existing `_version`
  byte: an old DB simply recomputes edges on the next index build.
- **(richer) a sibling `bucketArch` / `bucketEdges`** that mirrors the
  `dimensions` replace-all lifecycle — clean for SHA-snapshot edge sets (the
  current-vs-future diff wedge, §8 of the parent guide), where two commits' edge
  sets coexist.

**Constraint (G4).** Both go **behind the `ports.EdgeStore` port** (the §1.4
precondition). The adapter implements the port; the methods must **not** repeat the
off-interface shortcut (§4.3). The port contract includes `DeleteEdgesForFile` so
the keyed-by-file physical layout (§1.5) is enforced at the interface.

### 4.1 SHA snapshots for the diff wedge

The "what changed since `<ref>`" pack needs two commits' edge sets. The
replace-all sibling bucket keys edges by `(commitSHA, FileID)` so a delta is a
set-difference over two keyed snapshots — derived from AST, free to compute, no doc
to maintain. This is the structural feature graphify's single-build-artifact
architecture cannot match.

### 4.2 The `dimensions` precedent is the copy-paste template

`dimensions/` was added as an independent transactional sub-bucket with a
delete-then-recreate lifecycle (`store.go:461-484`). An `edges/` sibling drops in
identically — same `CreateBucket`/`DeleteBucket` shape, same per-project keying.

### 4.3 Clean-architecture caveat (flag under G4, compounds site (b))

`SaveAllDimensions`/`LoadAllDimensions` live on the concrete `*Store`
(`bbolt/store.go:461`/`:488`) but are **NOT declared in the `Storage` interface**
(`storage.go:12-56`, which ends at the Telemetry methods) — recon reaches them by
concrete type, bypassing the port. This is exactly the path §1.2 site (b) would
ride, so site (b) is not only recon-gated (the G2 blocker) but **also** the
G4-dirtier route. Site (a) + a new `EdgeStore` port avoids both. The edge methods
must enter through the port, never repeat this shortcut.

---

## 5. Seam 5 — the cobra `aoa arch` family

**What exists.** Cobra commands registered on `rootCmd` (`cmd/aoa/cmd/root.go:36-52`).
The `grammar` parent/child group (`grammar_cgo.go:16`, children added at `:57-60`)
is the structural precedent for a parent command with sub-commands, and the
`_cgo`/`_nocgo` build-tag split means `--light` degrades gracefully.

**Net-new.** An `archCmd` parent + children, structurally identical to `grammar`:

```
aoa arch views                      # catalog + status per view (live/mixed/declared/planned)
aoa arch view <id> [--scope p]      # one view's rendition JSON (= a shard)
aoa arch reach A B                  # reachability / shortest-path between two anchors
aoa arch blast <ref|PR>             # affected-set / PR blast-radius
aoa arch findings [--new]           # findings, baseline-aware
aoa arch facts <subject>            # raw facts + source pointers (the audit trail)
aoa arch pack <dd|pci|delta>        # evidence-pack export
```

Each emits JSON to stdout mirroring the shard contract exactly — so the CLI result
and the browser's fetched shard are the same bytes.

**Constraint.** Each command is a **thin delegate** to the daemon/App service
(never its own domain logic), and mirrors the `_cgo`/`_nocgo` tags so the light
build degrades cleanly. Registration sits beside the existing `AddCommand` calls
(`root.go:52`), gated the same way `grammarCmd` is.

---

## 6. Seam 6 — fsnotify reindex (where freshness becomes free)

**What exists.** `onFileChanged` (`watcher.go:20`) is the per-file reindex
callback, wired at `app.go:698` (`a.Watcher.Watch(a.ProjectRoot, a.onFileChanged)`).
It serializes under `a.mu.Lock` (`watcher.go:43`), does two full-map
`a.Index.Files` scans (`:65` find-ID, `:110` alloc-ID), re-runs `ParseFileToMeta`
inside the lock (`:132`), and on deletion calls `removeFileFromIndex`
(`watcher.go:558`).

**Net-new.** A per-file edge upsert/delete mirroring `removeFileFromIndex` — drop
the changed file's outbound edges, re-emit them from the fresh parse. Because the
edge is born in `ParseFileToMeta` (seam 3, site (a)), this callback *already*
re-derives it; the only net-new work is the delete-then-upsert against the edge
store.

**Constraint (the load-bearing one).** The per-file delete is "nothing structural"
**only if the edge store is keyed-by-file** (§1.5) so the delete is
O(edges-for-that-file), not O(all edges). The edge **fact** rides `a.mu` exactly as
the symbol write already does — it is index data, not a derived "arch write" (§1.6);
only *derived* renditions stay off `a.mu` per the locking law. And **freshness is
free only because the keystone landed at site (a)** — at site (b) the dimensions
pass has its own recon-gated update path (`dim_engine.go:222 updateDimForFile`) and
freshness would not be free.

---

## 7. Inheritance summary — what each seam gets from the keystone

Once the keystone lands at site (a) behind an `EdgeStore` port, the five easy seams
fall out with no new architecture:

| Seam | Inherits from keystone | Net-new effort |
|---|---|---|
| ④ bbolt | edges persist on the existing index write; self-recover via `_version` | a key or sibling bucket behind the port |
| ⑥ fsnotify | freshness for free — `onFileChanged` re-runs the emitting function | a keyed-by-file per-file upsert/delete |
| ① socket | sub-ms reads over the existing flat switch | one `case` arm + delegating handler |
| ⑤ cobra | the same service the socket calls | a thin parent/child command group |
| ② web | the same shard contract the agent reads, ETag-gated | reuse `withETag` + `//go:embed` |

**The one fact that makes this whole feature cheap:** the edge is born inside a
function the binary already runs on every index build and every file edit. Five of
six seams are additive transport/storage over machinery that already exists. The
sixth — the keystone — is the only place that costs build budget, and it is
G0-bounded precisely because it rides the always-on pass and never walks the tree a
second time.

---

## Appendix — falsifiable anchor index (red-team this list first)

| Claim | Anchor |
|---|---|
| Three node-maps, zero relations; `Parent` is a display string | `internal/ports/storage.go:59-63`; `:78` |
| Keystone site (a) = always-on index pass, never visits imports | `parser.go:108` (`ParseFileToMeta`) → `:104,235` (`extractSymbols`) → `:347,351-359` (`extractGo`, 3 node kinds, no `import_declaration`) |
| Edge emission rides the always-on index build | `internal/app/indexer.go:140-142` |
| Keystone site (b) = recon-gated dimensions walk, counts+discards names (NOT chosen) | `walker.go:568-583` (`countImportSpecs` returns `int`), reached only via `walkContext.walk` `walker.go:54` → `dim_engine.go:200` (`SaveAllDimensions`) |
| Hot-path write cost → store must be keyed-by-file, not flat `[]Edge` | `onFileChanged` two full-map scans: find-ID `watcher.go:65`, alloc-ID `watcher.go:110`; per-file delete must be O(edges-for-file) |
| Edge is index data, rides `App.mu` like every symbol (not a derived "arch write") | `onFileChanged` holds lock `watcher.go:43`; `ParseFileToMeta` inside `watcher.go:132`; reconciles with locking law `00-OVERVIEW.md:101` |
| Flat socket method switch, no JSON-RPC envelope (MCP rides alongside) | `socket/server.go:224-249` (cases `:226-245`, default `:246`); constants `protocol.go:39-48` |
| Web ETag + embed precedent (`/api/recon*`) | `web/server.go:92-113` (recon `:107-110`, `withETag` `:159`, root embed `:87`) |
| bbolt buckets + `_version` self-recover; `dimensions` replace-all template | `bbolt/store.go:29,32-37`; `SaveIndex`/`LoadIndex` `:98`/`:154`; `SaveAllDimensions`/`LoadAllDimensions` `:461`/`:488` (delete-recreate `:468-469`) |
| Off-interface caveat (compounds site (b)) | `SaveAllDimensions`/`LoadAllDimensions` concrete-only `bbolt/store.go:461`/`:488`; absent from `Storage` interface `storage.go:12-56` |
| fsnotify → reindex, wired; freshness free *iff site (a)* | `onFileChanged` `watcher.go:20` (lock `:43`, scans `:65`/`:110`, `ParseFileToMeta` `:132`, `removeFileFromIndex` `:558`); wired `app.go:698` |
| cobra parent/child precedent | `cmd/aoa/cmd/root.go:36-52`; `grammar_cgo.go:16` (children `:57-60`) |
| Manifest → shard → viewer contract (one truth, both faces) | `architecture-c4.html:32-37`; `03-visualization.md:108-124,343` |
| Locking law (governs derived writes, not the in-pass index fact) | `00-OVERVIEW.md:101` |
| G0 budget (≤+3% build) | `00-OVERVIEW.md:99` |
