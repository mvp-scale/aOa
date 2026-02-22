# aOa — Angle O(1)f Attack

**5 angles. 1 attack. 1 binary. Save your tokens. Save your time.**

---

CLI coding agents are inefficient. When Claude, Gemini, or any AI agent runs `grep` on your repo, it ignores `.gitignore`, scans everything — `node_modules`, build artifacts, vendored deps — and returns bloated, unorganized results. The agent then traverses that tree, reads every file, and burns through tokens you never see. A single grep can waste 100K+ tokens. They don't show you this because it doesn't look good.

aOa exposes it. And fixes it.

## What it does

aOa is a Go binary that sits in front of `grep` and `egrep`. When an AI agent calls grep, aOa intercepts it and returns precise, token-efficient results using an O(1) indexed lookup instead of brute-force file scanning.

- **O(1) token lookup** — pre-indexed search, not line-by-line scanning
- **Respects .gitignore** — never wastes tokens on files you already exclude
- **Drop-in replacement** — agents don't know the difference, they just get better results
- **95-99% token savings** — on real codebases, measured, not estimated
- **Zero config** — one binary, no Docker, no runtime deps

## Install

```bash
npm install -g @mvpscale/aoa
```

## Setup

```bash
# Index your project
aoa init

# Activate for AI tools — add to ~/.bashrc or ~/.zshrc:
alias claude='PATH="$HOME/.aoa/shims:$PATH" claude'
alias gemini='PATH="$HOME/.aoa/shims:$PATH" gemini'
```

That's it. Your agent now uses aOa for every grep and egrep call.

## Commands

| Command | What it does |
|---------|-------------|
| `aoa init` | Index the current project |
| `aoa grep <pattern>` | Search with O(1) token lookup |
| `aoa egrep <pattern>` | Extended regex search |
| `aoa locate <path>` | Find files by path pattern |
| `aoa tree` | Project structure overview |
| `aoa health` | Check daemon and index status |
| `aoa daemon` | Run the background indexer |

## Why "Angle of Attack"

You're not seeing the true token economics of your AI tools. The waste is hidden at the lowest layer — the shell commands agents run on your behalf. aOa puts you back in control at that base layer, shielding you from token waste and giving you visibility into what's actually happening. That's the angle of attack.

## License

MIT
