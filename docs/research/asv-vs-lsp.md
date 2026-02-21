2026-02-18 20:00
Status: #draft
Tags: #code_intelligence #ast #lsp #security_audit #static_analysis #tree_sitter
Links:
[https://www.npmjs.com/package/typescript](https://www.npmjs.com/package/typescript)
[https://www.npmjs.com/package/pyright?activeTab=readme](https://www.npmjs.com/package/pyright?activeTab=readme)
[https://marketplace.visualstudio.com/items?itemName=ms-python.vscode-pylance](https://marketplace.visualstudio.com/items?itemName=ms-python.vscode-pylance)
[https://www.vsixhub.com/vsix/32420/](https://www.vsixhub.com/vsix/32420/)
[https://pkgs.alpinelinux.org/package/edge/community/x86/gopls](https://pkgs.alpinelinux.org/package/edge/community/x86/gopls)
[https://download.eclipse.org/justj/?file=jdtls%2Fsnapshots](https://download.eclipse.org/justj/?file=jdtls%2Fsnapshots)
[https://pkgs.alpinelinux.org/package/edge/community/x86_64/jdtls](https://pkgs.alpinelinux.org/package/edge/community/x86_64/jdtls)
[https://github.com/rust-lang/rust-analyzer/releases](https://github.com/rust-lang/rust-analyzer/releases)
[https://github.com/clangd/clangd/issues/2341](https://github.com/clangd/clangd/issues/2341)
[https://github.com/OmniSharp/omnisharp-roslyn/releases/](https://github.com/OmniSharp/omnisharp-roslyn/releases/)
[https://go.dev/gopls/design/design](https://go.dev/gopls/design/design)

# AST vs LSP, why they feel similar, why they behave differently

## The common ground (what both give you)

Both Abstract Syntax Tree (AST) tooling and LSP-backed tooling start from the same core move, parse source into a tree, then walk that tree to answer questions like “where is this symbol defined?” or “what calls this method?”.

They both enable:

* Navigation, jump-to-definition, find-references (at least within a file for pure AST, project-wide for LSP)
* Structural search, “find all functions named X”, “find all calls with these arguments”
* Security scanning primitives like “find dangerous API usage patterns”, “find auth checks that are missing”, “find string concat into SQL calls”

So your intuition is right, they are the same *family*.

## The split (syntax vs meaning)

The key difference is what comes after parsing.

* **AST-only pipeline** stops at structure, it knows the shape of code.
* **LSP pipeline** continues into semantics, it binds names to symbols, resolves imports, infers or checks types, builds indexes, then exposes those answers over the Language Server Protocol.

So, AST is a “skeleton”, LSP is “skeleton plus identity plus relationships”.

---

# Story: why AST often wins for “early warning”, why LSP wins for “truth”

## If your goal is early warning (security, hygiene, regression signals)

AST-first shines because it is:

* Fast, deterministic, parallelizable
* Easier to run in CI at scale, even on partial or broken code
* Less coupled to per-language runtime and build system state

This makes AST great for:

* “Are we calling risky APIs anywhere?”
* “Did a new dependency or permission boundary appear?”
* “Did sensitive data start flowing into logging calls?”
* “Are we introducing eval, shell exec, weak crypto, SQL concat?”

Even if AST cannot resolve every call target perfectly, it can still raise “smoke alarms” cheaply.

## If your goal is semantic truth (precise call resolution, refactors, deep IDE features)

LSP wins because it can answer:

* Which overload or implementation is actually called
* What type a symbol is in context
* Cross-file and cross-module references with correct binding
* Call hierarchy, rename refactors, code actions

The trade is exactly what you said, LSPs are bulky, stateful, and per-language.

---

# Quick comparison table

| Dimension                  | AST-first tooling           | LSP-based tooling                   |
| -------------------------- | --------------------------- | ----------------------------------- |
| Core output                | Syntax tree, node ranges    | Semantic model exposed via protocol |
| Cross-file understanding   | Limited unless you build it | Built-in via project index          |
| Type awareness             | Usually none, or shallow    | Usually strong, language-dependent  |
| Scales for batch pipelines | Excellent                   | Often expensive, can be RAM heavy   |
| Works on broken code       | Often yes                   | Often degraded or fails features    |
| Security “early warning”   | Excellent                   | Good but heavier to run everywhere  |
| Refactors, code actions    | Limited                     | Best-in-class                       |
| Portability                | High (same pipeline shape)  | Low (each language server differs)  |

---

# Popular languages, practical stacks, and “bulk” reality

> Sizes below are “typical install artifacts”, they vary by platform, version, packaging, compression, and whether you include runtime dependencies.

## Language-by-language table (AST option vs LSP option)

| Language               | Common AST layer                                       |                      Common LSP server |                                                                                       Typical install size signal | What you gain with LSP that AST misses                                 |
| ---------------------- | ------------------------------------------------------ | -------------------------------------: | ----------------------------------------------------------------------------------------------------------------: | ---------------------------------------------------------------------- |
| JavaScript, TypeScript | Parser AST (often Tree-sitter or TS compiler parser)   | TypeScript language service (tsserver) |                                                          TypeScript npm package unpacked size ~23.6 MB ([npm][1]) | Accurate types, imports, project references, rename safety             |
| Python                 | Tree-sitter or lightweight parser AST                  |                Pylance, Pyright engine |                         Pyright npm unpacked size ~19 MB ([npm][2]), Pylance VSIX often ~29 MB ([vsixhub.com][3]) | Type-driven completion, better symbol resolution, smarter refactors    |
| Go                     | Tree-sitter or go/parser AST                           |                                  gopls |                                              Alpine reports installed size ~27.7 MiB ([Alpine Linux Packages][4]) | Module-aware symbol binding, workspace-wide references, code actions   |
| Rust                   | Tree-sitter or rustc-front-end style parsing           |                          rust-analyzer |                                                              Release assets commonly ~15 to 17.6 MB ([GitHub][5]) | Macro expansion awareness, trait resolution, real call hierarchy       |
| Java                   | Tree-sitter Java grammar, or parser front-end          |                         Eclipse JDT LS | JDT LS snapshots ~47 MB ([download.eclipse.org][6]), Alpine installed size ~50.2 MiB ([Alpine Linux Packages][7]) | Classpath, symbol resolution across jars, refactors that respect types |
| C, C++                 | Tree-sitter C/C++, clang AST layer                     |                                 clangd |                                               Win64 clangd size reported ~78,448,128 bytes (~75 MB) ([GitHub][8]) | Compilation database awareness, templates, overload resolution         |
| C#                     | Tree-sitter C# grammar, Roslyn parsing if you embed it |                              OmniSharp |                                         OmniSharp release assets include ~44 MB zips in some builds ([GitHub][9]) | Roslyn-level symbol binding, solution-wide understanding, refactors    |

---

# The “gaps” of ASTs, the ones that matter in practice

AST-only struggles when you need:

1. **Name binding**
   Is `Foo()` the local `Foo`, an import, a method on a type, a macro expansion, an overload, or a dynamic dispatch call?

2. **Types and overloads**
   Without types, “what does this call hit?” becomes probabilistic.

3. **Build context**
   C, C++ and Java especially depend on compile flags, include paths, classpaths. LSPs bake that in, AST-only must be taught it.

4. **Project indexing**
   You can build it yourself, but it’s a whole product, caching, invalidation, incremental updates.

That’s the real reason LSP “feels like it knows the truth”, it is hauling around the world needed to bind meaning.

---

# Why LSPs feel heavy (and why that isn’t just vibes)

Some language servers explicitly call out footprint challenges, for example gopls design notes discuss memory footprint concerns as projects grow. ([Go][10])

In other words, the heaviness is not accidental, it’s a consequence of doing semantic work continuously and incrementally.

---

# Practical recommendation: your “early warning” and “truth” split

## If your goal is security early warning and pipeline-scale scanning

Use AST-first as your default, then enrich selectively:

* AST rules catch 80 percent cheaply, everywhere
* LSP is invoked only when you need confirmation, disambiguation, or high-confidence call resolution

This matches your bias: lightweight, language-agnostic backbone, per-language semantic engines only where they earn their keep.

## A simple hybrid pattern that stays sane

1. **AST pass** (fast): flag suspect nodes, build a structural graph, store ranges and node kinds
2. **Resolve pass** (selective): for flagged hotspots, ask the LSP for symbol binding, types, call hierarchy
3. **Score pass**: early-warning score from AST, confidence boost from LSP when available
4. **Report**: show “what we know” and “what we infer”, keep auditability clean

This gives you “smoke alarms everywhere” plus “truth when it matters”, without making the whole system depend on bulky per-language servers.

---

# Mental model summary

* AST is your scalable, deterministic substrate for structural intelligence and early warning.
* LSP is your language-specific semantic oracle for correctness, refactors, and deep navigation.
* The best systems treat LSP as an enrichment layer, not the backbone.

---

# Viability Assessment: AST-Powered Dimensional Early Warning Engine

> Can tree-sitter ASTs provide enough structural information to power a fast, scalable early warning system across multiple dimensions? This section makes the case.

## The Premise

aOa does **not** aim to be a definitive security scanner, linter, or compliance auditor. It is an **early warning system** — a fast signal layer that says:

> "Someone should look at this."

The findings are not verdicts. They are flags. The user can:
- **Acknowledge** — "Yes, I see the concern, I'll address it."
- **Dismiss** — "I understand what you're saying, but this is intentional."
- **Ignore** — Tag it so it doesn't surface again.

This is a fundamentally different bar than what LSP-based tools aim for. We don't need type resolution. We don't need cross-file dataflow. We need **pattern recognition at speed** — structural shapes that correlate with known concerns.

## What the AST definitively knows

For any line of code, tree-sitter tells you:

1. **Node type** — Is this a function call? An assignment? A binary expression? A string literal? A control flow statement?
2. **Node text** — The exact source text of any node.
3. **Parent/child relationships** — What contains what. A string inside a call inside a loop inside a function.
4. **Sibling relationships** — What's next to what. Arguments to a function call. Arms of a conditional.
5. **Nesting depth** — How deep is this node? Branch complexity, callback nesting.
6. **Line/column positions** — Exact source location of every node.
7. **Named fields** — `receiver`, `arguments`, `condition`, `body`, `name` — semantic roles within syntax.

## What the AST does NOT know

1. **Types** — Is `db` a database handle or a variable named `db`? (AST sees identifier text only.)
2. **Cross-file references** — Where is this function defined? What does the import resolve to?
3. **Control flow across boundaries** — Does this tainted input eventually reach a sink in another file?
4. **Runtime behavior** — Is this branch actually reachable?

## Why that's enough for early warning

The things the AST doesn't know would matter if we were building a **proof system** — a tool that says "this IS a vulnerability." We're not. We're building a system that says "this LOOKS like a pattern that has historically been a problem."

Consider: a human code reviewer doesn't resolve types either when scanning for red flags. They see `db.Query("SELECT * FROM users WHERE id=" + userID)` and immediately flag it — not because they traced the type of `db` through the import chain, but because the **shape** is dangerous: string concatenation flowing into something that looks like a database call.

That's exactly what the AST gives us. Shape recognition.

**The accuracy bar for early warning:**

| Outcome | Acceptable? |
|---------|-------------|
| True positive (real concern flagged) | Yes — this is the goal |
| False positive (safe code flagged) | Yes, within reason — user dismisses it, no harm done |
| True negative (safe code not flagged) | Yes — no noise |
| False negative (real concern missed) | Acceptable — we're early warning, not exhaustive. Catch 70-80%, not 100%. |

A 70-80% detection rate with a <20% false positive rate is valuable. LSP-based tools aim for 95%+ but at 100x the cost and complexity. We trade precision for **speed, coverage, and zero-install universality**.

---

## Viability by Dimension

The question isn't just "does this work for security?" — it's "does this approach generalize across all six dimensional tiers?"

### Security (67 questions) — HIGH viability

AST structural patterns map directly to security anti-patterns:

| Pattern type | AST visibility | Confidence |
|---|---|---|
| SQL/command injection (concat → call) | Direct — binary_expression + call_expression | High |
| Hardcoded secrets (literal assignment) | Direct — string_literal in assignment context | High |
| Missing auth middleware | Structural — handler registration without decorator chain | Medium |
| Weak crypto (MD5, SHA1, ECB) | Text match — function name / constant | High |
| Path traversal (user input → file op) | Structural — request param → path operation | High |
| Deserialization of untrusted input | Text + structural — known-dangerous function + argument source | High |

**What we'd miss without LSP:** Tainted data flowing through 3+ function calls across files before hitting a sink. That's a SAST tool's job (Semgrep, CodeQL). We catch the direct patterns — which account for the majority of real-world vulnerabilities.

### Performance (est. 50-60 questions) — HIGH viability

| Pattern type | AST visibility | Confidence |
|---|---|---|
| N+1 query (loop → db call) | Direct — call_expression inside for_statement | High |
| Unbounded allocation (append in loop without pre-alloc) | Structural — append call inside for without make/cap | High |
| Mutex held over I/O | Structural — Lock() call → I/O call before Unlock() | Medium |
| Unclosed resource (open without defer close) | Structural — open call without corresponding close in same scope | High |
| String concatenation in loop (vs builder) | Direct — binary_expression with string inside for | High |
| Synchronous I/O in async context | Structural — blocking call inside async function | Medium |

**AST strength here:** Performance anti-patterns are almost entirely structural. You don't need types to see a database call inside a loop.

### Quality (est. 45-55 questions) — HIGH viability

| Pattern type | AST visibility | Confidence |
|---|---|---|
| God function (excessive branch count) | Direct — count if/switch/for nodes in function body | High |
| Ignored error (Go: `_, _ = f()`) | Direct — blank identifier in assignment from call | High |
| Deep nesting (>4 levels) | Direct — nesting depth from AST | High |
| Magic numbers (literal in non-const context) | Direct — numeric_literal not in const declaration | High |
| Dead code after return/break | Direct — statements after return_statement | High |
| Empty catch/except blocks | Direct — catch_clause with empty body | High |
| TODO/FIXME/HACK markers | Text — literal string match | High |

**AST strength here:** Quality metrics are the most natural fit for AST analysis. Complexity is literally tree depth and branching factor.

### Compliance (est. 30-40 questions) — MEDIUM viability

| Pattern type | AST visibility | Confidence |
|---|---|---|
| Known CVE function patterns | Text — AC match on known-bad function names/signatures | High |
| License header missing | Text — file doesn't start with expected comment block | High |
| PII in log output (email, SSN regex in log call) | Structural + regex — log call containing PII-shaped argument | Medium |
| Data retention (no TTL on stored data) | Low — requires understanding data lifecycle | Low |
| GDPR consent check missing | Low — requires cross-file flow analysis | Low |

**Honest limitation:** Some compliance questions require understanding data flow across the application, which is beyond single-file AST analysis. We flag what we can structurally see.

### Architecture (est. 35-45 questions) — MEDIUM-HIGH viability

| Pattern type | AST visibility | Confidence |
|---|---|---|
| Circular imports | Structural — import graph from AST (multi-file, but just import nodes) | High |
| Layer violation (handler importing DB directly) | Text + structural — import paths in wrong package | High |
| Global mutable state | Direct — package-level var declaration with assignment | High |
| Hardcoded config (magic strings for URLs, ports) | Text — URL/port patterns in non-config files | Medium |
| God class (too many methods) | Direct — count method declarations in type scope | High |
| Missing interface (concrete type passed everywhere) | Low — requires type analysis | Low |

### Observability (est. 20-25 questions) — HIGH viability

| Pattern type | AST visibility | Confidence |
|---|---|---|
| Swallowed error (empty catch, `_ = err`) | Direct — blank identifier or empty catch body | High |
| Leftover debug print (fmt.Println, console.log) | Text — known debug function names | High |
| Missing error context (return err without wrap) | Structural — return with bare error variable | High |
| Panic without recover | Structural — panic call without defer/recover in scope | Medium |
| Silent goroutine failure | Structural — go statement without error channel | Medium |

---

## Viability Verdict

| Dimension | Est. Questions | AST Viability | Notes |
|-----------|---------------|---------------|-------|
| Security | 67 | **High** | Direct structural patterns cover 70-80% of common vulns |
| Performance | 50-60 | **High** | Anti-patterns are inherently structural (loop + call) |
| Quality | 45-55 | **High** | Best fit — complexity IS tree structure |
| Compliance | 30-40 | **Medium** | Strong on pattern match, weak on data flow |
| Architecture | 35-45 | **Medium-High** | Import analysis and metrics work well; interface gaps are hard |
| Observability | 20-25 | **High** | Local patterns, all AST-visible |
| **Total** | **~250-290** | | |

**Estimated total question set: ~250-290 questions across 6 tiers producing a ~300-bit composite bitmask.**

### The honest boundaries

What this approach **will catch** (early warning territory):
- Direct injection patterns (concat → call, interpolation → call)
- Hardcoded secrets and credentials
- Structural complexity (god functions, deep nesting, high branching)
- Resource mismanagement (unclosed handles, mutex over I/O, N+1)
- Known-bad function usage (weak crypto, unsafe deserialization)
- Import/dependency anti-patterns
- Debug artifacts and swallowed errors

What this approach **will not catch** (beyond early warning):
- Multi-file tainted data flow (input → transform → transform → sink across 3 files)
- Type-dependent vulnerabilities (is this actually a database handle or just named `db`?)
- Runtime-conditional vulnerabilities (only exploitable under specific config)
- Business logic flaws (authorization checks that exist but are wrong)
- Race conditions requiring execution path analysis

**That's fine.** Those are SAST/DAST territory (Semgrep, CodeQL, Snyk). aOa is the fast, always-on canary. It's the difference between a smoke detector and a fire investigation team. You want both, but the smoke detector needs to be cheap, fast, and everywhere.

---

## The User Experience

When aOa finds something, it doesn't block. It surfaces.

**In search results:**
```
handleTransfer()  S:-23  P:0  Q:-4
```
Security debt of 23, no performance flags, minor quality issue.

**In the Recon tab:** Drill into a file, see which methods have flags, expand to see which questions triggered, dismiss or acknowledge individually.

**The workflow:**
1. aOa runs dimensional scan at index time (during `aoa init` and file watch re-index)
2. Findings persist in bbolt alongside the search index
3. Search results carry dimensional scores — you see them as you work
4. Recon tab gives the full dimensional view — filter by tier, sort by severity
5. User can tag findings: `dismiss` (intentional), `acknowledged` (will fix), or leave as-is
6. Dismissed findings don't count toward scores

This is lightweight. No CI pipeline. No config files. No server farm. One binary, running locally, answering ~290 yes/no questions per line at ~100ns each.

---

## Conclusion

Tree-sitter ASTs provide sufficient structural information to power a viable early warning dimensional analysis engine. The approach trades semantic precision for:

- **Speed** — 100ns/line, full project scan in seconds
- **Coverage** — 28 languages from one binary, same question set
- **Simplicity** — zero install, zero config, zero external dependencies
- **Interpretability** — every flag traces to a specific yes/no question

The bar is early warning, not proof. A 70-80% detection rate across ~290 questions, running continuously in the background, surfaced inline with search results — that's a meaningful signal layer that didn't exist before.

[1]: https://www.npmjs.com/package/typescript?utm_source=chatgpt.com "typescript"
[2]: https://www.npmjs.com/package/pyright?activeTab=readme&utm_source=chatgpt.com "pyright"
[3]: https://www.vsixhub.com/vsix/32420/?utm_source=chatgpt.com "Pylance 2025.12.104 VSIX (Latest Version)"
[4]: https://pkgs.alpinelinux.org/package/edge/community/x86/gopls?utm_source=chatgpt.com "gopls - Alpine Linux packages"
[5]: https://github.com/rust-lang/rust-analyzer/releases?utm_source=chatgpt.com "Releases · rust-lang/rust-analyzer"
[6]: https://download.eclipse.org/justj/?file=jdtls%2Fsnapshots&utm_source=chatgpt.com "snapshots - Downloads | The Eclipse Foundation"
[7]: https://pkgs.alpinelinux.org/package/edge/community/x86_64/jdtls?utm_source=chatgpt.com "jdtls - Alpine Linux packages"
[8]: https://github.com/clangd/clangd/issues/2341?utm_source=chatgpt.com "The new version of clangd is almost twice as large ..."
[9]: https://github.com/OmniSharp/omnisharp-roslyn/releases/?utm_source=chatgpt.com "Releases · OmniSharp/omnisharp-roslyn"
[10]: https://go.dev/gopls/design/design?utm_source=chatgpt.com "Gopls: Design"
