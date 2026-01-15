# aOa Usage Guide

> **5 angles. 1 attack.** Each angle solves a real problem.

---

## 1. Search Angle — Find code instantly

**Why:** Grep scans everything. aOa is O(1) indexed. 100x faster.

```bash
# Basic search
aoa grep handleAuth           # Find any symbol
aoa grep "auth token"         # Multi-term OR (ranked)
aoa grep -a auth,session      # Multi-term AND (all required)

# Unix grep parity
aoa grep -i auth              # Case insensitive
aoa grep -w token             # Whole word match
aoa grep -c error             # Count matches only
aoa grep -q config && echo y  # Quiet (exit code only)

# Regex with egrep
aoa egrep "TODO|FIXME"        # OR patterns
aoa egrep "def\s+handle"      # Regex patterns
aoa egrep -e auth -e login    # Multiple patterns

# Combine flags
aoa grep -i -w Auth           # Case insensitive + whole word
aoa grep --help               # All flags and examples
```

**Result format:**
```
file:Class.method[range]:line <grep output> @domain #tags
```
- `Class.method` — containing class and function
- `[range]` — function line range (read only what matters)
- `<grep output>` — standard grep content
- `@domain` — semantic domain
- `#tags` — intent indicators

---

## 2. File Angle — Navigate structure

**Why:** Stop guessing where things are. See the structure, jump to the symbol.

```bash
aoa find "*.py"               # Find files by pattern
aoa locate handler            # Fast filename search
aoa tree src/                 # Directory structure
```

---

## 3. Behavioral Angle — Work smarter

**Why:** The files you touch often are the files you need next. aOa learns your rhythm.

```bash
aoa hot                       # Files you access most
aoa touched                   # Files from this session
aoa predict                   # What you'll likely need next
aoa changes 1h                # Recently modified
```

---

## 4. Outline Angle — Semantic compression

**Why:** Compress meaning into searchable tags. One scan, searchable forever.

```bash
aoa outline <file>            # See structure without reading all
aoa outline --pending         # Files needing tags
aoa outline --json            # Machine-readable output
aoa quickstart                # Tag your codebase (~1 min)
```

---

## 5. Intent Angle — See your session

**Why:** See your session as aOa sees it—operations, patterns, savings.

```bash
aoa intent                    # Recent activity (default 20)
aoa intent -n 50              # Show more records
```

---

## Intel Angle — Knowledge sources (local + external)

**Why:** Understand your code semantically (domains) and reference external repos without bloat.

```bash
# Domains - semantic labels for your code
aoa domains                   # Show learned domains and terms
aoa domains -n 10             # Show top 10 domains
aoa domains --json            # Machine-readable output

# External repos - isolated from your code
aoa repo add flask https://github.com/pallets/flask
aoa repo flask search Blueprint  # Search Flask repo (explicit)
aoa repo list                    # Your intel sources
```

**External repos are isolated** - they never mix with your project code. Searching is explicit (`aoa repo <name> search`), not automatic. This saves tokens vs MCP servers by avoiding context bloat.

---

## System

```bash
aoa health                    # Check all angles
aoa help                      # Full command list
aoa <command> --help          # Flags for any command
aoa wipe                      # Reset project data
```

---

**Every command supports `--help`** for detailed flags and examples.

**The value:** 50 lines instead of 3,700. Instant search. Every session builds on the last.
