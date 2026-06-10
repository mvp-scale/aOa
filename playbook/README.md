# playbook/ — Architecture Views working area

The working pipeline for the **architecture-views** product exploration: a viewer that
renders standard architectural diagrams (C4, data flow, ER, deployment) from a JSON
contract, derived live from real code where we have extractors and honestly marked
SIMULATED where we don't.

Related research: `.context/details/2026-06-10-reference-architecture-rendering-playbook.md`
(rendering playbook), `2026-06-09-production-release-documentation-set.md` (document
matrix), `decisions/2026-06-08-architecture-views-product.md` (ADR).

## Run it

```bash
python3 playbook/build_c4_mockup.py            # regenerate archmodel.json + viewer
cd playbook && python3 -m http.server 8777     # serve
# open http://127.0.0.1:8777/architecture-c4.html
```

Headless verification (the loop used throughout):

```bash
chromium-browser --headless=new --no-sandbox --screenshot=out.png \
  --window-size=1700,1100 --virtual-time-budget=30000 \
  "http://127.0.0.1:8777/architecture-c4.html?auto=component:1200"
```

## Files

| File | What |
|---|---|
| `build_c4_mockup.py` | Generator: extracts real data (`go list`, graphify imports, bitmask proxy, git) + simulated estates → `archmodel.json` + viewer HTML |
| `architecture-c4.html` | The viewer (React Flow + elkjs via esm.sh, no build step). Generated — edit the `.py`, not this |
| `archmodel.json` | The contract (`aoa.archmodel/v1-mock`): estates → scopes → views, provenance per view |
| `estates/` | Synthetic estate shards (test fixtures; clean + faulted variants) |
| `arch_proxy.jsonl` | *(untracked — 1.3MB derived data, repo 1MB limit)* regenerate via the read-only extractor over `.aoa/aoa.db`; overlays degrade gracefully when absent |
| `generate_docs.py` / `architecture.md` / `architecture.html` | Generated arc42/C4 architecture document with Mermaid |
| `sbom.cdx.json` | Real CycloneDX 1.6 SBOM from the module graph |
| `build_blueprint_viewer.py` / `blueprint-viewer.html` | Viewer for the doc-construction blueprint |
| `sanity.png` | Last verification screenshot (regenerable, not meaningful history) |

## Viewer URL params

`?estate=` `?scope=` `?level=` `?dir=DOWN|RIGHT` `?density=compact|comfort`
`?ov=concerns,changed` (overlays) · `?model=<file>` (load an alternate contract, e.g. `estates/x-faulted.json`)
`?auto=<view>:<ms>` (test hook: simulates a user click — verifies the click path, not URL load)

## Status / provenance language

- **● REAL · derived** — rendered from data extracted now (green)
- **◌ SIMULATED · sourceable** — drawn from hand-authored data that names the source it *would* derive from (yellow)
- **MIXED** — system real, externals inferred (cyan)
- Header chips: **⚠ pattern problems** (detected anti-patterns: band violations, cycles, god/orphan components, tagged issues like `direct-db`) · **◌ model issues** (structural faults caught by the load validator: dangling edges quarantined, duplicate ids, budget overflow auto-collapsed)
