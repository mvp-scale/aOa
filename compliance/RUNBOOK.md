# Compliance Realignment Runbook (L20.2)

When Claude Code ships a new version, re-aligning aOa's session contract should
take **minutes, not a research session**. This is the exact procedure followed
for the v2.1.126 → v2.1.172 re-observation (2026-06-11). Run it whenever the
compliance suite reports `DRIFT` (or proactively after any Claude Code upgrade).

The session directory under test is this repo's own:
`~/.claude/projects/-home-corey-aOa-go/`. The latest `.jsonl` there is the live
sample — **the session you are working in right now is itself a capture source.**

---

## Fast path: generative harness (recommended for a new version)

`conformance/run.sh` drives `claude -p` to **generate** controlled sessions for
the installed version and validates them against the contract — every surface we
can script a trigger for, plus a closed-loop token check, from a fresh session.
This is the preferred new-version onboarding step; the manual steps below are the
fallback for surfaces the harness can't drive (away_summary, live mode toggles)
and for authoring the snapshot once drift is confirmed.

```bash
cd compliance/conformance && ./run.sh --all
```

It reports per-surface PASS/WARN/FAIL, a coverage matrix, and flags
installed-version ≠ contract-baseline. If it reports `SCHEMA-DRIFT`, continue
with steps 1–6 below to author the new snapshot. See `conformance/README.md`.

---

## 0. Detect drift (10 seconds)

```bash
go test -tags compliance ./compliance/claude-session/...
```

- `ok` → aligned, stop here.
- `FAIL` on **Pass0** → version moved; `DRIFT: live vX is newer than baseline vY`.
- `FAIL` on **Pass3** → new event type / envelope field / content block.

Note the live version `vX` from the Pass0 log — that's your target snapshot.

## 1. Enumerate the live schema (2 minutes)

Point `jq` at the newest session file. Helper to find it:

```bash
cd ~/.claude/projects/-home-corey-aOa-go
F=$(ls -t *.jsonl | head -1); echo "$F"; grep -o '"version":"[^"]*"' "$F" | sort -u
```

Run these enumerations (each answers one contract section):

```bash
# top-level event types (vs §3.2)
jq -r '.type' "$F" | sort | uniq -c | sort -rn

# envelope field union per recognized type (vs §3.1)
for t in user assistant system attachment ai-title last-prompt permission-mode; do
  echo "--- $t ---"; jq -c "select(.type==\"$t\") | keys" "$F" | jq -s 'add | unique'
done

# any NEW top-level types: inspect their shape
jq -c 'select(.type=="<NEWTYPE>")' "$F" | head -2

# usage fields + whether iterations is populated (vs §3.4, L18)
jq -c 'select(.type=="assistant") | .message.usage | select(.!=null)' "$F" | head -1

# message-level fields (vs §3.6)
jq -c 'select(.type=="assistant") | .message | keys' "$F" | jq -s 'add | unique'

# content block types (vs §3.3)
jq -r 'select(.message.content|type=="array") | .message.content[].type' "$F" | sort | uniq -c

# system subtypes (vs §3.5)
jq -r 'select(.type=="system") | "\(.subtype) level=\(.level)"' "$F" | sort | uniq -c

# toolUseResult shapes (vs §3.7) — by key signature
jq -r 'select(.toolUseResult!=null) | .toolUseResult | keys | join(",")' "$F" | sort | uniq -c
```

**Tip:** the live session may grow new fields *mid-capture* (v2.1.172's `slug`
appeared partway through). Re-run Pass3 after the session advances if you suspect
late-appearing fields.

## 2. Classify each drift item (the judgment step)

For every new type/field/shape, decide one of:

| Disposition | Where it goes in `compliance_test.go` |
|-------------|----------------------------------------|
| New event type, drop | add to `knownUnhandledTypes` |
| New event type, consume | add to `recognizedTypes` **and** wire a `translateX` in `reader.go` first |
| New envelope field, drop | add to `knownUnconsumedEnvelopeFields` |
| New envelope field, consume | add to `consumedEnvelopeFields` **and** read it in `parser.go` first |
| New system subtype | add to `knownUnhandledSystemSubtypes` |
| New toolUseResult shape | add to `consumedToolUseResultShapes` (collapses to `other`) or add a typed branch |
| New content block | add to `recognizedContentBlockTypes` / `knownUnhandledContentBlockTypes` |

Rule (matches the contract philosophy): **only mark `[CONSUMED]` after the parser
actually reads it.** Default new signals to the drop/known maps and file a sub-ID;
don't silently expand consumption to make a test pass.

## 3. Write the observed snapshot

```
compliance/claude-session/versions/v<X>-observed/
  manifest.json     # copy the prior version's, update the enumerated sets + a
                    # `new_since_v<prev>` block describing each drift item
  observations.md   # narrative: what's new, why, disposition table
  README.md         # header (version, date, sample size, headline)
```

`manifest.json` is what Pass0/Pass5 read as the baseline — the suite auto-picks
the **highest** `v*-observed/` dir, so creating it re-points the baseline.

## 4. Update the contract + history

- `CONTRACT.md` — bump header (`Current observed version`, `Last validated`);
  add rows for each new field/type/subtype/shape with `[CONSUMED|DROPPED|IGNORED]`.
- `versions/HISTORY.md` — append a `## v<prev>-observed → v<X>-observed` section,
  1–2 lines per change (what + likely why), terse.

## 5. Verify green + regenerate the report

```bash
go test -tags compliance ./compliance/claude-session/...     # expect: ok, Pass0 MATCH
go test -tags compliance ./compliance/claude-statusline/...  # second surface
```

Pass5 rewrites `compliance/REPORT.md` automatically (do not hand-edit it).
Confirm the report shows the new version and the honest gap list.

## 6. Route the findings

- Token/usage semantics changes → `.context/details/` numeric report, feed L18.
- Build the gap table (`signal × capture × entity × verdict`) → `.context/details/`.
- File sub-IDs for any "consume later" decisions on the board.

---

## One-shot drift check (paste-ready)

```bash
go test -tags compliance ./compliance/... 2>&1 | grep -E 'DRIFT|FAIL|MATCH|Status:'
```

## Known v2.1.172 baseline (current)

- Event types recognized: user, assistant, system, permission-mode, attachment,
  last-prompt, ai-title. Dropped: mode, queue-operation, progress,
  file-history-snapshot.
- Dropped envelope fields: promptSource, level, slug (+ the v2.1.126 set).
- Dropped subtype content: local_command. Populated-but-unused: usage.iterations.
- toolUseResult `other`-absorbed: ToolSearch, TaskCreate, TaskUpdate.

## Automation status

- **Generative validation — BUILT** (`conformance/run.sh`). Generates sessions
  on demand, validates schema + token economics against the contract. This is
  the repeatable new-version onboarding test.
- **Candidate-manifest auto-emit — backlog** (**L20.2-auto**). Steps 1–3 could
  emit a candidate `manifest.json` + a baseline diff, leaving only the
  classification judgment (step 2) and narrative (step 4) to a human.
