# aOa-recon — Structural Code Scanner

**500+ languages. AST-powered. Early recon for your entire codebase.**

---

aOa-recon is the tree-sitter companion to [aOa](https://www.npmjs.com/package/@mvpscale/aoa). It parses your code into abstract syntax trees and scans for patterns across multiple dimensions — structure, complexity, duplication, and more.

## What it does

Most code analysis tools are language-specific, subscription-gated, or both. aOa-recon uses tree-sitter to build a unified AST view across 500+ languages, then scans for patterns regardless of what you're writing in.

- **500+ languages** — one parser, unified view, no per-language plugins
- **Multi-dimensional scanning** — symbols, structure, complexity patterns
- **Early recon** — surface areas of inefficiency before they become problems
- **Searchable results** — feeds into aOa's O(1) index for instant lookup
- **No subscription required** — runs locally, your code stays on your machine

## Install

```bash
npm install -g @mvpscale/aoa-recon
```

aOa-recon works alongside aOa. Install both:

```bash
npm install -g @mvpscale/aoa @mvpscale/aoa-recon
```

## How it works

When you run `aoa init`, aOa-recon extracts symbols and structural metadata from every file in your project. This feeds into the aOa search index, giving you richer results — not just text matches, but functions, classes, imports, and their relationships.

```
Source code → tree-sitter AST → symbol extraction → aOa index
```

Without aOa-recon, aOa still works — it just indexes tokens from file content. With aOa-recon, you get structural awareness on top.

## Not a replacement for security tools

aOa-recon identifies patterns and inefficiencies. It is not a SAST scanner or a replacement for standard security measures. Think of it as early recon — a fast, broad sweep that shows you what's there so you know where to look.

## License

MIT
