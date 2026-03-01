# Changelog

All notable changes to aOa are documented here.

## v2.1.0 — First Open-Source Release

The Go rewrite of aOa, delivered as a single binary. No Docker, no Redis, no Python.

### Highlights

- **Single binary distribution** — one `aoa` binary with everything compiled in
- **28 languages** with tree-sitter structural parsing, 57 total with tokenization
- **134 semantic domains** embedded via `go:embed` — no AI calls for classification
- **GNU grep parity** — 22 of 28 flags, drop-in replacement for AI agents
- **npm distribution** — `npm install -g @mvpscale/aoa` with platform-specific binaries
- **Zero network** — no outbound connections, no telemetry, no phone-home
- **4 platform binaries** — linux/amd64, linux/arm64, darwin/amd64, darwin/arm64

### Performance vs Python aOa

| Metric | Python aOa | aOa (Go) | Improvement |
|--------|-----------|-----------|-------------|
| Search latency | 8-15ms | <0.5ms | 16-30x faster |
| Autotune | 250-600ms | ~2.5us | 100,000x faster |
| Startup | 3-8s | <200ms | 15-40x faster |
| Memory | ~390MB | <50MB | 8x reduction |
| Install | Docker + docker-compose | Single binary | Zero dependencies |
| Infrastructure | Redis + Python services | Embedded bbolt | Zero services |

### What's Included

- O(1) indexed search with inverted token map
- Self-learning system: observe, autotune (21-step), competitive displacement
- Session log tailing — learns from Claude Code sessions without hooks
- Hexagonal architecture — domain logic is dependency-free
- Unix socket daemon with JSON protocol
- HTTP dashboard (localhost-only, auto-refreshing)
- Status line generation with 25 configurable segments
- File system watcher (recursive, debounced, filtered)
- Aho-Corasick multi-pattern matching
- Comprehensive test suite (631 tests)

---

## v0.1.x — Development Releases

Development releases during the Go rewrite. These were internal milestones, not public releases.

### v0.1.10
- Auto-configure status line during `aoa init` with 25 configurable segments

### v0.1.9
- Gitignore-aware indexing, generated file detection, rule quality fixes
- Reduced recon findings 92% (49,838 to 4,149)

### v0.1.8
- Minimum method score for findings in recon handler
- Severity mapping and signal-based suppression

### v0.1.7
- Weekly grammar report generation

### v0.1.6
- Grammar validation CI workflow
- 28-language tree-sitter integration

### v0.1.5
- npm publishing pipeline with platform-specific packages
- Release workflow with 4-platform binary builds

### v0.1.4
- Session Prism: raw JSONL to canonical event decomposition
- Claude Code adapter for session log parsing

### v0.1.3
- Learner autotune with behavioral parity to Python
- Competitive displacement and bigram extraction

### v0.1.2
- Search engine: O(1) token lookup, OR/AND/regex modes
- Domain enrichment via Atlas (134 domains, 15 JSON files)

### v0.1.0
- Initial project structure
- Hexagonal architecture foundation
- bbolt persistence layer
