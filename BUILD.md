# Build & Distribution Guide

## Architecture

aOa ships as two binaries:

| Binary | Size | CGo | What it does |
|--------|------|-----|-------------|
| `aoa` | ~8 MB | No | Search, learning, dashboard, daemon. Pure Go. Works standalone. |
| `aoa-recon` | ~73 MB | Yes | Tree-sitter parsing + security scanning. Enhances the aoa index with symbols. |

`aoa` auto-discovers `aoa-recon` when present. No configuration needed.

---

## Building from Source

### Prerequisites

- Go 1.21+ (check with `go version`)
- GCC or Clang (only for aoa-recon)
- Node.js 18+ (only for npm packaging)

### Build Commands

```bash
# Pure Go binary (no CGo, no tree-sitter)
make build-pure          # or: CGO_ENABLED=0 go build -o aoa ./cmd/aoa/

# Full binary (CGo, all grammars compiled in)
make build               # or: go build -o aoa ./cmd/aoa/

# Recon binary (CGo, tree-sitter + scanning)
make build-recon         # or: go build -o aoa-recon ./cmd/aoa-recon/

# Both binaries for distribution
make build-pure && make build-recon
```

### Verify the Build

```bash
# Check binary type
file aoa                 # should say "statically linked" for pure build

# Check version
./aoa --version
./aoa-recon version

# Run tests
go test ./...            # all tests (CGo required)
make check               # vet + lint + test
```

---

## Testing the Integration

### Step 1: aoa standalone (no recon)

```bash
# Build pure binary
make build-pure

# Index this project
./aoa init
# Expected: "X files, 0 symbols, Y tokens"
# 0 symbols because there's no parser — tokenization only

# Search works at file level
./aoa grep SearchEngine
# Returns file-level hits (no symbol names or line ranges)
```

### Step 2: Enhance with aoa-recon

```bash
# Build recon binary
make build-recon

# Run recon against the existing index
./aoa-recon enhance --db .aoa/aoa.db --root .
# Expected: "enhanced N files, M total files in index"

# Same search now returns symbol-level results
./aoa grep SearchEngine
# Returns hits like: search.go:type SearchEngine struct[11-42]:11
```

### Step 3: Auto-discovery

aoa discovers aoa-recon in three locations, checked in order:

1. **PATH** — `which aoa-recon` (npm global install puts it here)
2. **Project-local** — `.aoa/bin/aoa-recon`
3. **Sibling** — same directory as the `aoa` binary

To test:

```bash
# Both binaries in the same directory (sibling discovery)
make build-pure && make build-recon

# Start the daemon
./aoa daemon start
# Log should show: "aoa-recon found at ./aoa-recon"

# Verify via the dashboard API
curl -s localhost:$(cat .aoa/http.port)/api/recon | python3 -m json.tool | grep recon_available
# Expected: "recon_available": true

# Dashboard Recon tab shows full scan results
./aoa open

./aoa daemon stop
```

### Step 4: Without recon (graceful degradation)

```bash
# Remove or rename aoa-recon
mv aoa-recon aoa-recon.bak

./aoa daemon start
# Log should NOT mention aoa-recon

curl -s localhost:$(cat .aoa/http.port)/api/recon | python3 -m json.tool | grep recon_available
# Expected: "recon_available": false

# Dashboard Recon tab shows install prompt:
# "aoa-recon not installed — npm install -g aoa-recon"

./aoa daemon stop
mv aoa-recon.bak aoa-recon
```

---

## Discovery Locations for aoa-recon

| Method | Path | When to use |
|--------|------|-------------|
| PATH | `aoa-recon` anywhere on $PATH | npm global install, system package |
| Project-local | `.aoa/bin/aoa-recon` | Per-project install, vendored binary |
| Sibling | Same directory as `aoa` binary | Manual download, both in ~/bin |

The project-local path (`.aoa/bin/`) is useful when different projects need different versions, or when you don't want to install globally.

---

## npm Distribution

### How npm Install Works

npm uses `optionalDependencies` with `os`/`cpu` constraints. When you run `npm install`, npm only downloads the package matching your platform. A postinstall script creates a symlink to the actual binary.

### Global Install (available everywhere)

```bash
# Install aoa globally — available as `aoa` in any terminal
npm install -g aoa

# Install recon globally — aoa auto-discovers it via PATH
npm install -g aoa-recon
```

After global install:
- `aoa` is on your PATH (works from any directory)
- `aoa-recon` is on your PATH (aoa finds it automatically)
- Every project benefits from both binaries

### Per-Project Install (scoped to one project)

```bash
cd my-project

# Install aoa as a dev dependency
npm install --save-dev aoa

# Install recon as a dev dependency
npm install --save-dev aoa-recon
```

After per-project install:
- Binaries are in `node_modules/.bin/aoa` and `node_modules/.bin/aoa-recon`
- Works via `npx aoa init`, `npx aoa grep`, etc.
- Or add to package.json scripts:

```json
{
  "scripts": {
    "aoa:init": "aoa init",
    "aoa:search": "aoa grep",
    "aoa:recon": "aoa-recon enhance --db .aoa/aoa.db --root ."
  }
}
```

### Which to Choose?

| Scenario | Recommendation |
|----------|---------------|
| Personal dev machine | `npm install -g aoa aoa-recon` |
| Team project (everyone gets it) | `npm install --save-dev aoa aoa-recon` |
| CI/CD pipeline | `npm install --save-dev aoa` (recon optional) |
| Trying it out | `npm install -g aoa` first, add recon later |

### Manual Alternative (no npm)

Download binaries from GitHub releases and place them together:

```bash
# Linux amd64
curl -L https://github.com/corey/aoa/releases/latest/download/aoa-linux-amd64 -o ~/bin/aoa
curl -L https://github.com/corey/aoa/releases/latest/download/aoa-recon-linux-amd64 -o ~/bin/aoa-recon
chmod +x ~/bin/aoa ~/bin/aoa-recon
```

Both in `~/bin/` means sibling discovery works automatically.

---

## npm Package Structure

```
npm/
  aoa/                      Wrapper: detects platform, symlinks binary
    package.json            optionalDependencies → platform packages
    install.js              Postinstall: resolve binary, create symlink
  aoa-recon/                Same pattern for recon
    package.json
    install.js
  aoa-linux-x64/            Platform package (linux, x64)
    package.json            { "os": ["linux"], "cpu": ["x64"] }
    bin/aoa                 Actual binary (populated by CI)
  aoa-linux-arm64/          ...
  aoa-darwin-x64/           ...
  aoa-darwin-arm64/         ...
  aoa-recon-linux-x64/      Platform packages for recon
  aoa-recon-linux-arm64/    ...
  aoa-recon-darwin-x64/     ...
  aoa-recon-darwin-arm64/   ...
```

npm only downloads the platform package matching the user's OS/arch. A Linux x64 user gets `@aoa/linux-x64` (~8 MB) and never downloads the darwin or arm64 packages.

---

## CI/Release Pipeline

Triggered by git tag push (`v*`):

1. **Build** — 8 matrix jobs (2 binaries x 4 platforms)
   - `aoa`: `CGO_ENABLED=0` on ubuntu/macos runners
   - `aoa-recon`: `CGO_ENABLED=1` on ubuntu/macos runners
2. **Release** — Create GitHub release with all 8 binaries + checksums
3. **Publish** — Copy binaries into npm platform packages, set version from tag, `npm publish`

```bash
# To trigger a release:
git tag v1.0.0
git push origin v1.0.0
# GitHub Actions builds, releases, and publishes to npm automatically
```

---

## Quick Reference

```bash
# Build
make build-pure              # aoa (8 MB, pure Go)
make build-recon             # aoa-recon (73 MB, CGo)
make build                   # aoa with CGo (76 MB, all-in-one)

# Test
go test ./...                # unit tests
make check                   # vet + lint + test

# Run
./aoa init                   # index project
./aoa daemon start           # start daemon + dashboard
./aoa grep <query>           # search
./aoa-recon enhance --db .aoa/aoa.db --root .   # enhance index with symbols
./aoa open                   # open dashboard in browser
./aoa daemon stop            # stop daemon
```
