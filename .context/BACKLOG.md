# Backlog

[Board](../GO-BOARD.md) | [Completed](COMPLETED.md) | [Backlog](#backlog)

> Deferred items, future ideas, and research topics not yet on the active board.

---

## Deferred Research

| Item | Context | When to revisit |
|------|---------|-----------------|
| Neural 1-bit embeddings | Pre-trained models encode semantic similarity, not security properties. Fine-tuned classifier needs thousands of labeled examples. | Only if AC/AST pattern library hits ceiling on novel code shapes |
| WebSocket push for dashboard | 2s poll works. WebSocket adds complexity for marginal UX improvement. | If users report lag or if real-time conversation feed demands it |
| Glob token cost from JSONL | Open question #6: Do session logs expose token counts per tool result? Unblocks AT-02/AT-08 with real numbers vs heuristic. | Before L0.9/L0.10 implementation |

## Future Dimensions (post L5.6-L5.8)

| Tier | Questions | Priority |
|------|-----------|----------|
| Compliance (CVE patterns, licensing, data handling) | ~30-40 | Medium |
| Architecture (import health, API surface, anti-patterns) | ~35-45 | Medium |
| Observability (silent failures, debug artifacts) | ~20-25 | Low |

## Ideas

| Idea | Notes |
|------|-------|
| `aoa peek` command | Locate-style command: give file + method, see just that section. More useful than raw locate. |
| Multi-project daemon | Single daemon managing multiple project indexes. v3 scope. |
| Grammar marketplace | Community-contributed detection patterns (YAML). Like Semgrep rules but for aOa's bitmask engine. |
| `aoa shell-init` | Shell integration for transparent grep/find aliasing. Part of alias strategy (open question #3). |
