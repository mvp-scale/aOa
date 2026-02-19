# Board Guide

Instructions for creating and maintaining a structured work board. This is a styling template — any agent can use it to build or transform a board for any project.

---

## Purpose

The board is a single-file project tracker that answers three questions at a glance:

1. **What are we doing?** (tasks)
2. **Why?** (goal alignment)
3. **Is it done?** (triple-green validation)

Everything fits in one markdown file. One table. No external tools.

---

## File Structure

A project uses three context files with a shared navigation bar:

```
.context/BOARD.md      — Active work (the board)
.context/COMPLETED.md  — Archive of finished work
.context/BACKLOG.md    — Future ideas, deferred items
```

All three files share the same nav bar at the top so you can jump between them:

```markdown
[Board](#board) | [Supporting Detail](#supporting-detail) | [Completed](COMPLETED.md) | [Backlog](BACKLOG.md)
```

---

## Board Sections (In Order)

Every board has these sections in this order:

### 1. Header

```markdown
# Work Board

[Board](#board) | [Supporting Detail](#supporting-detail) | [Completed](COMPLETED.md) | [Backlog](BACKLOG.md)

> **Updated**: [Date] (Session [N]) | **Phase**: [Current phase description]
> **Completed work**: See [COMPLETED.md](COMPLETED.md)
```

Keep the header minimal. Date, session number, current phase. Link to completed work so it's not cluttering the active board.

---

### 2. Goals

Goals are atomic architectural principles. They're abstract, directional, and rarely change. Every task on the board maps to one or more goals.

```markdown
## Goals

> Atomic architectural principles. Every task is evaluated against each goal independently.

| Goal | Statement |
|------|-----------|
| **G0** | **[Name]** — [One-sentence principle] |
| **G1** | **[Name]** — [One-sentence principle] |
| ...  | ... |
```

**Rules:**
- Use G0 through G9 (max 10 goals). Most projects need 4-7.
- Each goal is a principle, not a task. "Cost Guard" not "Reduce costs."
- Goals must be independently evaluable — a task can serve G2 without serving G0.
- **A task with no goal alignment should trigger a conversation: why are we doing this?**

**IMPORTANT: Goals require conversation.** An agent cannot invent goals. They emerge from discussion with the project owner about what matters. When building a board for a new project, the agent should:
1. Ask the user what their core architectural principles are
2. Propose goal candidates based on the discussion
3. Refine until the user confirms

Not all goals are transferable between projects. Cost may not matter for an internal tool. Security may not matter for a prototype. The goals reflect THIS project's values.

---

### 3. Board Structure

This section teaches anyone reading the board how to use it. It defines the layers, columns, and indicators.

#### Layers (Levels)

Layers define the build order. Each layer assumes the one below is solid. They're numbered L0 upward.

```markdown
## Board Structure

> Layered architecture. Each layer builds on the one below.

### Layers

| Layer | Name | Purpose | Gate Method |
|-------|------|---------|-------------|
| **L0** | [Name] | [What this layer establishes] | [How you prove it's done] |
| **L1** | [Name] | [What this layer establishes] | [How you prove it's done] |
| **L2** | [Name] | [What this layer establishes] | [How you prove it's done] |
| ...   | ...    | ...     | ...         |
```

**Rules:**
- L0 is always the foundation — the thing everything else depends on.
- Each layer has a gate method: how do you know it's done?
- Tasks within adjacent layers CAN overlap where dependencies allow. The Dep column tracks specific blockers. Layer gates are the aggregate checkpoints.
- Typically 3-6 layers. More than 6 suggests the scope is too large.

**IMPORTANT: Layers require conversation.** Like goals, layers emerge from understanding the project's architecture and build order. The agent should:
1. Understand what the foundational work is (L0)
2. Ask what depends on what
3. Propose a layer structure
4. Refine with the user

A web app might use: L0 Schema, L1 API, L2 UI, L3 Testing, L4 Deploy.
A data pipeline might use: L0 Schema, L1 Queries, L2 Flow Control, L3 Validation, L4 Scale.
A CLI tool might use: L0 Core Logic, L1 Commands, L2 Config, L3 Packaging.

There is no universal layer structure. It depends on the project.

#### Columns

```markdown
### Columns

| Column | Purpose |
|--------|---------|
| **Layer** | Layer grouping. Links to layer detail below. |
| **ID** | Task identifier (layer.step). Links to task reference below. |
| **G0-G9** | Goal alignment. `x` = serves this goal. Blank = not relevant. |
| **Dep** | ID of blocking task, or `-` |
| **Cf** | Confidence — see indicator reference below |
| **St** | Status — see indicator reference below |
| **Va** | Validation state — see indicator reference below |
| **Docs** | Links to supporting detail or external documents |
| **Task** | What we're doing |
| **Value** | Why we're doing this, what we expect to gain |
| **Va Detail** | How we prove it — specific test or assertion |
```

The column order is deliberate:
- **Left side** (Layer through Va): compact, scannable indicators
- **Right side** (Docs through Va Detail): narrative that needs room

#### Indicator Reference

```markdown
### Indicator Reference

| Indicator | Cf (Confidence) | St (Status) | Va (Validation) |
|:---------:|:----------------|:------------|:----------------|
| ⚪ | — | Not started | Not yet validated |
| 🔵 | — | In progress | — |
| 🟢 | Confident | Complete | Validated |
| 🟡 | Uncertain | Pending | Needs test strategy |
| 🔴 | Lost/Blocked | Blocked | Failed |

> 🟢🟢🟢 = done. Task moves to COMPLETED.md.
```

The three indicators track independent dimensions:
- **Cf (Confidence)**: Do we know HOW to do this? 🟢 = yes. 🟡 = mostly. 🔴 = need research.
- **St (Status)**: Is it DONE? ⚪ = not started. 🔵 = in progress. 🟢 = complete.
- **Va (Validation)**: Is it PROVEN? ⚪ = not tested. 🟢 = validated. 🔴 = failed.

A task can be St=🟢 (complete) but Va=🔴 (failed validation). That means the work was done but it doesn't pass the test — rework needed.

#### Layer Quality Gates (Optional)

If the project uses TDD or has specific validation criteria per layer, define them here:

```markdown
### Layer Quality Gates

**L0 Gate — [Name]**:
- [Specific assertion 1]
- [Specific assertion 2]

**L1 Gate — [Name]**:
- [Specific assertion 1]
- [Specific assertion 2]
```

Gates are optional but valuable. They make "done" unambiguous.

#### Validation Policy (Optional)

If different change types have different validation requirements:

```markdown
### Validation Policy

| Change Type | Before Deploy | After Deploy | Rollback |
|-------------|---------------|--------------|----------|
| [Type 1] | [Pre-check] | [Post-check] | [How to undo] |
| [Type 2] | [Pre-check] | [Post-check] | [How to undo] |
```

---

### 4. Mission

Short north-star statement. Where we are, where we're going.

```markdown
## Mission

**North Star**: [One sentence — the end goal]

**Current**: [Where we are right now]

**Approach**: [How we're getting there — brief]

**First Milestones**: [If applicable — what's the first tangible output?]
```

Keep this to 4-6 lines. It's context, not a strategy doc.

If there are open questions that need discussion before certain layers can proceed, list them:

```markdown
**Needs Discussion** (before [Layer N] implementation):
- [Open question 1]
- [Open question 2]
```

---

### 5. Board (The Table)

**ONE table. All tasks. All layers.** This is the heart of the board.

```markdown
## Board

| Layer | ID | G0 | G1 | G2 | G3 | G4 | G5 | G6 | G7 | G8 | G9 | Dep | Cf | St | Va | Docs | Task | Value | Va Detail |
|:------|:---|:--:|:--:|:--:|:--:|:--:|:--:|:--:|:--:|:--:|:--:|:----|:--:|:--:|:--:|:-----|:-----|:------|:----------|
| [L0](#layer-0) | [L0.1](#l01) | x |  |  | x |  |  |  |  |  |  | - | 🟢 | ⚪ | ⚪ | [detail](#l01) | Short task description | Why this matters | How we prove it |
```

**Rules:**
- ONE table — never split into per-layer tables
- Goal columns: `x` or blank, nothing else
- Dep column: task ID that blocks this, or `-` for none
- Indicator columns: use only the 5 defined indicators (⚪ 🔵 🟢 🟡 🔴)
- Layer column: links to `[LN](#layer-n)` in Supporting Detail
- ID column: links to `[LN.M](#lnm)` in Supporting Detail
- Task/Value/Va Detail: enough detail to understand what, why, and proof WITHOUT clicking through

**The triple-green rule**: When Cf=🟢, St=🟢, Va=🟢 — the task is done. Move it to COMPLETED.md.

**Adjust goal columns to fit.** If the project has 5 goals, use G0-G4 and drop G5-G9. Don't include empty goal columns — they waste horizontal space.

**Table size target**: Under ~40 rows. If it grows beyond that, triple-green tasks should move to COMPLETED.md more aggressively.

---

### 6. Supporting Detail

All phase context, task specifics, code references, and implementation notes go BELOW the board table. The board links to them via the Layer and ID columns.

```markdown
## Supporting Detail

### Layer 0

**Layer 0: [Name] ([Purpose])**

> Layer-level context, dependencies, and quality gate summary.
> **Quality Gate**: [What must pass]

#### L0.1

**[Task title]**

Detailed description. File references. Code snippets. Implementation notes.

**Files**: `path/to/file.ts`, `path/to/other.ts`

#### L0.2

**[Task title]**

...
```

**Rules:**
- Layer sections use `### Layer N` headings
- Task sections use `#### LN.M` headings
- Layer sections include: description, quality gate reference
- Task sections include: detailed description, file references, specific implementation guidance
- This is where the detail lives — the board table stays clean

---

### 7. Closing Sections

These sections go at the bottom. All are recommended:

#### What Works (Preserve)

```markdown
### What Works (Preserve)

| Component | Notes |
|-----------|-------|
| [Component name] | [Why to preserve it] |
```

Explicitly list things that should NOT be touched. Prevents well-meaning refactors from breaking working systems.

#### What We're NOT Doing

```markdown
### What We're NOT Doing

| Item | Rationale |
|------|-----------|
| [Deferred item] | [Why not now] |
```

Prevents scope creep. If someone asks "should we also do X?" the answer is here.

#### Key Documents

```markdown
### Key Documents

| Document | Purpose |
|----------|---------|
| [Doc name](path) | [What it contains] |
```

Links to strategy docs, audits, design docs, external references.

#### Quick Reference

```markdown
### Quick Reference

| Resource | Location |
|----------|----------|
| [Resource name] | `path/to/file` |
```

File paths and resource links for fast navigation.

---

## Board Maintenance Rules

### Moving Tasks to COMPLETED.md

When a task reaches 🟢🟢🟢:
1. Copy the row to COMPLETED.md under the current session heading
2. Remove it from BOARD.md
3. If other tasks had this as a Dep, update their Dep to `-` (or the next blocker)

### Updating the Board

- Update the header date and session number each session
- Indicators change as work progresses — keep them current
- Supporting Detail grows as tasks get detailed implementation notes
- Prune Supporting Detail for archived layers (moved to COMPLETED.md)

### Board Size

**Target**: 200-400 lines total.
- Board table: under 40 rows
- Supporting Detail: as long as needed, but prune completed layers
- Closing sections: stable, rarely change

---

## Creating a Board for a New Project

When an agent is asked to create a board, follow this sequence:

### Step 1: Establish Goals (Conversation Required)

Ask the user:
- "What are the core principles for this project?"
- "What matters most — cost, speed, correctness, security, simplicity?"
- "What trade-offs are you willing to make?"

Draft G0-G9 from their answers. Confirm before proceeding.

### Step 2: Establish Layers (Conversation Required)

Ask the user:
- "What's the foundational work everything depends on?"
- "What comes next? What depends on what?"
- "When do we validate? When do we ship?"

Draft L0-LN from their answers. Confirm before proceeding.

### Step 3: Inventory Tasks

Review the codebase, existing docs, or user requirements. Map each task to:
- A layer (where does it belong in the build order?)
- Goal alignment (which principles does it serve?)
- Dependencies (what blocks it?)
- Confidence (do we know how to do it?)
- Validation (how do we prove it worked?)

### Step 4: Write the Board

Follow the section order in this guide. Fill in the unified table. Write supporting detail for each task. Add the closing sections.

### Step 5: Review with User

Present the board. Confirm goals, layers, and task prioritization. Adjust.

---

## Transforming an Existing Board

When an agent has an existing board in a different format:

1. **Read the existing board** — understand all tasks, their status, and their grouping
2. **Map to this structure** — identify goals (from stated or implied principles), layers (from phase/priority grouping), and tasks
3. **Ask about goals and layers** — do not assume. Present candidates and confirm.
4. **Build the new board** — follow the section order above
5. **Preserve status** — carry over completion state. Don't reset progress.
6. **Move completed work** — anything that's done goes to COMPLETED.md, not the board.

---

## Anti-Patterns

| Don't | Do Instead |
|-------|------------|
| Multiple task tables (one per phase) | ONE unified table with Layer column |
| Text status ("in progress", "done") | Indicator dots (🔵, 🟢) |
| Goals that are tasks ("Add auth") | Goals that are principles ("Security First") |
| Code in the board table | Code in Supporting Detail, linked from board |
| Tasks with no goal alignment | Ask: why are we doing this? |
| 50+ row tables | Move 🟢🟢🟢 to COMPLETED.md |
| Inventing goals without conversation | Ask the user what matters |
| Inventing layers without conversation | Ask the user about build order |
| Empty goal columns (G5-G9 all blank) | Drop unused columns |
