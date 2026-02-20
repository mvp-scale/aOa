# Migration Guide: Python aOa to Go aOa

## Prerequisites

- Go 1.21+ installed (`go version`)
- CGo enabled (tree-sitter grammars are compiled-in)
- GCC/Clang available for CGo compilation

## Install

### From source (recommended during development)

```bash
cd aOa-go
go build ./cmd/aoa/
# Binary is at ./aoa
```

### Via `go install`

```bash
go install github.com/corey/aoa/cmd/aoa@latest
```

## Migration Steps

### 1. Stop the Python daemon

```bash
# In each project directory where Python aOa is running:
aoa daemon stop
```

Verify it's stopped:
```bash
ls /tmp/aoa-*.sock   # Should show no sockets for your projects
```

### 2. Initialize with Go

```bash
cd /path/to/your/project
./aoa init
```

This parses all source files with tree-sitter and builds a fresh index in `.aoa/aoa.db`. The Go index is stored in bbolt (separate from any Python pickle/sqlite files).

Expected output:
```
Indexed N files, M symbols, K tokens in Xms
```

### 3. Start the Go daemon

```bash
./aoa daemon start
```

The daemon opens a Unix socket at `/tmp/aoa-{hash}.sock` and an HTTP dashboard on a random localhost port.

### 4. Verify

```bash
# Health check
./aoa health

# Test search
./aoa grep handler

# Regex search
./aoa egrep 'func.*Handler'

# Dashboard
cat .aoa/http.port   # Get the port number
# Open http://localhost:{port} in a browser
```

### 5. Configure Claude Code hook (optional)

Add to your Claude Code hooks configuration:

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": ".*",
        "hooks": [
          {
            "type": "command",
            "command": "/path/to/aoa status-hook"
          }
        ]
      }
    ]
  }
}
```

The Go daemon also learns from Claude session logs directly (no hook needed for the learning pipeline).

## Rollback

If you need to go back to Python:

1. Stop the Go daemon: `./aoa daemon stop`
2. Restart the Python daemon in your project directory
3. The Python daemon uses its own data files (separate from `.aoa/aoa.db`)

No data is shared between Python and Go — they use independent storage. You can switch freely.

## Data

| Item | Python | Go |
|------|--------|----|
| Index storage | pickle/sqlite | bbolt (`.aoa/aoa.db`) |
| Learner state | pickle/sqlite | bbolt (`.aoa/aoa.db`) |
| Socket | `/tmp/aoa-{hash}.sock` | `/tmp/aoa-{hash}.sock` (same path scheme) |
| Dashboard | HTTP | HTTP (same `localhost:{port}` pattern) |
| Session logs | `~/.claude/projects/` (read-only) | Same path (read-only) |

## Behavioral Parity

The Go port maintains zero-tolerance behavioral parity with Python:

- **Search**: 26 fixture queries validated (symbol lookup, OR/AND modes, regex, all flags)
- **Learner**: 200-intent replay with 5 state checkpoints, zero float divergence
- **Autotune**: 21-step algorithm matches Python's decay/prune/rank math exactly
- **grep flags**: 16 flags implemented matching GNU grep interface (67% of GNU grep surface)

### Grep Flag Coverage

```
IMPLEMENTED (aoa grep):
  -i  --ignore-case       Case insensitive search
  -w  --word-regexp        Word boundary match
  -v  --invert-match       Select non-matching symbols
  -c  --count              Count only
  -q  --quiet              Exit code only (0=found, 1=not)
  -m  --max-count=NUM      Limit results (default 20)
  -E  --extended-regexp    Route to regex mode
  -e  --regexp=PATTERN     Multiple patterns (OR)
  -a  --and                AND mode (comma-separated terms)
      --include=GLOB       Include file filter
      --exclude=GLOB       Exclude file filter

IMPLEMENTED (aoa egrep):
  -c  --count              Count only
  -q  --quiet              Exit code only
  -v  --invert-match       Select non-matching
  -m  --max-count=NUM      Limit results (default 20)
  -e  --regexp=PATTERN     Multiple patterns (joined with |)
      --include=GLOB       Include file filter
      --exclude=GLOB       Exclude file filter

ACCEPTED BUT NO-OP:
  -r  --recursive          Always recursive (no-op)
  -n  --line-number        Always shows line numbers (no-op)
  -H  --with-filename      Always shows filenames (no-op)
  -F  --fixed-strings      Already literal (no-op)
  -l  --files-with-matches Default behavior (no-op)
```

### Performance

Benchmarked on Intel i9-10980XE:

| Operation | Go | Python | Speedup |
|-----------|-----|--------|---------|
| Search (500 files) | ~59 us | 8-15 ms | **135-254x** |
| Observe (50 events) | ~78 us | 150-250 ms | **1900-3200x** |
| Autotune (21-step) | ~24 us | 250-600 ms | **10,000-25,000x** |
| Index file (tree-sitter) | ~9 ms | 50-200 ms | **5-22x** |
| Startup (bbolt load) | ~8 ms | 3-8 s | **375-1000x** |
| Memory (500 files) | ~0.4 MB | ~390 MB | **975x** |
