# 17 — aOa goal-anchored discipline: litmus-first, agents on escalation (design sketch)

**Status:** design thinking — high-level. No skills/agents built; `GOALS.md` not yet
edited. This **revises the earlier sketch** (a 6-skill suite + 3 always-on reviewer
agents) down to a leaner, tiered model after the right challenge: *most of that machinery
is just checking work against the goals — so put the check in the goals as a litmus test,
and reserve parallel agents for the few moments an independent adversary earns its cost.*

**The why (unchanged):** as we add enhancements, the founding goals (`G0`–`G6`), the scope
law, and the architecture rules must keep steering every new piece so we don't drift from
why aOa exists. Today that discipline is *prose I have to remember to apply*. We make it
**inline, checkable, and hard to skip** — without standing machinery.

---

## 1. The revision, in one line

**Discipline belongs in the goals as a litmus test (default, free, always-on). Parallel
agents are an escalation, not a routine.** That replaces the gate skill *and* the
always-on reviewer agents with: a litmus section in `GOALS.md` + two or three agents parked
for high-stakes work.

| | Earlier sketch (over-built) | This design (tiered) |
|---|---|---|
| Routine enhancement | gate skill + 3 agents fire | **read the goal litmus, self-check inline** — zero machinery |
| High-stakes change | (same) | **escalate** to goalkeeper / architecture-cop / red-team agents |
| Net-new components | 6 skills + 3 agents | a litmus in `GOALS.md` + 2–3 parked agents |

---

## 2. Tier 1 — the litmus, in the goals (the default)

`GOALS.md` already says "every change must be validated against each goal independently" —
it just states each goal as a *description*, not a *test*. Make each answerable yes/no and
the check becomes an inline self-check any human or subagent applies, with no orchestration.

**Drafted litmus (for review before it goes into `GOALS.md`):**

| Goal | Litmus — answer before proceeding | Trip wire |
|---|---|---|
| **G0 Speed** | Does it add work to a hot path (O(1) search, keystroke reindex)? Is the build delta *measured* ≤+3% (not asserted)? Startup <200ms / <50MB intact? If not sub-ms, is it explicitly opt-in? | any "no/unknown" → restructure |
| **G1 Correctness** *(refine — see §4)* | Is there a Python oracle? If yes, parity fixture passes. If net-new (no oracle), is there a byte-stable golden fixture + an invariant test? | no test contract → stop |
| **G2 Two-Binary** | Does base `aoa` still work with zero deps? Does this make the base depend on recon / `//go:build core`? | base-depends-on-recon → stop |
| **G3 Agent-First** | Does the surface work in all three modes (direct/pipe/daemon) with grep-identical output + exit codes? Does grep+peek stay primary? | no → stop |
| **G4 Clean Architecture** | Domain import-free of adapters? Port defined *first*? No product entanglement? No write holding `App.mu` across a `db.Update`? | entangled / no port → stop |
| **G5 Self-Learning** | Does recon stay its own product (signal/usage), additive-only? Does this *require* recon to function vs *enhance*? | required-dependency / scope-creep → stop |
| **G6 Value-Proof** | Does it surface measurable value (tokens saved, runway, evidence)? Can the value be *shown*, not asserted? | unmeasurable → reconsider |

This is the whole default gate: read it, answer it, proceed or restructure. No tokens, no
agents, no waiting.

---

## 3. Tier 2 — escalation agents (on demand, not by default)

Reserve independent adversaries for the few moments they earn their cost — a new subsystem,
a concurrency-touching change, a release, a strategy call. (Exactly how we used the swarms
this session: deliberately, for big things — not on every edit.)

| Agent | Fires when (you invoke it) | What it adds over the litmus |
|---|---|---|
| **`aoa-architecture-cop`** | New subsystem / concurrency / locking change | Reads real code; catches the `App.mu`/`db.Update`/hexagonal class the litmus can only prompt for |
| **`aoa-goalkeeper`** | A release or a big feature | Independent per-goal (G0–G8) scorecard — a second set of eyes, not self-grading |
| **`aoa-redteam`** | Strategy/research/design with stakes | The "are we chasing shiny objects / is this anchored" lens — the pattern from our swarms |

These are *parked* (`.claude/agents/`), invoked by name when warranted. Generic execution
(brainstorming, TDD, debugging, writing-plans, verification) keeps riding **Superpowers** —
we don't re-implement it.

---

## 4. Proposed goal refinements & additions (your call — value-gated)

Adding goals has a cost (more to check), so only where the value is clear. A refinement
plus two additions earn it:

- **Refine G1 (Parity → "Correctness Contract").** Today G1 = "zero divergence from
  Python; fixtures are truth." But the arch layer is **net-new — there is no Python to
  parity against** (the audit flagged this). Broaden G1 to: *parity where a Python oracle
  exists; determinism + golden fixtures + invariants where it's net-new.* Keeps the
  original teeth, closes the gap the whole enhancement pool opened.

- **Add G7 — Provenance & Honesty.** *Truth is stamped.* Every derived answer carries
  `file:line:commit`; inference is labeled and leashed (the agent never adds a node/edge;
  inference never ships as fact). **Why:** this is aOa's actual moat and the scope-law in
  goal form — currently it lives only in an ADR, not as a first-class, checkable goal. It's
  the single most differentiating discipline; it deserves goal status.

- **Add G8 — Verifiable Quality.** *Gated, not asserted.* A quality claim must be backed by
  a gate — blind-judge for views, golden-fixture determinism for derivations, a benchmark
  for any performance claim. Evidence before assertion. **Why:** it's the discipline the
  entire pool (and Superpowers' `verification-before-completion`) already leans on; making
  it a goal is what stops "≤+3%, trust me" and "this looks right" from shipping.

*(Candidate considered and left out for now: a standalone "maintenance-free" goal — it's
real but already covered by G0 freshness + G6 value + G8 evidence; adding it would be
bloat. YAGNI.)*

If accepted, G7/G8 get litmus rows too (provenance: "every answer stamped? inference
leashed?"; quality: "claim backed by a gate, or asserted?").

---

## 5. Directory structure (now small)

```
aOa-go/
  .context/GOALS.md        # EDIT: add the per-goal litmus (+ G1 refine, + G7/G8 if accepted)
  CLAUDE.md                # already mandates "validate against goals" — now it's testable
  .claude/agents/
    beacon.md              # exists — continuity
    gh.md                  # exists — decomposition
    aoa-architecture-cop.md  # NEW (parked) — escalation reviewer
    aoa-goalkeeper.md        # NEW (parked) — escalation reviewer
    aoa-redteam.md           # NEW (parked) — escalation adversary
```

No skills suite. The only always-on change is a doc edit (the litmus). The agents are
optional and dormant until invoked.

---

## 6. Why this shape is right

- **Almost no machinery** — the default discipline is a doc you read, not a swarm you run.
- **Always on, free** — a litmus costs zero tokens and applies to *every* change, where an
  agent fleet only fires when triggered (and gets skipped under time pressure — exactly
  when discipline matters most).
- **Right-sized escalation** — heavy adversarial review is reserved for where it pays, so
  it stays credible instead of becoming ceremony.
- **Self-correcting by design** — this revision *is* the litmus working: "does this add
  machinery a simpler check would cover?" (a G0/YAGNI question) caught the over-build.

---

## 7. First slice (if/when we proceed)

1. Land the **litmus in `GOALS.md`** (G0–G6, + the G1 refine and G7/G8 if you approve them).
2. Park **`aoa-architecture-cop`** — the one escalation agent that catches the
   concurrency/locking/hexagonal class the board's red-team already found.
3. Use both on the real L19 keystone work; add `goalkeeper` / `redteam` only if a gap shows.

> Per the brainstorming gate: this is the design. `GOALS.md` stays untouched until you
> approve the litmus and the goal changes; then a writing-plans pass turns it into the edit.
