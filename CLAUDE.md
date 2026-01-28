## CRITICAL: CLI Build Requirement

**The `cli/aoa` file is GENERATED from `cli/src/*.sh` files.**

After ANY change to files in `cli/src/`, you MUST rebuild:

```bash
./cli/build.sh
```

If `aoa grep` returns 0 results but the API works, **you forgot to rebuild**.

---

# aOa - 5 Angles. 1 Attack.

## Hooks, Not API Keys

**CRITICAL: aOa uses Claude Code HOOKS to access Haiku - NOT direct API calls.**

- ❌ NEVER use `ANTHROPIC_API_KEY` or direct Anthropic API calls
- ✅ ALWAYS use hooks to trigger Haiku via Claude Code's built-in capabilities

Hooks allow aOa to call Haiku without requiring users to have API keys. This is a core architectural decision that enables zero-config learning.

**How it works:**
1. Hook detects a condition (e.g., `should_learn = true`)
2. Hook signals Claude Code to spawn a Haiku Task
3. Haiku Task runs in Claude Code's context (no API key needed)
4. Results are stored via API calls back to aOa services

This keeps aOa accessible to all users regardless of whether they have Anthropic API access.

---

## Confidence & Communication

### Traffic Light System

Always indicate confidence level before starting work:

| Signal | Meaning | Action |
|--------|---------|--------|
| 🟢 | Confident | Proceed freely |
| 🟡 | Uncertain | Try once. If it fails, research via Context7 or ask if architectural |
| 🔴 | Lost | STOP immediately. Summarize and ask: "Should we use 131?" |

### Set Expectations

Don't go too far without telling the user:
- What you're about to do
- Where to follow along (BOARD.md, logs, etc.)
- Your confidence level (traffic light)

### 1-3-1 Approach (For Getting Unstuck)

When hitting 🔴 or repeated 🟡 failures:

1. **1 Problem** - State ONE simple problem (not composite)
2. **3 Solutions** - Research three professional production-grade solutions
3. **1 Recommendation** - Give one recommendation (single solution, blend, or hybrid)

This breaks death spirals and forces clear thinking.

---

## Agent Conventions

When the user addresses an agent by name using "Hey [AgentName]", spawn that agent to handle the request.

| Trigger | Agent | Purpose |
|---------|-------|---------|
| "Hey B" / "Hey Beacon" | beacon | Project continuity - work board, progress tracking, session handoffs |
| "Hey 131" | 131 | Research-only problem solving with parallel solution discovery |
| "Hey GH" | gh | Growth Hacker - solutions architect, problem decomposer |

### aOa Setup (SPECIAL - Guided Onboarding)

When user says **"Hey aOa"**, **"/aoa-setup"**, or **"set up aoa"**:

1. Run `aoa domains --json` silently to check domain status
2. Check `domain_count` in the response

**If domain_count = 0** (fresh project, needs setup):

Run the `/aoa-setup` skill to guide them through personalized onboarding:
- Explains what's happening (trust, transparency)
- Estimates time based on file count
- Runs parallel Haiku analysis to generate 24 core semantic domains
- Runs semantic tagging for compressed file outlines
- Shows completion summary with next steps

The skill file is at `plugin/skills/setup/SKILL.md` - follow its instructions.

**If domain_count > 0** (already set up / returning user):

```
⚡ aOa is ready. What are we building?
```

Show them their current stats:
- `aoa domains` - semantic domain map
- `aoa intent` - real-time intent tracking

**DO NOT launch background agents.** Keep the guided experience in the main conversation.

### ⚠️ Subagents Don't Get Hooks

**DO NOT use subagents for codebase exploration.** Subagents run in a separate context and don't trigger aOa hooks. This means:
- ❌ No intent capture
- ❌ No predictions
- ❌ No learning
- ❌ Breaks the aOa value proposition

**Keep exploration in the main conversation** where hooks work.

**Exception:** `aoa-outline` is fine for background tagging (write-only, doesn't need hooks).

### Agent Context Loading

**All agents MUST read context files before exploring the codebase.**

When spawning any agent, instruct it to first read:
1. `.context/BOARD.md` - Current focus, active tasks, blockers
2. `.context/CURRENT.md` - Session context, recent decisions

---

## Rule #1: Symbol Angle First

**NEVER do this:**
```bash
# WRONG - Multiple tool calls, slow, wasteful
Grep(pattern="auth", path="src/")  # 1 call
Read(file1.py)                      # 2 calls
Read(file2.py)                      # 3 calls
Read(file3.py)                      # 4 calls
# = 4 tool calls, ~8,500 tokens, 2+ seconds
```

**ALWAYS do this:**
```bash
# RIGHT - One call, fast, efficient
aoa grep auth
# Returns: file:line for ALL matches in <5ms
# Then read ONLY the specific lines you need
```

## Rule #2: aOa Returns File:Line - Use It

aOa grep output:
```
⚡ 20 hits │ 4.73ms
  index/indexer.py:1308
  status_service.py:56
  status_service.py:115
```

This tells you EXACTLY where to look. Don't read entire files - read specific line ranges:
```bash
Read(file_path="src/index/indexer.py", offset=1305, limit=10)
```

## Rule #3: One Angle Replaces Many Tools

**WRONG:**
```bash
Grep("auth")    # call 1
Grep("login")   # call 2
Grep("session") # call 3
```

**RIGHT:**
```bash
aoa grep "auth login session"  # ONE call, ranked results
```

## Rule #4: Three Search Modes

**Instant Search (O(1) - full index):**
```bash
aoa grep tree_sitter                  # exact match
aoa grep "auth session token"         # multi-term OR search, ranked
```
**Note:** Space-separated terms are OR search, not phrase search.

**Multi-Term Intersection (full index):**
```bash
aoa grep -a auth,session,token        # files containing ALL terms (AND)
```

**Pattern Search (regex - working set only ~30-50 files):**
```bash
aoa egrep "tree.sitter"               # regex
aoa egrep "def\\s+handle\\w+"         # find patterns
```
**Warning:** Pattern search only scans local/recent files, not full codebase.

**When to use which:**
- `aoa grep` → Know the target, need speed, OR logic
- `aoa grep -a` → Need files matching ALL terms (AND logic)
- `aoa egrep` → Need regex matching (working set only)

**Tokenization:** Hyphens and dots break tokens (`app.post` → `app`, `post`).

## Rule #5: Clean Command Execution

**NEVER suppress output:**
```bash
# WRONG - Hides errors, looks suspicious
aoa grep auth 2>/dev/null

# RIGHT - Clean, transparent
aoa grep auth
```

Errors are information. Suppressing stderr:
- Hides useful debugging info
- Looks suspicious/hacky
- Breaks the clean aOa experience
- Reduces trust

## Unix grep/egrep Parity

aOa commands mirror Unix grep/egrep so they feel intuitive:

| Unix Command | aOa Equivalent | Behavior |
|--------------|----------------|----------|
| `grep "foo"` | `aoa grep foo` | Single term search |
| `grep -E "foo\|bar"` | `aoa grep -E "foo\|bar"` | Routes to egrep (regex) |
| `grep "foo\|bar"` | `aoa grep "foo\|bar"` | OR search (pipe converted) |
| `grep "foo bar"` | `aoa grep "foo bar"` | OR search (space-separated) |
| `grep -e foo -e bar` | `aoa grep -e foo -e bar` | OR search (multiple patterns) |
| `egrep "foo\|bar"` | `aoa egrep "foo\|bar"` | Regex OR search |
| `egrep "foo.*bar"` | `aoa egrep "foo.*bar"` | Regex pattern |
| `egrep -e foo -e bar` | `aoa egrep -e foo -e bar` | Multiple patterns (combined with `\|`) |
| `egrep -r/-n/-H` | `aoa egrep` | No-ops (always recursive, shows lines/files) |
| `grep -r` | `aoa grep` | Always recursive (no-op) |
| `grep -n` | `aoa grep` | Always shows line numbers (no-op) |
| `grep -H` | `aoa grep` | Always shows filenames (no-op) |
| `grep -F` | `aoa grep` | Already literal search (no-op) |
| `grep -c` | `aoa grep -c` | Count only |
| `grep -i` | `aoa grep -i` | Case insensitive |
| `grep -w` | `aoa grep -w` | Word boundary |
| `grep -q` | `aoa grep -q` | Quiet mode (exit code only) |
| `grep -l` | `aoa grep` (default) | List files with matches |

**Key difference:** aOa `grep` searches indexed symbols (O(1)), while `egrep` uses regex on the working set.

## Commands

| Command | Use For | Speed |
|---------|---------|-------|
| `aoa grep <term>` | Symbol lookup | <5ms |
| `aoa grep "term1 term2"` | Multi-term OR search | <10ms |
| `aoa grep -a t1,t2` | Multi-term AND search | <10ms |
| `aoa egrep "regex"` | Regex search | ~20ms |
| `aoa find "*.py"` | File discovery | <10ms |
| `aoa locate name` | Fast filename search | <5ms |
| `aoa tree [dir]` | Directory structure | <50ms |
| `aoa hot` | Frequently accessed files | <10ms |
| `aoa health` | Check services | instant |
| `aoa intent recent` | See what's being worked on | <50ms |
| `aoa analyze` | Generate project domains | ~30s |
| `aoa domains` | Show domain stats | <50ms |

## API Endpoints (localhost:8080)

For programmatic access via curl:

```bash
curl "localhost:8080/symbol?q=handleAuth"           # Instant search
curl "localhost:8080/multi?q=auth+login+handler"    # Multi-term ranked
curl "localhost:8080/files"                          # List indexed files
curl "localhost:8080/intent/recent"                  # Recent intents
curl "localhost:8080/domains/stats?project=ID"      # Domain learning stats
```

## Semantic Domains (GL-083)

aOa uses semantic domains to enhance search. Grep results show `@domain` tags in MAGENTA:

```
services/auth/handler.py:login()[10-45]:12 def login(user):  @authentication  #api #security
```

Domains are:
- **Generated** via `/aoa-setup` (personalized domains from your codebase structure)
- **Stored** in `.aoa/project-domains.json` (v2 format with terms and keywords)
- **Loaded** by `aoa quickstart` into Redis for fast lookup
- **Rebalanced** every 25 prompts (assigns orphan keywords to terms)

**Setup command**: `/aoa-setup` analyzes your codebase structure and generates 24 core semantic domains (no API key required).

## Intelligence & Intent Angles

aOa uses two learning angles:
- **Intelligence**: Analyze codebase → domains → terms → keywords (one-time setup)
- **Intent**: Track usage → refine domains (continuous)

### `/aoa-start` - Background Intelligence

Run `/aoa-start` to initialize aOa. This spawns a background agent that:
1. Scans project structure (`aoa tree`)
2. Generates 24 core semantic domains
3. Enriches each with terms and keywords (batches of 3)

**User experience:**
- Friendly welcome message explains what's happening
- Background agent runs silently
- Status line shows progress: `intelligence X/N` (dynamic count)
- User continues working uninterrupted
- When complete, shifts to `intent` phase

**Per-domain files** enable parallel enrichment:
```
.aoa/domains/@search.json
.aoa/domains/@rest_api.json
```

No hook prompts - the background agent handles everything.

## Efficiency Comparison

| Approach | Tool Calls | Tokens | Time |
|----------|------------|--------|------|
| Grep + Read loops | 7 | 8,500 | 2.6s |
| aOa grep | 1-2 | 1,150 | 54ms |
| **Savings** | **71%** | **86%** | **98%** |

## Decision Tree

1. **Need to find code?** → `aoa grep <term>` (NOT Grep)
2. **Need multiple terms?** → `aoa grep "term1 term2"` (NOT multiple Greps)
3. **Need files by pattern?** → `aoa find "*.py"` or `aoa locate name`
4. **Need file content?** → Read specific lines from aOa results (NOT entire files)
5. **Need regex matching?** → `aoa egrep "pattern"`
6. **Need to understand patterns?** → `aoa intent recent`

## Intent Tracking

Every tool call is captured automatically. The status line shows:
```
⚡ aOa │ 61 intents │ 14 tags │ 0.1ms │ searching shell markdown editing
```

This helps predict which files you'll need next.

## Project Structure

### Context Files

```
.context/
├── CURRENT.md      # Entry point - immediate context, next action
├── BOARD.md        # Master table - all work with status, deps, solution patterns
├── COMPLETED.md    # Archive of completed work with session history
├── BACKLOG.md      # Parked items with enough detail to pick up later
├── decisions/      # Architecture decision records (ADRs)
├── details/        # Deep dives, investigations (date-prefixed)
└── archive/        # Completed sessions, old bridges (date-prefixed)
```

## Docker Parity Rule

**CRITICAL: Both Docker approaches MUST be maintained in parity.**

We provide two deployment options:
- `docker-compose.yml` - Multi-container (gateway, index, status, proxy, redis) - better for debugging
- `Dockerfile` - Monolithic single container - simpler for end users

**When modifying services:**
1. Update the service code (e.g., `services/status/status_service.py`)
2. Verify change works in docker-compose: `docker-compose build && docker-compose up -d`
3. Verify change works in monolithic: `docker build -t aoa . && docker run ...`
4. Both must produce identical behavior

**Environment variables must match** - if you add an env var to `docker-compose.yml`, ensure the monolithic Dockerfile/entrypoint handles it too.

## Health Check

Run `aoa health` to verify services are running.
