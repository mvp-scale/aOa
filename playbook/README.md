# playbook/ — Architecture Views working area

Sandbox for the **architecture-views** idea (feature branch, not part of the main aOa
build): a viewer that renders standard architectural diagrams (C4, data flow, ER,
deployment) from a JSON contract — derived live from real code where extractors
exist, and honestly marked SIMULATED where they don't.

Related research: `.context/details/2026-06-10-reference-architecture-rendering-playbook.md`,
`2026-06-09-production-release-documentation-set.md`, `decisions/2026-06-08-architecture-views-product.md`.

## Layout

```
playbook/
  mockups/        ← open these in a browser
    architecture-c4.html    the main viewer (React Flow + elkjs)
    archmodel.json          the contract it renders (estates → scopes → views)
    estates/                synthetic estate fixtures (clean + faulted + manifest)
    architecture.html       generated arc42/C4 document (Mermaid)
    blueprint-viewer.html   doc-construction blueprint viewer
  generators/     ← scripts that build the mockups (edit these, not the HTML)
    build_c4_mockup.py      real data (go list, graphify imports, bitmask, git)
                            + simulated estates → archmodel.json + viewer
    generate_docs.py        architecture.md + CycloneDX SBOM
    build_blueprint_viewer.py
  data/           ← inputs & generated non-viewable artifacts
    arch_proxy.jsonl        (untracked, 1.3MB) per-method bitmask proxy from .aoa/aoa.db;
                            overlays degrade gracefully when absent
    architecture.md         generated document source
    sbom.cdx.json           real CycloneDX 1.6 SBOM
  screenshots/    ← headless verification captures (regenerable)
```

## Run it

```bash
python3 playbook/generators/build_c4_mockup.py     # regenerate contract + viewer
cd playbook && python3 -m http.server 8777         # serve
# open http://127.0.0.1:8777/mockups/architecture-c4.html
```

Headless verification (the loop used throughout):

```bash
chromium-browser --headless=new --no-sandbox --screenshot=playbook/screenshots/out.png \
  --window-size=1700,1100 --virtual-time-budget=30000 \
  "http://127.0.0.1:8777/mockups/architecture-c4.html?auto=component:1200"
```

## Viewer URL params

`?estate=` `?scope=` `?level=` `?dir=DOWN|RIGHT` `?density=compact|comfort`
`?ov=concerns,changed` (overlays) · `?model=<file>` (alternate contract, e.g. `estates/smoke.json`)
`?auto=<view>:<ms>` (test hook: simulates a user click — verifies the click path, not URL load)

## Status / provenance language

- **● REAL · derived** — rendered from data extracted now (green)
- **◌ SIMULATED · sourceable** — hand-authored data naming the source it *would* derive from (yellow)
- **MIXED** — system real, externals inferred (cyan)
- Header chips: **⚠ pattern problems** (detected anti-patterns: band violations, cycles,
  god/orphan components, tagged issues like `direct-db`) · **◌ model issues** (structural
  faults caught by the load validator: dangling edges quarantined, duplicate ids,
  budget overflow auto-collapsed). Corrupt model files produce a diagnostic red banner.

## Test harness status

Smoke-proven (see `mockups/estates/smoke*.json` + `screenshots/smoke-*.png`): every
Class A fault highlighted, every Class B fault contained or failed diagnostically.
Next: the 10-estate generation campaign (clean + faulted + manifest per estate, scored
against ground truth).
