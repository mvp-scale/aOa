# Work Board

[Board](#board) | [Supporting Detail](#supporting-detail) | [Completed](.context/COMPLETED.md) | [Backlog](.context/BACKLOG.md)

> **Updated**: 2026-02-22 (Session 65) | **Phase**: L6 — Two-binary distribution (aoa + aoa-recon). Parser interface decoupled, pure Go binary (CGO_ENABLED=0), tokenization-only fallback, recon bridge (git-lfs model), npm packaging, CI/release pipeline. **84% complete.**
> **Completed work**: See [COMPLETED.md](.context/COMPLETED.md) — Phases 1–8c + L0 + L1 + L2 + L3.2–L3.14 + L4.1–L4.3 + L6.1–L6.10 (470+ active tests, 32 skipped)

---

## Goals

> Atomic architectural principles. Every task is evaluated against each goal independently.

| Goal | Statement |
|------|-----------|
| **G0** | **Speed** — 50-120x faster than Python. Sub-millisecond search, <200ms startup, <50MB memory. |
| **G1** | **Parity** — Zero behavioral divergence from Python. Test fixtures are the source of truth. |
| **G2** | **Single Binary** — One `aoa` binary. Zero Docker, zero runtime deps, zero install friction. |
| **G3** | **Agent-First** — Replace `grep`/`find` transparently for AI agents. Minimize prompt education tax. |
| **G4** | **Clean Architecture** — Hexagonal. Domain logic is dependency-free. External concerns behind interfaces. |
| **G5** | **Self-Learning** — Adaptive pattern recognition. observe(), autotune, competitive displacement. |
| **G6** | **Value Proof** — Surface measurable savings. Context runway, tokens saved, sessions extended. |

---

## Board Structure

> Layered architecture. Each layer builds on the one below. TDD — validation gates at every layer.

### Layers

| Layer | Name | Purpose | Gate Method |
|-------|------|---------|-------------|
| **L0** | Value Engine | Burn rate, context runway, attribution signals | Runway API returns valid projections; attribution rubric covers all tool actions |
| **L1** | Dashboard | 5-tab layout, mockup implementation, hero narratives | All 5 tabs render with live data; mockup parity validated in browser |
| **L2** | Infra Gaps | File watcher, bbolt lock, CLI flags, sub-ms content search | `aoa init` works while daemon runs; file changes trigger re-index; `aoa grep` ≤1ms |
| **L3** | Migration | Parallel run Python vs Go, parity proof | 100 queries × 5 projects = zero divergence; benchmark confirms speedup |
| **L4** | Distribution | Goreleaser, grammar loader, install docs | `go install` or binary download works on linux/darwin × amd64/arm64 |
| **L5** | Dimensional Analysis | Bitmask engine, 6-tier scanning, Recon tab | Security tier catches known vulns in test projects; query time < 10ms |
| **L6** | Distribution v2 | Two-binary split, npm packaging, zero-friction install | `npm install -g aoa` works; `npm install -g aoa-recon` lights up Recon tab |
| **L7** | Onboarding UX | First-run experience, progress feedback, expectation setting | User sees meaningful progress during startup; understands system is working |

### Columns

| Column | Purpose |
|--------|---------|
| **Layer** | Layer grouping. Links to layer detail below. |
| **ID** | Task identifier (layer.step). Links to task reference below. |
| **G0-G6** | Goal alignment. `x` = serves this goal. Blank = not relevant. |
| **Dep** | ID of blocking task, or `-` |
| **Cf** | Confidence — see indicator reference below |
| **St** | Status — see indicator reference below |
| **Va** | Validation state — see indicator reference below |
| **Task** | What we're doing |
| **Value** | Why we're doing this |
| **Va Detail** | How we prove it |

### Indicator Reference

| Indicator | Cf (Confidence) | St (Status) | Va (Validation) |
|:---------:|:----------------|:------------|:----------------|
| ⚪ | — | Not started | Not yet validated |
| 🔵 | — | In progress | — |
| 🟢 | Confident | Complete | Automated test proves it (unit or integration) |
| 🟡 | Uncertain | Pending | Partial — manual/browser only, or unit but no integration. See Va Detail for gap. |
| 🔴 | Lost/Blocked | Blocked | Failed |

> 🟢🟢🟢 = done. Task moves to COMPLETED.md.

---

## Mission

**North Star**: One binary that makes every AI agent faster by replacing slow, expensive tool calls with O(1) indexed search — and proves it with measurable savings.

**Current**: Session 65 delivered two major features: (1) **GNU grep parity** — `aoa grep`/`aoa egrep` now handle three execution modes natively (file args, stdin piping, index search), 22 of 28 GNU grep flags implemented, 100% coverage of observed AI agent usage. Piped output auto-strips ANSI. Falls back to `/usr/bin/grep` for rare flags. (2) **Recon findings peek** — dashboard finding rows are clickable, toggling between description and live source line from the in-memory FileCache (zero disk I/O). Scan freshness indicator ("scanned Xs ago"). Noise filtering: drill-down only shows findings from tiers that registered at the file level, suppressing low-severity evidence that didn't accumulate enough signal. New `/api/source-line` endpoint serves lines from cache. Session 64 replaced 28 individual upstream grammar imports with 509 grammars from [go-sitter-forest](https://github.com/alexaandru/go-sitter-forest). Session 63 delivered L6 (Two-binary distribution).

**Approach**: TDD. Each layer validated before the next. Completed work archived to keep the board focused on what's next.

**Design Decisions Locked** (Session 48):
- **aOa Score** — Deferred. Not defined until data is flowing. Intel tab cleaned of coverage/confidence/momentum card.
- **Arsenal = Value Proof Over Time** — Not a config/status page. Session-centric value evidence: actual vs counterfactual token usage, learning curve, per-session savings. System status is compact/secondary.
- **Session as unit of measurement** — Each session gets a summary record (ID, date, prompts, reads, guided ratio, tokens saved, counterfactual). Multiple sessions per day. Chart is daily rollup, table is individual sessions.
- **Counterfactual is defensible** — "Without aOa" = sum of full file sizes for guided reads + observed unguided costs. Not fabricated.

**Design Decisions Locked** (Session 49):
- **Token cost heuristic** — `bytes / 4 = estimated tokens`. Standard industry approximation (1 token ≈ 4 characters; code is ASCII so bytes ≈ characters). Used for counterfactual cost of unguided Grep/Glob and for savings calculations. Resolves open question #6.
- **Session boundary = session ID change** — Each Claude JSONL file carries a session ID. When the reader sees a new session ID, flush the summary for the previous session and start accumulating for the new one. Revisiting an old session loads its existing summary from bbolt and appends. No timeout heuristics.

**Design Decisions Locked** (Session 50):
- **Dashboard split into 3 files** — `index.html` (354 lines, HTML shell), `style.css` (552 lines, all CSS), `app.js` (1022 lines, all logic). `embed.go` uses `//go:embed static` to pick up all three. Monolithic single-file approach abandoned after agent reliability issues with large files.
- **Soft glow animation system** — Replaced all harsh flash/glow animations with CSS transition-based diffuse effects. `soft-glow` (box-shadow, 2.5s ease-out) on ngram counts; `text-glow` (text-shadow, 2.5s ease-out) on hit values and ngram names; `.lit` class on term pills triggers a CSS `transition: background/color/box-shadow 2.5s ease-out` fade. No harsh instant-on effects anywhere.
- **Domain table: no Tier column** — Removed. Table is `#`, `@Domain`, `Hits`, `Terms`. Tier is surfaced via the domain name and stats grid (Core count), not a separate column.
- **Tab state in URL hash** — `#live`, `#recon`, `#intel`, `#debrief`, `#arsenal`. Restored on page load. Direct links to tabs work.
- **Tab-aware polling** — Each tab only fetches its relevant endpoints. Live: runway + stats + activity. Intel: stats + domains + bigrams. Debrief: conv/metrics + conv/feed. Arsenal: sessions + config + runway. Recon: health only. 3s interval.

**Design Decisions Locked** (Session 51):
- **Ngram limits: 10/5/5** — Bigrams capped at 10, Cohit KW→Term at 5, Cohit Term→Domain at 5. Section spacing 24px.
- **Debrief exchange grouping** — Consecutive user+assistant turns paired into "exchanges". Turn numbering counts exchanges, not individual messages. Oldest at top, newest at bottom (scroll lands at NOW).
- **Thinking as nested sub-element** — Always visible (no click-to-expand). Indented 16px with purple left border. Each thinking chunk is a separate line (backend separates with `\n`). 11px italic font, muted color. Thinking text truncation raised to 2000 chars.
- **Markdown rendering in Debrief** — Lightweight `renderMd()`: code blocks, inline code (cyan), bold, italic, line breaks. Applied to assistant text and thinking lines.
- **Token estimates on all message types** — User: `~text.length/4` (heuristic, prefixed with `~`). Thinking: `~text.length/4`. Assistant: actual `output_tokens` from API (no prefix). No double-counting.
- **Debrief actions: Save/Tok columns** — Two fixed-width (38px) right-aligned columns per turn. `Save`: green `↓N%` when aOa guided, blank otherwise. `Tok`: green when guided (reduced cost), red when unguided (full cost), blank for productive. Column headers at turn level with hover titles.
- **Full-file reads classified as unguided** — Read with limit=0 now gets `attrib="unguided"` and `tokens=fileSize/4` from index. Previously had no classification.
- **TurnAction carries Attrib/Tokens/Savings** — Backend `TurnAction` and `TurnActionResult` now include `attrib` (string), `tokens` (int), `savings` (int). Same data flows to both Live activity and Debrief actions.

**Design Decisions Locked** (Session 52):
- **Responsive compaction pattern** — At mobile breakpoint (<800px), hero and stats cards shed text and show values only. Hero: remove support text, keep headline + value. Stats cards: value only, no labels (labels visible in desktop view). This pattern repeats across all 5 tabs. Priority is to maximize space for the main value prop content below (domain table, ngrams, conversation feed, session history). Recon is hold-state, acceptable as-is.
- **Intel mobile** — Domain rankings and ngram sections must remain readable at mobile width. Current layout crowds them below oversized hero+stats. Compaction of hero+stats solves this.

**Design Decisions Locked** (Session 55):
- **Hot file cache** — `FileCache` in `internal/domain/index/filecache.go`. Pre-reads all eligible indexed files into `[]string` lines at startup via `WarmFromIndex()`. Binary filtering: extension blocklist + null byte check + MIME detection. 512KB per-file limit, 250MB configurable budget. `GetLines()` under RLock for concurrent searches. Re-warms on `Rebuild()` (file watcher triggers). 8 tests.
- **Version display via ldflags** — `internal/version/version.go` with `Version` + `BuildDate` vars, injected by `make build` via `-ldflags -X`. `aoa --version`, daemon start/stop all show version.
- **Search observer signal pipeline (approved, not yet fully implemented)** — Two paths, same analysis: (1) CLI search: top N hits → direct-increment domains/terms already on hits, content text → `ProcessBigrams` for bigram/cohit threshold gating (count ≥ 6 before promotion). (2) Session log: Claude's grep/egrep/search tool results → same pipeline, non-blocking via tailer goroutine. Current observer patched to stop re-tokenizing content through enricher; full pipeline refactor is next.

**Design Decisions Locked** (Session 54):
- **Attribution color system** — Three visual lanes: green = aOa guided (savings), red = unguided (cost), purple = creation. Unguided entries show red across attrib pill, impact text, and target text. Guided entries show green impact (savings display). No time claims on unguided entries — only token cost stated factually; red color communicates "not optimized" without accusing.
- **Creative words for Write/Edit** — Replaced static "productive" with cycling vocabulary: crafted, authored, forged, innovated. Purple pill, distinct from green (guided) and red (unguided). Each Write/Edit gets one word anchored to that entry; next creation may get a different word. Celebrates the creation moment.
- **Time savings: celebrate success, don't kick failure** — Time savings only shown on guided reads (provable counterfactual). Unguided reads show token cost only (no time claim). Cumulative savings tell the positive story over time. aOa doesn't claim value for net-new creation (Write/Edit).
- **Time savings algorithm (approved, not yet implemented)** — Two paths: simple (`tokens_saved * 0.0075s`) as fallback, dynamic (compute `ms/token` from tailer's `DurationMs` + `OutputTokens`, P25 percentile across recency windows, produce a range). Go has infrastructure advantage: tailer already extracts duration and token data in real-time, no separate JSONL re-parse needed.
- **Glob cost estimation uses `filepath.Match`** — Rewrote `estimateGlobCost`/`estimateGlobTokens` to accept `(dir, pattern)`. Uses `filepath.Match` for glob patterns against relative index paths, falls back to `HasPrefix` for directory-only lookups.

**Design Decisions Locked** (Session 56):
- **Trigram index: one lowered index serves both case modes** — Build trigram posting lists from lowercased lines only (~11MB). Case-sensitive search (grep default): trigram lookup with lowered query → verify candidates with `strings.Contains(originalLine, query)`. Case-insensitive (`-i`): same trigram lookup → verify with `strings.Contains(lowerLine, lowerQuery)`. One index, two verification functions. False positives possible at candidate stage (case-insensitive index over-selects for case-sensitive queries), false negatives impossible. Regex: extract literal segments, trigram those, verify with full regex. No extractable literals → brute-force fallback.
- **grep case-sensitivity parity (G1)** — `aoa grep` must be case-sensitive by default, matching Unix grep. `-i` flag enables case-insensitive. Current `buildContentMatcher` default path uses `containsFold` (always case-insensitive) — this is a G1 violation that must be fixed as part of L2.5 wiring. Symbol search already case-insensitive by design (token index is lowered); content search must respect the flag.
- **Trigram memory budget** — 17MB additional on top of existing 8MB cache (6MB original lines + 2MB token index). Breakdown: 6MB lowered lines + 11MB trigram posting lists = 17MB. Total after: ~25MB. 250MB budget has 225MB headroom. Scales linearly with corpus size. 500-file project ≈ 35MB total.

**Design Decisions Locked** (Session 60):
- **Learned column (not Learn action rows)** — Learning signal moved from a separate activity row into a dedicated `Learned` column between Impact and Target. Green pill, cycling AI vocabulary (trained/fine-tuned/calibrated/converged/reinforced/optimized/weighted/adapted). Empty when no learning. Cleaner feed, no row doubling. Session-log grep signals retain separate Learn entries (different signal path, not paired with a user action row).
- **Activity table color system (finalized)** — Source: `aOa` = blue, Claude = dim. Attrib pills: `indexed` = cyan, `regex`/`multi-and`/`multi-or` = yellow, `aOa guided` = green, `unguided` = red, creative words = purple, learn words = green. Impact: numbers in cyan, text in gray (via `highlightNumbers()`). Target: `aOa` blue, `grep`/`egrep` green, rest gray. Learned: green pill. Column widths: Target gets 39% (longest content).
- **L3.6–L3.14 parity complete** — 100% agent-critical (15/15), 96% overall (23/24). Remaining 1 gap (`-x`/`--line-regexp`) is not agent-critical. `searchTarget()` reconstructs full command with all new flags for activity feed display.

**Design Decisions Locked** (Session 61):
- **Build tags for lean binary** — `//go:build !lean` (default, all grammars) vs `//go:build lean` (empty `registerBuiltinLanguages()`). Clean split: `extensions.go` (always), `languages_builtin.go` (`!lean`), `languages_lean.go` (`lean`). No runtime cost for default builds.
- **Purego over Go plugin** — `github.com/ebitengine/purego` for `Dlopen`/`RegisterLibFunc`. No Go version matching needed. Works CGo-free on Linux/macOS. The `tree_sitter_{lang}()` C function returns `uintptr` via purego, converted to `unsafe.Pointer` for `tree_sitter.NewLanguage()`.
- **Grammar search path: project-local first** — `.aoa/grammars/` (per-project) takes priority over `~/.aoa/grammars/` (global). Same grammar `.so` can be shared globally or overridden per-project.
- **Graceful degradation on dynamic load failure** — `ParseFile()` returns `nil, nil` if grammar `.so` not found. Same behavior as extension-mapped-but-no-grammar today. No errors surfaced to user — just falls back to tokenization-only indexing.
- **57-language manifest with priority tiers** — `BuiltinManifest()` embedded in binary. P1 Core (11 langs every dev uses), P2 Common (11 langs most devs use), P3 Extended (17 niche but real), P4 Specialist (18 domain-specific). `aoa grammar list` shows tier/status at a glance.
- **Individual .so files, not regional bundles** — Simpler to build, simpler to download individually. Pack tarballs (core/common/extended/specialist) for bulk download.

**Design Decisions Locked** (Session 65):
- **GNU grep three-route architecture** — `runGrep()` routes: (1) file args present → `grepFiles()` native Go line filter, (2) stdin is pipe → `grepStdin()` native Go filter, (3) neither → daemon index search, fallback to `/usr/bin/grep` if daemon down. File args take priority over stdin detection. 22 flags implemented natively. `grepExit{code}` type propagates GNU exit codes (0/1/2) through cobra. Output auto-switches to `file:line:content` format when stdout is not a TTY.
- **Grep parity coverage model** — 22 of 28 GNU grep flags native (100% of observed AI agent usage). Remaining 6 flags (`-P`, `-x`, `-f`, `-b`, `-Z`, `-R`) never seen in agent session logs (<4% of flag surface). System grep fallback covers edge cases. README documents alignment honestly: "100% of AI agent use cases tested, <4% flag surface forwarded to system grep."
- **Recon findings peek = view toggle, not expand** — Click a finding row: description text swaps to live source line from FileCache. Click again: swaps back. No persistence, no disk I/O. `/api/source-line?file=X&line=12&context=0` resolves file path to ID, reads from in-memory cache. Finding row layout reordered: line number left, severity, rule ID, then toggleable desc/code.
- **Recon tier noise gating** — File-level drill-down only shows findings from tiers that registered > 0 at the file level. If PERF=0 at the file row, performance findings are suppressed in the method view. Individual low-severity warnings are evidence; only surface them when enough accumulate to register as a real concern. `reconAggregateFile()` computes active tiers, `elevatedFindings` filter applied before grouping by symbol.
- **Scan freshness indicator** — `/api/recon` response includes `scanned_at` (unix timestamp). Hero support line shows "scanned just now" / "12s ago" / "3m ago". Sets user expectation that data reflects the current cache state.

**Design Decisions Locked** (Session 64):
- **go-sitter-forest sub-package imports** — Import each forest sub-package directly (e.g., `go-sitter-forest/python`), NOT the root `forest.go`. Root depends on `go-tree-sitter-bare` (different bindings), causing duplicate C runtime symbols at link time. Sub-packages export `GetLanguage() unsafe.Pointer` with zero Go dependencies — just CGo to compile `parser.c`. Wrapped with `tree_sitter.NewLanguage()` from official bindings.
- **Code generator for grammar registration** — `gen_forest.go` (`//go:build ignore`) scans `_tmp_sitter_forest/go-sitter-forest-main/*/binding.go`, extracts package names, generates `languages_forest.go` with 509 imports and `registerBuiltinLanguages()`. Uniform `forest_` alias prefix avoids keyword conflicts (`go`, `func`), stdlib conflicts (`context`), and package name mismatches (`Go`, `ConTeXt`, `FunC`).
- **Local replace directives** — go.mod uses `replace` directives pointing to `_tmp_sitter_forest/go-sitter-forest-main/{lang}` for all 509 sub-packages. Each sub-package has its own `go.mod` with no external deps. Switch to published versions when ready to release.
- **Language name = directory name** — `c_sharp` not `csharp`, matching forest directory. Default C symbol derivation (`tree_sitter_c_sharp`) works without overrides. Extension mappings, symbol rules, and manifest updated accordingly.
- **fastbuild skipped** — Generate never completes (15min+). 6 other broken parsers (cfhtml, cfml, cpp2, enforce, rst, zsh) have no `binding.go` and are automatically excluded.

**Design Decisions Locked** (Session 63):
- **Two-binary distribution** — `aoa` (pure Go, CGO_ENABLED=0, ~8 MB) for search + learning + dashboard. `aoa-recon` (CGo, ~80 MB) for tree-sitter parsing + security scanning. Users who want fast search get a tiny binary; users who want scanning install the companion. npm packages for zero-friction install.
- **`cgo` build constraint for parser switching** — `parser_cgo.go` (CGO_ENABLED=1) returns `treesitter.NewParser()`, `parser_nocgo.go` (CGO_ENABLED=0) returns nil. Clean compile-time switching without custom tags. Same pattern for `grammar_cgo.go`/`grammar_nocgo.go`.
- **git-lfs discovery model** — `aoa` discovers `aoa-recon` via `exec.LookPath` → `.aoa/bin/` → sibling binary. Zero configuration. Auto-triggers `enhance` after reindex when parser is nil. Non-blocking (goroutine). Graceful skip when not found.
- **Scanner extracted to shared package** — `internal/adapters/recon/scanner.go` with `recon.Scan(idx, lines)` and `LineGetter` interface. Both web handler and aoa-recon CLI call the same scanning logic. No code duplication.
- **npm esbuild/turbo pattern** — Wrapper packages (`aoa`, `aoa-recon`) with `optionalDependencies` pointing to platform-specific packages (`@aoa/linux-x64`, etc.) with `os`/`cpu` constraints. JS postinstall shim resolves platform binary and creates symlink. npm only downloads the matching platform package.

**Design Decisions Locked** (Session 62):
- **Recon interim scanner** — Pattern-based text scanner using `strings.Contains`/regex on FileCache lines. Not the full bitmask engine (L5.1-L5.5). Demonstrates the UX and proves the data pipeline works. 10 pattern types: `hardcoded_secret`, `command_injection`, `weak_hash`, `insecure_random`, `defer_in_loop`, `ignored_error`, `panic_in_lib`, `print_statement`, `todo_fixme`, `global_state`, plus `long_function` from symbol metadata.
- **Code-only file filtering** — Patterns marked `codeOnly: true` skip non-code files (.md, .json, .html, .yaml, .txt). `todo_fixme` scans all files. Prevents 45% false positive rate from documentation and mockup files.
- **Dashboard time formatting** — `fmtTime(ms)` and `fmtMin(min)` now scale: `≥1d` → `Xd Yh`, `≥1h` → `Xh Ym`, `≥1m` → `Xm Ys`, `≥1s` → `X.Ys`, else `Xms`. Previously capped at minutes.
- **CacheHitRate as fraction** — `SessionMetrics.CacheHitRate()` returns 0.0–1.0 (matching `ports.TokenUsage.CacheHitRate()`). JS `fmtPct()` multiplies by 100. Previously returned percentage (87.0), causing 8700% display.

**Design Decision Locked** (Session 62 — Architecture):
- **Two-binary distribution: aoa + aoa-recon** — Clean separation of concerns. `aoa` ships lean (12 MB): search, indexing (tokenization-only), learning, dashboard (all 5 tabs), context savings. No tree-sitter grammars needed. Recon tab shows "install aoa-recon" prompt when scanner not detected. `aoa-recon` ships separately (~350 MB): all 490 languages via [go-sitter-forest](https://github.com/alexaandru/go-sitter-forest), YAML-driven bitmask engine, AC automaton, AST structural walker, per-method scoring. When installed, it lights up the Recon tab and enhances the search index with symbol-level results (function names, signatures, line ranges). Connection mechanism TBD (sidecar binary on PATH or `.aoa/bin/`, shared bbolt database, or Unix socket IPC).
- **aoa-recon uses go-sitter-forest** — Single dependency for 490+ tree-sitter grammars with Go bindings, uniform API, MIT licensed, daily upstream updates. Eliminates the 57-grammar manifest, the dynamic `.so` loader, and the `aoa grammar install` step. Users who want recon get everything. 350 MB binary is smaller than VS Code (400 MB), Chrome (500 MB), or a typical `node_modules`. Binary size has zero impact on runtime performance — only the grammar for the file being parsed is loaded into memory.
- **Universal lang_map ships in aoa-recon** — All 490 languages mapped to the ~10 unified AST concepts (call, assignment, if, for, return, etc.). YAML rules reference unified concepts, lang_map resolves per language. One rule works across all languages. Adding a new language = adding 10 entries to the map (data, not code).
- **Three-tier rule engine** — `engine: text` (AC only, fires on ALL files including .env/.ini/.xml/unknown), `engine: structural` (AST walker, fires where grammar exists), `engine: text+structural` (AC finds candidate, AST confirms — degrades gracefully to text-only with lower confidence where no grammar available). Eliminates the gap where config files / dotenv / unknown formats miss basic secret detection.
- **Lean build tag stays** — `go build -tags lean` still produces the 12 MB binary for `go install` from source (avoids CGo compilation). This IS the default `aoa` distribution binary. The full/fat build path moves to `aoa-recon`.

**Known Issues / UX Gaps** (Session 62):
- **Debrief tab: markdown rendering** — Assistant thinking and user messages appear to be truncated. Markdown tables, code blocks, and other structured content don't render properly in the conversation view. Need to parse markdown into HTML (at minimum: tables, code fences, inline code, bold/italic, lists, headers) instead of displaying raw text. Current coloring is good — just missing the formatting pass.
- **Actions tab: web search/fetch token costs** — Web search and web fetch activities show data size (~200 KB download) but don't display associated token costs. These API calls consume tokens and should be accounted for in the savings metrics. Need to capture token usage from web search/fetch events in the JSONL session log and surface it in the actions table.
- **Actions tab: agent/subagent activity** — When Claude Code spawns subagents (Task tool), the actions table doesn't reflect their activity. Should at minimum show a summary row for agent invocations (agent type, task description, duration, token cost) rather than listing every individual tool call the agent makes — otherwise the table gets too noisy.

**Known Issues / UX Gaps** (Session 65):
- **Startup progress feedback** — Daemon startup takes 20-30s on large projects (tree-sitter indexing + recon enhance). During this time the user sees nothing — just a spinner with no indication of progress. `aoa daemon restart` can timeout waiting for readiness. Need: file count progress, "indexing N files...", "enhancing...", percent complete. This is L7.1.
- **Finding ignore/dismiss** — Recon findings can't be suppressed. Users see false positives (e.g., scanner flagging its own rule definitions for `password=` patterns) with no way to mark them as reviewed. Future: `.aoa/ignore` file with rules like `hardcoded_secret:rules_security.go`, dashboard dismiss button writes to it.

**Resolved Discussion Items**:
- **Alias strategy** — Resolved (Session 65). `aoa grep`/`aoa egrep` now handle file args, stdin piping, and index search natively. 22 GNU grep flags implemented. Falls back to `/usr/bin/grep` for unrecognized flags. Shims in `~/.aoa/shims/` replace grep transparently for AI agents.
- **Real-time conversation** — Resolved (Session 58). Debrief tab now polls at 1s (vs 3s for other tabs). Auto-scroll sticks to bottom when user is near the live edge. Floating "Now ↓" button for jump-back after scrolling up. Thinking text appears within ~1s of generation.

---

## Board

| Layer | ID | G0 | G1 | G2 | G3 | G4 | G5 | G6 | Dep | Cf | St | Va | Task | Value | Va Detail |
|:------|:---|:--:|:--:|:--:|:--:|:--:|:--:|:--:|:----|:--:|:--:|:--:|:-----|:------|:----------|
| [L0](#layer-0) | [L0.1](#l01) | x | | | | | | x | - | 🟢 | 🟢 | 🟢 | Burn rate accumulator — rolling window tokens/min | Foundation for all savings metrics | Unit: `burnrate_test.go` — 6 tests (empty, single, multi, eviction, partial, reset) |
| [L0](#layer-0) | [L0.2](#l02) | | | | | | | x | - | 🟢 | 🟢 | 🟢 | Context window max lookup — map model tag to window size | Needed for runway projection | Unit: `models_test.go` — 2 tests (known models, unknown default) |
| [L0](#layer-0) | [L0.3](#l03) | | | | | | | x | L0.1 | 🟢 | 🟢 | 🟢 | Dual projection — with-aOa vs without-aOa burn rates | The core value comparison | Unit: `runway_test.go` — 3 tests (divergence under load, zero-rate edge, model lookup) |
| [L0](#layer-0) | [L0.4](#l04) | | | | | x | | x | L0.3 | 🟢 | 🟢 | 🟢 | Context runway API — `/api/runway` with both projections | Dashboard and CLI can show runway | Unit: `runway_test.go` + HTTP: `web/server_test.go` TestRunwayEndpoint — JSON shape with both projections |
| [L0](#layer-0) | [L0.5](#l05) | | | | | x | | x | L0.3 | 🟢 | 🟢 | 🟢 | Session summary persistence — per-session metrics in bbolt | Arsenal value proof, survives restart | Unit: `bbolt/store_test.go` (4 tests: save/load/list/overwrite) + `session_test.go` (5 tests: boundary detect, flush, restore) |
| [L0](#layer-0) | [L0.6](#l06) | | | | | | x | x | - | 🟢 | 🟢 | 🟢 | Verify autotune fires every 50 prompts | Trust the learning cycle is working | Unit: `autotune_integration_test.go` — 50 searches → autotune triggers, activity entry emitted |
| [L0](#layer-0) | [L0.7](#l07) | | | | | | x | x | L0.6 | 🟢 | 🟢 | 🟢 | Autotune activity event — "cycle N, +P/-D/~X" | Visible learning progress in activity feed | Unit: `autotune_integration_test.go` — asserts activity entry with promote/demote/decay counts |
| [L0](#layer-0) | [L0.8](#l08) | | | | | | | x | - | 🟢 | 🟢 | 🟢 | Write/Edit creative attrib (purple) | Celebrate creation moments | Unit: `activity_test.go` TestWriteEditCreative + TestCreativeWordCycles — cycling crafted/authored/forged/innovated |
| [L0](#layer-0) | [L0.9](#l09) | | | | | | | x | - | 🟢 | 🟢 | 🟢 | Glob attrib = "unguided" + estimated token cost | Show cost of not using aOa | Unit: `activity_test.go` TestGlobAttrib — Glob events tagged `attrib="unguided"`, impact contains token estimate |
| [L0](#layer-0) | [L0.10](#l010) | | | | | | | x | - | 🟢 | 🟢 | 🟢 | Grep (Claude) impact = estimated token cost | Show cost of not using aOa | Unit: `activity_test.go` TestGrepImpact — Claude Grep events show `~Ntok` in impact |
| [L0](#layer-0) | [L0.11](#l011) | | | | | | x | | - | 🟢 | 🟢 | 🟢 | Learn activity event — observe signals summary | Visible learning in feed | Unit: `activity_test.go` TestLearnEvent + `autotune_integration_test.go` — entry contains "+N keywords, +M terms" |
| [L0](#layer-0) | [L0.12](#l012) | | | | | | | x | - | 🟢 | 🟢 | 🟢 | Target capture — preserve full query syntax, no normalization | Accurate activity display | Unit: `activity_test.go` TestTargetCapture — all-flags, regex+boundary, simple query preserved verbatim |
| [L1](#layer-1) | [L1.1](#l11) | | | | | | | x | - | 🟢 | 🟢 | 🟢 | Rename tabs: Overview→Live, Learning→Intel, Conversation→Debrief | Brand alignment — tabs named by user intent | Manual: user confirmed (Session 52) — Live/Recon/Intel/Debrief/Arsenal tabs visible in browser |
| [L1](#layer-1) | [L1.2](#l12) | | | | | | | x | L1.1 | 🟢 | 🟢 | 🟢 | Add Recon tab (stub) — dimensional placeholder | Reserve the tab slot for v2 | Manual: user confirmed (Session 52) — `#recon` tab renders hero + placeholder stub |
| [L1](#layer-1) | [L1.3](#l13) | | | | | | | x | L1.1 | 🟢 | 🟢 | 🟢 | Add Arsenal tab — value proof over time, session history, savings chart | Prove aOa's ROI across sessions | Manual: user confirmed (Session 52) — `#arsenal` tab visible, framing up. Backend: `web/server_test.go` TestSessionsEndpoint |
| [L1](#layer-1) | [L1.4](#l14) | | | | | | | | L1.1 | 🟢 | 🟢 | 🟢 | 5-tab responsive layout — mobile compaction at <=520px | Works on all screen sizes | Manual: user confirmed (Session 53) — hero card hidden, stats as value-only chips, glow consistent across breakpoints. CSS: `style.css` @media 520px block. Mockups validated first in `docs/mockups/`. |
| [L1](#layer-1) | [L1.5](#l15) | | | | | x | | x | L0.5, L1.3 | 🟢 | 🟢 | 🟢 | Arsenal API — `/api/sessions` + `/api/config` | Backend for Arsenal charts and system strip | Unit: `web/server_test.go` TestSessionsEndpoint + TestConfigEndpoint — JSON shape validated |
| [L1](#layer-1) | [L1.6](#l16) | x | | | | | | x | L0.4 | 🟢 | 🟢 | 🟢 | Live tab hero — context runway as primary display | Lead with the value prop | Manual: user confirmed (Session 53) — hero + metrics panel render correctly with data, consistent across responsive breakpoints. Backend: `runway_test.go`. |
| [L1](#layer-1) | [L1.7](#l17) | | | | | | | x | L0.4 | 🟢 | 🟢 | 🟢 | Live tab metrics panel — savings-oriented cards | Replace vanity metrics with value | Manual: user confirmed (Session 53) — 6 stats cards render, glow effect consistent across breakpoints. Backend: API tests. |
| [L1](#layer-1) | [L1.8](#l18) | | | | | | | x | L0.9, L0.10 | 🟢 | 🟢 | 🟢 | Dashboard: render token cost for unguided Read/Grep/Glob | Show unguided cost inline | Pipeline proven live (Session 53): tailer→activity→API→dashboard confirmed with guided Read showing `↓99% (12.1k → 100)`. Unit: `activity_test.go` TestActivityReadNoSavings (full-file Read shows `~500 tokens`), TestActivityRubric rows 7/10/11 (Read/Grep/Glob all show `~N tokens`). Fixed: full-file Read impact was `-`, now shows estimated cost. |
| [L2](#layer-2) | [L2.1](#l21) | x | | | | x | x | | - | 🟢 | 🟢 | 🟡 | Wire file watcher — `Watch()` in app.go, change→reparse→reindex | Dynamic re-indexing without restart | Unit: `watcher_test.go` (4 tests: new/modify/delete/unsupported call `onFileChanged` directly) + `rebuild_test.go` (1 test). **Gap**: no integration test through fsnotify event pipeline — needs `TestDaemon_FileWatcher_ReindexOnEdit` |
| [L2](#layer-2) | [L2.2](#l22) | x | | | | x | | | - | 🟢 | 🟢 | 🟢 | Fix bbolt lock contention — in-process reindex via socket command | `aoa init` works while daemon runs | Integration: `TestInit_DaemonRunning_DelegatesToReindex` (init succeeds via socket), `TestInit_DaemonRunning_ReportsStats` (output has file/symbol/token counts), `TestInit_DaemonRunning_ThenSearchFindsNewSymbol` (new file found after reindex). Unit: `indexer_test.go` (3), `reindex_test.go` (1) |
| [L2](#layer-2) | [L2.3](#l23) | | x | | x | | | | - | 🟢 | 🟢 | 🟢 | Implement `--invert-match` / `-v` flag for grep/egrep | Complete grep flag parity | Integration: `TestGrep_InvertMatch` (CLI `-v` via daemon), `TestGrep_InvertMatch_CountOnly` (`-v -c`), `TestEgrep_InvertMatch` (`-v` regex). Unit: `invert_test.go` (8 tests: literal/regex/OR/AND/content/count/quiet/glob) |
| [L2](#layer-2) | [L2.4](#l24) | x | | | | | | | - | 🟢 | 🟢 | 🟢 | Trigram index in FileCache — one lowered index, dual-mode verify | Sub-ms content search foundation (74K→~50 candidates), +17MB | Unit: 9 tests — trigram build, lookup, intersection, substring match, dedup, case verify, rewarm |
| [L2](#layer-2) | [L2.5](#l25) | x | x | | x | | | | L2.4 | 🟢 | 🟢 | 🟢 | Wire trigram + fix case-sensitivity default (G1) | `aoa grep` ≤1ms, case-sensitive by default, `-i` for insensitive | Unit: 7 tests — case-sensitive default, `-i` flag, trigram dispatch, extractTrigrams, canUseTrigram. Benchmark: ~60µs/query on 500 files |
| [L2](#layer-2) | [L2.6](#l26) | x | | | | | | | L2.4 | 🟢 | 🟢 | 🟢 | Pre-lowercased line cache — lowerLines in cacheEntry | Faster brute-force fallback for <3 char queries and regex | Wired: brute-force uses `strings.Contains` on pre-lowered lines for `-i` mode. Benchmark: equivalent perf |
| [L2](#layer-2) | [L2.7](#l27) | x | x | | x | | | | L2.5 | 🟢 | 🟢 | 🟢 | Edge cases + regression suite — short queries, regex, InvertMatch, AND | All grep/egrep modes work at speed | Unit: 7 tests — short query fallback, regex, word boundary, AND, InvertMatch, glob filter. All 374 tests pass |
| [L3](#layer-3) | [L3.2](#l32) | | x | | | | | | - | 🟢 | 🟢 | 🟢 | Grep/egrep parity: 55 tests, 93% agent-critical, 58% overall | Search parity proof | `test/migration/grep_parity_test.go` — 55 tests. Agent-critical: 14/15 (93%). Overall: 14/24 (58%). L3.6-L3.14 close gaps to 100%/96%. |
| [L3](#layer-3) | [L3.3](#l33) | | x | | | | | | - | 🟢 | 🟢 | 🟢 | Learner parity: 200 intents, 5 checkpoints, zero tolerance | Learner parity proof | `TestAutotuneParity_FullReplay` — 200-intent replay, all fields match |
| [L3](#layer-3) | [L3.4](#l34) | x | | | | | | | - | 🟢 | 🟢 | 🟢 | 7 benchmarks: search 59µs, autotune 24µs, startup 8ms, 0.4MB | Confirm speedup targets | `test/benchmark_test.go` — all 7 passing, all targets exceeded |
| [L3](#layer-3) | [L3.5](#l35) | | | x | | | | | - | 🟢 | 🟢 | 🟢 | `docs/migration.md` — install, migrate, verify, rollback | Clean upgrade path | Step-by-step guide with flag reference and perf comparison |
| [L3](#layer-3) | [L3.6](#l36) | | x | | x | | | | - | 🟢 | 🟢 | 🟢 | egrep `-i` — case-insensitive regex | Agent-critical gap: 93%→100% | `TestEgrep_Flag_i`, `TestEgrep_Flag_i_Regex` — case-insensitive mode wired, parity with grep `-i` |
| [L3](#layer-3) | [L3.7](#l37) | | x | | x | | | | - | 🟢 | 🟢 | 🟢 | `-A/-B/-C` context lines for grep/egrep | Agents use `-A 3` for context | `TestGrep_Flag_A/B/C`, `TestGrep_Combo_A_B`, `TestGrep_Flag_C_OverridesAB` — `attachContextLines()` from FileCache; nil for symbol-only hits |
| [L3](#layer-3) | [L3.8](#l38) | | x | | x | | | | - | 🟢 | 🟢 | 🟢 | `--exclude-dir` glob for directory exclusion | More precise than `--exclude` | `TestGrep_Flag_excludeDir`, `TestEgrep_Flag_excludeDir`, `TestGrep_Flag_excludeDir_Nested` — `matchesAllGlobs()` checks `filepath.Dir(path)` |
| [L3](#layer-3) | [L3.9](#l39) | | x | | x | | | | - | 🟢 | 🟢 | 🟢 | `-o` / `--only-matching` — print matching part only | Pipeline extraction use case | `TestGrep_Flag_o`, `TestEgrep_Flag_o` — `extractMatch()` for literal/regex/case-insensitive modes |
| [L3](#layer-3) | [L3.10](#l310) | | x | | x | | | | - | 🟢 | 🟢 | 🟢 | egrep `-w` — word boundary regex | Parity with grep `-w` | `TestEgrep_Flag_w` — `\b`-wrapping in `searchRegex()` when `WordBoundary` set |
| [L3](#layer-3) | [L3.11](#l311) | | x | | x | | | | - | 🟢 | 🟢 | 🟢 | `-L` / `--files-without-match` | Inverse of `-l` | `TestGrep_Flag_L`, `TestGrep_Flag_L_NoResults` — `filesWithoutMatch()` respects glob filters |
| [L3](#layer-3) | [L3.12](#l312) | | x | | x | | | | - | 🟢 | 🟢 | 🟢 | `--no-filename` — suppress filename | Script/pipeline mode | `TestGrep_Flag_noFilename` — output-only flag, omits file prefix in `formatSearchResult()` |
| [L3](#layer-3) | [L3.13](#l313) | | x | | x | | | | - | 🟢 | 🟢 | 🟢 | `--no-color` | Pipeline mode (no ANSI) | `TestGrep_Flag_noColor` — all ANSI constants empty when set |
| [L3](#layer-3) | [L3.14](#l314) | | x | | x | | | | - | 🟢 | 🟢 | 🟢 | egrep `-a` / `--and` — AND mode for regex | Parity with grep `-a` | `TestEgrep_Flag_a_ANDMode` — `AndMode: true` wired to egrep `-a` flag |
| [L4](#layer-4) | [L4.1](#l41) | | | x | | | | | - | 🟢 | 🟢 | 🟢 | Purego .so loader for runtime grammar loading | Extend language coverage without recompile | E2E: compile Python .so, load via purego, parse → identical to compiled-in. 18 unit + 1 E2E test |
| [L4](#layer-4) | [L4.2](#l42) | | | x | | | | | L4.1 | 🟡 | 🟢 | 🟡 | Grammar CLI + build CI — `aoa grammar list/install/info`, manifest, CI workflow | Easy grammar distribution | `aoa grammar list` shows 57 langs by tier. CI workflow defined. **Gap**: actual download not yet implemented |
| [L4](#layer-4) | [L4.3](#l43) | | | x | | | | | L4.1 | 🟢 | 🟢 | 🟢 | Lean binary via build tags + goreleaser config | 80MB→12MB binary, cross-platform | `go build -tags lean` produces 12MB binary. Release workflow defined |
| [L4](#layer-4) | [L4.4](#l44) | | | x | x | | | | L4.3 | 🟢 | ⚪ | ⚪ | Installation docs — `go install` or download binary | Friction-free onboarding | New user installs and runs in <2 minutes |
| [L5](#layer-5) | [L5.1](#l51) | | | | | x | | | - | 🟢 | 🟢 | 🟢 | Rule schema as Go structs — Bitmask, Rule, Severity, FileAnalysis types | Foundation for all dimensional patterns | Unit: `types_test.go` — 7 tests (Set/Has/Or/PopCount, tier isolation, overflow guard) |
| [L5](#layer-5) | [L5.2](#l52) | x | | | | x | | | L5.1 | 🟢 | 🟢 | 🟢 | Tree-sitter AST walker — structural pattern matching on parsed AST | Structural detection engine | Unit: `walker_test.go` — 9 tests (defer_in_loop, ignored_error, panic, sql_concat, clean_code) |
| [L5](#layer-5) | [L5.3](#l53) | x | | | | x | | | L5.1 | 🟢 | 🟢 | 🟢 | AC text scanner — Aho-Corasick DFA, byte-offset matching | Text detection engine | Unit: `matcher_test.go` — 15 tests + 2 benchmarks |
| [L5](#layer-5) | [L5.4](#l54) | | | | | x | | | L5.2 | 🟢 | 🟢 | 🟢 | Language mapping layer — 10 concepts × 10 languages | Cross-language uniformity | Unit: `lang_map_test.go` — 11 tests (Go/Python/JS resolve, unknown fallback) |
| [L5](#layer-5) | [L5.5](#l55) | x | | | | x | | | L5.2, L5.3 | 🟢 | 🟢 | 🟢 | Bitmask composer — per-method attribution + weighted severity scoring | The scoring engine | Unit: `score_test.go` — 6 tests (scoring, method attribution, dedup) |
| [L5](#layer-5) | [L5.6](#l56) | | | | | | | | L5.1 | 🟢 | 🟢 | 🟢 | Security tier — 35 rules (20 text, 7 structural, 5 composite) | First dimensional tier | Unit: `rules_security_test.go` — 7 tests. Integration: `TestEngine_MultiLanguage` |
| [L5](#layer-5) | [L5.7](#l57) | | | | | | | | L5.1 | 🟡 | ⚪ | ⚪ | Performance tier — 4 dims (queries, memory, concurrency, resource leaks) | Second tier | Flags N+1, unbounded allocs |
| [L5](#layer-5) | [L5.8](#l58) | | | | | | | | L5.1 | 🟡 | ⚪ | ⚪ | Quality tier — 4 dims (complexity, error handling, dead code, conventions) | Third tier | God functions, ignored errors |
| [L5](#layer-5) | [L5.9](#l59) | | | | | | | | L5.5 | 🟢 | 🟢 | 🟢 | Wire analyzer into aoa-recon enhance — AC+AST scan, persist to bbolt | Connect engine to pipeline | Unit: `engine_test.go` — 13 tests. bbolt: `store_test.go` — 4 dimension tests |
| [L5](#layer-5) | [L5.10](#l510) | | | | x | | | | L5.5 | 🟢 | ⚪ | ⚪ | Add dimension scores to search results (`S:-1 P:0 C:+2`) | Scores visible inline | Scores appear in grep/egrep output |
| [L5](#layer-5) | [L5.11](#l511) | | | | x | | | | L5.5 | 🟢 | ⚪ | ⚪ | Dimension query support — `--dimension=security --risk=high` | Filter by dimension | CLI filters by tier and severity |
| [L5](#layer-5) | [L5.12](#l512) | | | | | | | x | L5.9 | 🟢 | 🟢 | 🟡 | Recon tab — dimensional engine with interim scanner fallback | Dashboard dimensional view | API: backward-compatible JSON. **Gap**: dashboard UI upgrade for bitmask scores |
| [L6](#layer-6) | [L6.1](#l61) | | | x | | x | | | - | 🟢 | 🟢 | 🟢 | Parser interface in ports — decouple app from concrete treesitter type | Clean architecture: domain doesn't depend on adapter | `ports.Parser` interface accepted everywhere; nil = no parser |
| [L6](#layer-6) | [L6.2](#l62) | | | x | | x | | | L6.1 | 🟢 | 🟢 | 🟢 | Make App.Parser optional — nil-guard indexer, watcher, init | Core aoa works without any parser | `CGO_ENABLED=0 go build ./cmd/aoa/` produces 7.5 MB binary; `go test ./...` passes |
| [L6](#layer-6) | [L6.3](#l63) | | | x | | | | | L6.2 | 🟢 | 🟢 | 🟢 | Tokenization-only indexer fallback — search works without symbols | File-level search with zero CGo | `TestBuildIndex_NilParser_TokenizationOnly` — 2 files, 0 symbols, tokens from content |
| [L6](#layer-6) | [L6.4](#l64) | | | | | x | | | L6.2 | 🟢 | 🟢 | 🟢 | Create `cmd/aoa-recon/` entry point — enhance + enhance-file subcommands | Separate binary owns all tree-sitter + scanning | `go build ./cmd/aoa-recon/` compiles (73 MB); enhance/enhance-file/version commands |
| [L6](#layer-6) | [L6.5](#l65) | | | | | x | | | L6.4 | 🟢 | 🟢 | 🟢 | Extract scanner to shared package `internal/adapters/recon/` | Both web handler and recon binary can call scanner | `recon.Scan()` callable from web API and CLI; no code duplication |
| [L6](#layer-6) | [L6.6](#l66) | | | x | x | | | | L6.4 | 🟢 | 🟢 | 🟢 | Recon bridge — aoa detects aoa-recon on PATH, invokes as subprocess | git-lfs model: zero config, auto-discovery | `ReconBridge` discovery: PATH → .aoa/bin/ → sibling; TriggerReconEnhance after reindex |
| [L6](#layer-6) | [L6.7](#l67) | | | | | | | x | L6.6 | 🟢 | 🟢 | 🟡 | Dashboard Recon tab install prompt — show "npm install aoa-recon" when not detected | Users know exactly what to do to unlock Recon | `recon_available` field in API; JS renders install prompt when false; "lite mode" indicator |
| [L6](#layer-6) | [L6.8](#l68) | | | x | | | | | L6.2 | 🟢 | 🟢 | 🟡 | npm package structure — wrapper packages + platform packages + JS shims | Zero-friction install via npm | `npm/aoa/` + `npm/aoa-{platform}/` with postinstall shim; esbuild/turbo pattern |
| [L6](#layer-6) | [L6.9](#l69) | | | x | | | | | L6.4, L6.8 | 🟢 | 🟢 | 🟡 | npm recon package structure — wrapper + platform packages for aoa-recon | Zero-friction recon install | `npm/aoa-recon/` + `npm/aoa-recon-{platform}/` — same pattern as L6.8 |
| [L6](#layer-6) | [L6.10](#l610) | | | x | | | | | L6.8, L6.9 | 🟡 | 🟢 | 🟡 | CI/Release — workflow builds both binaries, publishes to npm | Tag → build → publish, fully automated | `.github/workflows/release.yml` (8 matrix jobs + npm publish) |
| [L3](#layer-3) | [L3.15](#l315) | | x | | x | | | | - | 🟢 | 🟢 | 🟡 | GNU grep native parity — 3 modes, 22 flags, stdin/files/index routing | Drop-in grep replacement for AI agents | Smoke tests: stdin pipe, file grep, recursive, exit codes, no ANSI. **Gap**: no automated parity test suite yet |
| [L5](#layer-5) | [L5.13](#l513) | | | | | | | x | L5.12 | 🟢 | 🟢 | 🟡 | Recon findings peek — source line toggle, tier noise filter, scan age | Findings are actionable, not just labels | `/api/source-line` endpoint; click toggles desc↔code; tier gating filters noise. **Gap**: browser-only validation |
| [L7](#layer-7) | [L7.1](#l71) | x | | | x | | | x | - | 🟢 | 🟢 | 🟡 | Startup progress feedback — deferred loading, async cache warming | Daemon starts in <1s, caches warm in background with progress logging | `daemon start` returns in 0.1s; log shows step-by-step progress. **Gap**: no automated startup time test |
| [L7](#layer-7) | [L7.2](#l72) | x | | | | x | | | - | 🟡 | ⚪ | ⚪ | Database storage optimization — replace JSON blobs with binary encoding | 964MB bbolt with JSON serialization. Investigate gob/protobuf/msgpack for faster load | Profile load time; compare encoding formats; target <3s index load |
| [L7](#layer-7) | [L7.3](#l73) | | | | | | | x | - | 🟡 | ⚪ | ⚪ | Recon source line editor view — replace per-line peek with file-level source display | Show all flagged lines in an editor-like view grouped by severity, not one-at-a-time peeks | Design conversation needed on layout; toggle between dimensional view and source view |

---

## Supporting Detail

### Layer 0

**Layer 0: Value Engine (Burn rate, context runway, attribution)**

> The metrics backend that powers all value messaging. Without this, the dashboard has data but no story.
> **Quality Gate**: ✅ `/api/runway` returns valid dual projections; all attribution rows produce correct attrib/impact. 24 new tests, 284 total passing.

#### L0.1

**Burn rate accumulator** — 🟢 Complete

Rolling window (5-minute) `BurnRateTracker` with `Record`, `RecordAt`, `TokensPerMin`, `TotalTokens`, `Reset`. 6 unit tests (empty, single, multi-sample, eviction, partial eviction, reset).

**Files**: `internal/app/burnrate.go`, `internal/app/burnrate_test.go`

#### L0.2

**Context window max lookup** — 🟢 Complete

`ContextWindowSize()` map with Claude 3/3.5/4 family entries, 200k default for unknowns. 2 tests.

**Files**: `internal/app/models.go`, `internal/app/models_test.go`

#### L0.3

**Dual projection** — 🟢 Complete

`burnRate` (actual) and `burnRateCounterfact` (what-if) trackers wired into `onSessionEvent`. Counterfactual records the delta from guided reads (savings >= 50%). `sessionReadCount` and `sessionGuidedCount` tracked.

**Files**: `internal/app/app.go`

#### L0.4

**Context runway API** — 🟢 Complete

`GET /api/runway` returns `RunwayResult` JSON with model, context window, burn rates, both runway projections, delta, and tokens saved. `RunwayProjection()` on `AppQueries` interface. 3 unit tests + 1 HTTP endpoint test.

**Files**: `internal/adapters/socket/protocol.go`, `internal/adapters/socket/server.go`, `internal/adapters/web/server.go`, `internal/adapters/web/server_test.go`, `internal/app/app.go`, `internal/app/runway_test.go`

#### L0.5

**Session summary persistence** — 🟢 Complete

`SessionSummary` struct in ports. `SaveSessionSummary`/`LoadSessionSummary`/`ListSessionSummaries` on Storage interface. bbolt `sessions` bucket implementation. Session boundary detection via `handleSessionBoundary()` — flush on session ID change, restore on revisit. Flush in `Stop()`. 4 bbolt tests + 5 session boundary tests.

**Files**: `internal/ports/storage.go`, `internal/adapters/bbolt/store.go`, `internal/adapters/bbolt/store_test.go`, `internal/app/app.go`, `internal/app/session_test.go`

#### L0.6

**Autotune verification** — 🟢 Complete

Integration test fires 50 searches through `searchObserver`, confirms autotune triggers and produces activity entry.

**Files**: `internal/app/autotune_integration_test.go`

#### L0.7

**Autotune activity event** — 🟢 Complete

After autotune fires in `searchObserver`, pushes `ActivityEntry{Action: "Autotune", Attrib: "cycle N", Impact: "+P promoted, -D demoted, ~X decayed"}`. Verified by integration test.

**Files**: `internal/app/app.go`

#### L0.8

**Write/Edit attrib** — 🟢 Complete

Write/Edit tool events tagged with cycling creative words (`crafted`, `authored`, `forged`, `innovated`) in purple pills. Replaced static "productive" (Session 54). Updated rubric rows 8/9 (regex match), dedicated test `TestActivityWriteEditCreative`, and new `TestActivityCreativeWordCycles` verifying 4-word cycle order.

**Files**: `internal/app/app.go`, `internal/app/activity_test.go`, `internal/adapters/web/static/app.js`

#### L0.9

**Glob attrib** — 🟢 Complete

Glob tool events tagged `attrib = "unguided"` with estimated token cost. `estimateGlobCost(dir, pattern)` rewritten (Session 54) to use `filepath.Match` for glob patterns against relative index paths, with `HasPrefix` fallback for directory-only lookups. Target display now prefers pattern over directory. Updated rubric row 11, dedicated test `TestActivityGlobUnguided`, and new `TestActivityGlobPattern` for pattern-based matching.

**Files**: `internal/app/app.go`, `internal/app/activity_test.go`

#### L0.10

**Grep (Claude) impact** — 🟢 Complete

Claude Grep events show estimated scan cost via `estimateGrepCost()` (total indexed bytes / 4). Updated rubric row 10 and dedicated test.

**Files**: `internal/app/app.go`, `internal/app/activity_test.go`

#### L0.11

**Learn activity event** — 🟢 Complete (redesigned Session 60)

Learning signal redesigned: no longer a separate "Learn" action row. Two sources emit a cycling AI vocabulary word into the **Learned** field of their parent row: (1) `searchObserver` — when keywords/terms/domains are absorbed, the Search row gets `Learned = nextLearnWord()`. (2) Range-gated file reads — Read row gets `Learned` set. Words cycle: trained → fine-tuned → calibrated → converged → reinforced → optimized → weighted → adapted. Dashboard renders as a green pill in the dedicated **Learned** column (between Impact and Target). Session-log grep signals still emit separate Learn activity entries (different path, less frequent).

**Files**: `internal/app/app.go`, `internal/app/activity_test.go`, `internal/adapters/socket/protocol.go`, `internal/adapters/web/static/app.js`, `internal/adapters/web/static/index.html`, `internal/adapters/web/static/style.css`

#### L0.12

**Target capture** — 🟢 Complete

`searchTarget()` preserves full flag syntax verbatim (already correct). Verification test added with all-flags, regex+boundary, and simple query cases.

**Files**: `internal/app/activity_test.go`

---

### Layer 1

**Layer 1: Dashboard (5-tab layout, mockup implementation)**

> Transform the dashboard from data display to value narrative. Each tab tells a story.
> **Quality Gate**: ✅ All 5 tabs render with live API data. `/api/sessions` and `/api/config` both tested. 286 tests passing.

**Tab structure:**
- **Live** (was Overview) — Context runway hero, savings stats, activity feed
- **Recon** (new) — Dimensional analysis view (stub until L5)
- **Intel** (was Learning) — Domain rankings, intent score, n-gram metrics
- **Debrief** (was Conversation) — Exchange-grouped conversation feed (user→thinking→assistant), markdown rendered, Save/Tok value columns in actions
- **Arsenal** (new) — Value proof over time: session savings chart, learning curve, session history table, compact system status

**Design standard (locked Session 48):**
- **Hero row**: `min-height: 160px`, `flex: 2` wrapper + `flex: 1` metrics. `gap: 16px`. Gradient: `conic-gradient(green, blue, purple, green)` rotating at 6s. Card padding: `18px 24px`, gap `8px`. Identity font 22px, headline 17px, support 13px.
- **Hero metrics**: 2×2 grid with arrows. `font-size: 22px` values, `11px` labels.
- **Stats grid**: `repeat(N, 1fr)`, `gap: 12px`. Card: `padding: 16px`, `border-radius: 12px`. Value: `26px`. Label: `12px`. Sub: `11px`. N varies by tab (5-6).
- **Headline pattern**: `{Identity} {outcome} . . . {separator} {exclusion}.`
- **Three-tier narrative**: Hero (claim) → Stats grid (evidence) → Data (detail).
- **Nav**: 52px height. **Footer**: 36px height. Both identical across all tabs.
- **Recon sidebar**: Card-styled (`border-radius: 16px`), grid-aligned with first stat card (`repeat(5, 1fr)` matching stats grid). Pill-based dimension toggles (color-coded by tier, wrapping), toggle switches per tier.
- **Intel**: Domain table matches embedded dashboard: `#`, `@Domain` (purple), `Hits` (float, green, right-aligned), `Terms` (pills with hot/warm/cold states). N-gram sections: Bigrams (cyan), Cohits KW→Term (green), Cohits Term→Domain (purple).

**Static mockups** (validated Session 48): `docs/mockups/{live,recon,intel,debrief,arsenal}.html`

#### L1.1–L1.8

**5-tab SPA** — 🟢 Complete

Full dashboard rewrite delivered as 3 files in `internal/adapters/web/static/`:

- **`index.html`** (354 lines) — HTML shell. Nav with 5 button-tabs, tab content divs (`tab-live`, `tab-recon`, `tab-intel`, `tab-debrief`, `tab-arsenal`), footer.
- **`style.css`** (552 lines) — Unified CSS. Dark/light themes, hero gradient animation, stats grid, activity table, domain table, n-gram bars, conversation feed, arsenal charts, recon placeholder, responsive breakpoints. Soft glow animation system (`soft-glow`, `text-glow`, `.lit` on term pills).
- **`app.js`** (1022 lines) — Tab switching (URL hash), tab-aware 3s polling, hero story rotation, per-tab renderers with change-tracking visual effects.

**Per-tab**:
- **Live**: Runway projection hero (dual projection), 6 stats cards, activity feed table with colored pills
- **Recon**: Hero + stats placeholders + "available in future release" card
- **Intel**: Domain rankings (no Tier column, term pills with `.lit` glow on hit, surgical DOM updates), N-gram metrics (6 bigrams / 4 cohit KW→Term / 4 cohit Term→Domain, no scroll, soft-glow on count changes)
- **Debrief**: Exchange-grouped conversation feed (user→thinking→assistant paired into turns, chronological order), inline thinking (always visible, nested italic, per-chunk lines), markdown rendering, token estimates on all message types, actions column with Save/Tok value columns (guided green, unguided red)
- **Arsenal**: Savings chart (div-based dual bars), session history table (mini guided-ratio bar), learning curve canvas (HiDPI), system status strip

**New backend** (`L1.5`): `SessionSummaryResult`, `SessionListResult`, `ProjectConfigResult` types. `SessionList()` + `ProjectConfig()` on `AppQueries`. `GET /api/sessions` + `GET /api/config` on web server. `dbPath` and `started` fields added to App struct. 2 new endpoint tests.

**Files**: `internal/adapters/web/static/index.html`, `style.css`, `app.js`, `embed.go`, `internal/adapters/socket/protocol.go`, `server.go`, `internal/adapters/web/server.go`, `server_test.go`, `internal/app/app.go`

---

### Layer 2

**Layer 2: Infrastructure Gaps (File watcher, bbolt lock, CLI completeness, sub-ms search)**

> Fix the known gaps that prevent production-grade operation. L2.4-L2.7 extend with trigram-indexed content search to hit the G0 sub-millisecond target while preserving full grep/egrep parity (G1/G3).
> **Quality Gate**: ✅ All L2 tasks complete. `aoa init` works while daemon runs; file changes trigger re-index; `grep -v` works; `aoa grep` ≤1ms via trigram index (~60µs on 500 files); case-sensitive by default (G1 parity). 30 new tests in L2.4-L2.7, 374 total passing. Research: [sub-ms-grep.md](docs/research/sub-ms-grep.md).

#### L2.1

**Wire file watcher** — 🟢 Complete

`Rebuild()` method added to `SearchEngine` (reconstructs `refToTokens`, `tokenDocFreq`, `keywordToTerms` from current index state). `Parser *treesitter.Parser` added to `App` struct. `onFileChanged()` handles add/modify/delete: acquires mutex, checks extension support, computes relative path, removes old entries if modifying, parses with tree-sitter, adds tokens, calls `Rebuild()` + `SaveIndex()`. `removeFileFromIndex()` cleans all 3 index maps (Metadata, Tokens, Files). `Watch()` called in `Start()` (non-fatal on failure). 5 tests: rebuild after mutation, new file, modify file, delete file, unsupported extension.

**Files**: `internal/domain/index/search.go` (Rebuild), `internal/app/app.go` (Parser field, Start wiring), `internal/app/watcher.go` (new: onFileChanged, removeFileFromIndex), `internal/domain/index/rebuild_test.go` (new), `internal/app/watcher_test.go` (new)

#### L2.2

**Fix bbolt lock contention** — 🟢 Complete

`BuildIndex()` extracted from `init.go` into `internal/app/indexer.go` as shared function with `IndexResult` struct. `MethodReindex` socket command added with `ReindexResult` type. `Reindex()` on `AppQueries` interface — builds index outside mutex (IO-heavy walk/parse), then swaps maps in-place under lock and calls `Rebuild()`. Client `Reindex()` uses 120s timeout via new `callWithTimeout()` helper. `init.go` rewritten: delegates to daemon via socket when running (no lock error), falls back to direct `BuildIndex()` + bbolt otherwise. 4 tests: counts, large file skip, ignored dirs, full reindex cycle.

**Files**: `internal/app/indexer.go` (new: BuildIndex, IndexResult, skipDirs), `internal/adapters/socket/protocol.go` (MethodReindex, ReindexResult), `internal/adapters/socket/server.go` (Reindex on AppQueries, handleReindex), `internal/adapters/socket/client.go` (Reindex, callWithTimeout), `internal/app/app.go` (Reindex impl), `cmd/aoa/cmd/init.go` (rewritten), `internal/app/indexer_test.go` (new), `internal/app/reindex_test.go` (new)

#### L2.3

**Implement --invert-match** — 🟢 Complete

`InvertMatch bool` added to `SearchOptions`. `-v`/`--invert-match` flag registered in both `grep.go` and `egrep.go`, wired into opts. `invertSymbolHits()` method on `SearchEngine` computes the complement: builds set of matched `(fileID, line)` pairs, then collects all symbols NOT in that set (respecting glob filters). Content scanning flips the matcher result when `InvertMatch` is set. `-v` added to `searchTarget()` display. 8 tests: literal, regex, OR, AND, content, count-only, quiet, with-glob.

**Files**: `internal/ports/storage.go` (InvertMatch field), `cmd/aoa/cmd/grep.go` (-v flag), `cmd/aoa/cmd/egrep.go` (-v flag), `internal/domain/index/search.go` (invertSymbolHits), `internal/domain/index/content.go` (matcher flip), `internal/app/app.go` (searchTarget), `internal/domain/index/invert_test.go` (new)

#### L2.4

**Trigram index in FileCache** — 🟢 Complete

Trigram inverted index built during `WarmFromIndex`. Key type: `[3]byte` (no string allocation). Posting lists sorted by (fileID, lineNum) for merge-join intersection. File IDs iterated in sorted order so posting lists are naturally sorted. `lowerLines` built in same pass. 9 new tests.

**Files**: `internal/domain/index/filecache.go` (buildTrigramIndex, TrigramLookup, intersectContentRefs, GetLowerLines, HasTrigramIndex), `internal/domain/index/filecache_test.go` (9 tests)

#### L2.5

**Wire trigram + fix case-sensitivity default** — 🟢 Complete

Two changes delivered: (1) G1 parity fix — `buildContentMatcher` default path changed from `containsFold` to `strings.Contains` (case-sensitive by default, matching Unix grep). New `case_insensitive` case added. (2) Trigram dispatch — `scanContentTrigram()` extracts trigrams from lowered query, intersects posting lists via `TrigramLookup`, then verifies candidates with mode-aware matcher (case-sensitive: `strings.Contains(originalLine, query)`, case-insensitive: `strings.Contains(lowerLine, lowerQuery)`). `canUseTrigram()` gates dispatch: requires query ≥ 3 chars, no InvertMatch/regex/WordBoundary/AndMode. Benchmark: ~60µs per query on 500 files. 7 new tests.

**Files**: `internal/domain/index/content.go` (scanContentTrigram, extractTrigrams, canUseTrigram, updated buildContentMatcher and scanFileContents), `internal/domain/index/content_test.go` (7 tests)

#### L2.6

**Pre-lowercased line cache** — 🟢 Complete

`lowerLines` stored alongside original lines in `cacheEntry` (built in L2.4's `buildTrigramIndex`). Brute-force fallback now uses pre-lowered lines + `strings.Contains` for case-insensitive mode instead of `containsFold`. Trigram verification also uses `GetLowerLines` for `-i` mode.

**Files**: `internal/domain/index/filecache.go` (cacheEntry.lowerLines, GetLowerLines), `internal/domain/index/content.go` (brute-force lowerLines optimization)

#### L2.7

**Edge cases + regression suite** — 🟢 Complete

All grep/egrep modes validated after trigram integration:

| Mode | Path | Test |
|------|------|------|
| Query ≥ 3 chars, literal | Trigram | TestContentSearch_TrigramPath_UsedForLongQuery |
| Query < 3 chars | Brute-force | TestContentSearch_ShortQueryFallback, TestContentSearch_ShortQueryCaseInsensitive |
| Regex mode | Brute-force | TestContentSearch_RegexWithCache |
| InvertMatch (`-v`) | Brute-force | TestContentSearch_InvertMatchWithCache_ExcludesMatches |
| AND mode | Brute-force | TestContentSearch_ANDWithCache |
| Word boundary (`-w`) | Brute-force | TestContentSearch_WordBoundaryWithCache |
| Glob filters | Trigram + filter | TestContentSearch_GlobFilterWithTrigram |

374 tests passing, 0 failures. Benchmark: ~60µs on 500 files (both case-sensitive and case-insensitive).

**Files**: `internal/domain/index/content_test.go` (7 tests), `test/benchmark_test.go` (BenchmarkSearch_E2E_CaseInsensitive)

**Research**: [Sub-millisecond grep analysis](docs/research/sub-ms-grep.md)

---

### Layer 3

**Layer 3: Migration & Validation (Parallel run, parity proof)**

> Run both systems side-by-side and prove equivalence before the Python version is retired.
> **Quality Gate**: Parity tests (55 grep + 200-intent learner replay) prove equivalence. Go beats Python on all benchmarks.

#### L3.2

**Search diff**

100 queries per project, automated diff of results. Cover all search modes: literal, OR, AND, regex, case-insensitive, word-boundary, count, include/exclude.

**Files**: `test/migration/search-diff.sh`

#### L3.3

**Learner state diff**

After 200 intents of observation, diff the learner state (domains, terms, keywords, hits, bigrams). Zero tolerance for divergence. DomainMeta.Hits is float64 — the precision rule matters here.

**Files**: `test/migration/state-diff.sh`

#### L3.4

**Benchmark comparison**

Head-to-head: search latency, autotune latency, startup time, memory footprint. Confirm 50-120x speedup and 8x memory reduction.

**Files**: `test/benchmarks/compare.sh`

#### L3.5

**Migration docs**

Step-by-step: stop Python daemon, install Go binary, migrate bbolt data (or re-index), verify. Cover rollback if needed.

**Files**: `docs/migration.md`

#### L3.15

**GNU grep native parity** — 🟢 Complete (Session 65)

Three-route architecture makes `aoa grep`/`aoa egrep` drop-in replacements for GNU grep. 22 flags implemented natively covering 100% of observed AI agent usage. Piped output auto-strips ANSI codes. System grep fallback for rare flags.

**Execution modes:**
- File args present → `grepFiles()` — native Go line-by-line search
- Stdin is pipe (no file args) → `grepStdin()` — native Go stdin filter
- Neither → daemon index search → fallback to `/usr/bin/grep` if daemon down

**22 native flags:** `-i`, `-w`, `-c`, `-q`, `-v`, `-m`, `-E`, `-e`, `-F`, `-n`, `-H`, `-h`, `-l`, `-L`, `-o`, `-r`, `-A`, `-B`, `-C`, `-a`, `--include/--exclude/--exclude-dir`, `--color`

**GNU compat details:** exit codes 0/1/2, `file:line:content` format, `--` group separators, binary detection (NUL in first 512 bytes), multi-file filename prefix, ANSI auto-strip when not TTY.

**New files:** `cmd/aoa/cmd/tty.go`, `grep_native.go`, `grep_fallback.go`, `grep_exit.go`
**Modified:** `cmd/aoa/cmd/grep.go`, `egrep.go`, `output.go`, `cmd/aoa/main.go`

---

### Layer 4

**Layer 4: Distribution — Lean Binary + Grammar Packs**

> Ship it. 80MB→12MB binary. 57 languages via dynamic grammar loading. Four platforms.
> **Quality Gate**: Lean binary works on linux/darwin × amd64/arm64. `go install` compiles all grammars in. Dynamic loader produces identical results to compiled-in.

**Architecture (Session 61)**:
```
┌──────────────────────────────────────┐
│  aoa binary (~12 MB lean, ~80 MB full)│
│  Go code + tree-sitter core (CGo)     │
│  -tags lean: no grammars compiled in  │
│  default: all 28 grammars compiled in │
│                                       │
│  DynamicLoader: purego loads .so/.dylib│
│  from .aoa/grammars/ at runtime       │
└──────────────────────────────────────┘
          ↓ loads on demand
┌──────────────────────────────────────┐
│  .aoa/grammars/                       │
│  ├── python.so        (0.5 MB)       │
│  ├── javascript.so    (0.4 MB)       │
│  ├── ...                              │
│  Search: project/.aoa/grammars/ first │
│          ~/.aoa/grammars/ fallback    │
└──────────────────────────────────────┘
```

**Priority Tiers**: P1 Core (11 langs, ~12 MB), P2 Common (11, ~16 MB), P3 Extended (17, ~10 MB), P4 Specialist (18, ~60 MB).

**Key Decisions**:
- **Build tags, not separate binaries**: `//go:build !lean` vs `//go:build lean`. Default = all grammars. Release = lean.
- **Purego, not Go plugin**: CGo-free loading from `.so`/`.dylib`. No Go version matching needed.
- **Individual .so files**: One per grammar. Pack tarballs for bulk download.
- **Graceful fallback**: Parser tries compiled-in first, then dynamic loader. Unknown languages silently skip.

#### L4.1

**Purego .so loader** — 🟢 Complete

`DynamicLoader` using `github.com/ebitengine/purego`. `LoadGrammar(langName)` searches `.aoa/grammars/` paths, `Dlopen`s the `.so`/`.dylib`, calls `tree_sitter_{lang}()` via `RegisterLibFunc`, wraps result with `tree_sitter.NewLanguage()`. Platform-aware (`.so` on Linux, `.dylib` on macOS). Caches loaded languages. `Parser.SetGrammarPaths()` enables dynamic loading as fallback. Build tag split: `languages_builtin.go` (`!lean`, all 28 grammars), `languages_lean.go` (`lean`, empty), `extensions.go` (always, 97 extensions + symbolRules).

**E2E verified**: compile Python grammar from Go module cache to `.so` → load via purego → parse Python → symbols identical to compiled-in parser output.

**Files**: `internal/adapters/treesitter/loader.go` (new), `loader_test.go` (18 tests), `loader_e2e_test.go` (1 E2E test), `parser.go` (modified — dynamic fallback), `extensions.go` (new — split from languages.go), `languages_builtin.go` (new — `!lean` tag), `languages_lean.go` (new — `lean` tag)

#### L4.2

**Grammar CLI + Build CI** — 🟢 Code complete, 🟡 Download not implemented

`aoa grammar list` — shows all 57 grammars by priority tier, marks built-in (B) vs dynamic (D). `aoa grammar info <lang>` — version, extensions, repo URL, C symbol name, install status. `aoa grammar install <lang|pack>` — resolves pack names (core/common/extended/specialist/all) to grammar lists, shows what needs installing. `aoa grammar path` — shows search paths with existence indicators.

`grammars/manifest.json` — registry with 57 grammars: name, version, priority, extensions, repo URL, source files. `BuiltinManifest()` embedded in binary.

`.github/workflows/build-grammars.yml` — 4-platform matrix (linux/darwin × amd64/arm64). Clones 28 grammar repos, compiles to `.so`/`.dylib`, generates checksums, creates pack tarballs (core/common/extended/specialist). Uploads artifacts.

**Gap**: `aoa grammar install` doesn't yet download from GitHub Releases — shows what would be installed and manual build instructions.

**Files**: `cmd/aoa/cmd/grammar.go` (new), `internal/adapters/treesitter/manifest.go` (new), `grammars/manifest.json` (new), `.github/workflows/build-grammars.yml` (new)

#### L4.3

**Lean binary + Goreleaser** — 🟢 Complete

`go build -tags lean` produces 12MB binary (vs 80MB full). 85% reduction. All grammar imports excluded, only tree-sitter core compiled in. `make build-lean` target added.

`.github/workflows/release.yml` — triggered on tag push, builds + checksums + GitHub Release.

**Binary sizes** (measured):
- Full (default): 80 MB — all 28 grammars compiled in
- Lean (`-tags lean`): 12 MB — no grammars, loads from `.so` at runtime

**Files**: `.github/workflows/release.yml`, `Makefile` (updated — `build-lean` target), `internal/adapters/treesitter/languages_lean.go`

#### L4.4

**Installation docs** — ⚪ Not started

Two install paths: `go install github.com/corey/aoa/cmd/aoa@latest` (builds from source with all grammars) vs binary download (lean + grammar packs). Post-install: `aoa init`, `aoa daemon start`, grammar install.

**Files**: `README.md`

---

### Layer 5

**Layer 5: Dimensional Analysis (Bitmask engine, Recon tab)**

> Early warning system — yellow flags, not verdicts. Surfaces concerns via per-line bitmask scanning using tree-sitter AST + Aho-Corasick. Users can acknowledge, dismiss, or ignore findings.
>
> **Prerequisite**: Multi-language tree-sitter integration must be mature (L4 grammar loader working, validated across languages).
>
> **Quality Gate**: Security tier catches known vulns in test projects. Full project query < 10ms. ~250-290 questions across 6 tiers.
>
> **Research**: [Bitmask analysis](docs/research/bitmask-dimensional-analysis.md) | [AST vs LSP viability](docs/research/asv-vs-lsp.md)

**6 Tiers (22 dimensions):**

| Tier | Color | Dimensions |
|------|-------|-----------|
| Security | Red | Injection, Secrets, Auth Gaps, Cryptography, Path Traversal |
| Performance | Yellow | Query Patterns, Memory, Concurrency, Resource Leaks |
| Quality | Blue | Complexity, Error Handling, Dead Code, Conventions |
| Compliance | Purple | CVE Patterns, Licensing, Data Handling |
| Architecture | Cyan | Import Health, API Surface, Anti-patterns |
| Observability | Green | Silent Failures, Debug Artifacts |

**Execution pipeline**: Compute at index time (tree-sitter parse + AC scan + regex → bitmask → bbolt). Query time reads pre-computed bitmasks (< 10ms for entire project). See research doc for full pipeline detail.

#### L5.1

**Structural query YAML schema**

Define the format for detection patterns: AST structural rules, AC text patterns, lang_map entries, severity weights, bit positions.

**Files**: `dimensions/schema.yaml`

#### L5.2

**Tree-sitter AST walker**

Walk the already-parsed AST for structural patterns (call_with_arg, call_inside_loop, assignment_with_literal, etc.). ~8-10 parameterized pattern types cover the majority of questions.

**Files**: `internal/domain/analyzer/walker.go`

#### L5.3

**AC text scanner**

Compile ~115 text patterns into single AC automaton at startup. One pass per file over raw source bytes. Returns (pattern_id, byte_offset) pairs mapped to bit positions.

**Files**: `internal/domain/analyzer/text_scan.go`

#### L5.4

**Language mapping layer**

Normalize AST node names across 28 languages. ~280 entries (28 langs × 10 concepts). Most are identical or near-identical. ~20% of questions need per-language overrides.

**Files**: `internal/domain/analyzer/lang_map.go`

#### L5.5

**Bitmask composer**

Merge structural + text + regex bits into per-method bitmask. Compute weighted severity score (critical=3, high=2, medium=1). ~38 bytes per method across all dimensions.

**Files**: `internal/domain/analyzer/score.go`

#### L5.6

**Security tier**

67 questions across 5 categories: injection (16), secrets (13), auth gaps (14), cryptography (12), path traversal (12). Full question set defined in research doc.

**Files**: `dimensions/security/*.yaml`

#### L5.7

**Performance tier**

~50-60 questions: query patterns, memory, concurrency, resource leaks. N+1, unbounded alloc, mutex over I/O, unclosed handles.

**Files**: `dimensions/performance/*.yaml`

#### L5.8

**Quality tier**

~45-55 questions: complexity, error handling, dead code, conventions. God functions, ignored errors, deep nesting, magic numbers.

**Files**: `dimensions/quality/*.yaml`

#### L5.9

**Wire analyzer into init**

During `aoa init` (and file-watch re-index), run the dimensional engine alongside symbol extraction. Store per-method bitmasks and scores in bbolt "dimensions" bucket. ~0.2-0.6ms overhead per file.

**Files**: `internal/app/app.go`

#### L5.10

**Dimension scores in search results**

Append `S:-23 P:0 Q:-4` to search output. Negative = debt. Zero = clean. Visible in every grep/egrep result.

**Files**: `internal/domain/index/format.go`

#### L5.11

**Dimension query support**

`aoa grep --dimension=security --risk=high <query>` filters results to only show methods with security findings above threshold.

**Files**: `cmd/aoa/cmd/grep.go`

#### L5.12

**Recon tab** — 🔵 In progress (interim scanner)

NER-style dimensional view: tier toggle sidebar (5 tiers, color-coded), file→folder→symbol drill-down, severity scoring. Mockup validated in `docs/mockups/recon.html`.

**Interim implementation (Session 62)**: Backend `recon.go` scans FileCache lines with 10 pattern detectors + `long_function` from symbol metadata. `GET /api/recon` returns JSON tree keyed by `folder → file → {language, symbols, findings}`. Frontend: sidebar with tier toggles + dimension pills (localStorage-persisted), breadcrumb-navigated tree (root→folder→file→symbol), finding detail rows with severity/id/label/line. Code-only file filter prevents non-code false positives.

**Gap**: Full bitmask engine (L5.1–L5.5), AST-based structural patterns, AC text scanner, cross-language uniformity, acknowledge/dismiss per finding, unit tests for scanner.

**Files**: `internal/adapters/recon/scanner.go` (extracted from web/recon.go), `internal/adapters/web/recon.go` (thin handler calling shared scanner), `internal/domain/index/search.go` (Cache accessor), `internal/adapters/web/server.go` (route), `internal/adapters/web/static/index.html`, `style.css`, `app.js`

#### L5.13

**Recon findings peek + tier noise gating** — 🟢 Complete (Session 65)

Three enhancements to the Recon findings UX:

1. **Source line peek**: Finding rows are clickable. Click toggles between description text ("Potential hardcoded secret") and the actual source line from the in-memory FileCache. Zero disk I/O — reads from the same cache used for search. New `GET /api/source-line?file=X&line=12&context=0` endpoint resolves path→fileID→cache line.

2. **Tier noise gating**: File-level drill-down filters findings to only show tiers that registered > 0 at the file level. If `content_test.go` shows PERF=0, performance-tier findings are hidden when drilling into that file. Prevents walls of low-severity warnings from tiers that didn't accumulate enough signal. `reconAggregateFile()` computes active tiers; `elevatedFindings` filter applied before grouping by symbol.

3. **Scan freshness**: `/api/recon` response includes `scanned_at` timestamp. Hero support line shows "scanned just now" / "scanned 12s ago" / "scanned 3m ago".

**Files**: `internal/adapters/web/recon.go` (handleSourceLine, scanned_at), `internal/adapters/web/server.go` (route), `internal/adapters/web/static/app.js` (peek handler, tier filter, scan age), `internal/adapters/web/static/style.css` (finding-peek, finding-code)

---

### Layer 6

**Layer 6: Distribution v2 (Two-binary split, npm packaging)**

> aOa ships as two binaries: `aoa` (pure Go, ~11 MB) for search + learning + dashboard, and `aoa-recon` (CGo, ~361 MB, 509 languages via go-sitter-forest) for tree-sitter parsing + security scanning. Distributed via npm. `aoa` auto-discovers `aoa-recon` when both are installed (git-lfs model).
>
> **Quality Gate**: `CGO_ENABLED=0 go build ./cmd/aoa/` produces a working binary. `go build ./cmd/aoa-recon/` produces a CGo binary with 509 grammars. `aoa` discovers and invokes `aoa-recon` as subprocess. Dashboard shows install prompt when recon not available.

#### L6.1

**Parser interface in ports** — 🟢 Complete

`ports.Parser` interface with `ParseFileToMeta(path, source) ([]*SymbolMeta, error)` and `SupportsExtension(ext) bool`. Decouples `app.go` from concrete `treesitter.Parser` — hexagonal violation fixed.

**Files**: `internal/ports/parser.go` (new)

#### L6.2

**Make App.Parser optional** — 🟢 Complete

Changed `App.Parser` from `*treesitter.Parser` to `ports.Parser` interface. `Config.Parser` field accepts optional parser (nil = tokenization-only). All parser usage nil-guarded in `watcher.go` and `indexer.go`. Build-tag-guarded files: `parser_cgo.go` returns `treesitter.NewParser()`, `parser_nocgo.go` returns nil. `grammar.go` split into CGo/non-CGo variants.

**Gate**: `CGO_ENABLED=0 go build ./cmd/aoa/` → 7.5 MB statically linked binary. All existing tests pass.

**Files**: `internal/app/app.go`, `internal/app/indexer.go`, `internal/app/watcher.go`, `cmd/aoa/cmd/parser_cgo.go` (new), `cmd/aoa/cmd/parser_nocgo.go` (new), `cmd/aoa/cmd/grammar_cgo.go` (renamed from grammar.go), `cmd/aoa/cmd/grammar_nocgo.go` (new), `cmd/aoa/cmd/init.go`, `cmd/aoa/cmd/daemon.go`

#### L6.3

**Tokenization-only indexer fallback** — 🟢 Complete

`defaultCodeExtensions` map (97 extensions matching `treesitter/extensions.go`) enables file discovery without parser. `BuildIndex` tokenizes file content line-by-line via `TokenizeContentLine()` when parser is nil. Search works at file level (no symbol names/lines).

**Gate**: `TestBuildIndex_NilParser_TokenizationOnly` — 2 files discovered, 0 symbols, tokens from content.

**Files**: `internal/app/indexer.go` (defaultCodeExtensions + tokenization fallback)

#### L6.4

**Create cmd/aoa-recon/ entry point** — 🟢 Complete

Separate binary with Cobra CLI: `enhance --db <path> --root <project>` (full project scan), `enhance-file --db <path> --file <path>` (incremental), `version`. Opens bbolt, loads existing index, creates treesitter.Parser, walks files, writes enhanced symbol metadata back. 361 MB binary with 509 grammars (via go-sitter-forest).

**Files**: `cmd/aoa-recon/main.go` (new)

#### L6.5

**Extract scanner to shared package** — 🟢 Complete

Scanner logic extracted from `web/recon.go` to `internal/adapters/recon/scanner.go`. Exports `recon.Scan(idx, lines)` with `LineGetter` interface. Web handler is now a thin adapter that calls the shared scanner. Both web handler and aoa-recon binary can call the same scanning logic.

**Files**: `internal/adapters/recon/scanner.go` (new), `internal/adapters/web/recon.go` (refactored to thin handler)

#### L6.6

**Recon bridge** — 🟢 Complete

`ReconBridge` in `internal/app/recon_bridge.go` discovers aoa-recon binary (PATH → .aoa/bin/ → sibling). `Enhance()` and `EnhanceFile()` invoke subprocess with appropriate flags. Integrated into App lifecycle: `initReconBridge()` in `New()`, `TriggerReconEnhance()` after reindex (when parser is nil), `TriggerReconEnhanceFile()` after file change (when parser is nil). `ReconAvailable()` exposed for dashboard API.

**Files**: `internal/app/recon_bridge.go` (new), `internal/app/app.go` (wiring), `internal/app/watcher.go` (trigger)

#### L6.7

**Dashboard Recon tab install prompt** — 🟢 Complete

Added `ReconAvailable() bool` to `AppQueries` interface. Recon API response includes `recon_available` field. When `recon_available: false` and no scan data, dashboard renders install prompt with `npm install -g aoa-recon` command. When scan data present but recon not available, shows "lite mode" indicator in hero support text.

**Files**: `internal/adapters/socket/server.go` (interface), `internal/adapters/web/recon.go` (API field), `internal/adapters/web/static/app.js` (install prompt UI)

#### L6.8 + L6.9

**npm package structures** — 🟢 Complete

10 npm packages following esbuild/turbo pattern: 2 wrapper packages (`aoa`, `aoa-recon`) with ~30-line JS shims that detect platform, resolve binary from optional dependency, and create symlinks. 8 platform-specific packages (`@aoa/linux-x64`, `@aoa/darwin-arm64`, etc.) with `os`/`cpu` constraints so npm only downloads the matching binary.

**Files**: `npm/aoa/package.json`, `npm/aoa/install.js`, `npm/aoa-recon/package.json`, `npm/aoa-recon/install.js`, 8× `npm/aoa-{platform}/package.json`

#### L6.10

**CI/Release** — 🟢 Complete

Release workflow builds 8 binaries (2 binaries × 4 platforms), creates GitHub release, then publishes to npm: populate platform packages with binaries, set versions from git tag, publish platform packages first, then wrapper packages.

**Files**: `.github/workflows/release.yml`

---

### What Works (Preserve)

| Component | Notes |
|-----------|-------|
| Search engine (O(1) inverted index) | 26/26 parity tests, 4 search modes, content scanning, `Rebuild()` for live mutation. Hot file cache (22 tests incl. trigram). Trigram index for sub-ms content search (~60µs on 500 files). Case-sensitive by default (G1). `fileSpans` precomputed. |
| Learner (21-step autotune) | 5/5 fixture parity, float64 precision on DomainMeta.Hits. Do not change decay/prune constants. |
| Session Prism (Claude JSONL reader) | Defensive parsing, UUID dedup, compound message decomposition. Battle-tested. |
| Tree-sitter parser (509 languages) | go-sitter-forest provides 509 compiled-in grammars. Symbol extraction for Go, Python, JS/TS + generic. 163 file extension mappings. `gen_forest.go` generator. Behind `ports.Parser` interface; lives in `aoa-recon` binary (361 MB). |
| Two-binary distribution (L6, 10 tasks) | `aoa` (pure Go, 11 MB) + `aoa-recon` (CGo, 361 MB). ReconBridge auto-discovery. npm packaging. CI/release pipeline. |
| Socket protocol | JSON-over-socket IPC. Concurrent clients. `Reindex` command with extended timeout. Extend, don't replace. |
| Value engine (L0, 24 new tests) | Burn rate, runway projection, session persistence, activity enrichments. All wired. |
| Activity rubric (41 tests) | Three-lane color system: green (guided savings), red (unguided cost — pill, impact, target), purple (creative words for Write/Edit). Learned column with cycling AI vocabulary. Autotune enrichments. Target: `aOa` blue, `grep`/`egrep` green. Impact: numbers cyan. |
| Dashboard (L1, 5-tab SPA) | 3-file split: `index.html` + `style.css` + `app.js`. Tab-aware polling. Soft glow animations. All tabs render live data. |
| File watcher (L2, 5 new tests) | `fsnotify` → `onFileChanged` → re-parse → `Rebuild()` → `SaveIndex()`. Add/modify/delete. |
| Invert-match (L2, 8 new tests) | `-v` flag for grep/egrep. Symbol complement + content matcher flip. All 4 modes. |
| Reindex protocol (L2, 4 new tests) | `BuildIndex()` shared function. `aoa init` delegates to daemon or runs direct. No more lock errors. |

---

### Layer 7

**Layer 7: Onboarding UX (First-run experience, progress feedback)**

> Users don't know the system is working. A 20-30s silent startup with no feedback creates uncertainty and distrust.
> **Quality Gate**: User sees meaningful progress during daemon startup and `aoa init`.

#### L7.1

**Startup progress feedback** — 🟢 Complete (Session 66)

Solution: Deferred all heavy IO to background after the socket server is up.

`New()` is now instant — creates an empty index, fresh learner, unwarmed file cache. `Start()` brings up the socket + HTTP server immediately. `WarmCaches()` runs in a goroutine with a progress callback:

```
loading index from database...          (28.7s — 964MB bbolt)
index loaded: 3932 files, 48582 tokens
loading learner state...                (0.0s)
learner state loaded: 1611 prompts
warming file cache (3932 files)...      (17.1s — reads all files from disk)
file cache ready
scanning recon patterns...              (12.1s — 3932 files × patterns)
recon cache ready
all caches warm — 3932 files ready (57.9s total)
```

User sees `daemon started (pid N)` in 0.1 seconds. Dashboard works immediately. Searches and recon data populate as caches warm.

**Files**: `internal/app/app.go` (WarmCaches), `internal/domain/index/search.go` (SetCache/WarmCache split), `cmd/aoa/cmd/daemon.go` (deferred warm)

#### L7.2

**Database storage optimization** — ⚪ Not started

Problem: `aoa.db` is 964MB. `LoadIndex()` reads the entire file as JSON blobs and deserializes in ~28.7s. This is the single largest contributor to total startup time.

Investigation needed:
- Profile: how much time in bbolt read vs JSON unmarshal?
- Alternative encodings: gob (Go-native), msgpack, protobuf, flatbuffers
- Structural change: split monolithic token map into per-file buckets for incremental loading
- Compression: gzip/zstd on stored blobs
- Target: <3s index load for 4K-file projects

**Files**: `internal/adapters/bbolt/store.go` (LoadIndex/SaveIndex)

#### L7.3

**Recon source line editor view** — ⚪ Not started

Problem: The current per-line peek (click a finding → see one source line) is useful but limited. When a file has 10+ findings, the user wants to see all flagged lines in context, not click them one by one.

Design conversation needed:
- Toggle between dimensional view (current: symbols + findings grouped by severity) and source view (editor-like: all flagged lines shown with severity badges inline, like a code review)
- Source view shows line numbers, syntax context, severity markers in the gutter
- Severity grouping: critical lines first, then warning, info hidden by default
- Consider: is this a separate "Source" tab within the file drill-down, or a mode toggle on the existing view?

**Files**: `internal/adapters/web/static/app.js`, `internal/adapters/web/static/style.css`, `internal/adapters/web/recon.go` (may need new API endpoint for multi-line fetch)

---

### What We're NOT Doing

| Item | Rationale |
|------|-----------|
| Neural 1-bit embeddings | Investigated, deprioritized. Deterministic AST+AC gives better signal with full interpretability. |
| WebSocket push (dashboard) | 2s poll is sufficient. Upgrade deferred — complexity not justified yet. |
| Multi-project simultaneous daemon | Single-project scope per daemon instance. Multi-project is a v3 concern. |
| LSP integration | AST is sufficient for early warning. LSP adds 100x cost for 20% more precision. See research. |

### Key Documents

| Document | Purpose |
|----------|---------|
| [COMPLETED.md](.context/COMPLETED.md) | Archived phases 1-8c with validation notes |
| [Bitmask Analysis](docs/research/bitmask-dimensional-analysis.md) | Security worked example, execution pipeline, cross-language uniformity |
| [AST vs LSP](docs/research/asv-vs-lsp.md) | Viability assessment, per-dimension confidence ratings |
| [Sub-ms grep research](docs/research/sub-ms-grep.md) | Trigram index approach, 5 alternatives evaluated, implementation plan |
| [Feedback Outline](docs/research/feedback/OUTLINE.md) | User feedback on all system components |
| [CLAUDE.md](CLAUDE.md) | Agent instructions, architecture reference, build commands |

### Quick Reference

| Resource | Location |
|----------|----------|
| Build (full) | `make build` — CGo, all grammars (~76 MB) |
| Build (pure) | `make build-pure` or `CGO_ENABLED=0 go build ./cmd/aoa/` — pure Go (~8 MB) |
| Build (recon) | `make build-recon` or `go build ./cmd/aoa-recon/` — CGo, tree-sitter (~73 MB) |
| Test | `go test ./...` |
| CI check | `make check` |
| Database | `{ProjectRoot}/.aoa/aoa.db` |
| Socket | `/tmp/aoa-{sha256(root)[:12]}.sock` |
| Dashboard | `http://localhost:{port}` (port in `.aoa/http.port`) |
| Session logs | `~/.claude/projects/{encoded-path}/*.jsonl` |
| Mockups | `docs/mockups/{live,recon,intel,debrief,arsenal}.html` |
