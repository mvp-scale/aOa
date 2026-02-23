# aOa - Angle O(1)f Attack

> **5 angles. 1 attack. 1 binary.** Save your tokens. Save your time. Zero Docker.

---

## What Is aOa?

**aOa** is a semantically compressed, intent-driven, predictive code intelligence engine for [Claude Code](https://docs.anthropic.com/en/docs/claude-code).

Every time Claude searches your codebase, it burns tokens rediscovering code it already found yesterday. aOa captures the semantic fingerprint of every tool call, builds a map of your codebase by *meaning*, and feeds that context back automatically.

**Without aOa:**
```
You: "Fix the auth bug"
Claude: [17 tool calls, 4 minutes of searching, 17k tokens burned]
Claude: "Found it. Line 47 in auth.py."
```

**With aOa:**
```
You: "Fix the auth bug"
aOa: [Context injected: auth.py, session.py, middleware.py]
Claude: "I see the issue. Line 47."
```

**150 tokens.** Same result. **99% savings.**

---

## Why a Go Port?

The [original aOa](https://github.com/CTGS-Innovations/aOa) runs as a Python service in Docker. It works. But it can be better.

**aOa is a clean-room rewrite** that eliminates Docker entirely and delivers order-of-magnitude performance improvements:

| Metric | Python aOa | aOa | Improvement |
|--------|-----------|--------|-------------|
| Search latency | 8-15ms | <0.5ms | **16-30x faster** |
| Autotune | 250-600ms | ~2.5&micro;s | **100,000x faster** |
| Startup | 3-8s | <200ms | **15-40x faster** |
| Memory | ~390MB | <50MB | **8x reduction** |
| Install | Docker + docker-compose | Single binary | **Zero dependencies** |
| Infrastructure | Redis + Python services | Embedded bbolt | **Zero services** |

One binary. No Docker. No Redis. No Python. Just download and run.

---

## Features

- **O(1) indexed search** -- same speed whether you have 100 files or 100,000
- **28 languages** with structural parsing (tree-sitter compiled in), 57 total with tokenization
- **134 semantic domains** embedded in the binary -- no AI calls needed to classify code
- **Self-learning** -- gets smarter with every tool call, predicts files before you ask
- **Session log tailing** -- learns from Claude Code sessions without any hooks
- **Single 4.9MB binary** -- no runtime dependencies, no containers, no services

---

## The Five Angles

| Angle | What It Does |
|-------|--------------|
| **Search** | O(1) indexed lookup -- same syntax as grep, orders of magnitude faster |
| **File** | Navigate structure without reading everything |
| **Behavioral** | Learns your work patterns, predicts next files |
| **Outline** | Semantic compression -- searchable by meaning, not just keywords |
| **Intent** | Tracks session activity, shows savings in real-time |

All angles converge into **one confident answer**.

---

## Quick Start

### Install

```bash
# From source
git clone https://github.com/CTGS-Innovations/aOa
cd aOa
go build ./cmd/aoa/
```

### Initialize a Project

```bash
cd your-project
aoa init
```

### Start Searching

```bash
aoa grep handleAuth
```

Instant results. O(1) lookup.

### Run the Daemon

```bash
aoa daemon start
```

The daemon indexes your project, tails Claude Code session logs, and learns your patterns in the background.

---

## Commands

| Command | Description |
|---------|-------------|
| `aoa grep <pattern>` | O(1) indexed search (literal, OR, AND modes) |
| `aoa egrep <pattern>` | Regex search with full flag parity |
| `aoa find <glob>` | Glob-based file search |
| `aoa locate <name>` | Substring filename search |
| `aoa tree [dir]` | Directory tree display |
| `aoa domains` | Domain stats with tier/state/source |
| `aoa intent [recent]` | Intent tracking summary |
| `aoa bigrams` | Top usage bigrams |
| `aoa stats` | Full session statistics |
| `aoa config` | Project configuration display |
| `aoa health` | Daemon status check |
| `aoa wipe [--force]` | Clear project data |
| `aoa daemon start\|stop` | Manage background daemon |

### GNU Grep Parity

`aoa grep` and `aoa egrep` are drop-in replacements for GNU grep. When installed as shims (`~/.aoa/shims/grep`), AI agents use them transparently.

**Three execution modes** -- 100% aligned with GNU grep behavior:

| Mode | Invocation | Behavior |
|------|-----------|----------|
| **File grep** | `grep pattern file.py` | Searches named files, `file:line:content` output |
| **Stdin filter** | `echo text \| grep pattern` | Filters piped input line by line |
| **Index search** | `grep pattern` (no files, no pipe) | Falls back to aOa O(1) index |

**22 flags implemented** -- covers all flags used by AI agents in testing:

```
-i   Case insensitive          -n   Line numbers
-w   Word boundary             -H   Force filename prefix
-c   Count only                -h   Suppress filename
-q   Quiet (exit code only)    -l   Files with matches
-v   Invert match              -L   Files without matches
-m   Max count                 -o   Only matching part
-e   Multiple patterns         -r   Recursive directory search
-E   Extended regex            -F   Fixed strings (literal)
-A   After context             -B   Before context
-C   Context (both)            -a   AND mode (aOa extension)
--include / --exclude / --exclude-dir   File glob filters
--color=auto|always|never               TTY-aware color
```

**GNU grep compatibility details:**

- Exit codes: 0 (match), 1 (no match), 2 (error) -- identical to GNU grep
- Output: `file:line:content` with `:` for matches, `-` for context lines
- Group separators: `--` between non-contiguous context groups
- Binary detection: NUL in first 512 bytes prints `Binary file X matches`
- Multi-file: auto-prefixes filenames when >1 file; `-H` forces, `-h` suppresses
- ANSI: auto-stripped when stdout is not a TTY (piped/captured output is clean)
- Fallback: unrecognized flags or daemon-down forwards to `/usr/bin/grep`

**What's not implemented natively** (forwarded to system grep):

| Flag | Description | Frequency |
|------|-------------|-----------|
| `-P` | Perl/PCRE regex | Rare in agent use |
| `-x` | Match entire line | Rare |
| `-f` | Patterns from file | Rare |
| `-b` | Byte offset | Never seen in agent use |
| `-Z` | NUL-terminated filenames | Never seen in agent use |
| `-R` | Recursive + follow symlinks | Rare (agents use `-r`) |

In practice: **22 of 28 GNU grep flags are native**, covering 100% of observed AI agent usage. The remaining 6 flags (<4% of GNU grep's flag surface) have never appeared in agent session logs and fall back to system grep automatically.

```bash
# All of these work as expected:
aoa grep -rn "TODO" src/           # Recursive search with line numbers
echo "data" | aoa grep pattern     # Stdin filtering
aoa grep -E "err|warn" file.log    # Extended regex on a file
aoa egrep -i "handle.*auth" .      # Case-insensitive regex, recursive
aoa grep -e pat1 -e pat2 file.py   # Multiple patterns (OR)
aoa grep -c -l pattern src/        # Count + filenames
```

---

## How It Works

### Architecture

aOa uses a hexagonal (ports/adapters) architecture:

```
cmd/aoa/              CLI entrypoint (cobra)

internal/
  domain/
    index/            Search engine (O(1) token map, inverted index)
    learner/          observe(), autotune, competitive displacement
    enricher/         Keyword -> term -> domain resolution
    status/           Status line generation

  adapters/
    bbolt/            Embedded key-value storage (crash-safe)
    fsnotify/         File system watcher
    tailer/           Session log tailer (defensive JSONL parser)
    claude/           Claude Code session adapter
    treesitter/       28-language structural parser
    socket/           Unix socket daemon (JSON protocol)

atlas/v1/             134 semantic domains (embedded via go:embed)
```

### Signal Chain

```
Claude Code JSONL session log
  -> Tailer (file discovery, polling, dedup)
    -> Parser (defensive, multi-path field extraction)
      -> Claude Reader (Session Prism decomposition)
        -> App (signal routing)
          |-- User input   -> bigrams, status line
          |-- AI response  -> bigrams
          |-- Tool use     -> range gate -> file hits -> observe
          |-- Search       -> keywords -> enricher -> learner
```

### Learning

aOa watches Claude work and captures semantic signals:

1. **observe()** -- Every search and tool call generates signals (keywords, terms, domains, file hits)
2. **autotune** -- Every 50 prompts, a 21-step optimization runs: decay old signals, deduplicate, rank domains, promote/demote, prune noise
3. **competitive displacement** -- Top 24 domains stay in core, others compete for relevance. Domains that stop appearing naturally fade out.

All learning happens in-memory. State persists to bbolt on autotune and shutdown. No network calls. No AI calls for domain classification.

---

## Language Support

**28 languages** with full tree-sitter structural parsing (function/class/method extraction):

Python, JavaScript, TypeScript, Go, Rust, Java, C, C++, C#, Ruby, PHP, Kotlin, Scala, Swift, Bash, Lua, Haskell, OCaml, Zig, CUDA, Verilog, HTML, CSS, Svelte, JSON, YAML, TOML, HCL

**29 additional languages** with tokenization-based indexing (awaiting upstream Go bindings):

R, Julia, Markdown, Elixir, Erlang, Dart, Nim, Clojure, D, Gleam, Elm, PureScript, Odin, V, Ada, Fortran, Fennel, Groovy, GraphQL, CMake, Make, Nix, Objective-C, VHDL, GLSL, HLSL, SQL, Dockerfile, Vue

**101 file extensions** mapped total. See [languages.md](docs/languages.md) for the full breakdown.

---

## Status Line

aOa generates a status line that shows your session at a glance:

| Stage | Status Line |
|-------|-------------|
| Learning | `aOa 5 \| calibrating...` |
| Predicting | `aOa 35 \| 2k saved \| ctx:15k/200k (8%)` |
| Confident | `aOa 69 \| 80k saved \| ctx:36k/200k (18%)` |
| Long session | `aOa 247 \| 1.8M saved \| ctx:142k/200k (71%)` |

Written to `/tmp/aoa-status-line.txt` on every state change. Use the included hook or read it however you like.

---

## Your Data. Your Control.

- **Local-only** -- single binary, no network calls, no containers
- **No data leaves** -- your code stays on your machine
- **Open source** -- MIT licensed, fully auditable
- **Explainable** -- `aoa intent recent` shows exactly what it learned

---

## Uninstall

**Remove from a project:**
```bash
aoa wipe --force
```

**Remove the binary:**
```bash
rm $(which aoa)
```

Nothing else to clean up. No containers. No services. No config files scattered across your system.

---

## Project Status

aOa is in active development. **211 tests passing, 0 failing.**

- Phases 1-5: Complete (foundation, search, domains, learning, session integration)
- Phase 6: In progress (CLI complete, daemon wired, release pipeline pending)
- Phase 7: Planned (migration validation, parallel run with Python version)

See [GO-BOARD.md](GO-BOARD.md) for the full project board.

---

## License

MIT
