# aOa Agent Query Contract — v1

The canonical contract an agent composes against when querying an aOa-indexed repo.
Every command and output shape below was run live against this repo (742 files, daemon up,
2026-07-09). Doctrine references (C-IDs) point to `.context/details/2026-07-07-answering-doctrine.md` §4.

Scope: this contract covers what the graph KNOWS, what each verb ANSWERS, what it COSTS,
and what it REFUSES. If a question maps to the refusal table (§4), do not fake an answer.

---

## §1 The substrate

### Node kinds

| Kind | Example | Grain | Peekable body? |
|---|---|---|---|
| symbol | `Learner.Observe(event ObserveEvent)` | method/type, `[start-end]` boundaries | yes, via peek code |
| file | `internal/app/observer.go` | whole file | no — Read it |
| unit | `u_internal_adapters_bbolt` | package/directory | no — locate → grep → peek into it |
| group | `g_adapters` | top-level grouping of units | no |
| `ext:` | `ext:std`, `ext:json` | external package | **never** — package grain only, no source |

### Edge kind — imports, and only imports

There is exactly ONE edge kind: **import edges**. The importer side is stamped with a real,
peekable `file:line`. The imported side is unit grain (a unit ID, not a symbol).

- NEVER present an import edge as a call, a data flow, or a type relationship. Those edges
  do not exist in this substrate. "A imports B" is the whole claim.
- `ext:` edges: the importing `file:line` is real; the target resolves to package grain only.

### Overlays

| Overlay | What it is | Honesty caveat |
|---|---|---|
| domains | ~134 semantic domains (`@llm`, `@filesystem`) tagged on symbols in grep output | Coverage is partial (~5% of units carry a domain). A blank domain is honest silence, not an error. `@domain` is NOT a query filter — see §4. |
| findings | derived architecture findings (god components, budget overflows) with `file:line` sources | provenance `derived` (REAL); 9 findings live on this repo |
| heat | decayed attention counters per file (`/api/top-files`) | **"tool-observed sessions, not VCS history."** No timestamps — never say "recently". Heat ranks and tiebreaks; it never proves anything. |

### Provenance vocabulary

Every arch payload carries a `prov` stamp. Trust accordingly:

- `derived` — REAL. Deterministically computed from import edges. Cite it as fact.
- `mixed` — partially heuristic (e.g. `arch view code`: "MIXED · symbols real · subset
  heuristic · edges real import deps"). Cite the real parts; label the heuristic parts.
- A **refusal is an answer**. "No import path exists" and "0 hits" are first-class results.

---

## §2 The verbs

All CLI verbs run from the repo root. Under Claude Code (`CLAUDECODE=1`) the agent output
grammar (peek codes, `[start-end]`, domains) is automatic. Token costs are order-of-magnitude,
measured on this repo.

### Verb index

| Verb | Question (C-ID) | Cost | Escalation |
|---|---|---|---|
| `aoa grep <pat>` | where is X defined / what carries this name (C8, C10, C15) | ~700 tok | `aoa peek <code>` |
| `aoa egrep 'A\|B'` | candidates / precedent across terms (C15, C16, C17) | ~700 tok | peek best candidate |
| `aoa grep --scope <path> <pat>` | X within this package/file (C6, C9) | ~500 tok | peek |
| `aoa peek <code…>` | show me the body (C8 rung 5) | ~100–500/code | Read at `[start-end]` if peek fails |
| `aoa locate <name>` | which file is this (L0) | ~50 tok | tree or grep |
| `aoa find <glob>` | files matching pattern (L0) | ~50 tok | — |
| `aoa tree <dir> -d N` | what's inside this package (C6) | ~200 tok | scoped grep |
| `aoa arch views` | orient: what views exist + captions (C1) | ~200 tok | `arch view <id>` |
| `aoa arch view component` | how is this organized (C1) | **~20k raw — jq-trim to ~150** | expand ONE group |
| `aoa arch view dsm` | who depends on whom, incl. `ext:` (C4, C11) | ~1.3k tok | derive a specific path |
| `aoa arch view cycles` | any dependency cycles (C22-adjacent) | ~80 tok | findings |
| `aoa arch derive A B` | does A reach B via imports (C13, C14 floor) | ~30 tok | locate → grep → peek last hop |
| `aoa arch findings` | what's flagged (C22) | **~12k raw — jq-trim to ~100** | jq the sources of ONE finding |
| `aoa arch findings --new` | CI gate: did my diff add drift (C22) | exit code | jq messages |
| `aoa arch drift <target.aoa>` | does the real import graph match a declared target (GOV-1) | exit code (1=violation) | `--json` for file:line per VIOLATION |
| `GET /api/peek?code=a,b` | batch bodies, ≤50 codes (C8) | ~500/code | — |
| `GET /api/refs?token=t` | true reference totals, K=20 (C10) | ~800 tok | peek the `peek` fields |

### 2.1 `aoa grep` — find symbols (C8 define · C10 references · C15 candidates)

```
$ aoa grep observe
aOa: 20 hits | 7 files | 387 lines in ranges
  bs7ji  internal/domain/learner/observe.go:Learner.Observe(event ObserveEvent)[46-101]:46  @llm  #agents #lifecycle #scheduling
  5tb9w  internal/adapters/tailer/tailer_test.go:TestSignals_ReadEventToObserve(t *testing.T)[852-873]:852  @filesystem  #agents #api #attributes
  bs7l6  internal/domain/learner/observe.go:Learner.ObserveAndMaybeTune(event ObserveEvent)[106-113]:106  @functional  #agents #lifecycle #scheduling
  …
```

Line anatomy: `<peek-code>  <file>:<signature>[start-end]:<line>  @domain  #terms`.
A `--` instead of a peek code = no peekable body (non-code file, or method too large) — Read
the file at `[start-end]`. Regex works (`aoa grep 'Observe.*Event'`). Line-level hits inside
a method append the matched source line after `:<line>`.

No-match shape (real):
```
$ aoa grep xqzvwq
aOa: 0 hits | 0 files
  File search: aoa locate xqzvwq
```

**Result limit:** grep returns 20 symbols by default — that is the GNU `-m` flag's default,
not a cap; raise it with `aoa grep <query> -m 100`. Note the units: grep counts SYMBOLS
(definitions with ranges); `/api/refs` counts textual reference LINES (live: `aoa grep learner`
→ 20 symbols; `/api/refs?token=learner` → 119 reference lines — different questions, C8 vs C10).
Use `-m` for more definitions; use `/api/refs` when the question is "where is this referenced".
**Tokenizer honesty:** queries are sub-tokenized — a nonsense query containing a real subtoken
(`qqqzzznotoken`) matches that subtoken (`token`). Prefer exact identifiers.

Cost: ~2.5KB ≈ 700 tokens flat, regardless of corpus. Escalation: `aoa peek <code>`.

### 2.2 `aoa egrep` — multi-term OR (C15 already-implemented · C16 house pattern · C17 reading set)

```
$ aoa egrep --scope internal/app 'onSessionEvent|searchObserver'
aOa: 20 hits | 2 files | 1909 lines in ranges
  7j43f  internal/app/app.go:App.searchObserver(query string, opts ports.SearchOptions, ...)[635-761]:635  @animation  #algorithms #analysis
  --    internal/app/app.go:App.onSessionEvent(ev ports.SessionEvent)[1244-1788]:1244  @analytics  #api #attributes #checkout
  7hp4k  internal/app/activity_test.go:TestActivitySourceCasing(t *testing.T)[116-134]:129: a.onSessionEvent(ev)  #api #attributes
```

Label discipline: these are **candidates with #term evidence** (C15) or a **precedent set**
(C16) — never "the answer", never proof of absence. "No strong match" is a valid result.

### 2.3 `aoa peek` — read bodies (C8 rung 5, the core transaction)

```
$ aoa peek bs7l6 bs7iq
── Learner.ObserveAndMaybeTune(event ObserveEvent) [106-113] internal/domain/learner/observe.go  @functional ──
func (l *Learner) ObserveAndMaybeTune(event ObserveEvent) *AutotuneResult {
	l.Observe(event)
	if l.state.PromptCount > 0 && l.state.PromptCount%AutotuneInterval == 0 {
		result := l.RunMathTune()
		return &result
	}
	return nil
}

── type ObserveData struct [18-24] internal/domain/learner/observe.go  @push_notifications ──
type ObserveData struct { … }
```

Multiple codes in one call. Failure shape (real): `── zzzzz: symbol not found ──` (exit 0) —
fall back to `Read` at the `[start-end]` lines from the grep output. The grep→peek pair is
the flagship loop: 2 calls, ~1,000 tokens, exact body with boundaries.

### 2.4 `aoa locate` / `aoa find` / `aoa tree` — file identity (L0/C6)

```
$ aoa locate observer                 $ aoa find '*_handler.go'
⚡ 2 files                             ⚡ 2 files
  internal/app/observer.go              internal/adapters/web/arch_handler.go
  internal/app/observer_test.go         internal/adapters/web/index_handler.go

$ aoa tree internal/adapters/web -d 1
/home/corey/aOa-go/internal/adapters/web
├── static/
├── arch_handler.go
├── index_handler.go
├── server.go
…
```

`aoa locate nosuchfile` → `⚡ 0 files` (exit 0) — an honest empty, not an error.

### 2.5 `aoa arch` — structure (C1 organization · C4/C11 dependencies · C13 reachability · C22 gate)

**Orient first, always** — `views` is ~200 tokens and its captions often ARE the answer:

```
$ aoa arch views
{"scope":"local","rev":"8e370207ccdf","views":[
 {"id":"code","caption":"24 symbols along critical path — entrypoint: aoa","prov":"mixed"},
 {"id":"component","caption":"29 groups · 611 members — heaviest: g_adapters → g_other ×571 · ⚠ 9 findings","prov":"derived"},
 {"id":"cycles","caption":"0 cycles · 611 modules · ⚠ 9 findings","prov":"derived"},
 {"id":"dsm","caption":"1768 dependencies · 29 modules · ⚠ 9 findings","prov":"derived"}]}
```

**Reachability (C13)** — `derive` returns the BFS unit path over import edges, or a
first-class negative:

```
$ aoa arch derive cmd/aoa internal/domain/learner
["u_cmd_aoa","u_cmd_aoa_cmd","u_internal_app","u_internal_domain_learner"]

$ aoa arch derive internal/domain/index internal/adapters/bbolt
arch derive: no path found within 10 hops          # exit 1 — a real answer: structurally
                                                   # unreachable via imports. Never "control flow".
```

Caveat: an unresolvable seed produces the same "no path found" message — sanity-check seeds
with `aoa locate <path>` before citing a negative.

**Large views — trim, never dump** (P4 envelope caps are not yet applied; raw sizes measured
live: `view component` 79KB, `findings` 45KB — Law 3 says an unbounded dump is a defect):

```
$ aoa arch view component | jq -r '.count, (.buckets[] | "\(.label) (\(.members|length))")'
29 groups · 611 members — heaviest: g_adapters → g_other ×571 · ⚠ 9 findings
adapters (9)
app (1)
domain (8)
…

$ aoa arch findings | jq -r '.[] | "\(.severity) \(.rule): \(.message)"'
warn god: god component: bbolt (in 5 · out 18)
warn god: god component: treesitter (in 4 · out 530)
warn budget: budget overflow: other has 518 members (limit 40)
…9 total
```

Evidence for ONE finding (never all): `aoa arch findings | jq -r '.[] |
select(.message|contains("bbolt")) | .sources[:3][] | "\(.file):\(.line)"'` →
`cmd/aoa/cmd/arch.go:25` etc. **CI gate (C22):** `aoa arch findings --new` — exit 1 = new
findings exist; consume the exit code, jq the messages.

**Import-edge asymmetry:** to read code inside a TARGET unit from a derive/dsm result:
`aoa locate <path>` → `aoa grep <symbol>` → `aoa peek <code>`.
Do NOT use `aoa arch facts` — it is broken (returns empty for live finding subjects).

**`aoa arch drift <target.aoa>` (GOV-1, board #40)** — the Angle of Attack: diffs a
declared `.aoa` target file (`estate NAME` / `view KIND` / `allow FROM -> TO` lines,
unit paths like `internal/app`) against the REAL import graph read from the fact
substrate:

```
$ aoa arch drift .aoa/arch/target.aoa
VECTOR real vs my-estate: 3 VIOLATION · 1 MISSING · 41 CONFORMANT
  VIOLATION  internal/domain/arch -> internal/app  (internal/domain/arch/foo.go:12)
  MISSING    internal/app -> internal/kglab
```

VIOLATION = a real import the target doesn't declare (actionable, carries file:line).
MISSING = a declared import not yet built (no file:line — it doesn't exist). `--json`
emits `{result, findings}`, converting VIOLATIONs into the same Finding shape
`arch findings` uses. `--new`/`--baseline` gate on them exactly like
`arch findings --new` (own baseline scope `drift:<estate-name>`, same
`.aoa/arch/findings-baseline.json` file).

**Direct-RO only, not daemon-first** (an intentional asymmetry from every other
`arch` verb): the drift read path is not one of the 6 wired `MethodArch*` socket
methods, so this verb always opens the DB read-only directly — it can fail with a
lock timeout while a daemon holds the DB open for writing.

### 2.6 HTTP — `/api/peek` and `/api/refs` (port in `{root}/.aoa/run/http.port`, localhost-only)

```
$ curl -s "http://localhost:$(cat .aoa/run/http.port)/api/peek?code=bs7ji,5tb9w"
{"results":[{"code":"bs7ji","file":"internal/domain/learner/observe.go","symbol":"Observe",
 "signature":"Observe(event ObserveEvent)","span":[46,101],"body":"func (l *Learner) Observe…"}]}
```

Batch ≤50 codes per request (`{"error":"too many codes: max 50 per request"}`).
Unknown code embeds `{"code":"zzzzz","error":"symbol not found"}` per item.

**`/api/refs` is C10 — the honest "references" verb.** Label its output **"name references
(textual)"** — NEVER "callers", "usages", or "call sites":

```
$ curl -s "http://localhost:$(cat .aoa/run/http.port)/api/refs?token=observe"
{"token":"observe","total":247,"truncated":true,"refs":[
  {"file":"internal/adapters/tailer/tailer_test.go","line":852,"symbol":"TestSignals_ReadEventToObserve","peek":"5tb9w"},
  {"file":"CHANGELOG.md","line":0}, …]}      # K=20 hard cap; total is the TRUE count
```

`line:0` = file-grain textual hit (no symbol anchor). Refs with a `peek` field are one
`/api/peek` from a body. Unknown token → `{"total":0,"refs":[],"truncated":false}` — honest empty.

Heat surface (C5): `GET /api/top-files` → `{"items":[{"name":"…/internal/app/app.go","count":7},…]}`.
Always attach the caveat: attention observed by this tool's sessions, not change history.

---

## §3 Budgets and rank order

- **Rank order (T6, binding):** heat where present → posting-list length → path as
  deterministic final tiebreak. (Fan-in joins the order after P2 reverse adjacency lands.)
  Alphabetical-first-N is banned as a primary order.
- **Hard caps:** grep/egrep 20 lines · `/api/refs` K=20 with true `total` + `truncated` ·
  `/api/peek` batch 50 · derive BFS 10 hops.
- **Answer shape (Law 2):** one sentence (scope + verdict + honest total) + top-K + one
  evidence handle per item (`file:line` or peek code) + a real "+N more". If you truncate,
  say how many you cut.
- **Cost ladder (cheapest first):** locate/find/derive/views (~O(100) tok) → tree/cycles →
  grep/egrep/refs (~O(700)) → peek (~O(100–500)/code) → dsm/code views (~O(1.5k)) →
  jq-trimmed component/findings → raw component (20k) / findings (12k) — raw dumps of the
  last two are a defect, not a fallback.
- **Escalate once per answer.** grep → peek is one escalation. Cap follow-up queries at ~3
  per question; if the third doesn't land, tell the user what you found and what's missing.

---

## §4 The refusal vocabulary — what the graph cannot answer

A named refusal in one line is an answer; a faked answer is a defect. When a question maps
to this table, refuse by name and offer the nearest honest redirect.

| Question class | Refusal (verbatim) | Nearest honest redirect |
|---|---|---|
| Who calls X / call graph / control flow | "No call graph — this substrate has import edges and name references only." | `/api/refs?token=x` labeled **"name references (textual)"**; `aoa arch derive A B` for **import** reachability |
| Data flow / what reads-writes this value | "Reads and writes are indistinguishable here — textual co-occurrence only." | `aoa grep <name>` labeled "textual co-occurrence" |
| Why was this written / VCS history / who changed it | "No VCS history in the graph." | `git log`/`git blame` outside the graph; heat shows tool-observed attention only, never authorship or recency |
| What were we recently working on | "No timestamps — 'recently' cannot be computed from a decayed counter." | `/api/top-files` with the tool-observed caveat |
| Runtime behavior / performance / does it work | "No runtime, no traces, no execution." | run the code/tests; graph gives static import reachability at best (C13) |
| Type hierarchy / interface implementors / subclasses | "Inheritance edges are not in the substrate." | `aoa grep <TypeName>` name references; read the definitions via peek |
| Which domain owns this file (`@domain` as filter) | "`@domain` is display metadata — `aoa grep @streaming` is a literal token match, not a domain query." (verified live) | egrep concept terms as **candidates**, no ownership claim |
| Total impact of a change | "Import-grain blast floor only — not runtime impact." | `arch view dsm` + `derive`, labeled "imports only, lower bound" |
| Does the code match the intended architecture (calls, ownership, layering rules) | "Drift is import-only — only the `import` concept is wired as a graph edge (1/16 ontology concepts); no call/data/ownership drift." | `aoa arch drift <target.aoa>` labeled "import-edge drift, lower bound" |
| Peek an `ext:` target or a `--` symbol | "Package grain / too large — no peek body available." | `ext:` → say so and stop; `--` → Read the file at `[start-end]` |
| Is X absent from the codebase | "Absence is unprovable — the index shows no strong match, not nonexistence." | report "0 hits" with the exact query used |

---

## §5 Contract-to-verb map (doctrine C-IDs, v1 set)

C1 organization → `arch views` / trimmed `view component` · C4 external deps → `arch view dsm`
(`ext:` rows/cols) · C5 what's hot → `/api/top-files` + caveat · C6 package contents →
`tree` + scoped grep · C8 define (the SLA) → `grep` → `peek` · C9 file parts → scoped grep ·
C10 name references → `/api/refs` · C11 unit deps → `arch view dsm` · C13 reachability →
`arch derive` (negative is first-class) · C14 what moves → `derive` + dsm, labeled floor ·
C15/C16 candidates/precedent → `egrep` · C17 reading set → `egrep` top files, cap 15 ·
C22 arch gate → `arch findings --new` exit code · C23 new ext deps → dsm `ext:` diff ·
CR refusals → §4. Deferred (do not attempt): C2 domain ownership (P3), C12 reverse
dependents at scale (P2), C3 literal-string content search as a contract (dictionary amendment).
