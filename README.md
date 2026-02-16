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

### Grep Flags

Full parity with the Python version:

```bash
aoa grep -i pattern          # Case insensitive
aoa grep -w pattern          # Word boundary
aoa grep -c pattern          # Count only
aoa grep -q pattern          # Quiet (exit code only)
aoa grep -m 5 pattern        # Max 5 results
aoa grep --include "*.py"    # File glob filter
aoa grep --exclude "*.test"  # Exclude glob
aoa grep -a "term1 term2"    # AND mode (intersection)
aoa egrep "handle.*login"    # Regex mode
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

**101 file extensions** mapped total. See [LANGUAGES.md](LANGUAGES.md) for the full breakdown.

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
