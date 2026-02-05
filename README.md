# aOa - Angle O(1)f Attack

<p align="center">
  <img src="images/hero.png" alt="The O(1) Advantage" width="600">
</p>

> **5 angles. 1 attack.** Save your tokens. Save your time. Develop awesome code.

---

## Claude Code Is Amazing. Until It Isn't. 

Here's what nobody talks about.

**Generating new code?** Claude is incredible. You describe what you want, it writes it. Magic.

**But integrating? Refactoring? Pivoting?** That's where the wheels fall off.

Watch what happens when Claude needs to understand your existing codebase:

```
Claude: "Let me search for authentication..."
        [Grep tool - 2,100 tokens]
Claude: "Let me also check login handlers..."
        [Grep tool - 1,800 tokens]
Claude: "I should look at session management..."
        [Grep tool - 1,400 tokens]
Claude: "Let me read these 8 files to understand the pattern..."
        [Read tool x8 - 12,000 tokens]
Claude: "Now I understand."
```

**17,000+ tokens.** Just to find code you could have pointed to in 5 seconds.

And here's the part that drove me crazy: **Claude reads the same files. Every. Single. Session.**

Your auth system doesn't change between sessions. But Claude doesn't remember. So it burns tokens rediscovering what it already learned yesterday.

---

## I Built This Because I Was Tired

I couldn't figure out how to manage context across sessions. MCP servers felt like overkill. LSP setups were fragile. I just wanted Claude to *remember* what mattered.

So I built aOa. A passion project that turned into something real.

**The idea:** What if we could semantically compress everything Claude learns—every file it reads, every pattern it discovers—and feed it back automatically?

No configuration. No scaffolding. Just hooks that watch Claude work and learn from it.

---

## Just Watch It Work

```bash
aoa intent
```

That's it. Run that command and watch aOa learn in real-time. No secrets. No magic.

Here's a real session—building an AI dispatch agent for emergency response:

```
aOa Activity                                                 Session

SAVINGS         ↓847k tokens ⚡47m (rolling avg)
PREDICTIONS     97% accuracy (312 of 321 hits)
HOW IT WORKS    aOa finds exact locations, so Claude reads only what it needs

─────────────────────────────────────────────────────────────────────────────────────────────

ACTION     SOURCE   ATTRIB       aOa IMPACT                TAGS                                TARGET
Grep       Claude   aOa guided   ↓94% (4.2k → 252)         #llm #orchestration #dispatch       agent/orchestrator.py:89
Grep       Claude   aOa guided   ↓91% (6.1k → 549)         #streaming #realtime #websocket     core/stream_handler.py:156-203
Read       Claude   aOa guided   ↓87% (3.8k → 494)         #prompt-engineering #few-shot       prompts/triage.py:34-89
Grep       Claude   aOa guided   ↓89% (2.9k → 319)         #tool-use #function-calling         tools/dispatch.py:45-112
Edit       Claude   -            -                         #agent #memory #context-window      agent/memory.py:267
Grep       Claude   aOa guided   ↓96% (5.7k → 228)         #embeddings #retrieval #rag         retrieval/vector_store.py:45
Read       Claude   aOa guided   ↓82% (8.1k → 1.4k)        #fine-tuning #adapter #lora         training/adapter.py:23-89
Grep       Claude   aOa guided   ↓93% (3.2k → 224)         #safety #guardrails #moderation     safety/content_filter.py:78-134
Edit       Claude   -            -                         #agent #decision-tree #routing      agent/router.py:189
Bash       Claude   -            -                         #deployment #docker #gpu            docker compose up -d --build
```

Every tool call, aOa captures the semantic fingerprint. It builds a map of your codebase—not just files, but *meaning*.

When you come back tomorrow? **That context is already there.**

---

## The Difference Is Stark

<p align="center">
  <img src="images/convergence.png" alt="Five angles, one attack" width="500">
</p>

**Without aOa:**
```
You: "Fix the auth bug"
Claude: [17 tool calls, 4 minutes of searching, 17k tokens burned]
Claude: "Found it. Line 47 in auth.py."
```

**With aOa:**
```
You: "Fix the auth bug"
aOa: [Context injected: auth.py, session.py, middleware.py]
Claude: "I see the issue. Line 47."
```

**150 tokens.** Same result. **99% savings.**

Want the technical breakdown? See [AOA_COMPARISON.md](AOA_COMPARISON.md) for a real-world 11x token reduction case study.

---

## No LSP. No MCP. Just Semantic Compression.

aOa is a **semantically compressed, intent-driven, predictive code intelligence engine**.

- **60+ languages** supported—one system, zero config
- **O(1) lookup**—same speed whether you have 100 files or 100,000
- **Self-learning**—gets smarter with every tool call
- **Predictive**—has files ready before you ask

It taps into Claude Code hooks. That's it. No servers to configure. No language-specific setup. Just install, init, and go.

---

## The Five Angles

| Angle | What It Does |
|-------|--------------|
| **Search** | O(1) indexed lookup—same syntax as grep, 100x faster |
| **File** | Navigate structure without reading everything |
| **Behavioral** | Learns your work patterns, predicts next files |
| **Outline** | Semantic compression—searchable by meaning, not just keywords |
| **Intent** | Tracks session activity, shows savings in real-time |
| **Intel** | Domains (local semantic labels) + external repos (isolated) |

All angles converge into **one confident answer**.

---

## Quick Start

### 1. Install Once

```bash
git clone https://github.com/CTGS-Innovations/aOa
cd aOa
./install.sh
```

This starts the aOa services in Docker. One-time setup—works for all your projects.

### 2. Enable Per Project

```bash
cd your-project
aoa init
```

Each project gets its own isolated index. Your work-project doesn't pollute your side-project.

### 3. Start Searching

Your codebase is already indexed. Try it:

```bash
aoa grep handleAuth
```

Instant results. O(1) lookup.

### 4. Optional: Semantic Domains

Run `/aoa-start` in Claude Code to generate semantic domains:

```
/aoa-start
```

This adds 24 semantic domains that enrich your search results. You'll see:

```
file:Class.method[range]:line <grep output> @domain #tags
```

After completion, your status line shows:

```
⚡ aOa 🟢 42 │ ↓12k ⚡1m30s saved │ ctx:28k/200k (14%) │ Opus 4.5
```

---

## What You Get

**A status line built for developers.** Everything you need at a glance.

Your status line evolves as aOa learns:

| Stage | Status Line |
|-------|-------------|
| Learning | `⚡ aOa ⚪ 5 │ 4.2ms │ calibrating...` |
| Learning | `⚡ aOa ⚪ 28 │ 3.1ms │ almost ready` |
| Predicting | `⚡ aOa 🟡 35 │ ↓2k ⚡12s saved │ ctx:15k/200k (8%)` |
| Confident | `⚡ aOa 🟢 69 │ ↓80k ⚡2m58s saved │ ctx:36k/200k (18%) │ Opus 4.5` |
| Long session | `⚡ aOa 🟢 247 │ ↓1.8M ⚡1h32m saved │ ctx:142k/200k (71%) │ Opus 4.5` |

**What that long session means:** In a 1-2 hour coding session, aOa captured 247 intents, saved 1.8 million tokens (that's real money), and cut 1.5 hours of search time. You're using 71% of your context window, running Opus 4.5. All visible at a glance.

**Traffic lights:**
- ⚪ **Gray** = Learning your patterns (0-30 intents)
- 🟡 **Yellow** = Predicting, building accuracy
- 🟢 **Green** = Confident predictions, showing savings

**What you see:**
- Intent count (how much aOa has learned this session)
- Token & time savings (what you've avoided burning)
- Context usage (how much of your window is used)
- Model (which Claude you're running)

Every tool call teaches it your patterns. The more you code, the smarter it gets.

---

## Your Data. Your Control.

- **Local-first**—runs in Docker on your machine
- **No data leaves**—your code stays yours
- **Open source**—MIT licensed, fully auditable
- **Explainable**—`aoa intent recent` shows exactly what it learned

You host it. You own it. Your data. Your control.

---

## Who This Is For

You, if you've ever:

- Watched Claude burn 10 minutes rediscovering code it read yesterday
- Hit your weekly token limit on a Wednesday
- Felt the pain of "let me search for that again..."
- Wanted to just *code* without managing context yourself

This isn't for people who love configuring tools. It's for people who want to ship.

---

## How It Works

**One install. Many projects.**

```
~/.aoa/                  ← Global install (Docker services, CLI)
├── your-work-project/   ← aoa init (hooks, indexed, isolated)
├── your-side-project/   ← aoa init (hooks, indexed, isolated)
└── another-project/     ← aoa init (hooks, indexed, isolated)
```

Each project gets its own index. Your work doesn't pollute your side project.

---

## The Bottom Line

**Less than one minute.** Clone, install, init. Done.

```bash
# One-time global install
git clone https://github.com/CTGS-Innovations/aOa && cd aOa && ./install.sh

# Enable in any project
cd your-project && aoa init
```

Your codebase is already indexed. You're already searching faster. You're already saving tokens.

---

## Why Not a Plugin?

We'd love to make this a one-click Claude Code plugin. But the architecture requires background services—indexing, prediction, intent capture—that plugins can't provide yet.

So it runs as Docker. Single container or docker-compose. Your choice.

**Fully transparent.** Look at every line of code. Nothing hidden. Nothing phoning home.

---

## Not For You?

No hard feelings.

**Remove from a project:**
```bash
aoa remove
```

**Full uninstall (global):**
```bash
cd ~/.aoa && ./install.sh --uninstall
```

Everything gets removed. We leave no trace. **Boy Scouts.**

---

**Stop burning tokens. Start shipping code.**

```
⚡ aOa 🟢 247 │ ↓1.8M ⚡1h32m saved
```

*That's a real session. That could be you.*
