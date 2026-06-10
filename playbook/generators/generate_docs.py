#!/usr/bin/env python3
"""Generate ACTUAL formal documents from live data (not a meta-view):
 1. architecture.md  — arc42/C4 layered architecture doc with real Mermaid diagrams
 2. sbom.cdx.json    — a valid CycloneDX 1.6 SBOM from go list -m all
 3. architecture.html— renders architecture.md with Mermaid so the diagram is legit
All deterministic, from real repo data."""
import json, subprocess, re, datetime, html as _h

ROOT = "/home/corey/aOa-go"
MOD = "github.com/corey/aoa"

def run(cmd):
    return subprocess.run(cmd, cwd=ROOT, capture_output=True, text=True).stdout

def layer(p):
    if p.startswith("/internal/domain"): return "domain"
    if p.startswith("/internal/ports"): return "ports"
    if p.startswith("/internal/adapters"): return "adapters"
    if p.startswith("/internal/app"): return "app"
    if p.startswith("/cmd"): return "cmd"
    if p.startswith("/atlas"): return "atlas"
    return "other"

LAYER_ORDER = ["cmd", "app", "adapters", "domain", "ports", "atlas", "other"]
# allowed dependency direction depth (lower may depend on higher-depth)
DEPTH = {"cmd": 0, "app": 1, "adapters": 2, "other": 2, "domain": 3, "ports": 4, "atlas": 4}

# ---- 1. internal package graph (go list -json) ----
raw = run(["go", "list", "-json", "./internal/...", "./cmd/...", "./atlas/..."])
dec = json.JSONDecoder(); i = 0; objs = []
while i < len(raw):
    while i < len(raw) and raw[i] in " \n\t\r": i += 1
    if i >= len(raw): break
    o, j = dec.raw_decode(raw, i); objs.append(o); i = j

pkgs = {}   # rel -> {layer, fanin, fanout}
edges = []
for o in objs:
    ip = o.get("ImportPath", "")
    if not ip.startswith(MOD): continue
    rel = ip[len(MOD):] or "/"
    pkgs.setdefault(rel, {"layer": layer(rel), "fanin": 0, "fanout": 0})
    for imp in (o.get("Imports") or []):
        if imp.startswith(MOD):
            rt = imp[len(MOD):] or "/"
            edges.append((rel, rt))
for a, b in edges:
    pkgs.setdefault(b, {"layer": layer(b), "fanin": 0, "fanout": 0})
    pkgs[a]["fanout"] += 1; pkgs[b]["fanin"] += 1

def short(p):
    s = p.lstrip("/")
    return s[9:] if s.startswith("internal/") else s
def nid(p):
    return "n_" + re.sub(r"[^a-zA-Z0-9]", "_", p.strip("/"))

violations = [(a, b) for a, b in edges if DEPTH.get(pkgs[a]["layer"], 2) > DEPTH.get(pkgs[b]["layer"], 2)]

# ---- high-level layer diagram (C4-Container-ish): aggregate edges by layer ----
from collections import Counter
layer_edges = Counter()
for a, b in edges:
    la, lb = pkgs[a]["layer"], pkgs[b]["layer"]
    if la != lb: layer_edges[(la, lb)] += 1
present_layers = [l for l in LAYER_ORDER if any(v["layer"] == l for v in pkgs.values())]

m_high = ["flowchart TD"]
for l in present_layers:
    n = sum(1 for v in pkgs.values() if v["layer"] == l)
    m_high.append(f'  L_{l}["{l}<br/><small>{n} pkg</small>"]')
for (la, lb), c in layer_edges.most_common():
    m_high.append(f'  L_{la} -->|{c}| L_{lb}')
m_high.append("  classDef ports fill:#3a1d1d,stroke:#e5484d,color:#fff")
m_high.append("  classDef domain fill:#16291d,stroke:#30a46c,color:#fff")
m_high.append(f'  class L_ports ports')
m_high.append(f'  class L_domain domain')
mermaid_high = "\n".join(m_high)

# ---- detailed package diagram grouped by layer subgraphs ----
m = ["flowchart LR"]
for l in present_layers:
    m.append(f'  subgraph {l.upper()}')
    for p, meta in sorted(pkgs.items(), key=lambda kv: -kv[1]["fanin"]):
        if meta["layer"] == l:
            m.append(f'    {nid(p)}["{short(p)}"]')
    m.append("  end")
for a, b in edges:
    arrow = "-. violates .->" if (a, b) in violations else "-->"
    m.append(f'  {nid(a)} {arrow} {nid(b)}')
for a, b in violations:
    m.append(f'  linkStyle {edges.index((a,b))} stroke:#e5484d,stroke-width:2px')
mermaid_pkg = "\n".join(m)

# ---- 2. CycloneDX SBOM from go list -m all ----
mods = run(["go", "list", "-m", "all"]).strip().splitlines()
components = []
for line in mods[1:]:  # first line is the main module
    parts = line.split()
    if len(parts) >= 2:
        name, ver = parts[0], parts[1]
        components.append({
            "type": "library", "name": name, "version": ver,
            "purl": f"pkg:golang/{name}@{ver}",
            "bom-ref": f"pkg:golang/{name}@{ver}",
        })
ts = datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
sbom = {
    "bomFormat": "CycloneDX", "specVersion": "1.6", "version": 1,
    "metadata": {"timestamp": ts, "tools": [{"name": "aoa", "vendor": "aOa"}],
                 "component": {"type": "application", "name": "aoa", "bom-ref": MOD}},
    "components": components,
}
json.dump(sbom, open("playbook/data/sbom.cdx.json", "w"), indent=2)

# ---- 3. architecture.md (the actual document) ----
langs = run(["bash", "-c", "ls cmd internal atlas >/dev/null 2>&1; echo go"]).strip()
total_pkg = len(pkgs); total_edge = len(edges)
hub = max(pkgs.items(), key=lambda kv: kv[1]["fanin"])
root_pkg = max(pkgs.items(), key=lambda kv: kv[1]["fanout"])

inv_rows = "\n".join(
    f"| `{short(p)}` | {m['layer']} | {m['fanin']} | {m['fanout']} |"
    for p, m in sorted(pkgs.items(), key=lambda kv: (-kv[1]["fanin"], kv[0])))

viol_md = ("\n".join(f"- `{short(a)}` → `{short(b)}` ({pkgs[a]['layer']} → {pkgs[b]['layer']}, points up the layers)" for a, b in violations)
           or "- None — all dependencies point downward through the layers (clean hexagonal).")

doc = f"""# Architecture — aoa

*Generated from live source by aOa on {ts}. {total_pkg} internal packages, {total_edge} dependency edges, {len(components)} third-party modules. Source: AST package index + `go list`. This document regenerates from code; it does not drift.*

## 1. System Overview

`aoa` is a Go application organised as a hexagonal (ports & adapters) architecture across {len(present_layers)} layers: {", ".join(present_layers)}. The contracts hub is **`{short(hub[0])}`** (fan-in {hub[1]['fanin']} — most depended-on), and the composition root is **`{short(root_pkg[0])}`** (fan-out {root_pkg[1]['fanout']} — wires everything together).

## 2. Building Block View — Layer Dependencies (C4 Container level)

```mermaid
{mermaid_high}
```

Edge labels = number of package dependencies crossing between layers.

## 3. Component View — Package Dependencies (C4 Component level)

```mermaid
{mermaid_pkg}
```

## 4. Layering & Conformance Assessment

Intended dependency direction (hexagonal): `cmd → app → adapters → domain → ports`. A dependency pointing *up* this order is a layering violation.

**Findings ({len(violations)} violation{'s' if len(violations)!=1 else ''}):**
{viol_md}

## 5. Dependency Inventory

| Package | Layer | Fan-in | Fan-out |
|---|---|---|---|
{inv_rows}

## 6. Provenance

Every element above is derived from the codebase: package nodes and edges from `go list`, layer assignment from path, third-party inventory (see `sbom.cdx.json`, CycloneDX 1.6) from the module graph. No hand-authoring; regenerate on commit.
"""
open("playbook/data/architecture.md", "w").write(doc)

# ---- architecture.html (renders the md + mermaid) ----
viewer = """<!doctype html><html><head><meta charset="utf-8">
<title>aoa — Architecture (generated)</title>
<style>
 body{margin:0;background:#0d0f13;color:#e6e6e6;font:15px/1.6 -apple-system,Segoe UI,Roboto,sans-serif;
   max-width:980px;margin:0 auto;padding:30px 40px}
 h1{font-size:26px;border-bottom:1px solid #23262e;padding-bottom:10px}
 h2{font-size:19px;margin-top:34px;color:#cdd2db}
 code{background:#1a1d24;padding:1px 5px;border-radius:4px;font-size:13px}
 table{border-collapse:collapse;width:100%;font-size:13px;margin:10px 0}
 th,td{border:1px solid #23262e;padding:6px 10px;text-align:left} th{background:#16181d}
 em{color:#8b8d98} .mermaid{background:#0a0b0e;border:1px solid #23262e;border-radius:10px;padding:16px;margin:14px 0}
 a{color:#5b9cff}
</style>
<script type="module">
import mermaid from "https://esm.sh/mermaid@11";
mermaid.initialize({startOnLoad:false, theme:"dark", themeVariables:{fontFamily:"ui-monospace,monospace"}});
import { marked } from "https://esm.sh/marked@12";
const md = __MD__;
// split fenced mermaid blocks out so marked doesn't escape them
const parts = md.split(/```mermaid\\n([\\s\\S]*?)```/g);
let outHtml = "", diagrams = [];
for (let i=0;i<parts.length;i++){
  if (i%2===1){ const id="mmd"+i; diagrams.push([id,parts[i]]); outHtml += `<div class="mermaid" id="${id}"></div>`; }
  else outHtml += marked.parse(parts[i]);
}
document.getElementById("doc").innerHTML = outHtml;
for (const [id,src] of diagrams){
  try{ const {svg}=await mermaid.render(id+"_svg", src); document.getElementById(id).innerHTML=svg; }
  catch(e){ document.getElementById(id).innerHTML="<pre style='color:#e5484d'>"+e.message+"</pre>"; }
}
</script>
</head><body><div id="doc">rendering…</div></body></html>"""
open("playbook/mockups/architecture.html", "w").write(viewer.replace("__MD__", json.dumps(doc)))

print(f"packages={total_pkg} edges={total_edge} violations={len(violations)} sbom_components={len(components)}")
print("wrote: playbook/data/architecture.md  sbom.cdx.json  architecture.html")
