# grammars/

Pre-built tree-sitter grammar libraries for aOa. These are compiled weekly by CI from upstream open source grammar repositories and distributed here so that `aoa init` can install them without requiring a C compiler.

## Directory Structure

```
grammars/
  parsers.json              Manifest — every grammar with SHA-256, provenance, platform status
  GRAMMAR_REPORT.md         Weekly validation report — what compiled, who built it, platform matrix
  README.md                 This file
  linux-amd64/              Pre-built .so files for Linux x86_64
  linux-arm64/              Pre-built .so files for Linux ARM64
  darwin-amd64/             Pre-built .dylib files for macOS x86_64
  darwin-arm64/             Pre-built .dylib files for macOS ARM64 (Apple Silicon)
```

Each platform directory contains one shared library per grammar (e.g., `go.so`, `python.so`). Only grammars that compile successfully on a given platform are included.

## How It Works

1. **Weekly CI** compiles all grammars from [go-sitter-forest](https://github.com/alexaandru/go-sitter-forest) on 4 platforms (linux/darwin x amd64/arm64)
2. **Compiled binaries** are committed to the platform directories above
3. **parsers.json** is regenerated with SHA-256 hashes of every binary, source provenance, upstream repo links, and maintainer attribution
4. **GRAMMAR_REPORT.md** summarizes the validation results with a per-grammar platform matrix

## parsers.json

The manifest is the source of truth. Each entry contains:

```json
{
  "name": "python",
  "version": "0.25.0",
  "upstream_url": "https://github.com/tree-sitter/tree-sitter-python",
  "maintainer": "tree-sitter",
  "upstream_revision": "abc123...",
  "source_sha256": "...",
  "platforms": {
    "linux-amd64": { "status": "ok", "sha256": "...", "size_bytes": 319488 },
    "linux-arm64": { "status": "ok", "sha256": "...", "size_bytes": 307200 },
    "darwin-arm64": { "status": "ok", "sha256": "...", "size_bytes": 294912 },
    "darwin-amd64": { "status": "ok", "sha256": "...", "size_bytes": 311296 }
  }
}
```

## SHA-256 Verification

Every binary has a SHA-256 hash recorded in `parsers.json`. When `aoa init` downloads a grammar, it verifies the hash matches before installing. This ensures you get exactly what the CI compiled — nothing modified in transit.

To verify manually:

```bash
# Compare against the sha256 field in parsers.json
sha256sum grammars/linux-amd64/python.so
# or on macOS:
shasum -a 256 grammars/darwin-arm64/python.dylib
```

## How aoa init Uses This

```
aoa init
  → Fetches parsers.json from this directory
  → Scans your project for languages used
  → Downloads the .so/.dylib files for your platform
  → Verifies SHA-256 against parsers.json
  → Copies to .aoa/grammars/
  → Indexes your project with structural parsing
```

No C compiler required. No compilation step. Just download, verify, use.

## Source

All grammars originate from the open source tree-sitter ecosystem, aggregated by [@alexaandru](https://github.com/alexaandru) in [go-sitter-forest](https://github.com/alexaandru/go-sitter-forest). See [GRAMMAR_REPORT.md](GRAMMAR_REPORT.md) for per-grammar maintainer attribution and upstream links.
