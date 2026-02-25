 # Goals

> Non-negotiable. Every plan, code change, and architectural decision must be validated against each goal independently.

| Goal | Constraint |
|------|-----------|
| **G0** | **Speed** — 50-120x faster than Python. Sub-ms search, <200ms startup, <50MB memory. No O(n) on hot paths. |
| **G1** | **Parity** — Zero behavioral divergence from Python. Test fixtures are source of truth. |
| **G2** | **Two Binaries, Clean Split** — `aoa` works standalone with zero deps. `aoa-recon` is optional; when installed it enhances `aoa` through a defined bridge. `aoa` must never depend on `aoa-recon` being present. |
| **G3** | **Agent-First** — Drop-in shim for grep/egrep/find. Three Unix modes: direct (`grep pat file`), pipe (`cmd | grep pat`), index (`grep pat` → O(1) daemon). Same flags, same output format, same exit codes. Agents never know it's not GNU grep. |
| **G4** | **Clean Architecture** — Hexagonal. Domain logic dependency-free. External concerns behind interfaces. No feature entanglement. |
| **G5** | **Self-Learning** — Adaptive pattern recognition. observe(), autotune, competitive displacement. |
| **G6** | **Value Proof** — Surface measurable savings. Context runway, tokens saved, sessions extended. |
