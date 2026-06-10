#!/usr/bin/env python3
"""Generate a professional React Flow viewer for the doc-construction blueprint.
Self-contained HTML (blueprint embedded), React Flow via esm.sh. Per document it
renders the construction graph: document -> sections, colored by fillable_now."""
import json

blueprint = json.load(open(".context/details/doc-construction-blueprint.json"))
DATA = json.dumps(blueprint, separators=(",", ":"))

JS = r"""
import React, { useState, useMemo, useCallback } from "https://esm.sh/react@18.3.1";
import { createRoot } from "https://esm.sh/react-dom@18.3.1/client";
import { ReactFlow, Background, Controls, MiniMap, Handle, Position }
  from "https://esm.sh/@xyflow/react@12.3.5?deps=react@18.3.1,react-dom@18.3.1";
import htm from "https://esm.sh/htm@3.1.1";
const html = htm.bind(React.createElement);

const BP = __BLUEPRINT__;
const DOCS = BP.documents;

const COMP = { "complete-now": "#30a46c", "near-complete": "#f5a623", "skeleton-now": "#8b8d98" };
const FILL = { yes: "#30a46c", partial: "#f5a623", no: "#e5484d" };

function SectionNode({ data }) {
  const c = FILL[data.fill] || "#8b8d98";
  return html`<div style=${{
    background: "#16181d", border: "1px solid #2a2d35", borderLeft: "4px solid " + c,
    borderRadius: 8, padding: "8px 12px", width: 300, color: "#e6e6e6",
    boxShadow: "0 2px 10px #0006", cursor: "pointer" }}>
    <${Handle} type="target" position=${Position.Left} style=${{ background: c }} />
    <div style=${{ fontSize: 12.5, fontWeight: 600, lineHeight: 1.3 }}>${data.label}</div>
    <div style=${{ fontSize: 10.5, opacity: 0.62, marginTop: 3 }}>${data.sub}</div>
    <div style=${{ fontSize: 9.5, color: c, marginTop: 4, textTransform: "uppercase", letterSpacing: 0.5 }}>
      ${data.fill === "yes" ? "fillable now" : data.fill === "partial" ? "partial" : "gap"}${data.required ? " · required" : ""}
    </div>
  </div>`;
}
function DocNode({ data }) {
  const c = COMP[data.completeness] || "#8b8d98";
  return html`<div style=${{
    background: "#0e0f13", border: "2px solid " + c, borderRadius: 12, padding: "12px 16px",
    width: 260, color: "#fff", boxShadow: "0 4px 20px #0008" }}>
    <div style=${{ fontSize: 15, fontWeight: 700 }}>${data.label}</div>
    <div style=${{ fontSize: 11, color: c, marginTop: 4, fontWeight: 600 }}>${data.completeness}</div>
    <div style=${{ fontSize: 10.5, opacity: 0.6, marginTop: 6 }}>${data.fmt}</div>
    <${Handle} type="source" position=${Position.Right} style=${{ background: c }} />
  </div>`;
}
const nodeTypes = { section: SectionNode, doc: DocNode };

function App() {
  const [idx, setIdx] = useState(0);
  const [sel, setSel] = useState(null);
  const doc = DOCS[idx];
  const { nodes, edges } = useMemo(() => {
    const secs = doc.sections || [];
    const h = Math.max(secs.length * 96, 200);
    const ns = [{ id: "doc", type: "doc", position: { x: 0, y: h / 2 - 40 },
      data: { label: doc.document, completeness: doc.completeness, fmt: doc.output_format } }];
    const es = [];
    secs.forEach((s, i) => {
      ns.push({ id: "s" + i, type: "section", position: { x: 420, y: i * 96 },
        data: { label: s.section, sub: s.populated_by, fill: s.fillable_now, required: s.required } });
      es.push({ id: "e" + i, source: "doc", target: "s" + i, animated: s.fillable_now === "yes",
        style: { stroke: FILL[s.fillable_now] || "#8b8d98", strokeWidth: 1.8 } });
    });
    return { nodes: ns, edges: es };
  }, [idx]);

  const onNodeClick = useCallback((_, n) => {
    if (n.type === "section") setSel(doc.sections[parseInt(n.id.slice(1))]);
  }, [idx]);

  const counts = (d) => {
    const s = d.sections || []; const y = s.filter(x => x.fillable_now === "yes").length;
    return y + "/" + s.length;
  };

  return html`<div style=${{ display: "flex", height: "100vh", font: "13px ui-monospace,Menlo,monospace", background: "#0a0b0e", color: "#e6e6e6" }}>
    <div style=${{ width: 280, borderRight: "1px solid #1d2026", overflowY: "auto", flexShrink: 0 }}>
      <div style=${{ padding: "14px 16px", borderBottom: "1px solid #1d2026", position: "sticky", top: 0, background: "#0a0b0e" }}>
        <div style=${{ fontSize: 14, fontWeight: 700 }}>Document Construction</div>
        <div style=${{ fontSize: 11, opacity: 0.55, marginTop: 2 }}>${DOCS.length} documents · fillable-now / total</div>
      </div>
      ${DOCS.map((d, i) => html`<div key=${i} onClick=${() => { setIdx(i); setSel(null); }} style=${{
        padding: "10px 16px", borderBottom: "1px solid #15171c", cursor: "pointer",
        background: i === idx ? "#16181d" : "transparent" }}>
        <div style=${{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
          <span style=${{ fontSize: 12.5, fontWeight: i === idx ? 700 : 500 }}>${d.document}</span>
          <span style=${{ fontSize: 10.5, opacity: 0.6 }}>${counts(d)}</span>
        </div>
        <span style=${{ fontSize: 10, color: COMP[d.completeness], fontWeight: 600 }}>${d.completeness}</span>
      </div>`)}
    </div>
    <div style=${{ flex: 1, position: "relative" }}>
      <${ReactFlow} nodes=${nodes} edges=${edges} nodeTypes=${nodeTypes} onNodeClick=${onNodeClick}
        fitView minZoom=${0.2} proOptions=${{ hideAttribution: true }}>
        <${Background} color="#1a1d24" gap=${22} />
        <${Controls} />
        <${MiniMap} pannable zoomable style=${{ background: "#0e0f13" }}
          nodeColor=${(n) => n.type === "doc" ? COMP[doc.completeness] : (FILL[n.data.fill] || "#555")} />
      <//>
    </div>
    <div style=${{ width: 330, borderLeft: "1px solid #1d2026", padding: 16, overflowY: "auto", flexShrink: 0 }}>
      <div style=${{ fontSize: 14, fontWeight: 700 }}>${doc.document}</div>
      <div style=${{ fontSize: 11, color: COMP[doc.completeness], fontWeight: 600, margin: "2px 0 10px" }}>${doc.completeness}</div>
      <div style=${{ fontSize: 11, opacity: 0.8, marginBottom: 4 }}><b>Standard</b></div>
      <div style=${{ fontSize: 11, opacity: 0.65, marginBottom: 10, lineHeight: 1.4 }}>${doc.standard}</div>
      <div style=${{ fontSize: 11, opacity: 0.8, marginBottom: 4 }}><b>Output</b></div>
      <div style=${{ fontSize: 11, opacity: 0.65, marginBottom: 10 }}>${doc.output_format}</div>
      ${(doc.gaps && doc.gaps.length) ? html`<div style=${{ fontSize: 11, opacity: 0.8, marginBottom: 4 }}><b style=${{ color: "#e5484d" }}>Gaps</b></div>
        <ul style=${{ margin: "0 0 12px", paddingLeft: 16 }}>${doc.gaps.map((g, i) => html`<li key=${i} style=${{ fontSize: 10.5, opacity: 0.7, marginBottom: 3 }}>${g}</li>`)}</ul>` : null}
      ${sel ? html`<div style=${{ borderTop: "1px solid #1d2026", paddingTop: 10 }}>
        <div style=${{ fontSize: 11, color: FILL[sel.fillable_now], fontWeight: 700, marginBottom: 4 }}>${sel.section}</div>
        <div style=${{ fontSize: 10.5, opacity: 0.65, marginBottom: 6 }}><b>From:</b> ${sel.populated_by}</div>
        <div style=${{ fontSize: 10.5, opacity: 0.65 }}><b>How:</b> ${sel.extraction}</div>
      </div>` : html`<div style=${{ fontSize: 10.5, opacity: 0.4, borderTop: "1px solid #1d2026", paddingTop: 10 }}>Click a section node for its extraction method.</div>`}
    </div>
  </div>`;
}
createRoot(document.getElementById("root")).render(html`<${App} />`);
"""

HTML = """<!doctype html><html><head><meta charset="utf-8">
<title>aOa — Document Construction Blueprint</title>
<link rel="stylesheet" href="https://esm.sh/@xyflow/react@12.3.5/dist/style.css">
<style>html,body,#root{margin:0;height:100%;background:#0a0b0e}</style>
</head><body><div id="root"></div>
<script type="module">__JS__</script>
</body></html>"""

out = HTML.replace("__JS__", JS.replace("__BLUEPRINT__", DATA))
open("playbook/blueprint-viewer.html", "w").write(out)
print("wrote playbook/blueprint-viewer.html (%d KB)" % (len(out) // 1024))
