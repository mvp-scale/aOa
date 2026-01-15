# aOa vs Standard Search: A Token Efficiency Comparison

## What is aOa?

**aOa (Angle of Attack)** is a semantic code search tool designed for AI agents. It provides:
- **Fast indexed search** (~1-20ms per query)
- **Function-level results** with line ranges (e.g., `MyFunc()[10-50]`)
- **Semantic tags** indicating code purpose (`#http #event #handler`)
- **Intent tracking** for understanding developer workflow

The hypothesis: By returning richer, more targeted results, aOa reduces the need for follow-up searches and file reads, dramatically cutting token usage.

---

## Test Methodology

### Task
Research how Jambonz (a VoIP platform) handles WebSocket reconnection and error recovery.

### Research Questions
1. How does the Feature Server handle WebSocket disconnection to custom apps?
2. What retry policies exist?
3. How are sessions recovered after a reconnect?

### Expected Deliverables
- Key files and line ranges
- The retry/reconnection architecture
- Configurable options for resilience

---

## Agent Prompts

### aOa Agent Prompt

```
Research how Jambonz handles WebSocket reconnection and error recovery.

CRITICAL: Use `aoa grep` for ALL searches. Do NOT use Grep or Glob tools. aOa is 100x faster.

Examples:
- `aoa grep "reconnect retry"`
- `aoa grep "websocket error"`
- `aoa grep "connection lost"`

The search results will give you function names with line ranges like `MyFunction()[10-50]`. Use these ranges for targeted reads if needed.

Find:
1. How does the Feature Server handle WebSocket disconnection to custom apps?
2. What retry policies exist?
3. How are sessions recovered after a reconnect?

Report back with:
- Key files and line ranges
- The retry/reconnection architecture
- Any configurable options for resilience
```

### Standard Grep/Glob Agent Prompt

```
Research how Jambonz handles WebSocket reconnection and error recovery.

CONSTRAINTS:
- Use ONLY Grep and Glob tools for searching
- Do NOT use `aoa grep` or any aoa commands
- Do NOT use any MCP tools (no mcp__* tools)
- Stay local - only search files in /home/corey/projects/voice-ai/
- Search in the jambonz-source directory

Find:
1. How does the Feature Server handle WebSocket disconnection to custom apps?
2. What retry policies exist?
3. How are sessions recovered after a reconnect?

Report back with:
- Key files and line ranges
- The retry/reconnection architecture
- Any configurable options for resilience
```

---

## Fairness Considerations

The initial non-aOa test had to be constrained because:

1. **MCP Server Usage**: The agent immediately tried to use external MCP documentation servers (`mcp__jambonz-feature-server__*`), which would fetch remote content and skew the comparison.

2. **External Resources**: Without constraints, the agent would use web fetches and external documentation rather than local code search.

3. **Search Scope**: Both agents needed to search the same local codebase (`jambonz-source/`) to ensure comparable workloads.

The constraints ensured both agents:
- Searched only local files
- Used their respective search tools (aOa vs Grep/Glob)
- Answered the same research questions
- Produced comparable deliverables

---

## Results

### Token Usage

| Metric | aOa Agent | Grep/Glob Agent | Difference |
|--------|-----------|-----------------|------------|
| **Input Tokens** | 5,252 | 59,207 | **11.3x fewer** |
| **Output Tokens** | 58 | 36 | Similar |
| **Tool Calls** | 21 | 14 | +50% more |
| **Total Tokens** | ~5,310 | ~59,243 | **11x fewer** |

### Key Insight

The aOa agent made **more tool calls** but used **11x fewer tokens**.

This counterintuitive result occurs because:

1. **aOa returns compact, structured results**: Each search returns function names with line ranges and semantic tags, not raw file content.

2. **Grep returns raw content**: Each Grep search returns matching lines with context, requiring more tokens to process.

3. **Reduced follow-up reads**: aOa's line ranges (`[865-1093]`) enable targeted reads, while Grep often requires reading entire files to understand context.

---

## Quality Comparison

Both agents found the same core architecture:

| Component | Location Found |
|-----------|----------------|
| Main handler | `ws-requestor.js` |
| Retry policies | URL hash `#rc=X&rp=Y` |
| Backoff strategy | 500ms → 1000ms → 2000ms → +2000ms |
| Session recovery | `session:reconnect` event |
| Configuration | `config.js` env vars |

**Conclusion**: Same research quality, 11x token difference.

---

## Cost Implications

At typical Claude API pricing (~$3/M input tokens, ~$15/M output tokens):

| Agent | Input Cost | Output Cost | Total |
|-------|------------|-------------|-------|
| aOa | $0.016 | $0.0009 | **$0.017** |
| Grep/Glob | $0.178 | $0.0005 | **$0.178** |

**Savings per research task**: ~$0.16 (10x cheaper)

For teams running hundreds of research tasks per day, this compounds significantly.

---

## Why aOa Works

### Traditional Search Flow
```
Grep "reconnect" → 200 lines of matches → Read file A →
Grep "retry" → 150 lines → Read file B →
Grep "session" → 300 lines → Read file C →
... repeat until understanding emerges
```

### aOa Search Flow
```
aoa grep "reconnect retry" →
  WsRequestor()._scheduleReconnect()[461-479] #retry #websocket
  WsRequestor().request()[72-315] #session #reconnect

→ Targeted read of lines 461-479 only
→ Done
```

The difference: **Coordinates vs Content**

aOa gives you precise locations. Standard search gives you content to wade through.

---

## Reproducibility

To reproduce this test:

1. Install aOa in your project
2. Run the same prompts with isolated agents
3. Extract token counts from agent output files:

```bash
# Token extraction
grep -o '"input_tokens":[0-9]*' agent_output.json | \
  awk -F: '{sum+=$2} END {print sum}'
```

---

## Conclusion

For research and exploration tasks, aOa provides:
- **11x token reduction** with equivalent output quality
- **Faster iteration** through millisecond search times
- **Better targeting** via function-level line ranges
- **Semantic understanding** through intent tags

The angle of attack matters. Search smarter, not harder.
