# 02 — The Arch Service Plane

**Status:** implementation-grade spec · 2026-06-11
**Scope:** the `aoa arch` command family, the rendition engine (facts → view JSON), detectors/findings, conformance, journeys/focus-flow queries, and the agent interface.
**Companion:** `playbook/integration/01-facts-substrate.md` (sibling spec) defines `ports.FactStore` — this spec **consumes** it and assumes facts of kind `unit | dep | route | schema | deploy | owner | delta | finding` exist with source pointers and provenance per the three-layer ladder (`.context/decisions/2026-06-11-core-competence-and-scope-line.md:23-31`).

**Governing law:**

| Law | Source | What it binds here |
|---|---|---|
| G0 sub-ms | `.context/GOALS.md:7` | daemon-served renditions are cache reads; rendering is compact-time, never search-hot-path |
| G3 agent-first | `.context/GOALS.md:10` | CLI is the primary interface; JSON to stdout; MCP is a later thin adapter (`playbook/ENHANCEMENT-GUIDE.md:102-127`) |
| G4 hexagonal | `.context/GOALS.md:11` | all rendition/detector/conformance logic in `internal/domain/arch/`, dependency-free; CLI/socket/web are adapters |
| Rendering law | `playbook/standards/view-standards.json` | budgets (`:17-23`), palette resolution (`:26-43`), drill dock (`:44`), journeys contract (`:46`), 16 view intents (`:55-268`) |
| Output contract | `playbook/generators/build_c4_mockup.py:370-406` + `playbook/mockups/archmodel/manifest.json` | manifest + shard JSON shapes the existing viewer already consumes — the service emits these **byte-compatibly** |
| Scope line | ADR 2026-06-11 (`:33-58`) | everything below is a rendition of derived facts or a diff against a declaration; nothing pretends |

---

## 1. Command family: `aoa arch`

### 1.1 Registration and shape

One cobra command with subcommands, registered in `cmd/aoa/cmd/root.go` beside `treeCmd`/`peekCmd` (`root.go:36-43`). `arch` is a **query** command — always available, not gated by `isShimMode()` (`root.go:46`). Files: `cmd/aoa/cmd/arch.go` (root + shared client plumbing), `cmd/aoa/cmd/arch_views.go`, `arch_findings.go`, `arch_journey.go`, `arch_pack.go`.

```
aoa arch views                          # catalog: every view per scope + status + provenance
aoa arch view <id> [--scope s]          # one rendition (= one shard, byte-identical contract)
aoa arch findings [--new] [--severity t] [--scope s]
aoa arch journey <id> | aoa arch journey --list
aoa arch derive <A> <B> [--k n] [--via kind]
aoa arch facts <subject> [--kind k] [--limit n]
aoa arch pack <dd|pci|delta> [--since ref] [--out dir]
```

All subcommands emit **JSON to stdout** (the agent/viewer contract), human-readable errors to stderr — mirroring the grep split (`cmd/aoa/cmd/grep.go:94, 211`). `--pretty` indents for humans; default is compact (see §2.6 byte-stability).

### 1.2 Subcommand reference

| Command | Args/flags | Output (stdout) | Notes |
|---|---|---|---|
| `views` | `--scope <sid>` filter | the **manifest** object (§1.4), or one scope's `views` map when scoped | drives the sidebar/catalog; carries `shard.hash` so the viewer can cache-bust |
| `view <id>` | `<id>` = view id (`component`, `dsm`, `cycles`, `code`, `techportfolio`, …); `--scope <sid>` (default: first scope); `--out <file>` | the **shard** object (§1.5) | the workhorse. `aoa arch view component > shard.json` is loadable by the playbook viewer unchanged |
| `findings` | `--new` (baseline-aware, §4.4); `--severity error\|warn\|info` floor; `--scope` | `{"findings":[Finding…],"count":N,"baseline":{"ref","frozen_at"}}` | `--new` is the CI gate: exit 1 when new findings exist |
| `journey <id>` | `--list` enumerates | journey shard (§5.1) — same shape the viewer's journey player consumes | stored renditions; provenance stamped |
| `derive <A> <B>` | `--k` paths (default 3, cap 5); `--via dep\|dataflow\|both` (default both) | focus-flow object (§5.2) | A/B are unit ids or path prefixes; resolution errors list candidates |
| `facts <subject>` | `--kind unit\|dep\|route\|schema\|deploy\|owner\|delta\|finding`; `--limit` (default 200) | `{"facts":[Fact…],"count":N}` — raw facts with `source{file,line,commit}` | the audit trail behind any rendered element; `Fact` shape is owned by spec 01 |
| `pack <kind>` | `dd\|pci\|delta`; `--since <ref>` (delta only); `--out <dir>` (default `.aoa/arch/packs/`) | manifest of written files | Phase ④; assembly-only — zero new derivation (`ENHANCEMENT-GUIDE.md:129-141`) |

### 1.3 Execution paths, latency, exit codes

Same dual-path pattern as `executeSearch` (`grep.go:197-215`): try the daemon socket, fall back to direct.

| Path | Mechanism | Expected latency | When |
|---|---|---|---|
| **Daemon** | new socket methods (§1.6) over `/tmp/aoa-{hash}.sock` (`socket/protocol.go:17-24`) | shard cache hit: **<1ms** service time (it is a `[]byte` read, §2.5); cold render: <10ms at module grain (≤~200 units); + ~2-5ms process spawn for the CLI itself | normal operation |
| **Direct** | open `{root}/.aoa/aoa.db` read-only (bbolt supports `ReadOnly: true` — no lock conflict with a running daemon), construct `arch.Service` over the FactStore, render in-process | 20-80ms (db open + cold render) | daemon down; CI; `pack` (long-running, always direct) |

No system-grep-style fallback beyond that: if neither daemon nor db exists, print `Error: no facts substrate. Run: aoa init && aoa daemon start` to stderr, exit 2.

**Exit codes** (grep-conventional, matching `grepExit` semantics in `grep.go:96, 230-233`):

| Code | Meaning |
|---|---|
| 0 | success — and for `findings --new`: no new findings |
| 1 | empty result: unknown view id for this scope, journey not found, `derive` found no path, or `findings --new` found new findings (the CI-gate bit) |
| 2 | operational error: no substrate, bad flags, unresolvable subject |

### 1.4 Manifest contract (`aoa arch views`)

Byte-compatible with `playbook/mockups/archmodel/manifest.json`, produced today by `build_c4_mockup.py:374-405`. Product changes exactly two field values: `schema` and `generated.tool`.

```json
{
  "schema": "aoa.archmodel/v1",
  "sharded": true,
  "generated": {"tool": "aoa", "timestamp": "2026-06-11 06:04 UTC",
                "inputs": ["facts@<revision>", "arch.yaml@<sha>"]},
  "estates": {
    "local": {
      "label": "Local · this workspace", "sim": false,
      "scopes": {
        "aoa": {
          "label": "aOa", "tech": "Go · single binary",
          "views": {
            "component": {
              "kind": "buckets", "title": "Component diagram",
              "count": "25 packages · 42 dependencies", "dir": "DOWN",
              "prov": {"kind": "derived", "label": "REAL · derived from code"},
              "shard": {"path": "local/aoa/component.json",
                        "hash": "528d2a0e54a3", "bytes": 4052}
            }
          }
        }
      },
      "journeys": [{"id": "…", "label": "…", "kind": "dev", "steps": 6,
                    "prov": {…}, "shard": {"path": "local/journeys/….json"}}]
    }
  }
}
```

Rules carried over verbatim from the generator: the manifest entry holds only `kind/title/count/dir/prov` + the shard ref (`build_c4_mockup.py:390-392`); `hash` is `sha256(shard_bytes)[:12]` (`:383`); journey entries carry `steps` count, not steps (`:402-403`). The product's `views` map additionally lists **planned** views (in the standard catalog but not derivable yet) as `{"status":"planned","note":"…"}` entries with no `shard` — this backs the full-catalog dropdown (`build_c4_mockup.py:470-491`) from the service instead of viewer-side constants.

### 1.5 Shard contract (`aoa arch view <id>`)

Five `kind`s, exactly as the viewer validates and renders them today (`build_c4_mockup.py:431-441, 493-499`). Common header on every shard: `kind, title, count, dir ("DOWN"|"RIGHT"), prov{kind: "derived"|"mixed"|"simulated", label}`.

| kind | Body | Used by views | Example |
|---|---|---|---|
| `simple` | `nodes[{id,type:sys\|ext\|container\|store\|proc, icon, label, sub?, stats{…}?, real, drillTo?}]`, `edges[{id,source,target,label}]` | context, container, dataflow, sequence-as-flow | `archmodel/local/aoa/context.json` |
| `buckets` | `buckets[{id, layer, label, part:int, boundary?:bool, ico?, members[{id,label,sub?,stats{…}?,concerns?:int,changed?:bool}]}]`, `edges[{id,source,target,count,label?,tag?}]`, `palette?`, `labeled?:bool` | component, domains, deployment, trust | `archmodel/retail-enterprise-faulted/data/trust.json` |
| `entity` | `nodes[{id,type:"entity",label,tech,fields[string],stats?,real}]`, `edges[{id,source,target,label}]` | datamodel | `archmodel/local/aoa/datamodel.json` |
| `table` | `columns[string]`, `rows[[cell…]]` (⚠-prefixed first cell = flagged row) | techportfolio, sbom, glossary, cycles report, findings views | — |
| `matrix` | `items[string]`, `matrix[[int\|null]]` (row→column dependency counts) | dsm | `archmodel/retail-enterprise-faulted/supply/dsm.json` |

**Label budgets are renderer-enforced, lint-checked** (`view-standards.json:17-23`): node ≤30 chars, member ≤26, edge ≤48, ≤40 members/bucket, ≤30 nodes/simple-view. The Go renderers must never emit over budget — collapse instead (§3, budget overflow).

Member metadata goes in `stats{}` as 3-4 **named** reader-meaningful figures, never in the label (`view-standards.json:25`); `sub` is the legacy fallback the viewer still accepts.

### 1.6 Daemon protocol extension

New method constants beside `MethodSearch…MethodPeek` (`internal/adapters/socket/protocol.go:38-49`), dispatched in `handleRequest` (`server.go:206-231`):

```go
const (
    MethodArchViews    = "arch.views"    // params: {scope?}            → manifest JSON
    MethodArchView     = "arch.view"     // params: {view, scope?}      → ArchShardResult
    MethodArchFindings = "arch.findings" // params: {new?, severity?, scope?}
    MethodArchJourney  = "arch.journey"  // params: {id?, list?}
    MethodArchDerive   = "arch.derive"   // params: {from, to, k?, via?}
    MethodArchFacts    = "arch.facts"    // params: {subject, kind?, limit?}
)

// ArchShardResult — the shard travels pre-marshaled so daemon and direct
// paths are byte-identical and the cache stays a []byte read.
type ArchShardResult struct {
    View  string          `json:"view"`
    Scope string          `json:"scope"`
    Hash  string          `json:"hash"`
    Shard json.RawMessage `json:"shard"`
}
```

Handlers reach the service through one new method on `AppQueries` (`server.go:21-46`): `Arch() ports.ArchQuerier` (nil when no substrate — handlers answer with `Response.Error`, the CLI maps that to exit 2). The web adapter (`internal/adapters/web`) mounts the same querier at `GET /arch/manifest.json` and `GET /arch/{estate}/{scope}/{view}.json` so the playbook viewer ships in-product pointed at `?model=/arch/manifest.json` with **zero viewer changes** — the contract file IS the data source (`build_c4_mockup.py:416`).

---

## 2. Rendition engine

### 2.1 Package layout (G4)

```
internal/
  ports/
    facts.go            # ports.FactStore, ports.Fact          (owned by spec 01)
    arch.go             # ports.ArchQuerier — the service interface adapters see
  domain/arch/
    service.go          # Service: wires FactStore + enricher + declaration; owns shard cache
    model.go            # shard DTOs: Shard, Bucket, Member, Node, Edge, Prov, Manifest
    render_component.go # facts(unit,dep) → buckets shard
    render_dsm.go       # same dep edges → matrix shard
    render_cycles.go    # SCC results → table shard
    render_code.go      # unit+symbol facts → simple shard (critical-path subset)
    render_techport.go  # manifest/lockfile facts → table shard
    render_table.go     # shared table/sbom/glossary rendition helpers
    grouping.go         # path-prefix + atlas-domain grouping; overlay loader
    caption.go          # caption derivation (port of build_c4_mockup.py:901-926)
    detect.go           # detectors (§3): Tarjan, god/orphan, bands, budgets, dead-code
    conformance.go      # arch.yaml load/validate + reflexion diff (§4)
    journey.go          # stored journeys + k-shortest-path derive (§5)
    baseline.go         # finding fingerprints, freeze/--new (§4.4)
  app/
    arch.go             # App wiring: construct Service, expose via AppQueries.Arch()
cmd/aoa/cmd/
    arch.go …           # cobra layer (§1.1)
```

`internal/domain/arch` imports only `ports` and `enricher` types — no bbolt, no sockets, no cobra. Persistence (shard cache bucket, baseline bucket) goes through small interfaces defined in `ports/arch.go` and implemented in `internal/adapters/bbolt`.

```go
// ports/arch.go
type ArchQuerier interface {
    Manifest(scope string) ([]byte, error)            // §1.4
    View(scope, view string) (ArchShard, error)       // §1.5; ErrUnknownView → exit 1
    Findings(opts FindingsOptions) ([]Finding, error)
    Journey(id string) ([]byte, error)
    Derive(from, to string, k int, via string) (FocusFlow, error)
    Facts(subject, kind string, limit int) ([]Fact, error)
}
```

### 2.2 Renderer interface and per-view-kind renderers

```go
// renderer turns facts into one shard. Pure function of its inputs —
// no IO, no clock (timestamp injected by Service for the manifest only).
type renderer interface {
    ID() string                       // view id: "component", "dsm", …
    Kind() string                     // shard kind: buckets/matrix/table/simple/entity
    Inputs() []ports.FactKind         // which fact kinds it reads
    Render(in RenderInput) (*Shard, *Prov, error)
}

type RenderInput struct {
    Scope    string
    Units    []ports.Fact          // kind=unit
    Deps     []ports.Fact          // kind=dep (and dataflow-tagged routes for §5)
    Extra    map[ports.FactKind][]ports.Fact
    Grouping GroupingResult        // §2.3
    Decl     *Declaration          // §4 — nil when no arch.yaml
    Findings []Finding             // detector output relevant to this scope
}
```

| Renderer | View id(s) | Facts → shard | Phase (`ENHANCEMENT-GUIDE.md:60-81`) |
|---|---|---|---|
| component | `component` | units grouped (§2.3) → `buckets[]`; dep edges aggregated cross-group → `edges[{count}]` with `part` from declaration order or topological band; member `sub: "in N"` fan-in (matches `build_c4_mockup.py:62-66`) | ② |
| dsm | `dsm` | same groups as component, ordered by `part` then fan-in; `matrix[i][j]` = dep count i→j, `null` when 0 (matches `supply/dsm.json`) | ② |
| cycles | `cycles` | SCC detector output → `table` (columns: cycle, members, size, cheapest edge to cut = min count edge inside the SCC) | ② |
| code | `code` | symbol-grain units along a chain (entrypoint or `--scope`d) → `simple` nodes + chain edges; subset choice is MIXED | ② |
| techportfolio | `techportfolio` | manifest/lockfile facts (spec 01) + `FileMeta.Language` → `table` with lifecycle column | ② |
| sbom | `sbom` | lockfile facts → `table`; unpinned versions ⚠-flagged | ② |
| domains | `domains` | units bucketed by atlas `Domain` (already on `FileMeta`, `internal/ports/storage.go`) → `buckets` | ② |
| datamodel | `datamodel` | `schema` facts → `entity` nodes (fields = extracted columns) | ③ |
| container/deployment | | `deploy` facts → `simple`/`buckets` | ③ |
| context/dataflow/sequence | | `route` + SDK-dep facts, MIXED-stamped | ③ |
| trust/glossary/statemachine | | declarations + diff (§4) | ④ |

Every renderer ends by calling `caption.Derive(shard, findings)` — the straight Go port of `caption()` (`build_c4_mockup.py:901-926`): buckets → "N groups · M members — heaviest: A → B ×k"; matrix → "S dependencies · N modules · P mutual pairs: first"; table → "N rows · ⚠ F flagged — first: X"; entity → "N entities · E relationships — spine: …"; plus the `· ⚠ N findings` suffix. The caption is stored on the shard's manifest entry as `count` *and* available to the dock; today the viewer derives it client-side — emitting the same string keeps both paths honest and gives the lint something to assert.

### 2.3 Grouping strategy

Grouping decides bucket membership for `component`/`domains`/`dsm`. Three rungs, best available wins, and the rung used decides provenance (§2.4):

1. **Declaration** (`arch.yaml` role→path mapping, §4.1) — deterministic, REAL-compatible (the mapping is human-declared but the membership test is mechanical, like `layer()` in `build_c4_mockup.py:13-19`).
2. **Path-prefix heuristic** — first two path segments below the module root (`internal/domain` → `domain`), language-aware roots (`src/`, `pkg/`, `lib/` skipped). Deterministic, REAL-compatible.
3. **Atlas domain** — `FileMeta.Domain` from the enricher (134 domains, O(1)) for `domains`; honest MIXED per the matrix (`ENHANCEMENT-GUIDE.md:71`).

**The leashed-LLM hook is an optional EXTERNAL enricher, never inline.** An agent may write a *name overlay* file — `{root}/.aoa/arch/overlays/<scope>.json`:

```json
{"schema": "aoa.arch-overlay/v1",
 "renames":   {"b_pipeline": "Ingest & Analysis Pipeline"},
 "regroup":   {"m_internal_hints": "b_app"},
 "narratives":{"b_pipeline": "…"},
 "author": "claude-…", "at": "2026-06-11T…Z"}
```

The renderer applies overlays at render time under the leash rule (ADR `:26-31`): overlays may **rename, regroup, annotate** existing fact-backed elements; any id not present in facts is rejected with a warning fact; an applied overlay drops the view to MIXED. No LLM call ever happens inside the service — the hook is "a file exists or it doesn't." This keeps G0 (no network on any path) and makes the inference auditable (`aoa arch facts b_pipeline` shows the overlay as a source).

### 2.4 Provenance computation

Per shard, computed — never hand-set (the derive/infer ladder, ADR `:23-31`):

| Condition | `prov.kind` | `prov.label` |
|---|---|---|
| every node/edge backed by a fact with `prov=derived`; grouping rung 1-2; no overlay | `derived` | `REAL · derived from code` |
| any overlay applied, grouping rung 3, or any element from an inferred fact | `mixed` | `MIXED · <what is real> · <what is inferred>` (e.g. `MIXED · imports real · grouping inferred`, cf. `build_c4_mockup.py:309`) |
| view rendered from declarations only (trust zones, glossary), or authored journey | `simulated` | `SIMULATED · would derive from: <named sources>` (`build_c4_mockup.py:213`) |

Mixing rule: provenance is a **min over contributing elements** (derived > mixed > simulated); one simulated element drags the shard to its level, and the label must name the inferred part. The yellow palette reservation (`view-standards.json:40`) keys off these kinds in the viewer — emit exactly these three strings for `kind`.

### 2.5 Caching and invalidation

- Renders happen at **compact time** (when the FactStore folds JSONL into bbolt, spec 01) for the GREEN views, and lazily-on-first-request for the rest. Output: shard bytes + hash into a `arch_shards` bbolt bucket keyed `{scope}/{view}@{factsRevision}`.
- The daemon serves `arch.view` from this cache — a bucket get of pre-marshaled bytes, sub-ms (G0). A facts-revision bump (any compact) invalidates by key construction; no explicit eviction logic.
- Detectors (§3) also run at compact time, before renderers, so findings are available as render input and as facts.

### 2.6 Byte-stability rules (golden-fixture prerequisite)

Same facts in → identical bytes out, across runs and machines:

1. Shards marshal from **structs, not maps** (stable field order); all slices explicitly sorted (buckets by `part` then id; members by fan-in desc then label, matching `build_c4_mockup.py:65`; edges by id).
2. `json.Encoder` with `SetEscapeHTML(false)`, no trailing newline, compact (the generator writes `separators=(",",":")`, `build_c4_mockup.py:382`).
3. No timestamps inside shards — `generated.timestamp` lives only in the manifest and is excluded from fixture comparison.
4. Ids are deterministic slugs of fact subjects (`m_` + sanitized path, `build_c4_mockup.py:62`), never counters dependent on map iteration.

---

## 3. Detectors

All detectors run **at compact time** over the dep graph at group ("bucket") grain and unit grain, exactly once per facts revision (`ENHANCEMENT-GUIDE.md:83-89`). Each finding is persisted as a fact (`kind: finding`) carrying source pointers, so `aoa arch facts <subject>` shows why an element is flagged, and every shard renderer receives them.

Port of the generator's JS detectors (`build_c4_mockup.py:813-832`), upgraded where the JS was demo-grade:

| Detector | Generator logic (port from) | Go implementation | Severity |
|---|---|---|---|
| **Band violation** | `sp.part > tp.part` → edge `_viol` (`:817`) | edge whose source group's band index exceeds target's; band order from declaration (§4) when present, else topological layering of the group DAG. Without either, detector is **off** (no guessing) | `error` |
| **Tagged violation** | `e.tag` → `_viol` (`:818`) | conformance divergent edges (§4.3) inject `tag` on shard edges; the renderer prefixes the edge label `⚠ ` (`:787`) | `error` |
| **Cycle** | first-cycle DFS (`:822-830`) | **Tarjan SCC** over group graph AND unit graph — all cycles, not just the first; per SCC: members, internal edges, min-count edge = "cheapest to cut" (feeds `cycles` view hover, `view-standards.json:263-265`) | `error` |
| **God component** | `in ≥3 && out ≥3` (`:821`) | group grain: fan-in ≥ godIn && fan-out ≥ godOut (defaults 3/3, configurable in `arch.yaml: thresholds`) | `warn` |
| **Orphan** | `in+out == 0 && len(B)>1` (`:820`) | same, both grains | `info` |
| **Budget overflow** | `members > 40` → keep 23 + `+N more…` member (`:831-832`) | renderer collapses identically (id `{bucket}_more`, sub `over budget`) AND emits a finding so the overflow is visible in the dock, not just truncated | `warn` |
| **Dead-code candidate** | — (new; `ENHANCEMENT-GUIDE.md:85-87`) | unit with zero inbound `dep` facts and zero index reference hits; message always says **"candidate — no inbound references found"** with the reflection caveat (ADR `:38-39` — never a verdict) | `info` |
| **DSM mutual pair** | viewer-side (`:1236-1238`) | `matrix[i][j] && matrix[j][i]` at render — emitted as findings too, so they reach `aoa arch findings` | `warn` |

```go
type Finding struct {
    ID       string   `json:"id"`        // fingerprint, §4.4
    Rule     string   `json:"rule"`      // cycle|band|god|orphan|budget|dead-candidate|mutual|divergent|absent
    Severity string   `json:"severity"`  // error|warn|info
    Scope    string   `json:"scope"`
    Message  string   `json:"message"`   // human sentence, same phrasing as the generator problems[]
    Subjects []string `json:"subjects"`  // element ids (bucket/member/edge)
    Sources  []ports.SourceRef `json:"sources"` // file:line of contributing facts
    New      bool     `json:"new,omitempty"`    // vs baseline, §4.4
}
```

Message phrasing matches the generator so dock text is identical: `"band violation: A → B"`, `"orphan: X — no connections"`, `"god component: X (in 3 · out 4)"`, `"dependency cycle: A → B → A"` (`build_c4_mockup.py:817-829`). Shard-affecting flags (`_viol`-equivalent) are carried in the shard as the existing contract does: red dashed styling is a **viewer** concern keyed off `tag`/band data already in the shard — the service emits `tag` on violating edges and nothing else.

---

## 4. Conformance

The 17th view and the diff machinery (`ENHANCEMENT-GUIDE.md:90-96`; in-scope per ADR `:40-41`).

### 4.1 Declaration: `{root}/.aoa/arch.yaml`

```yaml
schema: aoa.arch/v1
pattern: hexagonal            # layered | hexagonal | onion | custom
roles:                        # role → path globs (doublestar), order = band order
  cmd:      ["cmd/**"]
  app:      ["internal/app/**"]
  adapters: ["internal/adapters/**"]
  domain:   ["internal/domain/**"]
  ports:    ["internal/ports/**", "atlas/**"]
rules:                        # optional; templates pre-fill these
  - allow: cmd -> app
  - allow: "* -> ports"
  - deny:  domain -> adapters
thresholds: {god_in: 3, god_out: 3}
zones:                        # optional, feeds the trust view (Phase ④)
  - {name: cde, classification: pci, paths: ["internal/payments/**"]}
```

**Pattern templates** ship in the binary (`internal/domain/arch/templates/*.yaml`, `//go:embed` like `atlas/v1`): `layered` (strict downward bands), `hexagonal` (domain depends only on ports; adapters/app may depend on domain+ports; cmd on app — literally the CLAUDE.md architecture section as data), `onion` (concentric allow-inward). `custom` requires explicit `rules`. `aoa arch init --pattern hexagonal` scaffolds the file with roles guessed from rung-2 grouping, then the human edits — declaration is layer-3 by definition.

### 4.2 Validation

Load at service construction + on fsnotify change to the file. Errors (unknown role in a rule, overlapping globs with different roles, unmatched paths >20%) are reported as `severity:error` findings with rule `declaration` — never silently ignored.

### 4.3 Reflexion diff

Diff declared rules against derived group-grain dep edges (O(edges), `ENHANCEMENT-GUIDE.md:75`):

| Class | Definition | Becomes |
|---|---|---|
| **convergent** | derived edge permitted by rules (explicit allow or band order) | nothing (countable in the conformance view) |
| **divergent** | derived edge denied (explicit deny, or upward band without allow) | finding `divergent`, severity `error`; shard edge gains `tag: "divergent: domain → adapters"` |
| **absent** | declared allow with **zero** derived edges behind it | finding `absent`, severity `info` ("declared but unused: cmd → app") |

The conformance **view** (`aoa arch view conformance`) is a `table` shard: one row per rule/edge-class with counts, ⚠ on divergent rows, caption "N rules · C convergent · D divergent · A absent — first divergence: …".

### 4.4 Baseline / freeze and `--new`

ArchUnit's freeze pattern (`ENHANCEMENT-GUIDE.md:94-95`):

- **Fingerprint:** `sha256(rule + scope + sorted(subjects))[:16]` — stable across re-renders and line drift (no line numbers in the print).
- **Storage:** bbolt bucket `arch_baseline`: fingerprint → `{first_seen_revision, frozen_at}`. Project-scoped like everything else (CLAUDE.md key paths).
- **`aoa arch findings --freeze`** writes all current fingerprints to the baseline (explicit human act, recorded with timestamp).
- **`aoa arch findings --new`** returns only findings whose fingerprint is absent from the baseline; exit 1 if any (CI gate), 0 if clean. Without `--new`, all findings return with `"new": true/false` marked.
- Baseline entries whose finding no longer occurs are reported once under `--stale` and pruned on the next `--freeze` (no silent shrinkage).

---

## 5. Journeys and focus flow

### 5.1 Stored journeys

A journey is an estate-level flow rendition (`view-standards.json:46`): ordered steps over existing views, kinds `customer|business|dev|change`, each step anchored to `(scope, view, element)` with present-tense narrative. Shard shape exactly as shipped (`archmodel/retail-enterprise-faulted/journeys/bopis-click-to-curbside.json`):

```json
{"id": "...", "label": "...", "kind": "customer", "question": "...",
 "prov": {"kind": "simulated", "label": "SIMULATED · would derive from: …"},
 "steps": [{"label": "...", "narrative": "...", "scope": "ecom",
            "view": "sequence", "sel": "s1"}]}
```

Sources, in order: (a) authored files in `{root}/.aoa/arch/journeys/*.json` (validated: every `(scope,view,sel)` anchor must resolve against current shards — broken anchors are `declaration` findings, mirroring the both-variants rule in `view-standards.json:46`); (b) **derived** journeys persisted from `derive` (below) with `prov.kind: "mixed"` (path REAL, narrative inferred/templated). `aoa arch journey <id>` returns the shard; the manifest lists them per §1.4.

### 5.2 `aoa arch derive <A> <B>` — focus flow

K-shortest simple paths over the union of `dep` and `dataflow`-bearing facts at **group grain first, unit grain within groups** (`ENHANCEMENT-GUIDE.md:97-100`).

- **Algorithm:** Yen's k-shortest-paths over Dijkstra. Graph size is groups (≤~50) or units (≤ a few thousand): O(k·V·(E + V log V)), comfortably sub-ms at group grain, <50ms worst-case unit grain. Loopless paths only.
- **Edge weights:** `w = 1 + 1/(1+count)` — hop count dominates (shortest explanation wins), heavier (higher-`count`) edges break ties. `dataflow` edges and `dep` edges weigh equally under `--via both`; `--via dep|dataflow` filters the edge set.
- **k:** default 3, flag-capped at 5 — this is an explanation tool, not a path enumerator.
- **Endpoint resolution:** exact unit id → unique path-prefix → unique label match; ambiguity returns exit 2 with a `candidates` array; no fuzzy guessing.
- **Output:**

```json
{"from": "internal/app", "to": "internal/adapters/bbolt", "via": "both", "k": 3,
 "prov": {"kind": "derived", "label": "REAL · paths over derived dep/dataflow facts"},
 "paths": [{"hops": [{"source": "app", "target": "ports", "kind": "dep",
                      "count": 12, "sources": [{"file": "…", "line": 4}]}],
            "weight": 2.08,
            "findings": ["dependency cycle: …"],     // findings touching any hop
            "deltas":   []}],                        // delta facts on hop endpoints
 "journey": {"id": "derived-app-to-bbolt", "steps": [...]}}   // §5.1 shape, replayable in the viewer
```

- **The empty-path answer is a first-class result**, not an error: `{"paths": [], "answer": "no dep or dataflow path from A to B at facts revision <r> — they are independent"}`, exit 1. "These two things don't touch" is precisely the question architects pay to answer.

---

## 6. Agent interface

### 6.1 CLAUDE.md guidance block

Appended to the `aOa-guidance` block emitted by `aoa init` (template in `cmd/aoa/cmd/init.go`, `aoaGuidance` const ~line 563). Follows the proven format rules (memory/ADR 2026-03-04): concrete output example first, two-line workflow, explicit subagent line, fallback documented — no prohibitions.

```markdown
### Architecture — use `aoa arch` before reading files to map structure

`aoa arch` answers structure questions from the fact substrate in one call —
package layout, dependencies, cycles, paths between modules. This applies to
you AND any subagents you spawn.

$ aoa arch view component | head -c 400
  {"kind":"buckets","title":"Component diagram","count":"25 packages · 42
  dependencies","prov":{"kind":"derived","label":"REAL · derived from code"},...

| Question | Command |
|---|---|
| What are the modules/layers?   | `aoa arch view component` |
| Anything circular or tangled?  | `aoa arch view cycles` · `aoa arch findings` |
| How does A reach B?            | `aoa arch derive internal/app internal/adapters/bbolt` |
| Why is this element flagged?   | `aoa arch facts <id>` (file:line evidence) |
| Did my change add drift?       | `aoa arch findings --new` (exit 1 = new findings) |

Workflow: `arch view component` → orient · `arch derive A B` → trace ·
`grep`/`aoa peek` → read the code at the file:line the facts cite.
Every JSON element carries provenance — trust `derived`, verify `mixed`.
If `aoa arch` reports "no facts substrate", fall back to `grep`/`aoa tree`.
```

### 6.2 MCP adapter sketch (Phase ④)

Per the CLI-first decision (`ENHANCEMENT-GUIDE.md:102-127`): a new adapter `internal/adapters/mcp/`, stdio JSON-RPC, **wrapping `ports.ArchQuerier` and `ports.Searcher` only** — zero logic, exactly as the socket server wraps `AppQueries`. Tools map 1:1 to subcommands: `arch_views`, `arch_view`, `arch_findings`, `arch_derive`, `arch_facts`, plus `search` and `peek`. Tool results are the same JSON bodies §1 defines (one contract, three transports: CLI, socket, MCP). Launched as `aoa daemon mcp` (stdio child of the MCP host); ships only after the CLI family is proven with agents.

---

## 7. Phased task list and test strategy

### 7.1 Tasks

Sizes per the guide's scale (S <1d · M 2-4d · L 1-2wk). Phase numbers follow `ENHANCEMENT-GUIDE.md:151-158`.

| # | Task | Depends on | Size | Phase |
|---|---|---|---|---|
| A1 | `ports/arch.go` (ArchQuerier, Shard/Finding DTOs) + `domain/arch/model.go` + byte-stable encoder (§2.6) | spec 01 FactStore | S | ② |
| A2 | grouping.go: declaration/path-prefix/atlas rungs + overlay loader & leash validation | A1 | M | ② |
| A3 | detect.go: Tarjan SCC, god/orphan, budgets, dead-candidates, mutual pairs; findings→facts at compact | A1 | M | ② |
| A4 | render_component + render_dsm + render_cycles + caption.go | A2, A3 | M | ② |
| A5 | render_code + render_techport + render_sbom + render_domains | A2 | M | ② |
| A6 | Service + shard cache (bbolt `arch_shards`) + app wiring (`app/arch.go`, `AppQueries.Arch()`) | A4 | S | ② |
| A7 | CLI: `arch views/view/findings/facts` + socket methods + direct fallback | A6 | M | ② |
| A8 | web adapter routes `/arch/...` + viewer pointed at live manifest | A6 | S | ③ |
| A9 | conformance.go: arch.yaml + templates + reflexion diff + conformance view | A3 | M | ④ |
| A10 | baseline.go: fingerprints, `--freeze`, `--new`, `--stale` | A9 | S | ④ |
| A11 | journey.go: stored journeys + anchor validation + `derive` (Yen's) + `arch journey/derive` CLI | A6 | M | ③ |
| A12 | `arch pack dd|pci|delta` assembly/export | A8-A11 | M | ④ |
| A13 | CLAUDE.md guidance block in `init.go` + `--claude-guidance` text | A7 | S | ② |
| A14 | MCP adapter | A7 | M | ④ |

### 7.2 Test strategy

**Golden-shard fixtures (the backbone)** — `test/fixtures/arch/`, same pattern as the existing parity suite (`test/parity_test.go` + `test/fixtures/search/`):

```
test/fixtures/arch/
  inputs/aoa-snapshot.facts.jsonl     # frozen facts for THIS repo at a pinned commit
  inputs/arch.yaml                    # hexagonal declaration
  expected/local/aoa/component.json   # golden shards — byte-compared
  expected/local/aoa/dsm.json
  expected/manifest.json              # timestamp field masked
  expected/findings.json
```

Assertion: same facts in → **byte-identical** shards out (`require.Equal(string(want), string(got))`); any diff is a contract break. §2.6 makes this possible; CI runs it on every commit (`make check`).

**Hartwell shards as rendition regression fixtures:** the 21-estate playbook corpus (`playbook/mockups/archmodel/*`, esp. `retail-enterprise-{clean,faulted}`) is the proven-rendered contract. Two test layers: (1) **schema conformance** — a Go validator (port of `validateView`, `build_c4_mockup.py:431-441`) decodes every Hartwell shard into the §2.1 DTOs and re-encodes byte-identically, proving the Go types cover the full contract surface including `boundary`, `stats`, `tag`, `ico`; (2) **detector parity** — feed Hartwell faulted-estate shard graphs through `detect.go` and assert the findings match the generator's `problems[]` strings (the faulted estates were authored to trip every detector).

**Unit tests** (colocated, per CLAUDE.md test structure): Tarjan vs known SCC graphs incl. self-loops; Yen's k-paths vs hand-computed graphs + the no-path case; fingerprint stability under subject reordering; budget collapse at exactly 40/41 members; provenance min-rule truth table; arch.yaml validation errors; overlay leash rejection (unknown id → warning, never applied).

**Latency guards:** benchmarks (`go test -bench`) — cached `arch.view` socket round-trip <1ms; cold component render on the aOa repo <10ms; `derive` group-grain <1ms. Wired into the existing bench run (CLAUDE.md build/test commands).

**End-to-end (Phase ③ gate):** clone a stranger repo → `aoa init` → daemon compact → `aoa arch views` shows the 5 GREEN views REAL-stamped → screenshot-render through the viewer → `Skill: model-standard` blind-judge pass — the same quality gate the playbook estates passed.
