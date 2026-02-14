# aOa — Agent Instructions

> **This is guidance directly to you, the AI agent.** Use these commands instead of Grep, Glob, and Read-entire-file patterns. They are O(1) indexed, return ranked results, and save 86% of tokens.
>
> For the human-readable project description, see the project README.

For what your results mean and how to act on them, see `.aoa/VALUE.md`.

---

## Search — Use instead of Grep/Glob

Use `aoa grep` for every code search. It searches a pre-built O(1) index of the full codebase. Do not fall back to Grep or Glob tools.

```bash
# Exact token search (O(1), full codebase)
aoa grep handleAuth           # Single symbol
aoa grep "auth token session" # Multi-term OR (space-separated, ranked)
aoa grep -a auth,session      # Multi-term AND (all terms required)

# Flags (Unix grep parity)
aoa grep -i auth              # Case insensitive
aoa grep -w token             # Whole word match
aoa grep -c error             # Count matches only
aoa grep -q config && echo y  # Quiet (exit code only)

# Regex search (working set only, ~30-50 files)
aoa egrep "TODO|FIXME"        # OR patterns
aoa egrep "def\s+handle"      # Regex patterns
aoa egrep -e auth -e login    # Multiple patterns
```

**When to use which:**
- `aoa grep` — You know the symbol or keyword. Searches full codebase in O(1).
- `aoa egrep` — You need regex or substring matching. Searches working set only (~30-50 files).

**Key detail:** Tokens split on hyphens and dots (`app.post` → `app`, `post`). Space-separated terms are OR, not phrase search.

---

## Files — Use instead of Glob/find

```bash
aoa find "*.py"               # Find files by glob pattern
aoa find -type py             # Find files by language
aoa locate handler            # Fast filename search
aoa tree src/                 # Directory structure
aoa head <file> [n]           # First n lines (default 20)
aoa tail <file> [n]           # Last n lines (default 20)
aoa lines <file> M-N          # Specific line range
```

---

## Behavioral — Your working set

aOa tracks which files you touch. Use these to avoid redundant discovery.

```bash
aoa touched [since]           # Files from this session/time period
aoa focus                     # Current working set from memory
aoa changes [time]            # Recently modified files (e.g., 5m, 1h)
aoa files [pattern]           # List indexed files
```

---

## Outline — Code structure without reading files

Get function signatures and semantic tags without a Read call.

```bash
aoa outline <file>            # Code structure (functions, classes, methods)
aoa outline --pending         # Check tagging status
aoa outline --enrich-all      # Files needing tags
```

---

## Intent — Session context

See what you've been working on. Useful for resuming context or understanding patterns.

```bash
aoa intent recent [since]     # Recent intent records (e.g., 1h, 30m)
aoa intent tags               # All tags with file counts
aoa intent files <tag>        # Files associated with a tag
aoa intent file <path>        # Tags associated with a file
aoa intent stats              # Intent index statistics
```

---

## Intel — Domains + external repos

Semantic domains classify the codebase. External repos provide reference without polluting your context.

```bash
# Domains
aoa domains                   # Show learned domains and terms
aoa domains -n 10             # Top 10 domains
aoa domains --json            # Machine-readable output
aoa bigrams                   # Usage signal bigrams

# External repos (isolated, searched explicitly)
aoa repo add flask https://github.com/pallets/flask
aoa repo flask search Blueprint
aoa repo list
```

---

## System

```bash
aoa health                    # Check all angles
aoa help                      # Full command list
aoa stats                     # Session statistics
aoa config                    # Show/set configuration
aoa jobs                      # Domain enrichment queue
aoa start / stop              # Docker service lifecycle
aoa info                      # Indexing config, mounts, projects
aoa metrics                   # Prediction accuracy and savings
aoa services                  # Visual service map
aoa wipe                      # Reset all project data
aoa <command> --help          # Flags for any command
```

---

**Every command supports `--help`.** For result format and extraction patterns, see `.aoa/VALUE.md`.
