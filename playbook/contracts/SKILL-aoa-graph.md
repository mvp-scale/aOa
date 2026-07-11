---
name: aoa-graph
description: "Use for ANY question about a codebase indexed by aOa (a .aoa/ directory exists at the repo root): where code is defined, what a package contains, who imports what, whether A reaches B, how the repo is organized, what's flagged. Routes the question to aOa's graph verbs (grep/peek/locate/tree/arch) BEFORE any Grep/Glob/Read exploration — one graph call replaces 5-10 raw searches and returns file:line evidence."
---

# aOa graph — query the codebase, don't crawl it

## Fast path (binding)

If `.aoa/` exists at the repo root, every codebase question goes to an aOa verb FIRST.
Do not open files speculatively, do not run raw grep/rg sweeps, do not use built-in
Grep/Glob for code questions. Escalate to Read only through the verbs' own handles
(`[start-end]` boundaries, `file:line` stamps). Full contract: `playbook/contracts/agent-query-contract.md`.

All verbs are Bash: `aoa grep`, `aoa egrep`, `aoa peek`, `aoa locate`, `aoa find`,
`aoa tree`, `aoa arch views|view|derive|findings`. Under Claude Code the agent output
grammar (peek codes, `[start-end]`, @domains) is automatic.

## Verb selection by question shape

| The question sounds like | Run | Then |
|---|---|---|
| "Where is X defined / show me X" | `aoa grep X` | `aoa peek <code>` |
| "How does X work" | `aoa grep X` | peek 2-3 top codes |
| "Where is X referenced" | `curl -s localhost:$(cat .aoa/run/http.port)/api/refs?token=x` | peek the `peek` fields; say "name references (textual)", never "callers" |
| "Is there precedent for / do we already have X" | `aoa egrep 'x\|synonym\|synonym'` | peek best candidate; label "candidates" |
| "Which file is / find files named" | `aoa locate name` / `aoa find '*.glob'` | — |
| "What's inside package P" | `aoa tree P -d 2` + `aoa grep --scope P <term>` | peek |
| "How is this repo organized" | `aoa arch views` | jq-trim `arch view component` |
| "Does A depend on / reach B" | `aoa arch derive A B` | locate → grep → peek the last hop |
| "What breaks if I touch X" | `aoa arch view dsm` + `derive` | label "imports only, lower bound" |
| "Anything flagged / cycles?" | `aoa arch findings \| jq -r '.[].message'` | jq ONE finding's sources |
| "Did my change add drift" (CI) | `aoa arch findings --new` | exit 1 = new findings |

## The core loop: grep → peek (2 calls, ~1k tokens)

```
$ aoa grep observe
aOa: 20 hits | 7 files | 387 lines in ranges
  bs7ji  internal/domain/learner/observe.go:Learner.Observe(event ObserveEvent)[46-101]:46  @llm  #agents #lifecycle
  bs7l6  internal/domain/learner/observe.go:Learner.ObserveAndMaybeTune(event ObserveEvent)[106-113]:106  @functional
  …
$ aoa peek bs7l6
── Learner.ObserveAndMaybeTune(event ObserveEvent) [106-113] internal/domain/learner/observe.go ──
func (l *Learner) ObserveAndMaybeTune(event ObserveEvent) *AutotuneResult {
	l.Observe(event)
	…
```

Fallbacks — both are normal, not errors:
- peek says `── zzzzz: symbol not found ──` → **Read the file at the `[start-end]` lines** from the grep hit.
- grep shows `--` instead of a peek code (too large, or non-code file) → Read at `[start-end]`.
- `aoa grep` returns 0 hits → it suggests `aoa locate <term>`; take the suggestion.

## Worked examples (real output, this repo)

**1. Orientation — "how is this codebase organized?"** (~350 tokens total)
```
$ aoa arch views
{"views":[{"id":"component","caption":"29 groups · 611 members — heaviest: g_adapters → g_other ×571 · ⚠ 9 findings","prov":"derived"},
          {"id":"cycles","caption":"0 cycles · 611 modules","prov":"derived"}, …]}
$ aoa arch view component | jq -r '.count, (.buckets[] | "\(.label) (\(.members|length))")'
adapters (9)
domain (8)
cmd (3)
…
```
NEVER cat `arch view component` or `arch findings` raw — 79KB/45KB live. Always jq-trim.

**2. Reachability — "does cmd/aoa reach the learner?"** (~30 tokens)
```
$ aoa arch derive cmd/aoa internal/domain/learner
["u_cmd_aoa","u_cmd_aoa_cmd","u_internal_app","u_internal_domain_learner"]
```
Negative case is a real answer, quote it as one:
```
$ aoa arch derive internal/domain/index internal/adapters/bbolt
arch derive: no path found within 10 hops        # exit 1: unreachable via imports
```
(An unresolvable seed gives the same message — `aoa locate` the seed before citing a negative.)

**3. References with a true total — "where is learner used?"**
```
$ curl -s "http://localhost:$(cat .aoa/run/http.port)/api/refs?token=learner"
{"token":"learner","total":119,"truncated":true,"refs":[
  {"file":"internal/app/lockorder_test.go","line":598,"symbol":"TestT17_LearnerObserveEvent","peek":"86zpy"}, …]}
```
Report: "119 name references (textual) — top 20 shown, +99 more." `line:0` entries are
file-grain textual hits. CLI grep caps at 20 with no true total — use /api/refs when the count matters.

**4. Scoped hunt — "how do session events get handled in app?"**
```
$ aoa egrep --scope internal/app 'onSessionEvent|searchObserver'
aOa: 20 hits | 2 files | 1909 lines in ranges
  7j43f  internal/app/app.go:App.searchObserver(query string, …)[635-761]:635
  --    internal/app/app.go:App.onSessionEvent(ev ports.SessionEvent)[1244-1788]:1244
```
`7j43f` → peek it. `--` → Read `internal/app/app.go` offset 1244, limit 545.

**5. Findings — "anything wrong with the architecture?"** (~120 tokens vs 45KB raw)
```
$ aoa arch findings | jq -r '.[] | "\(.severity) \(.rule): \(.message)"'
warn god: god component: bbolt (in 5 · out 18)
warn budget: budget overflow: other has 518 members (limit 40)
…9 total
```
Evidence for ONE finding only: `… | jq -r '.[] | select(.message|contains("bbolt")) | .sources[:3][] | "\(.file):\(.line)"'`.
Do NOT use `aoa arch facts` — broken.

**6. Batch bodies over HTTP** (when you have 3+ peek codes)
```
$ curl -s "http://localhost:$(cat .aoa/run/http.port)/api/peek?code=bs7ji,5tb9w"
{"results":[{"code":"bs7ji","file":"internal/domain/learner/observe.go","span":[46,101],"body":"func (l *Learner) Observe…"}]}
```
≤50 codes per request; unknown codes embed `{"error":"symbol not found"}` per item.

## Budget discipline

- Answer shape: one sentence (scope + verdict + honest total) → top-K with `file:line` →
  a REAL "+N more". If aOa truncated (grep's 20-hit cap, refs' K=20), say so with the number.
- One escalation per answer (grep→peek counts as one). Cap follow-up queries at ~3 per
  question; then report what you have and what's missing.
- Cheapest first: locate/derive/views (~100 tok) → grep/refs (~700) → peek → jq-trimmed
  arch views. Raw `view component`/`findings` dumps are a defect.
- Re-grep is permitted when a graph answer looks off — but log it: say "graph said X,
  verifying with raw grep" rather than silently switching.

## Refusal handling (binding)

aOa refuses what its substrate can't support. When it refuses — or the question matches the
contract's refusal table — DO NOT silently substitute raw exploration. Tell the user what the
graph CAN answer, then (only if they want more) go outside it, labeled:

- "Who calls X?" → no call graph. Offer: name references (/api/refs) + import reachability (derive).
- "Why was this changed / who wrote it?" → no VCS history. Offer: `git log`/`git blame`, outside the graph.
- "What did we work on recently?" → no timestamps. Offer: `/api/top-files` heat, caveat
  "tool-observed sessions, not change history" — never say "recently".
- "Does it work / how fast is it?" → no runtime. Offer: run the tests.
- "What implements this interface?" → no type hierarchy. Offer: name references + peek definitions.
- `@domain` in grep is a literal token match, not a domain filter — don't query with it.
- Absence claims: "0 hits" means no strong match in the index, never "it does not exist".

Import edges are the ONLY edge kind. Never present them as calls, data flow, or type
relationships. Cite provenance: `derived` = REAL, cite as fact; `mixed` = label the heuristic parts.

## Subagent propagation

This skill binds every subagent you spawn. When dispatching agents for codebase work, use
`general-purpose` (not Explore — it has its own toolset and won't inherit this guidance) and
paste into the prompt: the fast-path rule, the grep→peek loop with one real output example,
the `[start-end]` Read fallback, and the jq-trim rule for `arch view component`/`findings`.
A subagent that answers with raw grep sweeps over an indexed repo is misconfigured.
