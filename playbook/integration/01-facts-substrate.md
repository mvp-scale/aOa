# 01 — Facts Substrate: The Data Plane

**Status:** implementation-grade spec for Phase ①/② of `playbook/ENHANCEMENT-GUIDE.md` (§2
facts substrate, §3 matrix). **Scope law:** `.context/decisions/2026-06-11-core-competence-and-scope-line.md`
(derive → infer → declare ladder). **Binding constraint:** G0 in `.context/GOALS.md:7`
(sub-ms queries, <200ms startup, <50MB memory, zero avoidable allocations on hot paths).

This document specifies how aOa derives architecture FACTS from any codebase: the fact
model, the keystone import-edge extraction, bbolt storage, incremental/delta updates, the
performance budget, and the graphify absorption plan. A Go engineer should be able to
implement from this document without further design work.

**Clean-room note:** `repo/graphify/` is behavioral reference only. Per the Isolation Rule
(`CLAUDE.md`), no code is copied or imported — we absorb *behaviors* (resolution rules,
edge semantics) and re-derive them, validated by fixtures.

---

## 0. Ground truth — what exists today

| Surface | Reality | Citation |
|---|---|---|
| Index shape | `Index{Tokens, Metadata(SymbolMeta), Files(FileMeta)}` — no edge of any kind | `internal/ports/storage.go:59-89` |
| Parser port | `ParseFileToMeta(path, source) ([]*SymbolMeta, error)` — symbols only | `internal/ports/parser.go:7-16` |
| Parse pass (build) | `BuildIndex` reads every file once, calls `ParseFileToMeta` | `internal/app/indexer.go:83-179` |
| Parse pass (watch) | `onFileChanged` re-parses single file, updates index in place | `internal/app/watcher.go:20-184` |
| Symbol extraction | per-language extractors iterate root children — the **same loop where import nodes live** | `internal/adapters/treesitter/parser.go:235-246, 347-363, 458-487, 532-549` |
| Import nodes already visited | `countImportSpecs` walks `import_spec_list`/`import_spec` (recon walker; counted, discarded) | `internal/adapters/treesitter/walker.go:567-583` |
| 28-language reality | extension map + `symbolRules` table | `internal/adapters/treesitter/extensions.go:114-239, 243-388` |
| Persistence pattern | project bucket → sub-buckets (`index`, `learner`, `sessions`, `dimensions`, `telemetry`); binary posting lists for hot blobs | `internal/adapters/bbolt/store.go:26-32`, `encoding.go:1-13` |
| Git available on build path | `git ls-files` already shells out during indexing | `internal/app/indexer.go:184-216` |

**The structural gap** (ENHANCEMENT-GUIDE §1): the index answers "where is symbol X" in
O(1) but cannot answer "what does package A import." Everything below closes that gap
inside the existing parse pass.

---

## 1. Fact model

### 1.1 Go types (`internal/ports/facts.go` — new file)

Facts are shared data types, so they live in `ports/` beside `Index`/`SymbolMeta`
(same rule that put those types in `internal/ports/storage.go:58-89`). Domain logic
(resolution, detectors) lives in `internal/domain/facts/` and depends only on these types.

```go
package ports

// FactKind enumerates the eight fact kinds. String-typed for stable JSONL
// serialization and forward compatibility (unknown kinds are skippable).
type FactKind string

const (
	FactUnit    FactKind = "unit"    // a module/package/component (the node grain)
	FactDep     FactKind = "dep"     // import/include/require edge (the keystone)
	FactRoute   FactKind = "route"   // HTTP/RPC endpoint exposure (Phase ③)
	FactSchema  FactKind = "schema"  // entity/table/migration shape (Phase ③)
	FactDeploy  FactKind = "deploy"  // container/k8s/compose topology (Phase ③)
	FactOwner   FactKind = "owner"   // CODEOWNERS / git authorship (Phase ③)
	FactDelta   FactKind = "delta"   // change vs a baseline ref (this spec, §4)
	FactFinding FactKind = "finding" // detector output: cycle, god, orphan (§4.4)
)

// Provenance is the honesty stamp from the scope-line ADR ladder
// (.context/decisions/2026-06-11-core-competence-and-scope-line.md):
// layer 1 derive=REAL, layer 2 infer=MIXED, layer 3 declare/ingest.
type Provenance string

const (
	ProvDerived  Provenance = "derived"  // REAL — tree-sitter / git / manifest
	ProvInferred Provenance = "inferred" // MIXED — agent named/grouped, never added
	ProvDeclared Provenance = "declared" // human declaration (.aoa/arch.yaml)
	ProvObserved Provenance = "observed" // ingested external truth (APM etc.)
)

// FactSource is the audit pointer. Every fact carries one; this is what makes
// evidence packs auditable (ENHANCEMENT-GUIDE §2).
type FactSource struct {
	File   string `json:"f"`           // repo-relative path
	Line   uint32 `json:"l"`           // 1-based; 0 = whole-file fact
	Commit string `json:"c,omitempty"` // short hash at emission time
}

// Fact is the universal record. Subject/Object are canonical IDs (§1.3).
// Attrs is small and bounded (≤8 keys); large payloads are a design error.
type Fact struct {
	Kind    FactKind          `json:"k"`
	Subject string            `json:"s"`
	Object  string            `json:"o,omitempty"`
	Attrs   map[string]string `json:"a,omitempty"`
	Source  FactSource        `json:"src"`
	Prov    Provenance        `json:"p"`
	TS      int64             `json:"t,omitempty"` // unix seconds, set at emission
}
```

### 1.2 JSONL wire format

One fact per line, the JSON encoding of `Fact` with the short keys above. This is the
append-only staging format (`{root}/.aoa/facts/pending.jsonl`, §3.3) **and** the
playbook mock contract — the Phase ① playbook substrate emits exactly these lines, and
the Go implementation must round-trip them byte-compatibly (parity discipline, G1 style).

```jsonl
{"k":"unit","s":"go:internal/app","a":{"lang":"go","files":"9"},"src":{"f":"internal/app/app.go","l":1,"c":"147ba46"},"p":"derived","t":1781136000}
{"k":"dep","s":"go:internal/app","o":"go:internal/ports","a":{"spec":"github.com/corey/aoa/internal/ports"},"src":{"f":"internal/app/indexer.go","l":11,"c":"147ba46"},"p":"derived","t":1781136000}
{"k":"dep","s":"py:graphify/build","o":"ext:networkx","a":{"spec":"networkx","scope":"external"},"src":{"f":"graphify/build.py","l":4},"p":"derived"}
{"k":"finding","s":"go:internal/domain/index","a":{"rule":"fan_in_gt","value":"14","threshold":"12"},"src":{"f":"internal/domain/index","l":0,"c":"147ba46"},"p":"derived"}
```

Rules:
- UTF-8, LF-terminated, no pretty-printing. Unknown keys ignored on read.
- `dep` facts written by the **parser** are *raw* (Object empty, `attrs.spec` = the
  literal specifier). The **compactor** resolves them (§2.4) and writes resolved facts
  to bbolt; raw spec is preserved in attrs for audit.

### 1.3 Canonical IDs

```
<ns>:<path>      ns = language family or "file"/"ext"
go:internal/app                  Go package (directory, module-relative)
py:graphify/extract              Python module (file sans .py, root-relative)
ts:src/components/Button         TS/JS module (file sans extension)
java:com.example.billing         Java package (FQN)
file:internal/app/indexer.go     raw file (fallback unit for unmapped languages)
ext:networkx                     external dependency (not in repo)
```

ID rules: forward slashes, repo-relative, no leading `./`, lowercase namespace prefix.
The namespace makes cross-language estates unambiguous and matches graphify's
language-family insight (`repo/graphify/analyze.py:19-31`) without its label-collision
problem (§6).

**Grain:** raw `dep` facts are emitted at *file* grain (auditable file:line). The
compactor aggregates to *unit* grain for adjacency (§3.2). Unit per language: Go =
package dir; Python = module file, rolled up to package dir for views; TS/JS = module
file, rolled up to dir; Java = declared package. Rollup is pure path math at read time —
both grains are queryable.

### 1.4 Ports interfaces (`internal/ports/facts.go`, continued)

```go
// FactSink receives facts during the parse pass. Implementations MUST be
// O(1) amortized per call and must never block the parse loop (buffered
// append; flush happens off the hot path). Adapter: internal/adapters/factlog
// (JSONL writer). A nil-safe NullSink is provided for light builds.
type FactSink interface {
	Emit(f Fact)
	Flush() error
}

// FactStore is the durable, queryable substrate. Adapter: internal/adapters/bbolt
// (same DB file, new sub-buckets — §3). All methods project-scoped, mirroring
// ports.Storage (internal/ports/storage.go:12-56). Writes transactional.
type FactStore interface {
	// ReplaceFactsForFile atomically swaps all raw facts attributed to one file
	// (the incremental unit of work — §4.1). Empty facts slice = pure delete.
	ReplaceFactsForFile(projectID, path string, facts []Fact) error

	// PutResolved writes compactor output: unit records + adjacency. Overwrites.
	PutResolved(projectID string, units []Fact, adj *DepAdjacency) error

	FactsByKind(projectID string, kind FactKind) ([]Fact, error)
	FactsForSubject(projectID, subject string) ([]Fact, error)

	// O(1) bucket get + one posting-list decode each (§3.2, §5).
	Dependencies(projectID, unit string) ([]DepEdge, error) // unit → its imports
	Dependents(projectID, unit string) ([]DepEdge, error)   // who imports unit

	SaveBaseline(projectID, name string, b *FactBaseline) error
	LoadBaseline(projectID, name string) (*FactBaseline, error) // nil,nil if absent

	DeleteProjectFacts(projectID string) error // wired into `aoa remove`
}

// DepEdge is one resolved unit-grain edge with evidence count.
type DepEdge struct {
	Unit  string // the other endpoint
	Count uint16 // number of file-grain import sites backing this edge
}

// DepAdjacency is the compactor's resolved graph (forward + reverse).
type DepAdjacency struct {
	Fwd map[string][]DepEdge
	Rev map[string][]DepEdge
}

// FactBaseline is a frozen snapshot for delta/conformance (§4.2).
type FactBaseline struct {
	Ref       string   // git ref or user-chosen name
	Commit    string   // resolved short hash
	CreatedAt int64
	Units     []string
	Edges     []BaselineEdge // sorted, deduped: subject, object
	Findings  []string       // stable finding keys (rule|subject)
}

type BaselineEdge struct{ S, O string }
```

```go
// FactParser is the optional widening of ports.Parser (internal/ports/parser.go:7).
// One parse, two outputs — never a second tree walk over the same file.
// BuildIndex/onFileChanged type-assert: if the parser implements FactParser,
// facts ride the pass; otherwise (light build, --light) the substrate simply
// has no derived deps and `aoa arch` reports that honestly.
type FactParser interface {
	Parser
	// ParseFileToMetaAndFacts returns symbols (identical to ParseFileToMeta)
	// plus raw facts extracted from the same tree.
	ParseFileToMetaAndFacts(path string, source []byte) ([]*SymbolMeta, []Fact, error)
}
```

Hexagonal placement (G4, `CLAUDE.md` architecture):

```
internal/ports/facts.go            types + FactSink/FactStore/FactParser
internal/domain/facts/             resolver (spec→unit), compactor, detectors — dependency-free
internal/adapters/treesitter/      imports.go: per-language raw-fact extraction (§2)
internal/adapters/factlog/         JSONL sink (pending.jsonl writer/reader)
internal/adapters/bbolt/facts.go   FactStore implementation (§3)
internal/app/                      wiring: parse pass → sink → compactor → store; git delta (§4.2)
cmd/aoa/cmd/arch.go                `aoa arch facts ...` (separate spec, 02-)
```

---

## 2. Import-edge extraction — the keystone

### 2.1 Where the hooks go

There are two tree walks in the codebase today; the keystone hooks into the **index
parse path**, not the recon walker:

1. **Index path (the hook point).** `Parser.ParseFileToMeta` → `extractSymbols`
   dispatch (`internal/adapters/treesitter/parser.go:104, 235-246`). The language
   extractors (`extractGo:347`, `extractPython:458`, `extractJavaScript:532`,
   `extractGeneric:250`) already iterate the root node's children — exactly where
   `import_declaration` / `import_statement` / `import_from_statement` nodes sit as
   siblings of the symbol declarations. Adding import emission is **one more `case`
   in loops that already run**. No new walk, no second parse (G0).

2. **Recon walker (proof, not hook).** `walker.go:567-583` (`countImportSpecs`)
   already traverses `import_spec_list`/`import_spec` for the import-bloat rule —
   proof the nodes are reachable and cheap — but that walk only runs in the parked
   recon path (`WalkForDimensions`, `walker.go:21`). When recon is reactivated it can
   feed the same `FactSink`; it is not the keystone.

Concrete change set in `internal/adapters/treesitter/`:

- New file `imports.go`: `extractImportsGo/Python/JS/Java(root, source, path) []ports.Fact`.
- `parser.go`: add `ParseFileToMetaAndFacts` which parses once (existing
  `parser.Parse` at `parser.go:101`) and calls both `extractSymbols` and the
  import extractor for the detected language. `ParseFileToMeta` keeps its exact
  current behavior (zero regression risk for existing callers).
- `internal/app/indexer.go:140` and `internal/app/watcher.go:132`: type-assert
  `ports.FactParser`; when satisfied, collect facts per file and `sink.Emit` them
  right after the symbol loop. `unit` facts are synthesized by the compactor from
  the file table, not by the parser (the parser sees one file at a time).

### 2.2 Per-language node kinds (P1: Go, Python, TS/JS, Java)

Emission is **raw**: capture the literal specifier + alias + line; do not resolve on
the parse path (§2.4 explains why). One `dep` fact per import site.

| Lang | Node kinds to match (top-level) | Specifier extraction | Attrs captured |
|---|---|---|---|
| Go | `import_declaration` → optional `import_spec_list` → `import_spec` (also bare single `import_spec`) — same shapes `countImportSpecs` walks at `walker.go:570-581` | `path` field: `interpreted_string_literal`, strip quotes | `spec`; `alias` from leading `package_identifier`; `dot`/`blank` for `.`/`_` imports |
| Python | `import_statement` (children `dotted_name` \| `aliased_import`); `import_from_statement` (field `module_name` = `dotted_name` \| `relative_import`) | dotted name text; for `relative_import`, count `import_prefix` dots + optional dotted_name (graphify behavior at `repo/graphify/extract.py:1209-1217`) | `spec` (e.g. `..pkg.mod` kept literal); `names` = comma list of imported symbols (`dotted_name`/`aliased_import`/`wildcard_import` children) |
| JS/TS/TSX | `import_statement` (field `source`: `string`); `export_statement` **with** a `string` source child = re-export (graphify's from-clause test, `extract.py:1251-1260`); `call_expression` whose `function` is the `import` token = dynamic import (`extract.py:1337-1396`); `variable_declarator` whose value is a `require("…")` call, incl. member access `require("m").x` (`extract.py:1621-1700`) | the `string`/`template_string` literal (skip template strings containing `template_substitution` — unresolvable, graphify `extract.py:1368`) | `spec`; `kind`=`static\|reexport\|dynamic\|require`; `names` from `named_imports`→`import_specifier` / `export_clause`→`export_specifier` |
| Java | `import_declaration` → `scoped_identifier` (recursive scope/name fields, graphify `_walk_scoped` at `extract.py:1400-1415`); ALSO `package_declaration` → `scoped_identifier` (needed for resolution, §2.4) | full FQN text — **keep it whole** (graphify truncates to last segment, `extract.py:1420` — a defect we fix, §6) | `spec` = FQN; `wildcard`=`1` if `asterisk` child; `static`=`1` if static import; package facts: `kind`=`package` |

Dynamic-import and require detection require descending past the root level for JS
only. Bound it: descend into top-level statement subtrees with a node-kind allowlist
(`lexical_declaration`, `variable_declaration`, `expression_statement`,
`export_statement`) — matching where graphify finds them — not a full-tree scan.
Deeper dynamic imports (inside functions) are Phase ③ via the recon walker, stamped
the same `derived` (it's still the AST).

### 2.3 P2 languages (the other 24)

Same pattern, prioritized by estate frequency; each is a table entry in `imports.go`,
not new machinery. Node kinds for the next tier (matching graphify's coverage at
`extract.py:1825-2143` where it exists, going beyond where it doesn't):

| Lang | Kinds | Note |
|---|---|---|
| Rust | `use_declaration` | crate-relative paths; `mod` decls map files |
| C/C++ | `preproc_include` | quoted = relative resolve; `<...>` = `ext:` (graphify `extract.py:1438-1452`) |
| C# | `using_directive` | namespace FQN, same package-map strategy as Java |
| Kotlin | `import_header` | Java rules apply |
| Scala | `import_declaration` | Java rules apply |
| PHP | `namespace_use_clause` + `require`/`include` calls | |
| Ruby | `call` with method `require`/`require_relative` | graphify has none (`import_types=frozenset()`, `extract.py:1944`) — we exceed |
| Swift | `import_declaration` | module-level only |
| Elixir | `call` with `import`/`alias`/`use`/`require` atoms | |
| Lua | `require("a.b")` in `variable_declaration` (graphify `extract.py:2023-2064`) | |

Languages with no extractor yet emit nothing — the view layer states "deps: not
derived for <lang>" rather than pretending (scope-line honesty).

### 2.4 Resolution: specifier → unit (compact-time, two-phase)

**Design rule: emission is dumb, resolution is smart.** Resolution needs the *whole*
file table (you cannot resolve `from graphify import build` until you know whether
`graphify/build.py` exists) and manifest knowledge. Doing it per-file during parse
would be O(F²)-ish and would re-do work on every watch event. So:

- **Phase A (parse pass):** emit raw `dep` facts with `attrs.spec`. Cost: string slice.
- **Phase B (compactor, `internal/domain/facts/resolve.go`):** with the complete file
  table (`Index.Files`, `storage.go:62`) + manifest table, resolve every raw spec to a
  unit ID or `ext:`. Pure function: `Resolve(rawFacts, fileSet, manifests) (resolved []Fact, adj DepAdjacency, unresolved []Fact)`.

Manifest table (built once per compact; all are cheap reads, cached by mtime):

| Manifest | Gives | Used for |
|---|---|---|
| `go.mod` (every one found — monorepos have many) | module path → dir | Go internal-vs-external split |
| `package.json` `name` + `workspaces`; `pnpm-workspace.yaml` | package name → dir | bare-specifier → workspace package (graphify `extract.py:287-375`) |
| `tsconfig.json` `compilerOptions.paths` + `extends` chain (JSONC-tolerant) | alias prefix → base dir | TS path aliases (graphify `extract.py:179-269`) |
| `pyproject.toml` / presence of `<root>/<top>/__init__.py` or `src/<top>/` | python package roots | absolute-import resolution |
| Java `package_declaration` facts (from §2.2) | package FQN → source dirs | import FQN prefix match — **no** src-layout convention guessing |

Per-language resolution rules:

**Go.** Spec is always a full module path. If spec has any collected `go.mod` module
path as a prefix → internal: unit = `go:` + dir of that module + remainder. Else
`ext:<spec>`. Std lib (no dot in first path segment) → `ext:std/<spec>`.

**Python.** Relative (`.`/`..` prefix): walk up `dots-1` directories from the
importing file's dir, append `module.replace(".","/")`; probe `<p>.py` then
`<p>/__init__.py` (absorb graphify `extract.py:1209-1217`, plus the `__init__.py`
probe it lacks). Absolute: probe each python root for `<root>/<spec-as-path>.py` or
`/__init__.py`; first hit wins; miss → `ext:<top-level>`. Improvement over graphify:
graphify leaves absolute imports as bare-name nodes (`_make_id(raw)`,
`extract.py:1219`) which collide; we resolve to real module units or explicit `ext:`.

**TS/JS.** In order: (1) relative → join with importer dir, probe extension ladder
`.ts .tsx .js .jsx .mjs .cjs /index.{ts,tsx,js}` plus exact (graphify
`_resolve_js_import_path`, `extract.py:146-177`); (2) tsconfig alias prefix match,
longest-prefix-first; (3) workspace package name match, probing `exports`/`main`
entry candidates (graphify `extract.py:341-389`); (4) `ext:<bare-name>` (scoped
packages keep scope: `ext:@scope/pkg`).

**Java.** Import FQN longest-prefix match against the package→dir map derived from
`package_declaration` facts. `import a.b.C` resolves when package `a.b` is in-repo →
`java:a.b` (unit grain is package; class-level precision is attrs). Wildcards resolve
to the package. Miss → `ext:` with the top-two segments (`ext:org.springframework`).

**Unresolved handling:** specs that *look* internal (relative paths that probe to
nothing) are kept as raw facts in an `unresolved` set with `attrs.reason` — they are
findings fuel (broken import candidates) and re-resolve cheaply when a matching file
appears (§4.1). Never silently dropped, never fabricated (scope law).

**Determinism:** resolution is a pure function of (file set, manifests, raw facts).
Same inputs → byte-identical adjacency. This is the property graphify lacks (Leiden
clustering + LLM labels) and it is what makes parity fixtures possible (§7).

---

## 3. Storage — bbolt layout

Same DB (`{root}/.aoa/aoa.db`), same project-bucket pattern as
`internal/adapters/bbolt/store.go:26-32, 130-134`. New sub-buckets under the project
bucket; everything dies with `DeleteProject` / `aoa remove` (project-scoped law,
`CLAUDE.md`).

```
{projectID}/
  facts_meta       k: "format"|"commit"|"compacted_at"|"counts" → scalar
  facts_raw        k: file \x00 line \x00 seq        → Fact (JSON)        // audit trail
  facts_byfile     k: file                           → []rawKey (binary)  // incremental delete index
  facts_units      k: unitID                         → unit Fact (JSON)
  facts_dep_fwd    k: unitID                         → posting list of DepEdge
  facts_dep_rev    k: unitID                         → posting list of DepEdge
  facts_unresolved k: spec \x00 file \x00 line       → Fact (JSON)
  facts_findings   k: rule \x00 subject              → Fact (JSON)
  facts_baseline   k: name                           → FactBaseline (gzip JSON)
  facts_kind_*     (route|schema|deploy|owner)       → Fact (JSON), keyed subject\x00line   // Phase ③
```

**Adjacency encoding** mirrors the proven posting-list format
(`internal/adapters/bbolt/encoding.go:1-13`): little-endian,
`edgeCount:uint32`, then per edge `unitLen:uint16 + unit + count:uint16`. One bucket
`Get` + one linear decode of a single unit's edges = the O(1)-ish read the guide
demands (ENHANCEMENT-GUIDE §2). Unit IDs are short strings (~20-40B); a 50-edge unit
decodes in ~2-5µs.

**Why both raw and resolved:** raw facts are the audit trail (`aoa arch facts <subject>`
returns file:line evidence for every edge — the anti-"seller diagram" property);
resolved adjacency is the query plane. Raw is written per-file (incremental unit);
resolved is rewritten wholesale by the compactor (it's small: units ≤ ~2k, edges ≤
~20k for any sane repo — full rewrite is one tx of ~1-3MB, tens of ms, off hot path).

**Compaction cadence:**
1. **Index build:** parse pass streams facts to `pending.jsonl`; on completion,
   compactor runs once, writes raw + resolved in one tx, truncates the JSONL.
2. **Watch:** per-file re-emit (§4.1) updates `facts_raw`/`facts_byfile` immediately;
   resolved-graph recompute is debounced 500ms after the last event (same spirit as
   the existing fsnotify debounce) and recomputes **in memory from the raw buckets**
   — not from re-parsing.
3. **Detectors** (§4.4) run at the end of every compaction, never at render time
   (ENHANCEMENT-GUIDE §4).

### 3.1 JSONL staging (`{root}/.aoa/facts/`)

```
.aoa/facts/pending.jsonl   append-only during a parse pass; truncated after successful compact tx
.aoa/facts/baseline-<name>.jsonl   optional human-readable export of a baseline (aoa arch pack)
```

Purpose: (a) the parse pass never holds a bbolt write tx (crash mid-build loses
nothing committed — same guarantee as `storage.go:10-11`); (b) byte-identical
contract with the Phase ① playbook mock substrate; (c) agents and humans can read it.
The JSONL is staging, not the store — queries never touch it.

---

## 4. Incremental updates and deltas

### 4.1 fsnotify-driven re-emission

Hook: `internal/app/watcher.go:20` (`onFileChanged`), immediately after the existing
re-parse at `watcher.go:132`:

```
file changed → ParseFileToMetaAndFacts (already parsing!) →
  store.ReplaceFactsForFile(projectID, relPath, newFacts)   // one tx: delete via facts_byfile, insert new
  → mark factsDirty; debounce 500ms → compactor.RecomputeResolved()
file deleted → ReplaceFactsForFile(projectID, relPath, nil) → same debounce
file created → additionally: probe facts_unresolved for specs this path satisfies
               (key prefix scan on spec) → move to resolved on next compact
```

Cost per event: the parse was already paid (`watcher.go:132`); the fact swap is one
small tx; the debounced recompute is O(total raw deps) map work (~1-5ms at 5k deps).
The "touch-one-package demo" (ENHANCEMENT-GUIDE §8 Phase ①) is exactly: edit a file →
within ~600ms `Dependencies()`/views reflect the new edge, with the new file:line.

### 4.2 Delta facts — "what changed since ref X"

Two complementary mechanisms; both lazy, neither on the hot path:

**(a) Git-diff deltas (no baseline needed).** `aoa arch facts --since <ref>`:
1. `git diff --name-status <ref>..HEAD` (one exec, same pattern as
   `indexer.go:186`) → changed file set.
2. Map files → units (path math) → seed set.
3. Affected closure = reverse-BFS over `facts_dep_rev`, bounded depth (default 2,
   flag `--depth`) — graphify's `affected_nodes` behavior
   (`repo/graphify/affected.py:74-110`) on O(1) adjacency instead of a networkx scan.
4. Emit transient `delta` facts: `{k:"delta", s:unit, a:{change:"modified|affected", via:"dep", depth:"1"}, src:{c:"<ref>"}}`.

**(b) Baseline snapshots (freeze semantics, the ArchUnit pattern — ENHANCEMENT-GUIDE §4).**
`aoa arch baseline save <name>` → `FactBaseline` (units + sorted edges + finding
keys) into `facts_baseline`. `aoa arch findings --new` / `aoa arch facts --baseline <name>`
diff current resolved state against the snapshot as set operations (graphify's
`graph_diff`, `analyze.py:539-620`, absorbed wholesale but as persisted facts):
`delta` facts with `attrs.change ∈ {dep_added, dep_removed, unit_added, unit_removed, finding_new, finding_resolved}`.
Diff cost: O(E log E) on ≤20k edges ≈ 1-2ms.

Commit stamping: compactor records `git rev-parse --short HEAD` once per compact into
`facts_meta/commit`; every fact emitted in that compact inherits it. Dirty worktree →
commit suffixed `+` (honesty over prettiness).

### 4.3 Light builds

`./build.sh --light` has no tree-sitter, so no `FactParser`. The substrate exists but
holds only manifest/owner facts (no AST needed). `aoa arch` must say "dep facts:
unavailable (light build)" — never an empty diagram presented as truth.

### 4.4 Detectors (findings facts, compact-time)

Run in `internal/domain/facts/detect.go` over `DepAdjacency` at the end of each
compact; output `finding` facts (ENHANCEMENT-GUIDE §4 — findings are facts):

| Rule | Algorithm | Cost | graphify equivalent |
|---|---|---|---|
| `cycle` | Tarjan SCC over unit graph; each SCC >1 → one finding per cycle, members in attrs | O(V+E), <1ms | `find_import_cycles` via `nx.simple_cycles` with explosion guards (`analyze.py:623-724`) — Tarjan is deterministic and complete |
| `god_unit` | fan-in + fan-out > threshold (default 12/12, configurable) | O(V) | degree-sorted `god_nodes` (`analyze.py:95-116`) |
| `orphan` | unit with zero inbound deps and not an entrypoint glob (`cmd/**`, `main.*`, tests) | O(V) | isolated-nodes question (`analyze.py:499-510`) |
| `dead_candidate` | orphan + zero symbol references; ALWAYS "candidate", reflection caveat in attrs | O(V) | none — scope-line item |
| `broken_import` | unresolved spec that probed as internal | O(unresolved) | none |

---

## 5. Performance budget (G0 compliance)

Reference numbers: tree-sitter parse of a typical source file is ~0.3-1ms; aOa-scale
repo ≈ 1k files / ~5k import sites; a big monorepo ≈ 30k files / ~200k import sites.

| Path | Budget | How it's met |
|---|---|---|
| Parse-pass overhead (emission) | **≤ +3% BuildIndex wall time; ≤ +5% allocs** (benchmark-gated, §7) | import nodes are siblings in loops that already run (`parser.go:347-363` etc.); per fact: 1 struct + 2-3 short strings ≈ 200B; no resolution, no I/O per fact (sink buffers 64KB, flushes off-loop) |
| Compact (full) | ≤ 50ms @ 5k deps; ≤ 1s @ 200k deps | pure map work + one bbolt tx; runs post-build / debounced, never blocks a query |
| Watch event | parse already paid; fact swap ≤ 2ms; debounced recompute ≤ 5ms typical | §4.1 |
| `Dependencies`/`Dependents` query | **≤ 50µs warm** (sub-ms end-to-end via socket, G0) | one bucket Get + posting-list decode (§3) |
| `FactsForSubject` (evidence) | ≤ 200µs | prefix cursor scan on `facts_raw` keyed by file; subjects map to ≤ dozens of raw facts |
| Delta `--since` | ≤ 100ms incl. git exec | git dominates; graph math is µs-ms |
| Memory steady-state | **≤ +3MB** within the 50MB ceiling (GOALS.md:7) | resolved adjacency cached lazily on first arch query (~1-2MB at 20k edges); raw facts stay on disk; staging buffer capped 4MB, flushed |
| Startup | +0ms | nothing loads eagerly; facts buckets opened on demand |

**Eager vs lazy:** eager = raw `dep`/`unit` emission (rides the pass) and compaction.
Lazy = adjacency cache load (first query), detectors beyond the core three (first
`findings` call after a compact marks them fresh), Phase ③ extractors
(route/schema/deploy run only when their views are first requested — per the matrix,
ENHANCEMENT-GUIDE §3), git deltas (per invocation).

---

## 6. Graphify parity+ table

What `repo/graphify/` actually does, and where aOa absorbs or surpasses it. Graphify's
pipeline: per-file tree-sitter extractors → node/edge dicts (`extract.py`) → networkx
graph build + label dedup (`build.py`) → Leiden clustering + LLM labels (`cluster.py`)
→ analysis (`analyze.py`) → JSON file storage (`global_graph.py:10-12`, `~/.graphify/`).

| Feature | graphify behavior | aOa equivalent | Improvement |
|---|---|---|---|
| Import edges, Python | `import_statement`/`import_from_statement`; relative resolved to paths, absolute kept as bare-name nodes (`extract.py:1187-1229`) | same node kinds, raw-then-resolve (§2.4) | absolute imports resolved against package roots or explicit `ext:`; `__init__.py` probing; no name-collision nodes |
| Import edges, JS/TS | static + re-export + dynamic `import()` + CJS require, symbol-level edges for named imports (`extract.py:1250-1396, 1636-1700`) | same four mechanisms (§2.2) | resolution deferred to compact = deterministic + incremental; named imports kept as attrs not extra nodes (no node soup) |
| tsconfig aliases / workspaces | full support incl. `extends` chains, JSONC, pnpm/yarn workspaces (`extract.py:179-389`) | **port behavior** into manifest table (§2.4) | cached by mtime; shared across all files in the compact instead of per-import lookup |
| Import edges, Java | FQN truncated to last segment → cross-package collisions (`extract.py:1399-1435`, esp. `:1420`) | full FQN + package-declaration map (§2.2, §2.4) | correct targets in any repo with duplicate class names |
| Import edges, Go | `import_spec(_list)` → `imports_from`, alias tracking for call gating (`extract.py:5722+`) | same kinds (already walked at `walker.go:567-583`), go.mod-aware | multi-module monorepos; std-lib vs external split |
| Language coverage | ~14 languages with import extraction; Ruby explicitly none (`extract.py:1944`) | P1 4 langs Phase ②, P2 tiered to 28 (§2.3) | wider; and honest "not derived" for the rest |
| Confidence tags | `EXTRACTED`/`INFERRED`/`AMBIGUOUS` per edge | `Provenance` ladder per fact (§1.1) | aligned to the scope-line ADR; renders as REAL/MIXED stamps |
| Grouping | Leiden community detection + LLM labels (`cluster.py:86`) — nondeterministic, needs LLM | path-prefix + atlas domain (`FileMeta.Domain`, `storage.go:87`) for grouping; agent may *name* groups (MIXED) | deterministic substrate; inference confined to layer 2 per scope law |
| Cycles | `nx.simple_cycles` + early-stop guards (`analyze.py:623-724`) | Tarjan SCC at compact (§4.4) | complete, deterministic, O(V+E), persisted as findings |
| God nodes | degree ranking with noise blocklists (`analyze.py:95-116`) | fan-in/out threshold detector | thresholded finding with evidence, not a top-N listicle |
| Affected set | reverse BFS depth-2 over relation allowlist (`affected.py:74-110`) | reverse adjacency BFS (§4.2a) | O(1) per hop via `facts_dep_rev`; bound + via-edge reported |
| Graph diff | snapshot-vs-snapshot node/edge sets (`analyze.py:539-620`) | baseline facts + set diff (§4.2b) | persisted, named baselines; freeze semantics; finding-aware |
| Incremental | watch + pending-file lock dance + stat-hash cache, then rebuild (`watch.py`, `cache.py:27-37`) | fsnotify per-file fact swap + debounced recompute (§4.1) | no rebuild lock, no pending-file protocol; rides the existing watcher |
| Storage / query | networkx in RAM, JSON files in `~/.graphify` (`global_graph.py:10-12`) | bbolt buckets, project-scoped `.aoa/` (§3) | O(1) reads without loading a graph; survives restarts; `aoa remove` wipes clean |
| Provenance pointers | `source_file` + `L<line>` on edges | `FactSource{file,line,commit}` on every fact | commit stamping; evidence query (`aoa arch facts <subject>`) |
| Multi-repo | global graph merge by repo tag (`build.py:467`, `global_graph.py:58`) | out of scope here; Phase ④ estate rollup (ENHANCEMENT-GUIDE §3, estate landscape) | project-scoping law preserved until ④ |
| Calls/inheritance edges | rich symbol-level `calls`/`inherits` edges with cross-file resolver (`symbol_resolution.py`) — also its noisiest output (resolver pollution suppressed in `analyze.py:217-224`) | **deliberately not Phase ②** | the keystone is unit-grain deps; symbol-level call edges arrive with the sequence view (Phase ③) only with per-hop evidence — avoiding graphify's pollution by construction |

**Net:** after Phase ②, every architectural question graphify answers for one Python
repo (deps, cycles, affected, diff, hotspots) is answered by aOa across the P1
languages, deterministically, with commit-stamped evidence, on O(1) storage — and
graphify itself is just an estate in the dropdown (ENHANCEMENT-GUIDE §7).

---

## 7. Phased tasks and test strategy

### 7.1 Tasks (board-ready; sizes per the guide's scale S<1d, M≈2-4d)

| ID (suggested) | Task | Size | Depends |
|---|---|---|---|
| FS.1 | `ports/facts.go`: types, JSONL codec, `FactSink`/`FactStore`/`FactParser`; `adapters/factlog` pending.jsonl writer/reader; `NullSink` | S | — |
| FS.2 | `adapters/bbolt/facts.go`: buckets, posting-list adjacency, `ReplaceFactsForFile`, baselines; wire `DeleteProjectFacts` into `aoa remove` | M | FS.1 |
| FS.3 | Keystone Go: `treesitter/imports.go` Go extractor + `ParseFileToMetaAndFacts`; hook `indexer.go:140` + sink | M | FS.1 |
| FS.4 | `domain/facts/resolve.go`: manifest table (go.mod first), resolver, compactor, `DepAdjacency` | M | FS.2, FS.3 |
| FS.5 | Python + TS/JS extractors + resolvers (relative, roots, ext ladder, tsconfig, workspaces) | M | FS.4 |
| FS.6 | Java extractor + package-map resolution | S | FS.4 |
| FS.7 | Incremental: `watcher.go:132` hook, fact swap, debounced recompute, unresolved re-probe | M | FS.4 |
| FS.8 | `domain/facts/detect.go`: cycle/god/orphan/dead-candidate/broken-import → finding facts | S | FS.4 |
| FS.9 | Git deltas + baselines: `--since`, baseline save/diff, commit stamping | M | FS.4 |
| FS.10 | Benchmark gate + perf fixtures (≤+3% wall, ≤+5% allocs, query ≤50µs) in CI (`make check`) | S | FS.3 |
| FS.11 | P2 language tier 1 (Rust, C/C++, C#, Kotlin, Ruby) | M | FS.5 |

`aoa arch` CLI surface and renditions are the next spec (02-), per ENHANCEMENT-GUIDE §5.

### 7.2 Test strategy (mirrors `test/fixtures/` + parity discipline)

```
test/fixtures/facts/
  repos/                      mini-repos, one per scenario (checked in, ~10-30 files each)
    go-multimod/              two go.mod modules, internal+external+std imports
    py-roots/                 src/ layout + flat layout, relative depths 1-3, __init__ chains
    ts-aliases/               tsconfig paths + extends, pnpm workspace, CJS+dynamic+re-export
    java-pkgs/                two packages sharing a class name (the graphify-defect case)
    broken/                   imports that resolve to nothing (unresolved fixtures)
  expected/
    <repo>.facts.jsonl        golden raw facts (sorted: file, line, seq)
    <repo>.resolved.json      golden adjacency {fwd, rev} + unit list
    <repo>.findings.json      golden findings (cycle planted in go-multimod)
    <repo>.delta.json         golden delta for a scripted git mutation
```

Assertions, parity-style (`test/parity_test.go` pattern — exact match, zero tolerance):

1. **Emission parity:** `BuildIndex` over each fixture repo → emitted JSONL equals
   golden byte-for-byte after canonical sort (commit/ts fields zeroed).
2. **Resolution determinism:** resolver over golden raw facts → golden adjacency;
   run twice, assert identical (no map-order leakage).
3. **Round-trip:** JSONL → Fact → JSONL identity; bbolt put/get identity; posting-list
   encode/decode identity (mirror `encoding.go` test approach).
4. **Incremental equivalence:** full build of repo state B **==** (build state A, then
   apply file events A→B). This is the substrate's most important invariant.
5. **Known-repo assertion (self-test):** run on aOa itself; assert hexagonal truths:
   `go:internal/domain/* → go:internal/adapters/*` edge set is **empty**, all
   `internal/*` units depend on `go:internal/ports`, cycle findings == 0. The repo's
   own architecture (CLAUDE.md) becomes a regression test.
6. **Graphify cross-check (absorption proof):** run aOa over `repo/graphify/` and
   assert the resolved Python edge set ⊇ the file-level `imports_from` edges
   graphify extracts for itself (modulo its bare-name absolute imports, which we
   resolve more precisely — documented exceptions list in the fixture).
7. **Perf gate:** `go test -bench BenchmarkBuildIndexFacts` asserting overhead and
   query budgets of §5; runs in `make check`.

### 7.3 Definition of done (Phase ② exit)

- The five GREEN views' data needs (component/dsm/cycles/code/techportfolio —
  ENHANCEMENT-GUIDE §3) are answerable from `FactStore` alone on a stranger's repo.
- `aoa arch facts <unit>` returns every edge with file:line:commit evidence.
- Touch-one-package demo passes end-to-end on a live watcher.
- All §7.2 suites green; perf gate green; `make check` clean.
