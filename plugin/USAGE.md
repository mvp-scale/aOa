# aOa Usage Guide

> **5 angles. 1 attack.** Each angle solves a real problem.

---

## 1. Search Angle — Find code instantly

**Why:** Grep scans everything. aOa is O(1) indexed. 100x faster.

```bash
aoa grep handleAuth           # Find any symbol
aoa grep "auth token"         # Multi-term OR (ranked)
aoa grep -a auth,session      # Multi-term AND (all required)
aoa egrep "TODO|FIXME"        # Regex patterns
```

**Result:** `file:function()[45-89]:52 #tags`
- Function name and line range `[45-89]` — read only what matters
- Line 52 matched — jump directly there
- Tags — semantic context

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
aoa outline <file>            # See structure
aoa outline --pending         # Files needing tags
aoa quickstart                # Tag your codebase (~1 min)
aoa grep "#authentication"    # Search by concept
```

---

## 5. Intent Angle — See your session

**Why:** See your session as aOa sees it—operations, patterns, savings.

```bash
aoa intent                    # Activity dashboard
aoa intent recent             # Recent operations
aoa intent tags               # All semantic tags
aoa intent stats              # Totals and savings
```

---

## Intel (Extension) — Reference repos without bloat

**Why:** Get external repo knowledge without server overhead or token bloat.

```bash
aoa repo add flask https://github.com/pallets/flask
aoa repo flask search Blueprint
aoa repo list                 # Your intel sources
```

Same O(1) search on external repos. Fast, isolated, no bloat.

---

## System

```bash
aoa health                    # Check all angles
aoa help                      # Full command list
aoa <command> --help          # Flags for any command
```

---

**The value:** 50 lines instead of 3,700. Instant search. Every session builds on the last.
