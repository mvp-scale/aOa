
import {React,useState,useEffect,useCallback,memo,createRoot,
 ReactFlow,Background,Controls,Handle,Position,BaseEdge,EdgeLabelRenderer,useReactFlow,ReactFlowProvider,
 ELK,htm} from "./vendor/bundle.js";
const html=htm.bind(React.createElement); const elk=new ELK();
// the contract file IS the data source — anything that emits a valid archmodel gets every view
const MQ=new URLSearchParams(location.search);
const EMBED=MQ.get("embed")==="1"; // embed=1: hide top header/estate-scope chrome; keep view-switcher + catalog (used by dashboard iframe)
function showFatal(msg){let d=document.getElementById("fatal");if(!d){d=document.createElement("div");d.id="fatal";
  d.style.cssText="position:fixed;top:0;left:0;right:0;z-index:9999;background:#7f1d1d;color:#fff;font:600 12px ui-monospace,monospace;padding:8px 14px;border-bottom:2px solid #f87171";
  document.body.appendChild(d);}if(!d._set){d._set=true;d.textContent="RENDER FAILURE · "+msg;}}
window.addEventListener("error",ev=>showFatal(ev.message));
window.addEventListener("unhandledrejection",ev=>showFatal(String(ev.reason&&ev.reason.message||ev.reason)));
const MODEL_PATH=MQ.get("model")||"/api/arch/manifest"; // L19.20: default is served API; ?model= overrides for dev
const BASE=MODEL_PATH.includes("/")?MODEL_PATH.slice(0,MODEL_PATH.lastIndexOf("/")+1):"";
let MODEL; let _eTag=""; // L19.20: boot ETag captured so first poll uses If-None-Match
try{const _r0=await fetch(MODEL_PATH);
  if(!_r0.ok)throw new Error("HTTP "+_r0.status+" loading "+MODEL_PATH);
  _eTag=_r0.headers.get("ETag")||""; MODEL=await _r0.json();}
catch(err){showFatal("MODEL LOAD FAILED · "+(err&&err.message||err));throw err;}
const ISSUES=[];
function validateView(path,v){
  const nodeIds=new Set(),memIds=new Set();
  if(v.kind==="buckets"){(v.buckets||[]).forEach(b=>{
    if(nodeIds.has(b.id))ISSUES.push(path+": duplicate bucket id "+b.id);nodeIds.add(b.id);
    if(!(b.members||[]).length)ISSUES.push(path+": empty bucket "+b.id);
    (b.members||[]).forEach(m=>{if(memIds.has(m.id))ISSUES.push(path+": duplicate member id "+m.id);memIds.add(m.id);});
    if((b.members||[]).length>40)ISSUES.push(path+": bucket "+b.id+" exceeds view budget ("+b.members.length+") — auto-collapsed");});}
  else{(v.nodes||[]).forEach(n=>{if(nodeIds.has(n.id))ISSUES.push(path+": duplicate node id "+n.id);nodeIds.add(n.id);});}
  v.edges=(v.edges||[]).filter(e=>{const ok=nodeIds.has(e.source)&&nodeIds.has(e.target);
    if(!ok)ISSUES.push(path+": edge "+(e.id||"?")+" → missing node ("+e.source+" → "+e.target+") — quarantined");
    return ok;});}
// legacy single-file contracts validate everything at boot; sharded manifests validate per shard on fetch
if(!MODEL.sharded){
 for(const[eid,ev]of Object.entries(MODEL.estates||{}))
  for(const[sid,sv]of Object.entries(ev.scopes||{}))
   for(const[vid,v]of Object.entries(sv.views||{}))validateView(eid+"/"+sid+"/"+vid,v);}
// Elevation scale — lightness stages importance, hue stays reserved for meaning:
// L0 well (canvas, darkest) -> L1 chrome (header/sidebar/footer/bar) -> L2 raised
// (expanded dock, hover card, legend) -> L3 interactive (hover/selected fills).
const T={bg:"#0a0a0c",chrome:"#121215",raise:"#18181b",card:"#161618",cardH:"#202024",
 band:"#101013",border:"#252528",borderR:"#34343a",text:"#e8e8ec",
 dim:"#8b8b96",mute:"#55555f",green:"#34d399",blue:"#60a5fa",purple:"#c084fc",cyan:"#22d3ee",
 yellow:"#fbbf24",red:"#f87171",arch:"#fb923c",neutral:"#94a3b8"};
let PALETTES={
 aoa:{cmd:T.purple,app:T.blue,adapters:T.arch,domain:T.green,ports:T.dim,atlas:T.cyan,supporting:T.neutral},
 gf:{cli:T.purple,serve:T.blue,ingest:T.cyan,pipeline:T.green,render:T.dim,infra:T.arch,supporting:T.neutral},
 dep:{dev:T.blue,ci:T.purple,registry:T.arch,user:T.green},
 retail:{order:T.arch,inventory:T.cyan,customer:T.purple,shared:T.neutral},
 mc:{aws:T.arch,gcp:T.blue,shared:T.neutral}};
let ESTATES=MODEL.estates; // L19.20: let — poll updates on 200 before re-render
const firstScope=e=>Object.keys(ESTATES[e].scopes)[0];
const firstView=(e,sc)=>Object.keys(ESTATES[e].scopes[sc].views)[0];
// firstBucketsView: prefer the first buckets-kind view; fall back to firstView.
// Used when ?expandedGroups= is in the URL so the param lands on a view that can expand.
const firstBucketsView=(e,sc)=>{const vs=ESTATES[e].scopes[sc].views;
  return Object.keys(vs).find(id=>vs[id].kind==="buckets")||firstView(e,sc);};
// generic catalog for simulated scopes: derived from the views the scope actually has
const VIEWFAM={context:["C4 Model","System Context"],container:["C4 Model","Container"],
 component:["C4 Model","Component"],domains:["C4 Model","Domain map"],
 deployment:["Technology & Ops","Deployment"],dataflow:["Flows & Behavior","Data Flow (DFD)"],
 datamodel:["Data","Data Model / ER"]};
// THE STANDARD CATALOG (from the canonical-documents research) — shown IN FULL for
// every system; items the system has render live/sim, the rest stay listed as planned.
const STD_CATALOG=[
 {grp:"C4 Model",tag:"modern",items:[
   {vid:["context"],label:"System Context",aka:"C4 L1 / Context Diagram"},
   {vid:["capability","container"],label:"Capability Map",aka:"C4 Container Diagram"},
   {vid:["component","domains"],label:"Component"},
   {vid:["domains"],label:"Domain map",aka:"Domain Decomposition"},
   {vid:["deployment"],label:"Deployment",aka:"C4 Deployment Diagram"},
   {vid:["sequence"],label:"Dynamic (sequence)",note:"needs call-edge resolution"},
   {vid:["code"],label:"Code (L4)",note:"symbol table · not drawn by design — needs call-edge resolution"}]},
 {grp:"Interfaces & Change",items:[
   {vid:["api-contract"],label:"API Contract",aka:"API Surface / OpenAPI"},
   {vid:["change"],label:"Change Map",aka:"Hotspots / churn×complexity"}]},
 {grp:"Flows & Behavior",items:[
   {vid:["dataflow"],label:"Data Flow (DFD)"},
   {vid:["trust"],label:"Trust Boundaries (STRIDE)",note:"DFD overlay · rule-pack"},
   {vid:["statemachine"],label:"State Machine",note:"needs state extraction"}]},
 {grp:"Data",items:[
   {vid:["datamodel"],label:"Data Model",aka:"Entity-Relationship Diagram / ERD"},
   {vid:["glossary"],label:"Glossary",note:"atlas seed + writer"}]},
 {grp:"Technology & Ops",items:[
   {vid:["techportfolio"],label:"Tech Stack",aka:"Technology Portfolio",note:"config scan"},
   {vid:["sbom"],label:"SBOM",aka:"Software Bill of Materials / CycloneDX",note:"document · manifests"},
   {vid:["ownership"],label:"Ownership",aka:"CODEOWNERS map"}]},
 {grp:"Classical structure",items:[
   {vid:["component","domains"],label:"Layered Architecture",alias:true},
   {vid:["dsm"],label:"Dependency Matrix (DSM)",note:"matrix renderer"},
   {vid:["cycles"],label:"Cycle / Tangle Report",note:"SCC pass"}]}];
// AI-generation prompt for planned rows: copy, hand to any agent, get a loadable shard
const PROMPT_SCHEMA=`View JSON shapes (pick ONE matching the view type):
simple:  {"kind":"simple","title","count","dir":"DOWN|RIGHT","prov":{"kind":"simulated","label":"SIMULATED · would derive from: <source>"},"nodes":[{"id","type":"sys|ext|container|store|proc","icon":"sys|ext|container|store|app|domain","label","sub","stats":{"<named stat>":"<value>"}?,"real":false}],"edges":[{"id","source","target","label"}]}
buckets: {...same header...,"buckets":[{"id","layer (prefer: core|channel|integration|data|external|supporting)","label","part":0,"boundary":true(optional, dashed zone),"members":[{"id","label","sub"?,"stats":{"<named stat>":"<value>"}?}]}],"edges":[{"id","source","target","count","label"?,"tag"?}]}
entity:  {...,"nodes":[{"id","type":"entity","label","tech","fields":["..."],"stats":{...}?,"real":false}],"edges":[{"id","source","target","label"}]}
stats = 3-4 named, reader-meaningful figures for the hover card (e.g. "stores":"≈2,300", "throughput":"14M txns/day") — NEVER packed into the label.
table:   {...,"columns":["..."],"rows":[["...","..."]]}
matrix:  {...,"items":["a","b"],"matrix":[[null,3],[1,null]]}`;
// per-view intent — fetched from daemon at boot time
let VIEW_INTENT={};
try{const _vs=await fetch("/api/arch/standards").then(r=>{
  if(!r.ok)throw new Error("HTTP "+r.status+" loading standards");
  return r.json();});
VIEW_INTENT=(_vs&&_vs.views)||{};
// pull named palettes from view-standards if present (falls back to built-in)
if(_vs&&_vs.global&&_vs.global.palette&&_vs.global.palette.named_palettes){
  const np=_vs.global.palette.named_palettes;
  // red/yellow are RESERVED (violations/warnings) — no palette entry may claim them.
  const CMAP={purple:T.purple,blue:T.blue,arch:T.arch,green:T.green,cyan:T.cyan,neutral:T.neutral,dim:T.dim};
  Object.entries(np).forEach(([pid,pm])=>{PALETTES[pid]={};
    Object.entries(pm).forEach(([layer,cname])=>{PALETTES[pid][layer]=CMAP[cname]||T.neutral;});});}}
catch(err){showFatal("STANDARDS LOAD FAILED · "+(err&&err.message||err));}
function genPrompt(estateId,scopeLabel,vid,label){
 const VI=VIEW_INTENT[vid];
 return "Generate an architecture view for the aOa playbook viewer.\n"+
 "Target: estate \""+estateId+"\" · system \""+scopeLabel+"\" · view id \""+vid+"\" ("+label+").\n"+
 "Author realistic content consistent with that system. Mark provenance SIMULATED with the real-world source it would derive from.\n"+
 (VI?"\nIntent — the standard this view must honor (one design, one perspective):\n"+
 "- question it answers: "+VI.question+"\n"+
 "- canvas-vital (must be present and readable): "+VI.vital.join("; ")+"\n"+
 (VI.hover.length?"- hover-tier (put in 'sub'/detail fields, NEVER in labels): "+VI.hover.join("; ")+"\n":"")+
 "- labels: nodes ≤30 chars, members ≤26, edge labels ≤48 — names stay clean, metadata goes to the hover tier\n":"")+
 "\n"+PROMPT_SCHEMA+"\n\n"+
 "Output: merge the view object into playbook/mockups/estates/"+estateId+".json under estates.*.scopes.*.views."+vid+
 ", then run python3 playbook/generators/build_c4_mockup.py and refresh the viewer.";}
function dynamicCatalog(sv){
  return STD_CATALOG.map(g=>({grp:g.grp,tag:g.tag,items:g.items.map(it=>{
    // R1: code view is always listed-not-drawn regardless of manifest presence
    if((it.vid||[]).includes("code")){
      return {label:it.label,status:"planned",note:"symbol table · not drawn by design — needs call-edge resolution"};}
    const hit=(it.vid||[]).find(v=>sv.views&&sv.views[v]);
    if(hit){const v=sv.views[hit];
      return {id:hit,label:it.label,alias:it.alias,aka:it.aka,
        status:(v.prov&&v.prov.kind==="simulated")?"sim":"live"};}
    return {label:it.label,aka:it.aka,status:"planned",note:it.note||"not yet derived for this system",vid0:(it.vid||[])[0]};})}));}
const snap=v=>Math.round(v/8)*8;
// ROLE_IP: the 6-role spine (roles.go roleFor(), LOCKED glossary v1) — industry-standard
// glyphs (Lucide silhouettes hand-adapted from 24x24 to this file's 14x14/1.3-stroke house
// style), checked FIRST so any role-stamped bucket/member renders its role's icon regardless
// of legacy layer name. Never invented: hexagon=Lucide "hexagon", iface=Lucide "circle-dot"
// (glossary's boundary/interface/lollipop-socket glyph — concentric rings read as a socket/port
// at 14px; a literal lollipop-and-socket blobbed into an illegible smudge at this size), plug=
// Lucide "plug", cylinder=Lucide "database", cloud=Lucide "cloud" (== legacy IP.ext, reused),
// gear=Lucide "settings" cog (circle + 8 teeth, unlike legacy IP.supporting's plain dial).
const ROLE_IP={
 hexagon:'<polygon points="7,1.5 12,4.2 12,9.8 7,12.5 2,9.8 2,4.2" fill="none"/>',
 iface:'<circle cx="7" cy="7" r="3.4" fill="none"/><circle cx="7" cy="7" r="1" fill="none"/>',
 plug:'<line x1="5" y1="4.8" x2="5" y2="1.5"/><line x1="9" y1="4.8" x2="9" y2="1.5"/><path d="M3.3 4.8h7.4v2.7a3.7 3.7 0 0 1-3.7 3.7 3.7 3.7 0 0 1-3.7-3.7z" fill="none"/><line x1="7" y1="11.2" x2="7" y2="13.3"/>',
 cylinder:'<ellipse cx="7" cy="3.5" rx="4" ry="1.6" fill="none"/><path d="M3 3.5v7a4 1.6 0 0 0 8 0v-7" fill="none"/><path d="M3 7a4 1.6 0 0 0 8 0" fill="none"/>',
 cloud:'<path d="M4 10a2.2 2.2 0 0 1 .2-4.3 3 3 0 0 1 5.8-.4 2.2 2.2 0 0 1 .2 4.7z" fill="none"/>',
 gear:'<circle cx="7" cy="7" r="2.9" fill="none"/><circle cx="7" cy="7" r="1.1" fill="none"/>'+
  '<line x1="7" y1="2.4" x2="7" y2="3.7"/><line x1="7" y1="10.3" x2="7" y2="11.6"/>'+
  '<line x1="2.4" y1="7" x2="3.7" y2="7"/><line x1="10.3" y1="7" x2="11.6" y2="7"/>'+
  '<line x1="3.75" y1="3.75" x2="4.67" y2="4.67"/><line x1="10.25" y1="10.25" x2="9.33" y2="9.33"/>'+
  '<line x1="10.25" y1="3.75" x2="9.33" y2="4.67"/><line x1="3.75" y1="10.25" x2="4.67" y2="9.33"/>'};
const IP={
 cmd:'<polyline points="3,4 7,7 3,10" fill="none"/><line x1="7" y1="10" x2="11" y2="10"/>',
 app:'<circle cx="7" cy="7" r="2"/><circle cx="7" cy="2" r="1"/><circle cx="7" cy="12" r="1"/><circle cx="2" cy="7" r="1"/><circle cx="12" cy="7" r="1"/><line x1="7" y1="3" x2="7" y2="5"/><line x1="7" y1="9" x2="7" y2="11"/><line x1="3" y1="7" x2="5" y2="7"/><line x1="9" y1="7" x2="11" y2="7"/>',
 adapters:'<line x1="5" y1="1.5" x2="5" y2="4"/><line x1="9" y1="1.5" x2="9" y2="4"/><path d="M3 4h8v2a4 4 0 0 1-8 0z" fill="none"/><line x1="7" y1="10" x2="7" y2="12.5"/>',
 domain:'<polygon points="7,1.5 12,4.2 12,9.8 7,12.5 2,9.8 2,4.2" fill="none"/>',
 ports:'<rect x="2.5" y="2.5" width="9" height="9" rx="2" fill="none"/><circle cx="7" cy="7" r="1.6"/>',
 atlas:'<path d="M2.5 2.5h4a2 2 0 0 1 2 2v7a2 2 0 0 0-2-1.6h-4z" fill="none"/><path d="M11.5 2.5h-4a2 2 0 0 0-2 2v7a2 2 0 0 1 2-1.6h4z" fill="none"/>',
 supporting:'<circle cx="7" cy="7" r="3" fill="none"/><line x1="7" y1="1.5" x2="7" y2="3.5"/><line x1="7" y1="10.5" x2="7" y2="12.5"/>',
 sys:'<polygon points="7,1.5 12,4.2 12,9.8 7,12.5 2,9.8 2,4.2" fill="none"/><circle cx="7" cy="7" r="2"/>',
 ext:'<path d="M4 10a2.2 2.2 0 0 1 .2-4.3 3 3 0 0 1 5.8-.4 2.2 2.2 0 0 1 .2 4.7z" fill="none"/>',
 container:'<rect x="2.5" y="2.5" width="9" height="9" rx="1.5" fill="none"/><line x1="2.5" y1="5.5" x2="11.5" y2="5.5"/>',
 store:'<ellipse cx="7" cy="3.5" rx="4" ry="1.6" fill="none"/><path d="M3 3.5v7a4 1.6 0 0 0 8 0v-7" fill="none"/>',
 cli:'<polyline points="3,4 7,7 3,10" fill="none"/><line x1="7" y1="10" x2="11" y2="10"/>',
 serve:'<circle cx="7" cy="7" r="2"/><circle cx="2" cy="7" r="1"/><circle cx="12" cy="7" r="1"/><line x1="3" y1="7" x2="5" y2="7"/><line x1="9" y1="7" x2="11" y2="7"/>',
 ingest:'<path d="M7 1.5v7" /><polyline points="4,6 7,9 10,6" fill="none"/><line x1="2.5" y1="12" x2="11.5" y2="12"/>',
 pipeline:'<circle cx="2.5" cy="7" r="1.2"/><circle cx="7" cy="7" r="1.2"/><circle cx="11.5" cy="7" r="1.2"/><line x1="3.7" y1="7" x2="5.8" y2="7"/><line x1="8.2" y1="7" x2="10.3" y2="7"/>',
 render:'<rect x="2" y="3" width="10" height="8" rx="1" fill="none"/><line x1="2" y1="6" x2="12" y2="6"/>',
 infra:'<rect x="2.5" y="2.5" width="9" height="9" rx="2" fill="none"/><line x1="2.5" y1="7" x2="11.5" y2="7"/><line x1="7" y1="2.5" x2="7" y2="11.5"/>',
 dev:'<polyline points="3,4 7,7 3,10" fill="none"/><line x1="7" y1="10" x2="11" y2="10"/>',
 ci:'<circle cx="7" cy="7" r="4" fill="none"/><polyline points="7,4.5 7,7 9,8.5" fill="none"/>',
 registry:'<path d="M2.5 4.5l4.5-2.5 4.5 2.5-4.5 2.5z" fill="none"/><path d="M2.5 4.5v5l4.5 2.5 4.5-2.5v-5" fill="none"/>',
 user:'<circle cx="7" cy="5" r="2.2" fill="none"/><path d="M2.8 12a4.2 4.2 0 0 1 8.4 0" fill="none"/>',
 proc:'<circle cx="7" cy="7" r="2"/>'};
function Ico({k,c,s}){return html`<svg width=${s||16} height=${s||16} viewBox="0 0 14 14"
  stroke=${c} stroke-width="1.3" fill="none" style=${{flexShrink:0}}
  dangerouslySetInnerHTML=${{__html:ROLE_IP[k]||IP[k]||IP.supporting}}/>`;}

const ElkEdge=memo(function ElkEdge({id,data,style,markerEnd}){
  if(!data||!data.section) return null;
  const s=data.section;
  const pts=[s.startPoint,...(s.bendPoints||[]),s.endPoint];
  const d=pts.map((p,i)=>(i===0?"M ":"L ")+p.x+" "+p.y).join(" ");
  const m=data.meta;
  return html`<${React.Fragment}>
    <${BaseEdge} id=${id} path=${d} markerEnd=${markerEnd}
      style=${data._sel?{...style,strokeWidth:3,opacity:1,transition:"opacity 150ms ease, stroke-width 150ms ease"}
        :data._dim?{...style,opacity:(style&&style.opacity||1)*0.8,transition:"opacity 150ms ease"}
        :{...style,transition:"opacity 150ms ease"}}/>
    ${data.label?html`<${EdgeLabelRenderer}>
      <div className=${"nodrag nopan"+(m?" hv":"")} style=${{position:"absolute",
        transform:`translate(${data.label.x}px,${data.label.y}px)`,
        background:T.bg,border:`1px solid ${data._sel?T.blue:T.border}`,borderRadius:5,
        padding:"0 5px",height:16,lineHeight:"15px",fontSize:10.5,fontWeight:600,
        color:T.text,opacity:data._dim?0.8:1,transition:"opacity 150ms ease",
        pointerEvents:"all",zIndex:5,cursor:"pointer"}}>${data.label.text}
        ${m?html`<${HoverCard} title=${m.s+" → "+m.t}
          rows=${[["flow",m.verb],["volume",m.count?"×"+m.count:""],[m.tag?"violation":"",m.tag||""]]}
          hint="click to inspect"/>`:null}
      </div>
    <//>`:null}
  <//>`;});

// Tier rules (playbook/standards/view-standards.json): canvas = identity + the view's
// one signal; supporting metadata lives HERE, shown on hover. ?hover=<id> forces one
// card open so the screenshot loop can verify the hover tier.
const HQ=MQ.get("hover");
function HoverCard({title,rows,hint}){
  const rs=(rows||[]).filter(r=>r[1]!==undefined&&r[1]!==null&&r[1]!=="");
  return html`<div class="hovercard">
    <div style=${{fontSize:11.5,fontWeight:700,color:T.text,marginBottom:rs.length?5:0}}>${title}</div>
    ${rs.map((r,i)=>html`<div key=${i} style=${{fontSize:10.5,color:T.dim,lineHeight:1.55}}>
      <span style=${{color:T.mute,fontSize:9,fontWeight:700,letterSpacing:.8,textTransform:"uppercase",marginRight:6}}>${r[0]}</span>${r[1]}</div>`)}
    ${hint?html`<div style=${{fontSize:9,color:T.mute,marginTop:6,letterSpacing:.4}}>${hint}</div>`:null}
  </div>`;}
const hvc=id=>"hv"+(id===HQ?" force":"");
function BoxNode({data}){
  const t=data.type,col=t==="sys"?T.blue:t==="container"?T.arch:t==="store"?T.green:t==="proc"?T.blue:t==="ext"?T.dim:T.dim;
  const mock=data.real===false,drill=!!data.drillTo;
  const NAME={sys:"system",ext:"external",container:"container",store:"store",proc:"process"};
  return html`<div class=${hvc(data.id)} style=${{background:T.card,border:`1.5px ${mock?"dashed":"solid"} ${col}`,
    borderRadius:t==="store"?18:t==="proc"?12:8,padding:"9px 13px",width:data.w-2,height:data.h-2,boxSizing:"border-box",
    color:T.text,cursor:"pointer",opacity:data._dim?0.8:1,transition:"opacity 150ms ease, box-shadow 150ms ease",
    boxShadow:data._sel?`0 0 0 2px ${T.blue}, 0 0 10px ${T.blue}77`:"none"}}>
    <${Handle} type="target" position=${Position.Top} style=${{opacity:0}}/><${Handle} type="target" position=${Position.Left} style=${{opacity:0}}/>
    <div style=${{display:"flex",alignItems:"center",gap:8,height:"100%",opacity:mock?0.88:1}}>
      <${Ico} k=${data.icon} c=${col} s=${17}/>
      <span style=${{fontSize:13.5,fontWeight:600,whiteSpace:"nowrap"}}>${data.label}</span>
      ${drill?html`<span style=${{marginLeft:"auto",fontSize:10,color:col,border:`1px solid ${col}`,borderRadius:5,padding:"0 5px"}}>open ▸</span>`:null}
    </div>
    <${HoverCard} title=${data.label} rows=${(data.stats?Object.entries(data.stats):[["type",NAME[t]||t],["detail",data.sub]]).concat(mock?[["status","inferred · sourceable"]]:[])}
      hint=${drill?"click to select · Deep dive opens "+data.drillTo+" view":"click → details"}/>
    <${Handle} type="source" position=${Position.Bottom} style=${{opacity:0}}/><${Handle} type="source" position=${Position.Right} style=${{opacity:0}}/>
  </div>`;}
function EntityNode({data}){
  const c=T.green;
  return html`<div class=${hvc(data.id)} style=${{width:data.w-2,cursor:"pointer",
    opacity:data._dim?0.8:1,transition:"opacity 150ms ease, box-shadow 150ms ease",
    boxShadow:data._sel?`0 0 0 2px ${T.blue}, 0 0 10px ${T.blue}77`:"none",borderRadius:6}}>
    <div style=${{background:T.card,border:`1.5px solid ${c}`,borderRadius:6,
      boxSizing:"border-box",color:T.text,overflow:"hidden"}}>
      <${Handle} type="target" position=${Position.Top} style=${{opacity:0}}/><${Handle} type="target" position=${Position.Left} style=${{opacity:0}}/>
      <div style=${{padding:"7px 12px",borderBottom:`1px solid ${T.border}`,display:"flex",alignItems:"center",gap:7,background:T.band}}>
        <${Ico} k="store" c=${c} s=${14}/>
        <span style=${{fontSize:13,fontWeight:700,whiteSpace:"nowrap"}}>${data.label}</span>
      </div>
      ${data.fields.map((f,i)=>html`<div key=${i} style=${{padding:"3px 12px",fontSize:10.5,
        fontFamily:"ui-monospace,monospace",color:f.startsWith(" ")?T.dim:T.text,
        borderBottom:i<data.fields.length-1?`1px solid ${T.border}22`:"none"}}>${f}</div>`)}
      <${Handle} type="source" position=${Position.Bottom} style=${{opacity:0}}/><${Handle} type="source" position=${Position.Right} style=${{opacity:0}}/>
    </div>
    <${HoverCard} title=${data.label} rows=${data.stats?Object.entries(data.stats):[["store",data.tech]]} hint="click → details"/>
  </div>`;}
function BucketNode({data}){const c=data.col;const dash=data.layer==="supporting"||data.boundary;
  // _fromCapsule: expanded from capsule mode — UNMISTAKABLE container treatment so a cold
  // reader can answer "which group is open, what members does it contain, what does it import"
  // from a static screenshot alone.
  const fromCapsule=data._fromCapsule;
  // A3 calm default: _god/_cyc/_dead/_over are still DETECTED upstream (layoutBuckets) so counts
  // stay accurate, but the alarm PAINT (red border/glow, CYCLE/ORPHAN/COLLAPSED badges, dead-dim)
  // only renders when the Findings lens (showFindings) is on — a stranger sees a calm role-colored
  // map, not a wall of red.
  const sf=data._showFindings;
  const god=sf&&data._god,dead=sf&&data._dead,cyc=sf&&data._cyc,over=sf&&data._over;
  return html`<div class=${hvc(data.id)} style=${{width:"100%",height:"100%",position:"relative",
    background:fromCapsule?c+"2a":T.band,
    border:`${fromCapsule?"4px":"1.5px"} ${dash?"dashed":"solid"} ${god?T.red:c}`,
    borderRadius:fromCapsule?8:4,boxSizing:"border-box",
    // hex-alpha fills (c+"2a" etc.) double-fade under opacity, so dim uses filter:brightness
    // instead — keeps the recession visually consistent with the solid-background node types.
    filter:data._dim?"brightness(.8)":"none",transition:"filter 150ms ease, box-shadow 150ms ease",
    opacity:dead?0.45:1,cursor:"pointer",
    boxShadow:fromCapsule?`0 0 0 3px ${c}88, 0 0 32px ${c}55`:data._sel?`0 0 0 2px ${T.blue}, 0 0 10px ${T.blue}77`:god?`0 0 0 2px ${T.red}55, 0 0 22px ${T.red}44`:"none"}}>
    <${Handle} type="target" position=${Position.Top} style=${{opacity:0}}/><${Handle} type="target" position=${Position.Left} style=${{opacity:0}}/>
    <div style=${{display:"flex",alignItems:"center",gap:7,padding:"7px 12px",height:data.head,boxSizing:"border-box",
      background:fromCapsule?c+"40":"transparent",
      borderBottom:fromCapsule?`1px solid ${c}66`:undefined,
      borderRadius:"3px 3px 0 0"}}>
      <${Ico} k=${data.ico||data.layer} c=${c} s=${fromCapsule?16:14}/>
      <span style=${{fontSize:fromCapsule?13:11,fontWeight:700,color:c,textTransform:"uppercase",letterSpacing:1.1}}>${data.label}</span>
      ${fromCapsule?html`<span style=${{fontSize:9.5,fontWeight:700,color:c,border:`1.5px solid ${c}`,borderRadius:4,padding:"1px 6px",letterSpacing:.5,marginLeft:2}}>OPEN</span>`:null}
      ${cyc?html`<span style=${{fontSize:8.5,fontWeight:700,color:T.red,border:`1px solid ${T.red}`,borderRadius:4,padding:"0 4px"}}>⟳ CYCLE</span>`:null}
      ${dead?html`<span style=${{fontSize:8.5,fontWeight:700,color:T.yellow,border:`1px solid ${T.yellow}`,borderRadius:4,padding:"0 4px"}}>ORPHAN</span>`:null}
      ${over?html`<span style=${{fontSize:8.5,fontWeight:700,color:T.yellow,border:`1px solid ${T.yellow}`,borderRadius:4,padding:"0 4px"}}>${data._over} · COLLAPSED</span>`:null}
      <span style=${{marginLeft:"auto",fontSize:10.5,color:fromCapsule?c:T.dim,fontWeight:fromCapsule?700:400}}>${(data._allMembers||data.members).length}${fromCapsule?" members":""}</span>
      ${fromCapsule?html`<span data-expand-chip="1" title="collapse" style=${{fontSize:11,color:c,marginLeft:6,
        fontWeight:700,border:`1px solid ${c}66`,borderRadius:4,padding:"1px 5px"}}>▴</span>`:null}
    </div>
    <${HoverCard} title=${data.label} rows=${[["members",(data._allMembers||data.members).length]]} hint="click to inspect"/>
    <${Handle} type="source" position=${Position.Bottom} style=${{opacity:0}}/><${Handle} type="source" position=${Position.Right} style=${{opacity:0}}/>
    ${data._foldOpen&&data._allMembers?html`<${FoldOverlay} data=${data} c=${c}/>`:null}
  </div>`;}
// A8 FIX 2a: the "+N more" fold row's in-node scrollable reveal — a pure CSS overlay (max-height
// + overflow-y:auto) anchored inside the already-ELK-sized bucket box, so opening/closing it never
// changes b.w/b.h and never triggers an ELK relayout. Lists everything past the canvas budget
// (data._allMembers minus the already-rendered truncated + fold row, i.e. from data.members.length-1
// onward) with a "show less ▴" row that folds it back via the live _closeFold closure.
function FoldOverlay({data,c}){
  const shown=(data.members||[]).length-1; // members = budget items + 1 fold row
  const rest=(data._allMembers||[]).slice(shown);
  // nowheel: without it, scrolling this list bubbles to the ReactFlow pane and zooms/pans the
  // canvas instead of scrolling the overlay (react-flow's documented escape-hatch class, same
  // family as nodrag/nopan already used on edge labels elsewhere in this file).
  return html`<div class="nodrag nopan nowheel" style=${{position:"absolute",left:6,right:6,top:data.head+2,
    maxHeight:220,overflowY:"auto",background:T.bg,border:`1px solid ${c}77`,borderRadius:6,
    padding:"2px 0",zIndex:5,boxShadow:"0 10px 28px rgba(0,0,0,.5)"}}>
    ${rest.map((m,i)=>html`<div key=${m.id||i} style=${{display:"flex",alignItems:"center",gap:8,
      padding:"5px 10px",fontSize:11,color:T.text,borderBottom:`1px solid ${T.border}22`}}>
      <span style=${{flex:1,overflow:"hidden",textOverflow:"ellipsis",whiteSpace:"nowrap"}}>${m.label}</span>
      ${m.sub?html`<span style=${{fontSize:9.5,color:T.mute,flexShrink:0}}>${m.sub}</span>`:null}
    </div>`)}
    <div onClick=${ev=>{ev.stopPropagation();data._closeFold&&data._closeFold();}}
      style=${{textAlign:"center",padding:"7px 0 5px",fontSize:10.5,fontWeight:600,color:T.mute,cursor:"pointer"}}>
      show less ▴
    </div>
  </div>`;}
function SoloNode({data}){const c=data.col;const dash=data.layer==="supporting";
  return html`<div class=${hvc(data.member.id)} style=${{background:T.band,border:`1.5px ${dash?"dashed":"solid"} ${c}`,
    borderRadius:4,padding:"8px 12px",width:data.w-2,height:data.h-2,boxSizing:"border-box",color:T.text,
    cursor:"pointer",opacity:data._dim?0.8:1,transition:"opacity 150ms ease, box-shadow 150ms ease",
    boxShadow:data._sel?`0 0 0 2px ${T.blue}, 0 0 10px ${T.blue}77`:"none"}}>
    <${Handle} type="target" position=${Position.Top} style=${{opacity:0}}/><${Handle} type="target" position=${Position.Left} style=${{opacity:0}}/>
    <div style=${{display:"flex",alignItems:"center",gap:7}}>
      <${Ico} k=${data.ico||data.layer} c=${c} s=${14}/>
      <span style=${{fontSize:11,fontWeight:700,color:c,textTransform:"uppercase",letterSpacing:1.1,whiteSpace:"nowrap"}}>${data.label}</span>
      <span style=${{fontSize:12.5,fontWeight:600,marginLeft:6,whiteSpace:"nowrap"}}>${data.member.label}</span>
    </div>
    <${HoverCard} title=${data.member.label} rows=${data.member.stats?Object.entries(data.member.stats):[["layer",data.label],["detail",data.member.sub]]} hint="click to inspect"/>
    <${Handle} type="source" position=${Position.Bottom} style=${{opacity:0}}/><${Handle} type="source" position=${Position.Right} style=${{opacity:0}}/>
  </div>`;}
function MemberNode({data}){const c=data.col;
  // Decision 3 (A7 consensus): the fold row is a quiet placeholder, not a real member — no
  // icon/badges, dashed border, centered — reads as "more exists here" not "another item".
  if(data._foldMore){
    return html`<div style=${{background:"transparent",border:`1px dashed ${T.border}`,borderRadius:6,
      width:data.w,height:data.h,boxSizing:"border-box",color:T.mute,display:"flex",alignItems:"center",
      justifyContent:"center",cursor:"pointer",fontSize:10.5,fontWeight:600}}>${data.label} ▾</div>`;}
  return html`<div class=${hvc(data.id)} style=${{background:T.card,border:`1px solid ${T.border}`,borderLeft:`3px solid ${c}`,
    borderRadius:6,padding:data.wrap?"4px 10px":"5px 10px",width:data.w,height:data.h,boxSizing:"border-box",color:T.text,
    display:"flex",alignItems:"center",cursor:"pointer",opacity:data._dim?0.8:1,
    transition:"opacity 150ms ease, box-shadow 150ms ease",
    boxShadow:data._sel?`0 0 0 2px ${T.blue}, 0 0 10px ${T.blue}77`:"none"}}>
    <div style=${{display:"flex",alignItems:"center",gap:6,width:"100%"}}>
      <${Ico} k=${data.lay} c=${c} s=${12}/>
      <span style=${{fontSize:data.wrap?10.5:11.5,fontWeight:600,whiteSpace:data.wrap?"normal":"nowrap",lineHeight:1.25}}>${data.label}</span>
      ${(data.showC||data.showH||data.showFinding)?html`<span style=${{marginLeft:"auto",display:"flex",gap:3,flexShrink:0}}>
        ${data.showC?html`<span style=${{fontSize:8.5,fontWeight:700,color:T.red,border:`1px solid ${T.red}`,
          borderRadius:7,padding:"0 4px",lineHeight:"12px"}}>⚠${data.concerns>99?"99+":data.concerns}</span>`:null}
        ${data.showH?html`<span style=${{fontSize:8.5,fontWeight:700,color:T.yellow,border:`1px solid ${T.yellow}`,
          borderRadius:7,padding:"0 4px",lineHeight:"12px"}}>Δ</span>`:null}
        ${data.showFinding?html`<span title="daemon finding" style=${{fontSize:8.5,fontWeight:700,color:T.red,
          border:`1px solid ${T.red}`,borderRadius:7,padding:"0 4px",lineHeight:"12px"}}>⚠</span>`:null}
      </span>`:null}
    </div>
    <${HoverCard} title=${data.label} rows=${(data.stats?Object.entries(data.stats):[["detail",data.sub]]).concat([
      [data.concerns?"findings":"",data.concerns?data.concerns+" recon findings":""],
      [data.changed?"recent":"",data.changed?"touched in last 15 commits":""]])} hint="click to inspect"/>
  </div>`;}
// Shared numeric-cell idiom (also used by DockTable below): a cell "looks numeric" if it
// starts with a digit or one of the approx/mult/tilde glyphs the renderers already emit
// (e.g. "≈40", "×3", "~12").
const isNumCell=cell=>String(cell).match(/^[≈×~\d]/);
// Quiet mono chip (provenance-chip styling, neutral color — NO per-value rainbow): used for
// the api-contract method cell, glossary owning-domain, ownership owner(s), sbom unpinned.
const quietChip=(label,mono)=>html`<span style=${{fontSize:9,fontWeight:600,letterSpacing:.3,
  whiteSpace:"nowrap",color:T.dim,border:`1px solid ${T.border}`,borderRadius:5,padding:"1px 7px",
  fontFamily:mono?"ui-monospace,monospace":"inherit"}}>${label}</span>`;
// Inline proportional data-ink bar behind a risk/count numeral — column-max normalized,
// single dim accent (R8/A3: never red/yellow, those are reserved for violations/warnings).
const dataInkBar=(cell,max)=>{const v=parseFloat(cell)||0;const pct=max>0?Math.min(100,(v/max)*100):0;
  return html`<span style=${{position:"relative",display:"block"}}>
    <span style=${{position:"absolute",top:0,bottom:0,right:0,width:pct+"%",
      background:T.dim+"26",borderRadius:2}}></span>
    <span style=${{position:"relative"}}>${cell}</span>
  </span>`;};
function TableView({view,onSel,selId,vid,den}){
  // row click = select: the dock shows the full record (long prose cells live there untruncated)
  const pick=(r,ri)=>onSel&&onSel({label:String(r[0]),chip:"record",
    rows:(view.columns||[]).map((c,ci)=>[c,r[ci]]),relations:[]},"row:"+ri);
  if(vid==="cycles"&&!(view.rows||[]).length){
    // Parse "N modules" from the grain-scoped count string (e.g. "0 cycles · 29 modules · ⚠ 8 findings")
    const modMatch=view.count&&view.count.match(/(\d+)\s+modules/);
    const nMods=modMatch?parseInt(modMatch[1],10):15;
    return html`<div style=${{position:"absolute",inset:0,display:"flex",alignItems:"center",justifyContent:"center"}}>
      <div style=${{textAlign:"center",maxWidth:480,padding:"40px 32px",background:T.card,
        border:`1px solid ${T.border}`,borderRadius:12}}>
        <div style=${{fontSize:28,marginBottom:12,color:T.green}}>✓</div>
        <div style=${{fontSize:16,fontWeight:700,color:T.text,marginBottom:8}}>no cycles among ${nMods} modules this rev</div>
        <div style=${{fontSize:11.5,color:T.dim,lineHeight:1.6}}>
          checked every import edge among ${nMods} modules this rev · Tarjan SCC · re-derived on change
        </div>
        ${view.prov?html`<div style=${{marginTop:14,fontSize:10,color:T.mute}}>${view.prov.label}</div>`:null}
      </div>
    </div>`;}
  const cols=view.columns||[],allRows=view.rows||[];
  const rowPad=den==="comfort"?9:5;
  // header click-sort: DEFAULTS to shard order (null) — shard order IS each view's answer
  // (e.g. change is already risk-desc) — cycling a header asc -> desc -> back to shard order.
  const[sortState,setSortState]=useState({col:null,dir:1});
  useEffect(()=>{setSortState({col:null,dir:1});},[vid]);
  const toggleSort=ci=>setSortState(s=>s.col!==ci?{col:ci,dir:1}:s.dir===1?{col:ci,dir:-1}:{col:null,dir:1});
  const numCols=cols.map((_,ci)=>ci>0&&allRows.length>0&&allRows.every(r=>!r[ci]||isNumCell(r[ci])));
  const rows=React.useMemo(()=>{
    if(sortState.col==null)return allRows.map((r,ri)=>[r,ri]);
    const ci=sortState.col,num=numCols[ci];
    return allRows.map((r,ri)=>[r,ri]).sort((a,b)=>{
      const cmp=num?(parseFloat(a[0][ci])||0)-(parseFloat(b[0][ci])||0)
        :String(a[0][ci]).localeCompare(String(b[0][ci]));
      return cmp*sortState.dir;});
  },[allRows,sortState,numCols]);
  // per-view treatments (BEAU-1 scope B): sbom 'unpinned' chip, api-contract method chip,
  // glossary owning-domain chip + wrapped definition, ownership owner(s) chips, change/
  // techportfolio inline data-ink bar on the risk/count numeral.
  const barCol=vid==="change"?cols.length-1:vid==="techportfolio"?cols.indexOf("count"):-1;
  let barMax=0;
  if(barCol>=0)allRows.forEach(r=>{const v=parseFloat(r[barCol])||0;if(v>barMax)barMax=v;});
  const renderCell=(cell,ci)=>{
    if(vid==="sbom"&&cols[ci]==="unpinned")return cell==="true"?quietChip("unpinned",true):"";
    if(vid==="api-contract"&&ci===0)return cell&&cell!=="—"?quietChip(cell,true):cell;
    if(vid==="glossary"&&ci===2)return quietChip(cell,false);
    if(vid==="ownership"&&cols[ci]==="Owner(s)"&&cell!=="—")
      return html`<span style=${{display:"flex",gap:4,flexWrap:"wrap"}}>
        ${String(cell).split(", ").map((o,oi)=>html`<span key=${oi}>${quietChip(o,false)}</span>`)}</span>`;
    if(ci===barCol)return dataInkBar(cell,barMax);
    return cell;};
  const cellExtra=ci=>vid==="glossary"&&ci===1?{maxWidth:"60ch",whiteSpace:"normal",lineHeight:1.5}:null;
  return html`<div style=${{position:"absolute",inset:0,overflow:"auto",padding:"56px 40px 40px"}}>
    <table style=${{borderCollapse:"collapse",minWidth:560,fontSize:12.5,color:T.text}}>
      <thead><tr>${cols.map((c,i)=>html`<th key=${i} onClick=${()=>toggleSort(i)}
        style=${{position:"sticky",top:0,zIndex:2,background:T.bg,cursor:"pointer",userSelect:"none",
        textAlign:numCols[i]?"right":"left",
        padding:`${rowPad}px 16px`,borderBottom:`2px solid ${T.border}`,color:T.dim,fontSize:10.5,
        textTransform:"uppercase",letterSpacing:1}}>${c}${sortState.col===i
          ?html`<span style=${{color:T.text,marginLeft:4}}>${sortState.dir===1?"▲":"▼"}</span>`
          :html`<span style=${{color:T.mute,marginLeft:4}}>▾</span>`}</th>`)}</tr></thead>
      <tbody>${rows.map(([r,ri])=>html`<tr key=${ri} class="table-row-hover" onClick=${()=>pick(r,ri)}
        style=${{cursor:"pointer",...(selId==="row:"+ri?{background:T.cardH}:null)}}>
        ${r.map((cell,ci)=>html`<td key=${ci} style=${{padding:`${rowPad}px 16px`,
          borderBottom:`1px solid ${T.border}55`,
          color:String(cell).startsWith("⚠")?T.red:ci===0?T.text:T.dim,
          fontWeight:ci===0?600:400,fontFamily:ci===0?"inherit":"ui-monospace,monospace",
          textAlign:numCols[ci]?"right":"left",fontVariantNumeric:"tabular-nums",
          fontSize:ci===0?12.5:11.5,...cellExtra(ci)}}>${renderCell(cell,ci)}</td>`)}</tr>`)}</tbody>
    </table></div>`;}
function DSMView({view,onSel,selId}){
  const items=view.items||[],M=view.matrix||[];
  const n=items.length;
  const qp=new URLSearchParams(location.search);
  const[expandedGroup,setExpandedGroup]=useState(qp.get("dsmExpand")||null);
  const[extOpen,setExtOpen]=useState(qp.get("extOpen")==="1");
  const[unitGraph,setUnitGraph]=useState(null);
  // groups: DSM row label -> member unit ids, joined from the component shard's
  // buckets (the manifest's view entry never carries a groups field — the shard
  // is the source of truth for membership).
  const[dsmGroups,setDsmGroups]=useState(null);
  const[dsmUnitLabels,setDsmUnitLabels]=useState({});
  const[unitGraphErr,setUnitGraphErr]=useState(null);
  const[unitGraphLoading,setUnitGraphLoading]=useState(false);
  const[winW,setWinW]=useState(window.innerWidth);
  const[winH,setWinH]=useState(window.innerHeight);
  useEffect(()=>{const upd=()=>{setWinW(window.innerWidth);setWinH(window.innerHeight);};
    window.addEventListener("resize",upd);return()=>window.removeEventListener("resize",upd);},[]);
  const scrollRef=React.useRef(null);
  const[pendingPin,setPendingPin]=useState(null);

  // DEFECT 2: classify items into actors / targets-only / unlinked
  const rowSums=items.map((_,i)=>items.reduce((s,__,j)=>i===j?s:s+((M[i]||[])[j]||0),0));
  const colSums=items.map((_,j)=>items.reduce((s,__,i)=>i===j?s:s+((M[i]||[])[j]||0),0));
  const actorItems=items.filter((_,i)=>rowSums[i]>0);
  const targetItems=items.filter((_,i)=>rowSums[i]===0&&colSums[i]>0);
  const unlinkedItems=items.filter((_,i)=>rowSums[i]===0&&colSums[i]===0);

  // Summary sentence — actor/target/unlinked aggregation
  let depTotal=0;const mutPairs=[];let worstMut=null;
  actorItems.forEach(rLabel=>{
    const ri=items.indexOf(rLabel);
    [...actorItems,...targetItems].forEach(cLabel=>{
      const ci=items.indexOf(cLabel);
      if(ri!==ci)depTotal+=(M[ri]||[])[ci]||0;
    });
  });
  for(let ii=0;ii<actorItems.length;ii++)for(let jj=ii+1;jj<actorItems.length;jj++){
    const ai=items.indexOf(actorItems[ii]),aj=items.indexOf(actorItems[jj]);
    const fwd=(M[ai]||[])[aj],rev=(M[aj]||[])[ai];
    if(fwd&&rev){mutPairs.push({a:actorItems[ii],b:actorItems[jj],fwd,rev});
      if(!worstMut||fwd+rev>worstMut.fwd+worstMut.rev)worstMut={a:actorItems[ii],b:actorItems[jj],fwd,rev};}}
  const summaryText=`${actorItems.length} actors · ${depTotal.toLocaleString()} dependencies · ${mutPairs.length} mutual pair${mutPairs.length!==1?"s":""}${worstMut?` — worst: ${worstMut.a} ⇄ ${worstMut.b}`:""}${targetItems.length>0?` · ${targetItems.length} targets-only`:""}${unlinkedItems.length>0?` · ${unlinkedItems.length} unlinked`:""}`;

  // Band expansion
  const K=20;
  const expandedUnitPaths=expandedGroup!==null&&dsmGroups&&dsmGroups[expandedGroup]
    ?dsmGroups[expandedGroup]:[];
  const visibleUnits=expandedUnitPaths.slice(0,K);
  const moreUnits=expandedUnitPaths.length>K?expandedUnitPaths.length-K:0;
  const extraRows=visibleUnits.length+(moreUnits>0?1:0);
  const actorN=actorItems.length;
  const targetN=targetItems.length;
  // DEFECT 1: parent col stays when expanded → actorN + extraRows (not actorN-1+extraRows)
  const totalCols=actorN+(expandedGroup?extraRows:0);

  // Cell size: fit matrix in viewport
  const ROW_HDR_W=160,COL_HDR_H=26,SUMM_H=36,PAD=32;
  const canvasW=Math.max(600,winW-300-PAD);
  const canvasH=Math.max(400,winH-130-PAD-208);
  const availW=canvasW-ROW_HDR_W;
  const availH=canvasH-COL_HDR_H-SUMM_H;
  const MIN_CELL=22;
  const idealCell=totalCols>0?Math.min(Math.floor(availW/totalCols),Math.floor(availH/totalCols)):28;
  const cellSz=Math.max(MIN_CELL,idealCell);
  const needsScroll=idealCell<MIN_CELL;

  // Lazy fetch: fires for ANY expanded state lacking data — click or ?dsmExpand.
  useEffect(()=>{
    if(expandedGroup!==null&&!unitGraph&&!unitGraphLoading){fetchUnitData();}
  },[expandedGroup]);
  const fetchUnitData=()=>{
      setUnitGraphLoading(true);setUnitGraphErr(null);
      Promise.all([
        fetch("/api/arch/graph?grain=unit").then(r=>r.ok?r.json():Promise.reject(new Error("HTTP "+r.status))),
        fetch("/api/arch/local/component").then(r=>r.ok?r.json():Promise.reject(new Error("HTTP "+r.status)))
      ]).then(([g,comp])=>{
        const grp={},lbl={};
        (comp.buckets||[]).forEach(b=>{grp[b.label]=(b.members||[]).map(m=>{lbl[m.id]=m.label;return m.id;});});
        setDsmGroups(grp);setDsmUnitLabels(lbl);setUnitGraph(g);setUnitGraphLoading(false);
      }).catch(e=>{setUnitGraphErr(e.message);setUnitGraphLoading(false);});};
  const toggleGroup=(grpLabel,evt)=>{
    if(scrollRef.current&&evt&&evt.currentTarget){
      const rowEl=evt.currentTarget.closest?evt.currentTarget.closest("tr"):evt.currentTarget;
      const rowRect=rowEl?rowEl.getBoundingClientRect():null;
      if(rowRect){
        const screenY=rowRect.top;
        setPendingPin({key:"g_"+grpLabel,screenY});}}
    setExpandedGroup(expandedGroup===grpLabel?null:grpLabel);};

  useEffect(()=>{
    if(!pendingPin||!scrollRef.current)return;
    const rows=scrollRef.current.querySelectorAll("tbody tr");
    for(const tr of rows){
      const th=tr.querySelector("th");
      if(th&&th.textContent&&th.textContent.includes(pendingPin.key.replace("g_",""))){
        const rowRect=tr.getBoundingClientRect();
        const delta=rowRect.top-pendingPin.screenY;
        if(Math.abs(delta)>0.5)scrollRef.current.scrollTop+=delta;
        break;}}
    setPendingPin(null);
  },[expandedGroup,pendingPin]);
  // Unit edge lookup: build once from unitGraph
  let unitEdgeMap=null,unitToGroup=null;
  if(unitGraph&&dsmGroups){
    unitToGroup={};
    Object.entries(dsmGroups).forEach(([grp,paths])=>paths.forEach(p=>{unitToGroup[p]=grp;}));
    unitEdgeMap={};
    (unitGraph.edges||[]).forEach(e=>{
      if(!unitEdgeMap[e.from])unitEdgeMap[e.from]={};
      unitEdgeMap[e.from][e.to]=(unitEdgeMap[e.from][e.to]||0)+1;});}

  const getCellVal=(rItem,cItem,rIsUnit,cIsUnit)=>{
    if(rIsUnit&&cIsUnit)return unitEdgeMap?(unitEdgeMap[rItem]?.[cItem]||0):0;
    if(rIsUnit&&!cIsUnit){if(!unitEdgeMap||!dsmGroups?.[cItem])return 0;
      return (dsmGroups[cItem]||[]).reduce((s,t)=>s+(unitEdgeMap[rItem]?.[t]||0),0);}
    if(!rIsUnit&&cIsUnit){if(!unitEdgeMap||!dsmGroups?.[rItem])return 0;
      return (dsmGroups[rItem]||[]).reduce((s,f)=>s+(unitEdgeMap[f]?.[cItem]||0),0);}
    const ri=items.indexOf(rItem),ci=items.indexOf(cItem);
    return ri>=0&&ci>=0?(M[ri]||[])[ci]||0:0;};

  // Target cell value: group-grain only — targets don't expand
  const getTargetCellVal=(rE,tLabel)=>{
    if(rE.isMore)return 0;
    const ri=rE.isUnit?-1:items.indexOf(rE.label);
    const ci=items.indexOf(tLabel);
    return ri>=0&&ci>=0?(M[ri]||[])[ci]||0:0;};

  // DEFECT 1: Build flat rows — parent band row STAYS as first entry when expanded
  const flatRows=[];
  const dispNums=[];
  let gnum=0;
  actorItems.forEach((label)=>{
    gnum++;
    const isExpanded=label===expandedGroup;
    if(isExpanded){
      // Parent band row stays — isExpandedParent marks it
      flatRows.push({key:"g_"+label,label,isUnit:false,idx:items.indexOf(label),isExpandedParent:true});
      dispNums.push(String(gnum));
      visibleUnits.forEach((upath,ui)=>{
        const ulabel=dsmUnitLabels[upath]||upath.replace(/^u_/,"").split("_").pop();
        const isLast=ui===visibleUnits.length-1&&moreUnits===0;
        flatRows.push({key:"u_"+upath,label:ulabel,fullLabel:upath,isUnit:true,unitPath:upath,
          parentIdx:items.indexOf(label),parentLabel:label,treeGuide:isLast?"└":"├"});
        dispNums.push(gnum+"."+(ui+1));});
      if(moreUnits>0){
        flatRows.push({key:"u_more_"+label,label:`+${moreUnits} more`,isUnit:true,isMore:true,
          parentIdx:items.indexOf(label),parentLabel:label});
        dispNums.push(gnum+"+");}
    }else{
      flatRows.push({key:"g_"+label,label,isUnit:false,idx:items.indexOf(label)});
      dispNums.push(String(gnum));}});

  // DEFECT 2: target columns — columns-only, no row
  const flatTargetCols=targetItems.map((label,ti)=>({
    key:"t_"+label,label,idx:items.indexOf(label),dispNum:`T${ti+1}`}));
  const extRanked=targetItems.map(tLabel=>{
    const ci=items.indexOf(tLabel);
    const importers=[];
    actorItems.forEach(aLabel=>{
      const ri=items.indexOf(aLabel);
      const v=(M[ri]||[])[ci]||0;
      if(v>0)importers.push({actor:aLabel,count:v});
    });
    importers.sort((a,b)=>b.count-a.count);
    const total=importers.reduce((s,x)=>s+x.count,0);
    return{label:tLabel,total,importers};
  }).sort((a,b)=>b.total-a.total);

  const pickCell=(rE,cE)=>{
    const rItem=rE.isUnit?rE.unitPath:rE.label;
    const cItem=cE.isUnit?cE.unitPath:cE.label;
    if(rItem===cItem||rE.isMore||cE.isMore)return;
    const rv=getCellVal(rItem,cItem,rE.isUnit,cE.isUnit);
    if(!rv||!onSel)return;
    const cv=getCellVal(cItem,rItem,cE.isUnit,rE.isUnit);
    onSel({label:(rE.fullLabel||rE.label)+" → "+(cE.fullLabel||cE.label),
      chip:cv?"mutual":"dependency",
      rows:[["from",rE.fullLabel||rE.label],["to",cE.fullLabel||cE.label],["imports",rv]]
        .concat(cv?[["reverse",cv],["status","MUTUAL — cycle"]]:[]),
      relations:[]},"dsm:"+rE.key+","+cE.key);};

  const pickRow=(rE)=>{
    if(!onSel||rE.isMore)return;
    const item=rE.isUnit?rE.unitPath:rE.label;
    let fanOut=0,fanIn=0;
    flatRows.forEach(re=>{if(re.key===rE.key||re.isMore)return;
      const t=re.isUnit?re.unitPath:re.label;
      fanOut+=getCellVal(item,t,rE.isUnit,re.isUnit);
      fanIn+=getCellVal(t,item,re.isUnit,rE.isUnit);});
    flatTargetCols.forEach(tc=>{fanOut+=getTargetCellVal(rE,tc.label);});
    onSel({label:rE.fullLabel||rE.label,chip:rE.isUnit?"unit":"group",
      rows:[["fan-out",fanOut+" dependencies"],["fan-in",fanIn+" dependents"]],
      relations:[]},"dsm:"+rE.key);};

  const stickyH=needsScroll?{position:"sticky",background:T.chrome,zIndex:2}:{};
  const stickyV=needsScroll?{position:"sticky",background:T.chrome,zIndex:1}:{};

  return html`<div style=${{position:"absolute",inset:0,overflow:needsScroll?"auto":"hidden",
    display:"flex",flexDirection:"column",padding:"8px 16px",boxSizing:"border-box"}}>
    <div style=${{fontSize:11,color:T.dim,marginBottom:6,flexShrink:0,height:SUMM_H,
      display:"flex",alignItems:"center",gap:10,lineHeight:1.4}}>
      <span style=${{fontWeight:600,color:T.text}}>${summaryText}</span>
      ${unitGraphLoading?html`<span style=${{color:T.mute,fontSize:9}}>loading unit data…</span>`:null}
      ${unitGraphErr?html`<span style=${{color:T.yellow,fontSize:9}}>unit expansion unavailable · daemon not running</span>`:null}
    </div>
    <div ref=${scrollRef} style=${{overflow:needsScroll?"auto":"visible",flex:1}}>
      <table style=${{borderCollapse:"collapse",tableLayout:"fixed",fontSize:10,color:T.text}}>
        <thead><tr>
          <th style=${{...stickyH,...stickyV,top:0,left:0,zIndex:3,
            width:ROW_HDR_W,minWidth:ROW_HDR_W,height:COL_HDR_H,
            background:T.chrome,borderBottom:`1px solid ${T.border}`,borderRight:`1px solid ${T.border}`}}></th>
          ${flatRows.map((cE,ci)=>html`<th key=${ci}
            onClick=${(e)=>{if(cE.isUnit){pickRow(cE);}else{toggleGroup(cE.label,e);}}}
            title=${cE.fullLabel||cE.label}
            style=${{...stickyH,top:0,width:cellSz,minWidth:cellSz,maxWidth:cellSz,height:COL_HDR_H,
              textAlign:"center",fontWeight:700,fontSize:8.5,letterSpacing:.3,
              color:selId==="dsm:"+cE.key?T.blue:cE.isUnit?T.dim:cE.isExpandedParent?T.arch:T.text,
              cursor:cE.isMore?"default":"pointer",
              background:selId==="dsm:"+cE.key?T.cardH:(cE.isUnit||cE.isExpandedParent)?T.band:T.chrome,
              borderBottom:`1px solid ${(cE.isUnit||cE.isExpandedParent)?T.arch+"44":T.border}`,
              overflow:"hidden",whiteSpace:"nowrap",userSelect:"none"}}>
            ${cE.isMore?"":dispNums[ci]}</th>`)}
          ${targetN>0?html`<th key="ext-toggle"
            onClick=${()=>setExtOpen(!extOpen)}
            style=${{cursor:"pointer",width:90,minWidth:90,maxWidth:90,height:COL_HDR_H,
              textAlign:"center",fontWeight:600,fontSize:8,letterSpacing:.3,
              color:extOpen?T.arch:T.yellow,
              background:extOpen?T.band:T.chrome,
              borderBottom:`1px solid ${T.border}`,
              borderLeft:`2px solid ${T.borderR}`,
              whiteSpace:"nowrap",userSelect:"none",paddingLeft:4,paddingRight:4}}>
            ${extOpen?"Externals ▾":`Externals (${targetN}) ▸`}</th>`:null}
        </tr></thead>
        <tbody>${flatRows.map((rE,ri)=>{
          const rItem=rE.isUnit?rE.unitPath:rE.label;
          const isExpPar=rE.isExpandedParent;
          const isChild=rE.isUnit&&!!rE.parentLabel&&!rE.isMore;
          return html`<tr key=${ri}>
            <th onClick=${(e)=>{if(rE.isMore)return;if(rE.isUnit){pickRow(rE);}else{toggleGroup(rE.label,e);}}}
              title=${rE.fullLabel||rE.label}
              style=${{...stickyV,left:0,
                width:ROW_HDR_W,minWidth:ROW_HDR_W,maxWidth:ROW_HDR_W,height:cellSz,
                textAlign:"right",paddingRight:8,paddingLeft:isChild?32:8,
                fontWeight:rE.isUnit?400:600,fontSize:rE.isUnit?9:10.5,
                color:selId==="dsm:"+rE.key?T.blue:rE.isUnit?T.dim:isExpPar?T.arch:T.text,
                cursor:rE.isMore?"default":"pointer",
                background:selId==="dsm:"+rE.key?T.cardH:(isChild||isExpPar)?T.band:T.chrome,
                whiteSpace:"nowrap",overflow:"hidden",textOverflow:"ellipsis",
                borderRight:`1px solid ${(isChild||isExpPar)?T.arch+"44":T.border}`,userSelect:"none"}}>
              ${rE.isMore
                ?html`<span style=${{color:T.mute,marginRight:4,fontSize:9}}>└</span><span style=${{color:T.mute}}>${rE.label}</span>`
                :isChild
                  ?html`<span style=${{color:T.mute,marginRight:5,fontSize:8,fontWeight:400}}>${dispNums[ri]}</span><span style=${{color:T.mute,marginRight:4,fontSize:9}}>${rE.treeGuide||"├"}</span><span>${rE.label}</span>`
                  :html`<span style=${{color:T.mute,marginRight:5,fontSize:8,fontWeight:400}}>${dispNums[ri]}</span><span>${rE.label}</span><span style=${{color:T.mute,marginLeft:4,fontSize:8}}>${isExpPar?"▾":"▸"}</span>`}
            </th>
            ${flatRows.map((cE,ci)=>{
              const cItem=cE.isUnit?cE.unitPath:cE.label;
              const isSelfGroup=!rE.isUnit&&!cE.isUnit&&rE.label===cE.label;
              const isSelfUnit=rE.isUnit&&cE.isUnit&&rE.unitPath===cE.unitPath;
              const isDiag=isSelfGroup||isSelfUnit;
              const isChildBand=(rE.isUnit&&!!rE.parentLabel)||(cE.isUnit&&!!cE.parentLabel);
              if(rE.isMore||cE.isMore)return html`<td key=${ci} style=${{width:cellSz,height:cellSz,
                border:`1px solid ${T.border}22`,background:"transparent"}}></td>`;
              if(isDiag)return html`<td key=${ci} style=${{width:cellSz,height:cellSz,
                textAlign:"center",background:T.band,color:T.mute,fontSize:8,
                border:`1px solid ${T.border}44`,lineHeight:cellSz+"px",cursor:"default"}}>·</td>`;
              const rv=getCellVal(rItem,cItem,rE.isUnit,cE.isUnit);
              const cv=getCellVal(cItem,rItem,cE.isUnit,rE.isUnit);
              const selKey="dsm:"+rE.key+","+cE.key;
              const isCellSel=selId===selKey;
              const bg=isCellSel?T.cardH:rv&&cv?T.red+"33":rv?T.blue+"22":isChildBand?T.band+"88":"transparent";
              const fc=rv&&cv?T.red:rv?T.text:"transparent";
              return html`<td key=${ci} onClick=${()=>pickCell(rE,cE)}
                title=${rv?`${rE.fullLabel||rE.label} → ${cE.fullLabel||cE.label} · ${rv} imports${cv?" (mutual)":""}`:null}
                style=${{width:cellSz,height:cellSz,textAlign:"center",
                  border:`1px solid ${isCellSel?T.blue:isChildBand?T.arch+"33":T.border+"44"}`,
                  cursor:rv?"pointer":"default",background:bg,color:fc,
                  fontWeight:700,fontSize:cellSz<26?8:9.5,
                  lineHeight:cellSz+"px",overflow:"hidden",userSelect:"none"}}>
                ${rv||""}</td>`;})}
            ${targetN>0?html`<td key="ext-cell"
              style=${{width:90,minWidth:90,maxWidth:90,height:cellSz,
                borderLeft:`2px solid ${T.borderR}`,
                background:"transparent"}}></td>`:null}
          </tr>`;})}</tbody>
      </table>
      ${unlinkedItems.length>0?html`<div style=${{marginTop:8,padding:"4px 0",display:"flex",
        flexWrap:"wrap",gap:6,alignItems:"center"}}>
        <span style=${{fontSize:9.5,color:T.mute,marginRight:4,flexShrink:0,fontWeight:600}}>UNLINKED (${unlinkedItems.length})</span>
        ${unlinkedItems.map((label,i)=>html`<span key=${i} style=${{fontSize:9,color:T.mute,
          background:T.band,borderRadius:3,padding:"1px 6px",border:`1px solid ${T.border}`}}>${label}</span>`)}
        <span style=${{fontSize:9,color:T.mute,marginLeft:2}}>no dependencies at this grain</span>
      </div>`:null}
    </div>
    ${extOpen&&targetN>0?html`<div style=${{
      position:"absolute",right:0,top:SUMM_H,bottom:208,
      width:320,background:T.raise,borderLeft:`1px solid ${T.border}`,
      overflowY:"auto",padding:"12px 14px",zIndex:4,
      boxShadow:"-6px 0 16px #0006"}}>
      <div style=${{fontSize:9,fontWeight:700,letterSpacing:1.2,color:T.yellow,marginBottom:10,
        display:"flex",alignItems:"center",gap:8}}>
        <span>EXTERNALS · ${targetN} targets</span>
        <button onClick=${()=>setExtOpen(false)}
          style=${{marginLeft:"auto",background:"transparent",border:"none",
          color:T.mute,cursor:"pointer",fontSize:13}}>×</button>
      </div>
      <div style=${{fontSize:9,color:T.mute,marginBottom:10,lineHeight:1.5}}>
        Ranked by import count — what this codebase pulls from outside.
      </div>
      ${extRanked.map((ext,i)=>html`<div key=${i} style=${{
        marginBottom:10,paddingBottom:10,
        borderBottom:i<extRanked.length-1?`1px solid ${T.border}33`:"none"}}>
        <div style=${{display:"flex",alignItems:"baseline",gap:8,marginBottom:3}}>
          <span style=${{fontFamily:"ui-monospace,monospace",fontSize:10,
            color:T.yellow,fontWeight:600,flex:1,minWidth:0,
            overflow:"hidden",textOverflow:"ellipsis",whiteSpace:"nowrap"}}
            title=${ext.label}>${ext.label}</span>
          <span style=${{fontSize:10,fontWeight:700,color:T.text,flexShrink:0}}>×${ext.total.toLocaleString()}</span>
        </div>
        <div style=${{fontSize:9,color:T.mute,paddingLeft:8,lineHeight:1.6}}>
          ${ext.importers.map((im,j)=>html`<span key=${j} style=${{display:"inline-block",marginRight:8}}>
            ${im.actor}<span style=${{color:T.dim}}> ×${im.count}</span></span>`)}
        </div>
      </div>`)}
    </div>`:null}
    <div style=${{fontSize:9.5,color:T.dim,marginTop:5,flexShrink:0,display:"flex",gap:12,flexWrap:"wrap"}}>
      <span>cell = imports row → column</span>
      <span><span style=${{color:T.red}}>■</span> mutual pair (cycle)</span>
      <span style=${{color:T.mute}}>· diagonal (self)</span>
      <span style=${{color:T.mute}}>▸/▾ row header — expand units</span>
      ${targetN>0?html`<span style=${{color:T.yellow}}>Externals (${targetN}) ▸ — click to inspect</span>`:null}
    </div>
  </div>`;}
// invisible nodes covering edge-label extents so fitView never clips a label
// (React Flow fits to NODE bounds only; detour-routed edge labels can fall outside)
function SpacerNode(){return html`<div style=${{pointerEvents:"none"}}></div>`;}
// CapsuleNode: collapsed or expandable group capsule (Rung 0/1 per §4.1)
// Collapsed: header + count chip only. Expanded: full bucket content (via BucketNode).
function CapsuleNode({data}){
  const c=data.col;
  const ext=data._external;
  const dash=ext||data.layer==="supporting"||data.boundary;
  if(data.expanded){
    return html`<${BucketNode} data=${data}/>`;
  }
  const cnt=data.memberCount!==undefined?data.memberCount:data.members&&data.members.length||0;
  return html`<div class=${hvc(data.id)} style=${{width:"100%",height:"100%",background:T.band,
    border:`1.5px ${dash?"dashed":"solid"} ${ext?T.mute:c}`,borderRadius:4,boxSizing:"border-box",
    cursor:"pointer",display:"flex",alignItems:"center",gap:7,padding:"7px 12px",
    opacity:data._dim?0.8:1,transition:"opacity 150ms ease, box-shadow 150ms ease",
    boxShadow:data._sel?`0 0 0 2px ${T.blue}, 0 0 10px ${T.blue}77`:"none"}}>
    <${Handle} type="target" position=${Position.Top} style=${{opacity:0}}/><${Handle} type="target" position=${Position.Left} style=${{opacity:0}}/>
    <${Ico} k=${data.ico||data.layer} c=${ext?T.mute:c} s=${14}/>
    <span style=${{fontSize:11,fontWeight:700,color:ext?T.mute:c,textTransform:"uppercase",letterSpacing:1.1,
      whiteSpace:"nowrap",flex:1,overflow:"hidden",textOverflow:"ellipsis"}}>${data.label}</span>
    <span style=${{fontSize:10.5,fontWeight:700,color:ext?T.mute:T.dim,flexShrink:0,
      background:T.chrome,borderRadius:8,padding:"1px 6px"}}>${cnt}</span>
    <span data-expand-chip="1" title="expand" style=${{fontSize:9,fontWeight:700,color:T.dim,flexShrink:0,
      border:`1px solid ${T.border}`,borderRadius:4,padding:"1px 5px"}}>▸</span>
    <${HoverCard} title=${data.label} rows=${[["members",cnt]]} hint="click to select · ▸ to expand"/>
    <${Handle} type="source" position=${Position.Bottom} style=${{opacity:0}}/><${Handle} type="source" position=${Position.Right} style=${{opacity:0}}/>
  </div>`;}
function labelSpacers(laidById){const sp=[];let i=0;
  Object.values(laidById).forEach(le=>{(le.labels||[]).forEach(l=>{
    if(l.x===undefined)return;
    sp.push({id:"_lsp"+(i++),type:"spacer",position:{x:l.x,y:l.y},
      width:l.width||40,height:l.height||16,draggable:false,selectable:false,data:{}});});});
  return sp;}
const nodeTypes={box:BoxNode,bucket:BucketNode,solo:SoloNode,member:MemberNode,entity:EntityNode,spacer:SpacerNode,capsule:CapsuleNode};
const edgeTypes={elk:ElkEdge};

const SP={
 compact:{layers:"48",edgeNodeBL:"24",edgeNode:"16",edgeEdge:"12",edgeEdgeBL:"12",nodeNode:"32",edgeLabel:"4",
          head:30,px:10,py:8,iw:144,ih:28,gx:8,gy:6,solo:[232,40]},
 comfort:{layers:"80",edgeNodeBL:"32",edgeNode:"24",edgeEdge:"16",edgeEdgeBL:"16",nodeNode:"48",edgeLabel:"8",
          head:40,px:16,py:16,iw:152,ih:48,gx:16,gy:8,solo:[248,56]}};
const EOPT=(dir,d)=>({"elk.algorithm":"layered","elk.direction":dir,"elk.edgeRouting":"ORTHOGONAL",
 "elk.layered.nodePlacement.strategy":"NETWORK_SIMPLEX",
 "elk.layered.nodePlacement.favorStraightEdges":"true",
 "elk.layered.considerModelOrder.strategy":"NODES_AND_EDGES",
 "elk.layered.thoroughness":"50",
 "elk.spacing.edgeNode":d.edgeNode,"elk.layered.spacing.edgeNodeBetweenLayers":d.edgeNodeBL,
 "elk.spacing.edgeEdge":d.edgeEdge,"elk.layered.spacing.edgeEdgeBetweenLayers":d.edgeEdgeBL,
 "elk.layered.spacing.nodeNodeBetweenLayers":d.layers,"elk.spacing.nodeNode":d.nodeNode,
 "elk.spacing.edgeLabel":d.edgeLabel,
 "elk.edgeLabels.placement":"CENTER","elk.edgeLabels.inline":"true"});
const lblW=t=>snap(t.length*7+14);

function rfEdge(e,laidById,col,nameOf){
  const le=laidById[e.id]||{};
  const section=(le.sections&&le.sections[0])||null;
  const lab=(le.labels&&le.labels[0])||null;
  return {id:e.id,source:e.source,target:e.target,type:"elk",interactionWidth:20,
    data:{section,label:lab?{x:lab.x,y:lab.y,text:lab.text}:null,
      meta:{sid:e.source,tid:e.target,s:nameOf?nameOf(e.source):e.source,t:nameOf?nameOf(e.target):e.target,
        verb:e.label||"",count:e.count,tag:e.tag,stats:e.stats}},
    markerEnd:{type:"arrowclosed",color:col(e),width:13,height:13},
    style:{stroke:col(e),strokeWidth:2,opacity:.85}};}

async function layoutSimple(view,dir,d,showFindings){
  const ent=view.kind==="entity";
  const sizes={};
  // M6-P1: missing/empty nodes|edges (Go omitempty drops empty slices) must lay out as a
  // trivially-empty canvas, not throw — the empty-shard card upstream is the real UX for this
  // case, but a bare-JSON caller (or a future kind that skips the isEmpty gate) must not crash.
  const nodes=view.nodes||[],edges=view.edges||[];
  nodes.forEach(n=>{
    if(ent){sizes[n.id]={w:Math.min(400,Math.max(264,snap((n.label||"").length*7.6+62))),h:34+n.fields.length*21+2};}
    else {const base=n.type==="sys"?248:208;
      // canvas carries identity only (sub moved to hover) — width follows the label, never ellipsis
      sizes[n.id]={w:Math.min(360,Math.max(base,snap((n.label||"").length*7.8+(n.drillTo?118:66)))),h:44};}});
  const g={id:"root",layoutOptions:EOPT(dir,d),
    children:nodes.map(n=>({id:n.id,width:sizes[n.id].w,height:sizes[n.id].h})),
    edges:edges.map(e=>({id:e.id,sources:[e.source],targets:[e.target],
      labels:[{text:e.label||"",width:lblW(e.label||""),height:16}]}))};
  const r=await elk.layout(g);
  const pos={};r.children.forEach(c=>pos[c.id]={x:snap(c.x),y:snap(c.y)});
  const laidById={};(r.edges||[]).forEach(e=>laidById[e.id]=e);
  const problems=[];const nById={};nodes.forEach(n=>nById[n.id]=n);
  edges.forEach(e=>{if(e.tag){e._viol=true;
    problems.push(e.tag+": "+((nById[e.source]||{}).label)+" → "+((nById[e.target]||{}).label));}});
  return {nodes:nodes.map(n=>({id:n.id,type:ent?"entity":"box",position:pos[n.id],
      width:sizes[n.id].w,height:sizes[n.id].h,
      data:{...n,w:sizes[n.id].w,h:sizes[n.id].h}})).concat(labelSpacers(laidById)),
    edges:edges.map(e=>{const r=rfEdge(e,laidById,()=>T.dim,id2=>(nById[id2]||{}).label||id2);
      // A3 calm default: violation is still DETECTED (e._viol, problems[] above) but the red/dashed
      // PAINT only shows when the Findings lens is on — a stranger sees a plain map, not alarms.
      if(e._viol&&showFindings){r.style={...r.style,stroke:T.red,strokeDasharray:"6 3"};
        r.markerEnd={...r.markerEnd,color:T.red};
        if(r.data.label)r.data.label.text="⚠ "+r.data.label.text;}
      return r;}),problems};}

// Color is meaning: same layer name => same color, app-wide. Resolution order:
// canonical role pin (always wins for the 6-role spine) -> view palette ->
// stable name hash. Red/yellow are RESERVED (violations / warnings) and never
// appear in the rotation.
const CYCLE=[T.arch,T.cyan,T.purple,T.green,T.blue,T.neutral];
const LAYER_PIN={core:T.blue,channel:T.purple,integration:T.arch,data:T.green,
  external:T.dim,supporting:T.neutral,platform:T.cyan,edge:T.cyan};
// CANON_ROLES: the 6-role spine (roles.go roleFor()) — a palette must never
// repaint these; only non-canonical/empty layers may fall back to a palette
// entry or the hash probe.
const CANON_ROLES=new Set(["core","edge","integration","data","external","supporting"]);
const lhash=s=>{let h=0;for(let i=0;i<s.length;i++)h=(h*31+s.charCodeAt(i))>>>0;return h;};
// Per-view color map: canonical roles are absolute (LAYER_PIN); remaining
// palette/pinned names are absolute; hashed names probe to a free color on
// collision so one view never shows two layers in the same color.
const viewLayerColors=view=>{const pal=PALETTES[view.palette||"aoa"]||{};
  const used=new Set(),map={};
  const layers=[...new Set((view.buckets||[]).map(b=>b.layer))];
  layers.forEach(l=>{const c=CANON_ROLES.has(l)?LAYER_PIN[l]:(pal[l]||LAYER_PIN[l]);
    if(c){map[l]=c;used.add(c);}});
  layers.forEach(l=>{if(map[l])return;
    let i=lhash(l||"")%CYCLE.length,n=0;
    while(n<CYCLE.length&&used.has(CYCLE[i])){i=(i+1)%CYCLE.length;n++;}
    map[l]=CYCLE[i];used.add(CYCLE[i]);});
  return map;};
const layerColor=(view,l)=>viewLayerColors(view)[l]||LAYER_PIN[l]||CYCLE[lhash(l||"")%CYCLE.length];
// mergeExternalBuckets: fold all g_ext_* buckets into ONE EXTERNALS capsule (R3).
// Returns [internals[], externals_capsule_or_null].
function mergeExternalBuckets(buckets){
  buckets=buckets||[]; // M6-P1: Go omitempty drops an empty buckets slice from the shard JSON
  const internal=buckets.filter(b=>!b.id.startsWith("g_ext_"));
  const external=buckets.filter(b=>b.id.startsWith("g_ext_"));
  if(!external.length)return[buckets,null];
  // Each g_ext_* bucket arrives Go-side sorted WITHIN itself (fan-in desc then label,
  // render_component.go:53), but flatMap only concatenates bucket-by-bucket — it does NOT
  // interleave by weight ACROSS the merge. Re-sort the merged list by the same "in N" fan-in
  // label Go stamps on every member (A7 punch: load-bearing externals like std ×N must top
  // EXTERNALS, not whichever bucket happened to sort first — e.g. collections/copy noise).
  const finOf=m=>{const mm=/in (\d+)/.exec((m&&m.sub)||"");return mm?parseInt(mm[1],10):0;};
  const allExtMembers=external.flatMap(b=>b.members||[])
    .sort((a,b)=>finOf(b)-finOf(a)||String((a&&a.label)||"").localeCompare(String((b&&b.label)||"")));
  // Dominant-role icon (80/20): every folded bucket is already role="external" (roles.go),
  // so the majority vote over member layers is unanimous — no new role-plumbing needed to
  // support a real mixed-role vote; wire one in only if a future merge point mixes roles.
  const extCapsule={id:"g_EXTERNALS",layer:"external",boundary:true,ico:"cloud",
    label:"EXTERNALS",members:allExtMembers,
    memberCount:allExtMembers.length,part:99,
    _external:true};
  return[internal,extCapsule];}
async function layoutBuckets(view,dir,d,ov,opts){
  ov=ov||{};opts=opts||{};
  const capsuleMode=!!opts.capsuleMode;
  const showFindings=!!opts.showFindings;
  const expandedSet=capsuleMode?(opts.expandedGroups||new Set()):null;
  // EXTERNALS fold: merge g_ext_* into one capsule (R3)
  const[internalBuckets,extCapsule]=mergeExternalBuckets(view.buckets);
  const B=extCapsule?[...internalBuckets,extCapsule]:internalBuckets;
  const colOf=l=>layerColor(view,l);
  // remapId: redirects g_ext_* → g_EXTERNALS when the external fold is active
  const remapId=id=>extCapsule&&id&&id.startsWith("g_ext_")?"g_EXTERNALS":id;
  const problems=[];const deg={};B.forEach(b=>deg[b.id]={i:0,o:0});
  const bById={};B.forEach(b=>bById[b.id]=b);
  // Also keep original bById for label lookups (before fold)
  const origBById={};(view.buckets||[]).forEach(b=>origBById[b.id]=b);
  const nameOfBucket=id=>(bById[id]||origBById[id]||{}).label||id;
  // Build remapped edges for degree + ELK (g_ext_* → g_EXTERNALS)
  const remappedEdges=(view.edges||[]).map(e=>({...e,source:remapId(e.source),target:remapId(e.target)}));
  // Deduplicate remapped edges by (source,target) so merged externals don't produce parallel ELK edges
  const edgeSeen=new Set();
  // Use let so first-paint edge restraint can reassign below.
  let elkEdges=remappedEdges.filter(e=>{if(e.source===e.target)return false;
    const k=e.source+"\x00"+e.target;if(edgeSeen.has(k))return false;edgeSeen.add(k);return true;});
  elkEdges.forEach(e=>{if(deg[e.source])deg[e.source].o++;if(deg[e.target])deg[e.target].i++;
    const sp=bById[e.source],tp=bById[e.target];
    // A3 rule (4): band violations are PERMANENTLY suppressed on inferred layers — not gated by
    // showFindings, killed outright. roleFor() heuristics are the only Layer/Part source today
    // (Bucket.inferred is always true from Go), so this never fires until a real declared-layer
    // (V2) contract sets inferred:false on both endpoints. Prescriptive findings must never come
    // from a descriptive/inferred layer — that's what made gin look broken (false alarms).
    if(sp&&tp&&sp.part>tp.part&&sp.inferred===false&&tp.inferred===false){e._viol=true;problems.push("band violation: "+nameOfBucket(e.source)+" → "+nameOfBucket(e.target));}
    if(e.tag){e._viol=true;problems.push(e.tag+": "+nameOfBucket(e.source)+" → "+nameOfBucket(e.target));}});
  B.forEach(b=>{const dg=deg[b.id];
    if(dg.i+dg.o===0&&B.length>1){b._dead=true;problems.push("orphan: "+b.label+" — no connections");}
    if(dg.i>=3&&dg.o>=3){b._god=true;problems.push("god component: "+b.label+" (in "+dg.i+" · out "+dg.o+")");}});
  (function(){const adj={};elkEdges.forEach(e=>{(adj[e.source]=adj[e.source]||[]).push(e.target);});
   const st={};let cyc=null;
   function dfs(u,stk){st[u]=1;stk.push(u);
     for(const v2 of adj[u]||[]){if(st[v2]===1){cyc=stk.slice(stk.indexOf(v2)).concat(v2);return true;}
       if(!st[v2]&&dfs(v2,stk))return true;}
     st[u]=2;stk.pop();return false;}
   for(const b of B){if(!st[b.id]&&dfs(b.id,[]))break;}
   if(cyc){problems.push("dependency cycle: "+cyc.map(id=>bById[id]?bById[id].label:id).join(" → "));
     cyc.forEach(id=>{if(bById[id])bById[id]._cyc=true;});}})();
  // First-paint edge restraint (§4 design brief): in capsule mode with no groups expanded,
  // show only the TOP-K=12 heaviest inter-group flows; the rest are summarised in the caption.
  // When any group is expanded the user has engaged — show full edge set.
  // Degrees + findings above are computed on the FULL set so findings remain accurate.
  const FIRST_PAINT_K=12;
  let moreFlows=0;
  if(capsuleMode&&expandedSet&&expandedSet.size===0&&elkEdges.length>FIRST_PAINT_K){
    const sorted=elkEdges.slice().sort((a,b)=>(b.count||0)-(a.count||0));
    moreFlows=elkEdges.length-FIRST_PAINT_K;
    elkEdges=sorted.slice(0,FIRST_PAINT_K);}
  B.forEach(b=>{if(!capsuleMode&&(b.members||[]).length>40){b._over=b.members.length;
    b.members=b.members.slice(0,23).concat([{id:b.id+"_more",label:"+"+(b._over-23)+" more…",sub:"over budget"}]);}});
  // Decision 3 (A7 consensus): inside an EXPLICITLY expanded capsule group, render only the
  // top-N members by weight — members already arrive Go-side ranked by fan-in desc
  // (render_component.go), so this is a pure slice, no client-side re-ranking needed. Budgets:
  // EXTERNALS (reference material) ~2, internal groups ~5-7. b._allMembers preserves the FULL
  // ranked list so the dock's SELECTION segment can still show more than the canvas budget when
  // the group (or its "+N more" fold row, via onNodeClick) is selected.
  // A7 punch (regression lens): this must NEVER mutate the bucket object in place. view.buckets
  // is a persistent reference reused across renders/toggles — in-place mutation left a
  // collapse-after-expand permanently stuck showing the truncated count (CapsuleNode's badge)
  // and corrupted the view's own caption() member total, because collapsing only flips
  // expandedGroups and never restored the original members. Swap in a fresh copy (in B and
  // bById) instead: the original bucket is untouched, so collapsing simply drops the copy and
  // the next expand starts from the true, full member list again. This also makes the
  // DOWN/RIGHT dual layout pass (fitScale trial) naturally idempotent — each call re-derives B
  // from the still-pristine view.buckets, so no re-truncation guard is needed.
  B.forEach((b,bi)=>{
    if(!(capsuleMode&&expandedSet&&expandedSet.has(b.id)))return;
    const budget=b._external?2:6;
    const total=(b.members||[]).length;
    if(total<=budget)return;
    const nb={...b,_allMembers:b.members,
      members:b.members.slice(0,budget).concat([{id:b.id+"_more",
        label:"+"+(total-budget)+" more",sub:"",_foldMore:true,_parentId:b.id}])};
    B[bi]=nb;bById[b.id]=nb;});
  B.forEach(b=>{
    // Capsule mode: collapsed buckets use header-only height; EXTERNALS capsule always collapsed
    if(capsuleMode&&!expandedSet.has(b.id)){
      b._capsule=true;b._collapsed=true;
      b.w=Math.max(d.solo[0],snap((b.label||"").length*7.4+120));b.h=d.head;return;}
    const n=(b.members||[]).length;
    b.solo=(n===1);
    if(b.solo){b.w=Math.max(d.solo[0],snap((b.members[0].label||"").length*7.2+(b.label||"").length*7+96));b.h=d.solo[1];return;}
    // member cell width follows the longest label (sub moved to hover); very long labels wrap to 2 lines
    const need=snap(Math.max(...b.members.map(m=>(m.label||"").length))*6.7+44);
    b.wrap=need>240;
    b.iw=b.wrap?240:Math.max(d.iw,need);
    b.ih=b.wrap?d.ih+13:d.ih;
    const cols=n<=4?n:Math.min(5,Math.ceil(n/2));
    const rows=Math.ceil(n/cols);
    b.cols=cols;
    b.w=d.px*2+cols*b.iw+(cols-1)*d.gx;
    b.h=d.head+d.py+rows*b.ih+(rows-1)*d.gy+d.py;});
  const g={id:"root",
    layoutOptions:{...EOPT(dir,d),"elk.partitioning.activate":"true"},
    children:B.map(b=>({id:b.id,width:b.w,height:b.h,
      layoutOptions:{"elk.partitioning.partition":String(b.part),
                     "elk.portConstraints":"FIXED_SIDE"}})),
    edges:elkEdges.map(e=>({id:e.id,sources:[e.source],targets:[e.target],
      labels:[{text:(view.labeled&&e.label)?e.label+" ×"+e.count:"×"+e.count,
               width:lblW((view.labeled&&e.label)?e.label+" x"+e.count:"x"+e.count),height:16}]}))};
  const r=await elk.layout(g);
  const pos={};r.children.forEach(c=>pos[c.id]={x:snap(c.x),y:snap(c.y)});
  const laidById={};(r.edges||[]).forEach(e=>laidById[e.id]=e);
  const nodes=[];
  B.forEach(b=>{const bp=pos[b.id];const col=colOf(b.layer);
    // Capsule mode: collapsed buckets render as CapsuleNode (header-only, no members)
    if(b._collapsed){
      nodes.push({id:b.id,type:"capsule",position:bp,width:b.w,height:b.h,
        style:{width:b.w,height:b.h},zIndex:0,draggable:false,selectable:false,
        data:{...b,head:d.head,col,expanded:false,_showFindings:showFindings}});
      return;}
    if(b.solo){
      nodes.push({id:b.id,type:"solo",position:bp,width:b.w,height:b.h,
        zIndex:0,draggable:false,selectable:false,
        data:{layer:b.layer,label:b.label,member:b.members[0],w:b.w,h:b.h,col}});
      return;}
    nodes.push({id:b.id,type:"bucket",position:bp,width:b.w,height:b.h,
      style:{width:b.w,height:b.h},zIndex:0,draggable:false,selectable:false,
      data:{...b,head:d.head,col,_fromCapsule:capsuleMode&&!b._collapsed,_showFindings:showFindings}});
    b.members.forEach((m,i)=>{const c2=i%b.cols,row=Math.floor(i/b.cols);
      nodes.push({id:m.id,type:"member",draggable:false,selectable:false,zIndex:10,
        width:b.iw,height:b.ih,
        position:{x:bp.x+d.px+c2*(b.iw+d.gx),y:bp.y+d.head+d.py+row*(b.ih+d.gy)},
        // Member ⚠/Δ badges are sub-toggles UNDER the Findings lens (consensus doc CONTROLS) —
        // they only show when showFindings is on, even if ov.concerns/ov.changed are set via URL.
        data:{...m,lay:b.layer,col,w:b.iw,h:b.ih,wrap:b.wrap,
          showC:!!(showFindings&&ov.concerns&&m.concerns>0),showH:!!(showFindings&&ov.changed&&m.changed)}});});});
  const bcol={};B.forEach(b=>bcol[b.id]=colOf(b.layer));
  const edges2=elkEdges.map(e=>{const r=rfEdge(e,laidById,ee=>bcol[ee.source]||T.dim,id2=>nameOfBucket(id2));
    // A3 calm default: violation is still DETECTED (e._viol, problems[] above) but the red/dashed
    // PAINT only shows when the Findings lens is on.
    if(e._viol&&showFindings){r.style={...r.style,stroke:T.red,strokeDasharray:"6 3",opacity:.95};
      r.markerEnd={...r.markerEnd,color:T.red};
      if(r.data.label)r.data.label.text="⚠ "+r.data.label.text;}
    // Expanded group: its flows drawn full-weight (§4 brief "edges fully drawn");
    // flows not touching the open group are restrained — dim + thin so they recede.
    if(expandedSet&&expandedSet.size>0&&!e._viol){
      const touching=expandedSet.has(e.source)||expandedSet.has(e.target);
      r.style={...r.style,opacity:touching?1:0.18,strokeWidth:touching?2.5:1};}
    return r;});
  return {nodes:nodes.concat(labelSpacers(laidById)),edges:edges2,problems,moreFlows};}

// Docked footer: ONE derived legend — shows only what the current view actually renders —
// plus the provenance stamp anchored right.
// Bottom dock — the drill tier. THREE FIXED SEGMENTS, always present, content changes:
//   VIEW (this view's record: question · pass · sources · counts)
//   SELECTION (the clicked element/edge: stat table + relations table, violations first)
//   CONCERNS (this view's diagnostics; rows touching the selection highlight)
// Persistent: 26px collapsed bar <-> 208px expanded. Never unmounts; the nav rail never reflows.
// A8 FIX 2b (owner: dock member table "hard to read" — breathing room + scannable numerals):
// padding/lineHeight bumped up from the original 3px/1.45 for legibility at the dense house
// aesthetic's edge.
const DK={th:{fontSize:8.5,fontWeight:700,letterSpacing:.8,textTransform:"uppercase",color:T.mute,
    textAlign:"left",padding:"2px 10px 5px 0",borderBottom:`1px solid ${T.border}`},
  td:{fontSize:11.5,color:T.text,padding:"5px 10px 5px 0",borderBottom:`1px solid ${T.border}55`,
    verticalAlign:"top",lineHeight:1.6}};
// rightCols: optional array of booleans (by column index) forcing a real right-aligned numeral
// column (header + cells) — opt-in per caller so existing tables keep their prior layout; the
// member table (A8 FIX 2b) is the first to use it for its fan-in weight column.
function DockTable({cols,rows,rightCols}){
  const isRight=ci=>!!(rightCols&&rightCols[ci]);
  return html`<table style=${{borderCollapse:"collapse",width:"100%"}}>
    <thead><tr>${cols.map((c,i)=>html`<th key=${i} style=${{...DK.th,textAlign:isRight(i)?"right":"left"}}>${c}</th>`)}</tr></thead>
    <tbody>${rows.map((r,ri)=>html`<tr key=${ri} style=${r._hl?{background:T.cardH}:null}>${r.map((cell,ci)=>html`<td key=${ci}
      style=${{...DK.td,color:r._viol?T.red:ci===0?T.dim:T.text,
        textAlign:isRight(ci)?"right":"left",
        fontFamily:ci>0&&isNumCell(cell)?"ui-monospace,monospace":"inherit"}}>${cell}</td>`)}</tr>`)}
  </tbody></table>`;}
// The caption: the view ANSWERS its own question, derived at render time from data
// already on screen — counts, heaviest edge, mutual pairs, flagged rows. A3 calm default:
// caption() itself NEVER mentions findings — it always reads as one calm sentence (house
// ruling "calm like a map"). The findings tail is a separate clause (findingsClause below)
// the CALLER appends only when the Findings lens (showFindings) is on — this is presentation
// gating only, the shard/goldens are untouched.
function caption(view,moreFlows){
  if(view.kind==="buckets"){
    const B=view.buckets||[],members=B.reduce((a,b)=>a+(b.members||[]).length,0);
    const he=(view.edges||[]).reduce((m,e)=>((e.count||0)>(m.count||0)?e:m),{});
    const bn=id=>{const b=B.find(x=>x.id===id);return b?b.label:id;};
    const mf=moreFlows?` · +${moreFlows} more flows — click groups to reveal`:"";
    return `${B.length} groups · ${members} members`+
      (he.id?` — heaviest: ${bn(he.source)} → ${bn(he.target)} ×${he.count}`:"")+mf;}
  if(view.kind==="matrix"){
    const it=view.items||[],M=view.matrix||[];let sum=0;const mut=[];
    for(let i=0;i<it.length;i++)for(let j=0;j<it.length;j++){sum+=(M[i]||[])[j]||0;
      if(i<j&&(M[i]||[])[j]&&(M[j]||[])[i])mut.push(`${it[i]} ⇄ ${it[j]} (${M[i][j]}/${M[j][i]})`);}
    return `${it.length} modules · ${sum.toLocaleString()} dependencies · ${mut.length} mutual pair${mut.length===1?"":"s"}`+
      (mut.length?` — worst: ${mut[0]}`:"");}
  if(view.kind==="table"){
    return `${(view.rows||[]).length} rows`;}
  if(view.kind==="entity"){
    const deg={};(view.edges||[]).forEach(e=>{deg[e.source]=(deg[e.source]||0)+1;deg[e.target]=(deg[e.target]||0)+1;});
    const top=Object.entries(deg).sort((a,b)=>b[1]-a[1])[0];
    const nm=id=>{const n=(view.nodes||[]).find(x=>x.id===id);return n?n.label:id;};
    return `${(view.nodes||[]).length} entities · ${(view.edges||[]).length} relationships`+
      (top?` — spine: ${nm(top[0])} (${top[1]} relations)`:"");}
  return `${(view.nodes||[]).length} elements · ${(view.edges||[]).length} labeled flows`;}
// findingsClause: the ⚠-tail caption() used to carry inline. Appended by the caller ONLY when
// showFindings is on. Table kind keeps its own "⚠ N flagged" shape (row-scanned, not probs-based);
// default/simple keeps its tag-based shape; buckets/entity share the generic "N findings" shape.
function findingsClause(view,probs){
  if(view.kind==="matrix")return""; // matrix caption already ends on "mutual pairs" — no separate clause
  if(view.kind==="table"){
    const rows=view.rows||[];const fl=rows.filter(r=>r.some(c=>String(c).trim().startsWith("⚠")));
    return fl.length?` · ⚠ ${fl.length} flagged — first: ${fl[0][0]}`:" · none flagged";}
  if(view.kind==="buckets"||view.kind==="entity")
    return probs&&probs.length?` · ⚠ ${probs.length} finding${probs.length>1?"s":""}`:"";
  const tag=(view.edges||[]).find(e=>e.tag);
  return tag?` · ⚠ ${tag.tag}: ${(tag.label||"").slice(0,48)}`
    :(probs&&probs.length?` · ⚠ ${probs.length} finding${probs.length>1?"s":""}`:"");}
// M6-P1: a shard can honestly have zero content (e.g. no IaC in this repo at all — the first
// empty "buckets" shard in the product's life; a "simple"/"entity" shard could go empty the
// same way). shardIsEmpty is the single gate deciding whether to hand the loaded shard to its
// normal renderer/layout or to the calm empty-state card below — table/matrix already render
// their own zero-row/zero-item state gracefully (cycles' "no cycles" card lives inside
// TableView, untouched here), so only buckets/simple/entity need this gate.
const shardIsEmpty=view=>{
  if(view.kind==="buckets")return!(view.buckets||[]).length;
  if(view.kind==="simple"||view.kind==="entity")return!(view.nodes||[]).length;
  return false;};
// Per-view honest empty copy (deployment is the concrete case that surfaced this — "nothing
// ships from this repo" answers the view's own question rather than a generic placeholder).
// Any other buckets/simple/entity view that goes empty falls back to a generic-but-honest
// message built from the view's own title + Go-derived count string (never fabricated).
const EMPTY_SHARD_COPY={
  deployment:{headline:"no deploy artifacts found — nothing ships from this repo",
    sub:"0 deploy artifacts · no Dockerfile, compose.yaml, or Kubernetes manifest found in this codebase"}};
function EmptyShardView({view,vid}){
  const copy=EMPTY_SHARD_COPY[vid]||{
    headline:`no ${(view.title||"content").toLowerCase()} found in this repo`,
    sub:(view.count||"0 elements")+" · nothing collected from this codebase"};
  return html`<div style=${{position:"absolute",inset:0,display:"flex",alignItems:"center",justifyContent:"center"}}>
    <div style=${{textAlign:"center",maxWidth:480,padding:"40px 32px",background:T.card,
      border:`1px solid ${T.border}`,borderRadius:12}}>
      <div style=${{fontSize:28,marginBottom:12,color:T.dim}}>○</div>
      <div style=${{fontSize:16,fontWeight:700,color:T.text,marginBottom:8}}>${copy.headline}</div>
      <div style=${{fontSize:11.5,color:T.dim,lineHeight:1.6}}>${copy.sub}</div>
      ${view.prov?html`<div style=${{marginTop:14,fontSize:10,color:T.mute}}>${view.prov.label}</div>`:null}
    </div>
  </div>`;}
// Dock SELECTION empty-state text (calm default, THE FIX §"ambient cue"): canvas views
// (buckets/simple) get the map's invitation; table (BEAU-1: rows are the only clickable
// element, so name the row) and matrix (DSM: cells + row-header expand) keep their own
// affordance text since neither has clickable canvas elements.
const emptySelText=view=>(view.kind==="buckets"||view.kind==="simple")
  ?"click any element to inspect →"
  :view.kind==="table"
  ?"click a row to inspect"
  :"click a cell to inspect · click a row header to expand";
// A5 SELECTION action row — ⧉ Copy: a compact plain-text record of the selection, pasteable
// anywhere. Kept deliberately terse (label/role/members/relations) — this is a clipboard
// payload, not the prompt (see genElementPrompt below).
function selRecordText(sel){
  const lines=[sel.label+(sel.chip?" ("+sel.chip+")":"")];
  if(sel.members&&sel.members.length)
    lines.push("members: "+sel.members.slice(0,40).map(m=>m.label).join(", "));
  if(sel.relations&&sel.relations.length){
    const ins=sel.relations.filter(r=>r.dir==="in").map(r=>r.peer);
    const outs=sel.relations.filter(r=>r.dir==="out").map(r=>r.peer);
    if(ins.length)lines.push("in ← "+ins.join(", "));
    if(outs.length)lines.push("out → "+outs.join(", "));
  }
  return lines.join("\n");}
// A5 SELECTION action row — ✎ Prompt agent: composes an EDITABLE prompt (system/scope context
// + selection identity/role/members/relations + a task placeholder) for the developer to paste
// into their own agent. Parallel to genPrompt() (view-authoring prompt above) — not a rewrite of
// it, this is a different flow (per-element, not per-view) that happens to reuse the same
// VIEW_INTENT question framing.
function genElementPrompt(estateId,scopeLabel,vid,sel){
  const VI=VIEW_INTENT[vid];
  const lines=["You are working in the \""+estateId+"\" estate · \""+scopeLabel+"\" system, view \""+vid+"\"."];
  if(VI)lines.push("This view answers: "+VI.question);
  lines.push("","Selected element: "+sel.label+(sel.chip?" ("+sel.chip+")":""));
  if(sel.members&&sel.members.length)
    lines.push("Members: "+sel.members.slice(0,40).map(m=>m.label).join(", "));
  if(sel.relations&&sel.relations.length){
    lines.push("Relations:");
    sel.relations.forEach(r=>lines.push("  "+(r.dir==="out"?"→":"←")+" "+r.peer+
      (r.verb?" ("+r.verb+")":"")+(r.count?" ×"+r.count:"")));}
  lines.push("","[TASK: describe what you want the agent to do with this element]");
  return lines.join("\n");}
function PromptComposer({text,onClose}){
  const[val,setVal]=useState(text);
  // position:fixed (viewport-anchored), not absolute within the SELECTION segment — that
  // column is only ~182px tall (208px dock minus the 26px collapsed bar) and a usable
  // multi-line textarea does not fit there; a small centered overlay avoids clipping/overflow.
  return html`<div onClick=${onClose} style=${{position:"fixed",inset:0,background:"#000000c0",
    zIndex:50,display:"flex",alignItems:"center",justifyContent:"center"}}>
    <div onClick=${e=>e.stopPropagation()} style=${{width:440,maxWidth:"92%",background:T.raise,
      border:`1px solid ${T.borderR}`,borderRadius:8,padding:14,boxShadow:"0 12px 40px #000a"}}>
      <div style=${{fontSize:9,fontWeight:700,letterSpacing:1,color:T.mute,marginBottom:8}}>PROMPT FOR AGENT · edit before copying</div>
      <textarea value=${val} onChange=${e=>setVal(e.target.value)}
        style=${{width:"100%",height:180,background:T.card,color:T.text,border:`1px solid ${T.border}`,
        borderRadius:6,padding:8,fontSize:11,fontFamily:"ui-monospace,monospace",resize:"vertical",boxSizing:"border-box"}}/>
      <div style=${{display:"flex",gap:8,marginTop:10,justifyContent:"flex-end"}}>
        <button onClick=${onClose} style=${{background:"transparent",border:`1px solid ${T.border}`,color:T.dim,
          borderRadius:6,padding:"5px 12px",fontSize:11,cursor:"pointer"}}>close</button>
        <button onClick=${()=>navigator.clipboard.writeText(val)} style=${{background:T.card,border:`1px solid ${T.blue}`,
          color:T.blue,borderRadius:6,padding:"5px 12px",fontSize:11,cursor:"pointer",fontWeight:600}}>copy prompt</button>
      </div>
    </div>
  </div>`;}
function BottomDock({vid,view,sel,selId,clearSel,probs,findings,onFindingClick,highlightMemberId,
  expanded,setExpanded,moreFlows,showFindings,estate,scopeLabel,setLevel,setDirOv}){
  const VI=VIEW_INTENT[vid]||null;
  const hl=p=>sel&&sel.label&&String(p).includes(String(sel.label).slice(0,24));
  const sortedProbs=sel?[...probs].sort((a,b)=>(hl(b)?1:0)-(hl(a)?1:0)):probs;
  // A5 (3a): buckets/simple/entity kinds get the daemon-derived structured FINDINGS list
  // (rule · message · subject, clickable); table/matrix keep their own local shape (probs) —
  // the scan-a-row/mutual-pair detection there has no canvas subject to select.
  const isCanvasKind=view.kind==="buckets"||view.kind==="simple"||view.kind==="entity";
  const findingRows=isCanvasKind?(findings||[]):null;
  const dockCount=findingRows?findingRows.length:probs.length;
  // A3 calm default: the caption is calm by itself; the ⚠ findings tail is appended here, the
  // caller, only when the Findings lens is on (caption()/findingsClause() split above). A5: the
  // tail must agree with the FINDINGS segment it sits above — both are gated on dockCount
  // (daemon-derived) instead of two different detectors disagreeing in the same dock. "simple"
  // keeps its more informative violating-edge label when dockCount confirms the daemon actually
  // flagged something in THIS scope; otherwise (e.g. a local edge tag with zero matching daemon
  // findings for the current scope) it falls silent like the FINDINGS segment does, instead of
  // showing a warning the segment below immediately contradicts.
  const countTail=view.kind==="buckets"||view.kind==="entity"
    ?(dockCount?` · ⚠ ${dockCount} finding${dockCount>1?"s":""}`:"")
    :view.kind==="simple"
    ?(dockCount?(()=>{const tag=(view.edges||[]).find(e=>e.tag);
        return tag?` · ⚠ ${tag.tag}: ${(tag.label||"").slice(0,48)}`
          :` · ⚠ ${dockCount} finding${dockCount>1?"s":""}`;})():"")
    :findingsClause(view,probs);
  const cap=caption(view,moreFlows||0)+(showFindings?countTail:"");
  const rowSelected=match=>{if(!match)return false;
    if(match.kind==="member")return selId===match.memberId||highlightMemberId===match.memberId;
    return selId===(match.bucketId||match.nodeId);};
  const sortedFindingRows=findingRows
    ?[...findingRows].sort((a,b)=>(rowSelected(b.match)?1:0)-(rowSelected(a.match)?1:0)):null;
  const[promptOpen,setPromptOpen]=useState(false);
  const Seg=({title,col,flex,wash,children})=>html`<div style=${{flex,minWidth:0,padding:"9px 16px",
    borderRight:`1px solid ${T.border}`,overflowY:"auto",position:"relative",background:wash||"transparent"}}>
    <div style=${{fontSize:9.5,fontWeight:700,letterSpacing:1.2,color:col,marginBottom:7,
      position:"sticky",top:0,background:T.raise,paddingBottom:3}}>${title}</div>
    ${children}</div>`;
  return html`<div style=${{borderTop:`1px solid ${T.border}`,background:T.raise,height:208,
    display:"flex",flexDirection:"column",flexShrink:0,position:"relative",boxShadow:"0 -6px 16px #0009"}}>
    <div onClick=${()=>setExpanded(sel?!expanded:false)}
      style=${{height:26,flexShrink:0,display:"flex",alignItems:"center",gap:0,cursor:"pointer",userSelect:"none",
        borderBottom:expanded?`1px solid ${T.border}`:"none",background:T.chrome}}>
      <div style=${{flex:1.4,minWidth:0,padding:"0 16px",display:"flex",gap:7,alignItems:"center",overflow:"hidden"}}>
        <span style=${{fontSize:8.5,fontWeight:700,letterSpacing:1,color:T.blue,flexShrink:0}}>VIEW</span>
        ${VI?html`<span style=${{fontSize:9.5,color:T.mute,whiteSpace:"nowrap",overflow:"hidden",
          textOverflow:"ellipsis",flexShrink:2,minWidth:0}}>${VI.question}</span>`:null}
        <span style=${{fontSize:10.5,color:T.text,fontWeight:600,whiteSpace:"nowrap",overflow:"hidden",
          textOverflow:"ellipsis",flexShrink:1}}>${cap}</span>
      </div>
      <div style=${{flex:1,minWidth:0,padding:"0 16px",display:"flex",gap:7,alignItems:"center",borderLeft:`1px solid ${T.border}`,overflow:"hidden"}}>
        <span style=${{fontSize:8.5,fontWeight:700,letterSpacing:1,color:T.text,flexShrink:0}}>SELECTION</span>
        ${sel
          ?html`<span style=${{fontSize:10.5,color:T.text,whiteSpace:"nowrap",overflow:"hidden",textOverflow:"ellipsis"}}>${sel.label}</span>`
          // CTA as a pressable-looking chip (bordered, calm) — not plain gray hint text (owner flagged twice).
          :html`<span style=${{fontSize:9.5,color:T.dim,fontWeight:600,whiteSpace:"nowrap",overflow:"hidden",
            textOverflow:"ellipsis",border:`1px solid ${T.border}`,borderRadius:5,padding:"1px 8px"}}>${emptySelText(view)}</span>`}
      </div>
      ${showFindings?html`<div style=${{flex:.6,padding:"0 16px",display:"flex",gap:7,alignItems:"center",borderLeft:`1px solid ${T.border}`,
        background:dockCount?"#f8717110":"transparent"}}>
        <span style=${{fontSize:8.5,fontWeight:700,letterSpacing:1,color:dockCount?T.red:T.mute}}>FINDINGS</span>
        <span style=${{fontSize:10.5,color:dockCount?T.red:T.green}}>${dockCount||"✓"}</span>
      </div>`:null}
      <span style=${{padding:"0 12px",color:T.mute,fontSize:11}}>${expanded?"⌄":"⌃"}</span>
    </div>
    <div style=${{display:expanded?"flex":"none",flex:1,minHeight:0,position:"relative"}}>
    <${Seg} title="VIEW" col=${T.blue} flex=${1}>
      <div style=${{fontSize:12,fontWeight:650,color:T.text,lineHeight:1.45}}>${cap}</div>
      ${VI?html`<div style=${{fontSize:10.5,color:T.dim,lineHeight:1.5,marginTop:6}}>
        <span style=${{color:T.mute,fontSize:8.5,fontWeight:700,letterSpacing:.8}}>QUESTION </span>${VI.question}</div>`:null}
      ${VI&&VI.pass?html`<div style=${{fontSize:10.5,color:T.dim,lineHeight:1.5,marginTop:6}}>
        <span style=${{color:T.mute,fontSize:8.5,fontWeight:700,letterSpacing:.8}}>PASS </span>${VI.pass}</div>`:null}
      ${view.prov?html`<div style=${{fontSize:10.5,color:T.dim,lineHeight:1.5,marginTop:6}}>
        <span style=${{color:T.mute,fontSize:8.5,fontWeight:700,letterSpacing:.8}}>SOURCE </span>${view.prov.label}</div>`:null}
    <//>
    <${Seg} title="SELECTION" col=${T.text} flex=${1.6}>
      ${sel?html`<${React.Fragment}>
        ${promptOpen?html`<${PromptComposer} text=${genElementPrompt(estate,scopeLabel,vid,sel)}
          onClose=${()=>setPromptOpen(false)}/>`:null}
        <div style=${{display:"flex",alignItems:"center",gap:8,marginBottom:5}}>
          <span style=${{fontSize:13,fontWeight:700,color:T.text}}>${sel.label}</span>
          ${sel.chip?html`<span style=${{fontSize:8.5,fontWeight:700,letterSpacing:.6,color:T.dim,
            border:`1px solid ${T.border}`,borderRadius:4,padding:"0 5px",textTransform:"uppercase"}}>${sel.chip}</span>`:null}
          <button onClick=${clearSel} title="Clear selection"
            style=${{marginLeft:"auto",background:"transparent",border:"none",color:T.mute,cursor:"pointer",fontSize:13}}>×</button>
        </div>
        <div style=${{display:"flex",alignItems:"center",gap:4,marginBottom:9}}>
          <span onClick=${()=>navigator.clipboard.writeText(selRecordText(sel))} title="Copy a plain-text record of this selection"
            style=${{fontSize:10,color:T.mute,cursor:"pointer",border:`1px solid ${T.border}`,borderRadius:5,
            padding:"2px 7px",display:"flex",alignItems:"center",gap:4}}>⧉ Copy</span>
          <span onClick=${()=>setPromptOpen(true)} title="Compose an editable agent prompt from this selection"
            style=${{fontSize:10,color:T.mute,cursor:"pointer",border:`1px solid ${T.border}`,borderRadius:5,
            padding:"2px 7px",display:"flex",alignItems:"center",gap:4}}>✎ Prompt agent</span>
          ${sel.drillTo?html`<span onClick=${()=>{setLevel(sel.drillTo);setDirOv&&setDirOv(null);}}
            title=${"Open "+sel.drillTo+" view"}
            style=${{fontSize:10,color:T.mute,cursor:"pointer",border:`1px solid ${T.border}`,borderRadius:5,
            padding:"2px 7px",display:"flex",alignItems:"center",gap:4}}>⤤ Deep dive</span>`:null}
        </div>
        <div style=${{display:"flex",gap:24,alignItems:"flex-start"}}>
          <div style=${{flex:1,minWidth:0}}>
            <${DockTable} cols=${["stat","value"]} rows=${sel.rows||[]}/>
          </div>
          ${(sel.relations&&sel.relations.length)?html`<div style=${{flex:1.2,minWidth:0}}>
            <${DockTable} cols=${["dir","peer","flow"]}
              rows=${sel.relations.map(r=>{const v=r.viol&&showFindings; // A3: violation red styling is opt-in
                return Object.assign([r.dir==="out"?"→":"←",r.peer,
                (v?"⚠ ":"")+(r.verb||"")+(r.count?" ×"+r.count:"")],{_viol:v});})}/>
          </div>`:null}
          ${(sel.members&&sel.members.length)?html`<div style=${{flex:1.2,minWidth:0}}>
            <${DockTable} cols=${["member ("+sel.members.length+")","fan-in"]} rightCols=${[false,true]}
              rows=${sel.members.map((m,_i,arr)=>{
                // Disambiguate duplicate labels: qualify with path segment from member ID
                const dups=arr.filter(x=>x.label===m.label).length>1;
                const pathQual=dups?(()=>{const p=(m.sub||m.id||"").replace(/^[gu]_/,"").replace(/_/g,"/");
                  const segs=p.split("/");return segs.length>1?"("+segs.slice(0,-1).pop()+")":"";})():"";
                // A8 FIX 2b: the raw detail text is "in N" (fan-in count, render_component.go)
                // for internal groups — the column header now says fan-in, so show the bare,
                // right-aligned numeral instead of repeating "in" on every row; anything that
                // isn't that exact shape (e.g. "over budget") passes through unchanged.
                const rawDetail=m.stats?Object.values(m.stats)[0]:(m.sub||"");
                const fanIn=typeof rawDetail==="string"&&rawDetail.match(/^in\s+(\d+)$/);
                // A5 (3a): a FINDINGS-row click on a member-grain subject selects the parent
                // group and marks highlightMemberId — that row reads as "this one" here.
                return Object.assign([m.label+(pathQual?" "+pathQual:""),fanIn?fanIn[1]:rawDetail],
                  {_hl:highlightMemberId===m.id});})}/>
          </div>`:null}
        </div>
        ${sel.agent?html`<div style=${{marginTop:10,paddingTop:8,borderTop:`1px solid ${T.border}33`}}>
          <div style=${{fontSize:9,fontWeight:700,letterSpacing:.8,color:T.mute,marginBottom:5}}>AGENT</div>
          <div style=${{display:"flex",alignItems:"center",gap:8,background:T.card,
            borderRadius:6,padding:"5px 10px",border:`1px solid ${T.border}`}}>
            <code style=${{fontSize:10.5,fontFamily:"ui-monospace,monospace",color:T.cyan,flex:1}}>${sel.agent.cmd}</code>
            <span title="Copy command" onClick=${()=>navigator.clipboard.writeText(sel.agent.cmd)}
              style=${{cursor:"pointer",color:T.mute,fontSize:11,flexShrink:0}}>⧉</span>
          </div>
          <div style=${{fontSize:9.5,color:T.mute,marginTop:4}}>⧉ copies a command your agent can run — facts, derive, or read at file:line.</div>
        </div>`:null}
      <//>`:html`<div style=${{color:T.mute,fontSize:11,marginTop:28,textAlign:"center"}}>
        <span style=${{fontSize:11,color:T.dim,fontWeight:600,border:`1px solid ${T.border}`,borderRadius:6,
          padding:"5px 14px",display:"inline-block"}}>${emptySelText(view)}</span>
      </div>`}
    <//>
    ${showFindings?html`<${Seg} title=${"FINDINGS · "+dockCount} col=${dockCount?T.red:T.mute} flex=${1}
      wash=${dockCount?"#f8717108":null}>
      ${sortedFindingRows
        ?(sortedFindingRows.length?sortedFindingRows.map((fr,i)=>{const m=rowSelected(fr.match);
            // Punch (4): selected row gets its own accent — not just a background tint — so it
            // stays distinguishable from every other (already-red-bordered) finding row.
            return html`<div key=${fr.finding.id||i} onClick=${()=>onFindingClick&&onFindingClick(fr.match)}
              style=${{fontSize:10.5,lineHeight:1.5,marginBottom:6,cursor:"pointer",
                color:m?T.text:T.dim,borderLeft:`2px solid ${T.red}`,
                borderRight:m?`3px solid ${T.blue}`:"3px solid transparent",paddingLeft:9,
                background:m?T.cardH:"transparent"}}>
              <b style=${{color:T.red,fontWeight:700}}>${fr.finding.rule}</b> · ${fr.finding.message}
              <span style=${{color:T.mute}}> · ${fr.match.label}</span></div>`;})
          :html`<div style=${{color:T.green,fontSize:11}}>✓ no findings in this view</div>`)
        :(probs.length?sortedProbs.map((p,i)=>html`<div key=${i} style=${{fontSize:10.5,lineHeight:1.5,marginBottom:6,
            color:hl(p)?T.text:T.dim,borderLeft:`2px solid ${T.red}`,
            borderRight:hl(p)?`3px solid ${T.blue}`:"3px solid transparent",paddingLeft:9,
            background:hl(p)?T.cardH:"transparent"}}>${p}</div>`)
          :html`<div style=${{color:T.green,fontSize:11}}>✓ no findings in this view</div>`)}
    <//>`:null}
    </div>
  </div>`;}
const ETYPE_NAME={sys:"system",ext:"external",container:"container",store:"store",proc:"process"};
const ETYPE_COLR={sys:T.blue,ext:T.dim,container:T.arch,store:T.green,proc:T.blue};
// In-context legend: pinned to the canvas corner so color meaning sits where you look.
// Derived from what is actually on screen — same resolution the nodes use, by construction.
// Collapse state persists across views/reloads (module-level cache + localStorage) — a reader
// who dismisses it once shouldn't have it pop back open on the next view switch.
let _legendCollapsed=(()=>{try{return localStorage.getItem("aoa:legendCollapsed")==="1";}catch{return false;}})();
function CanvasLegend({view}){
  const[collapsed,setCollapsed]=useState(_legendCollapsed);
  let items=[];
  if(view.kind==="buckets"){
    const seen=new Set();
    (view.buckets||[]).forEach(b=>{if(!seen.has(b.layer)){seen.add(b.layer);
      items.push({txt:b.layer,c:layerColor(view,b.layer),ico:b.ico});}});}
  else if(view.kind==="simple"){
    const seen=new Set();
    (view.nodes||[]).forEach(n=>{if(!seen.has(n.type)){seen.add(n.type);
      items.push({txt:ETYPE_NAME[n.type]||n.type,c:ETYPE_COLR[n.type]||T.dim});}});}
  if(!items.length)return null;
  // Buckets views deserve their legend even with a single role (R: house rulings); non-bucket
  // (simple) views keep the old "not worth a legend for one color" bar.
  if(view.kind!=="buckets"&&items.length<2)return null;
  const toggle=()=>{const next=!collapsed;setCollapsed(next);_legendCollapsed=next;
    try{localStorage.setItem("aoa:legendCollapsed",next?"1":"0");}catch{}};
  if(collapsed)return html`<div onClick=${toggle} title="show legend" style=${{position:"absolute",top:10,right:14,
    zIndex:6,width:22,height:22,borderRadius:11,background:"#18181bf0",border:`1px solid ${T.borderR}`,
    display:"flex",alignItems:"center",justifyContent:"center",cursor:"pointer",color:T.mute,fontSize:10}}>▸</div>`;
  return html`<div style=${{position:"absolute",top:10,right:14,zIndex:6,background:"#18181bf0",
    border:`1px solid ${T.borderR}`,borderRadius:8,padding:"8px 22px 8px 13px",
    display:"flex",flexDirection:"column",gap:5}}>
    <span onClick=${toggle} title="hide legend" style=${{position:"absolute",top:6,right:6,fontSize:10,
      color:T.mute,cursor:"pointer",lineHeight:1}}>▾</span>
    ${items.map((it,i)=>html`<div key=${i} style=${{display:"flex",alignItems:"center",gap:7,fontSize:12,color:T.dim}}>
      <span style=${{width:9,height:9,borderRadius:2,background:it.c+"30",border:`1.5px solid ${it.c}`,flexShrink:0}}></span>
      ${it.ico?html`<${Ico} k=${it.ico} c=${it.c} s=${12}/>`:null}
      ${it.txt}</div>`)}
  </div>`;}
function Footer({view,ov}){
  const groups=[];
  if(view.kind==="buckets"){
    // layer colors live in the canvas legend (where the eye is) — footer owns edge grammar only
    const ed=[{txt:"dependency · color = source layer",glyph:"━"},{txt:"bundled count",glyph:"×N"}];
    if((view.buckets||[]).some(b=>b.layer==="supporting"))ed.push({txt:"supporting / inferred",glyph:"┄"});
    groups.push({label:"EDGES",items:ed});
  } else if(view.kind==="matrix"){
    const nitems=(view.items||[]).length;
    groups.push({label:"MATRIX",items:[
      {txt:nitems+"×"+nitems+" DSM",glyph:"⊞"},
      {txt:"dependency row → col",glyph:"→"},
      {txt:"mutual pair · cycle",glyph:"↔",c:T.red}]});
  } else if(view.kind==="table"){
    groups.push({label:"DOCUMENT",items:[{txt:view.rows?view.rows.length+" rows":"table",glyph:"≣"}]});
  } else if(view.kind==="entity"){
    groups.push({label:"ELEMENTS",items:[{txt:"bucket table",c:T.green,ico:"store"}]});
    groups.push({label:"EDGES",items:[{txt:"contains",glyph:"━"}]});
  } else {
    const seen={};(view.nodes||[]).forEach(n=>{if(n.type&&!seen[n.type])seen[n.type]=n.icon;});
    const elemItems=Object.keys(seen).map(t=>({txt:ETYPE_NAME[t]||t,c:ETYPE_COLR[t]||T.dim,ico:seen[t]}));
    if(elemItems.length)groups.push({label:"ELEMENTS",items:elemItems});
    const ed=[{txt:"labeled flow",glyph:"━"}];
    if((view.nodes||[]).some(n=>n.real===false))ed.push({txt:"inferred · sourceable",glyph:"┄"});
    groups.push({label:"EDGES",items:ed});
  }
  if(ov&&(ov.concerns||ov.changed)&&view.kind==="buckets"&&(view.palette||"aoa")==="aoa"){
    const o=[];
    if(ov.concerns)o.push({txt:"recon findings",glyph:"⚠",c:T.red});
    if(ov.changed)o.push({txt:"touched · last 15 commits",glyph:"Δ",c:T.yellow});
    groups.push({label:"OVERLAYS",items:o});}
  return html`<div style=${{borderTop:`1px solid ${T.border}`,display:"flex",alignItems:"center",
    gap:14,padding:"7px 16px",fontSize:10.5,color:T.dim,flexWrap:"wrap",background:T.chrome}}>
    ${groups.map((g,gi)=>html`<${React.Fragment} key=${g.label}>
      ${gi>0?html`<span style=${{color:T.border}}>│</span>`:null}
      <span style=${{fontSize:9,fontWeight:700,letterSpacing:1,color:T.mute}}>${g.label}</span>
      ${g.items.map((it,ii)=>html`<span key=${ii} style=${{display:"flex",alignItems:"center",gap:4}}>
        ${it.ico?html`<${Ico} k=${it.ico} c=${it.c} s=${11}/>`:html`<b style=${{color:it.c||T.text,fontWeight:700}}>${it.glyph}</b>`}
        ${it.txt}</span>`)}
    <//>`)}
    <span style=${{marginLeft:"auto",color:T.mute,flexShrink:0}}>generated ${MODEL.generated.timestamp}</span>
  </div>`;}

// SUR-1: CATALOGS/FIRST (per-system hardcoded catalogs) were dead code — never read by any
// render path — and had drifted from reality (e.g. techportfolio/sbom marked "planned" after
// they went live). STD_CATALOG + dynamicCatalog() above is the one real source of truth; every
// system's catalog is derived from its manifest, never hand-maintained per estate.
const STATUS={live:{dot:"●",col:T.green,lbl:"derived live"},
              sim:{dot:"◌",col:T.yellow,lbl:"simulated · sourceable"},
              planned:{dot:"○",col:T.mute,lbl:"planned · extractor gated"}};

const ago=t=>{const s=(Date.now()-t)/1000;return s<5?"now":s<60?Math.floor(s)+"s ago":s<3600?Math.floor(s/60)+"m ago":Math.floor(s/3600)+"h ago";};
// narrateOne: converts a Finding to a plain-language headline per §4.2 patterns.
// rank=0 (top finding) gets full sentence; higher ranks get shorter diagnostics.
function narrateOne(f,rank){
  if(!f)return"";
  rank=rank||0;
  const msg=f.message||"";
  if(f.rule==="god"){
    const m=msg.match(/in (\d+).*out (\d+)/);
    const inN=m?m[1]:"?",outN=m?m[2]:"?";
    const subj=f.subjects&&f.subjects[0]?f.subjects[0].replace(/^[gu]_/,""):msg.split(":")[1]||"this package";
    if(rank===0)return `${subj} is load-bearing — ${inN} packages lean on it and it reaches into ${outN}. Changes here ripple widest.`;
    return `also load-bearing — ${inN} packages lean on ${subj}, reaches ${outN}`;}
  if(f.rule==="cycle"){
    const parts=msg.replace("dependency cycle: ","").split(" → ");
    if(parts.length>=2){const a=parts[0].replace(/^[gu]_/,""),b=parts[1].replace(/^[gu]_/,"");
      if(rank===0)return `${a} and ${b} depend on each other — a cycle. Cheapest cut: the ${a} → ${b} edge (${f.cheapestCut||"unknown"}).`;
      return `cycle: ${a} ↔ ${b}${f.cheapestCut?" · cut: "+f.cheapestCut:""}`;}
    return msg;}
  if(f.rule==="dead-candidate"||f.rule==="dead"||f.rule==="orphan"){
    const subj=f.subjects&&f.subjects[0]?f.subjects[0].replace(/^[gu]_/,""):msg.split(":")[1]||"this package";
    if(rank===0)return `${subj} looks dead — nothing imports it and search has never touched it. Removal candidate.`;
    return `${subj} — no importers, no search hits`;}
  if(f.rule==="budget"){
    const subj=f.subjects&&f.subjects[0]?f.subjects[0].replace(/^[gu]_/,""):msg;
    if(rank===0)return `${subj} exceeds the group size budget. Consider splitting it.`;
    return `${subj} over size budget`;}
  return msg;}

// Spell out finding rule codes for human-readable header
const RULE_NAMES={god:"load-bearing",cycle:"cycle",dead:"unreachable","dead-candidate":"unreachable",
  orphan:"orphan",budget:"over budget"};
function spellRuleCount(rule,n){
  const name=RULE_NAMES[rule]||rule;
  return `${n} ${name}${n>1?"s":""}`;}

// FindingsDrawer: slide-over panel showing all daemon findings with narration + sources
function FindingsDrawer({findings,open,setOpen,expandedId,setExpandedId}){
  if(!open)return null;
  const bySev={error:[],warn:[],info:[]};
  findings.forEach(f=>{(bySev[f.severity]||bySev.warn).push(f);});
  const ruleCounts={};
  findings.forEach(f=>{ruleCounts[f.rule]=(ruleCounts[f.rule]||0)+1;});
  const ruleHeader=Object.entries(ruleCounts).sort((a,b)=>b[1]-a[1]).map(([r,n])=>spellRuleCount(r,n)).join(" · ");
  // Budget header: "N groups over size budget" instead of "budget ×N"
  const budgetCount=ruleCounts["budget"]||0;
  const ruleHeaderDisplay=budgetCount>0?ruleHeader.replace(spellRuleCount("budget",budgetCount),budgetCount+" group"+(budgetCount>1?"s":"")+" over size budget"):ruleHeader;
  return html`<div style=${{position:"absolute",top:0,right:0,width:460,height:"100%",
    background:T.chrome,borderLeft:`1px solid ${T.border}`,zIndex:20,display:"flex",
    flexDirection:"column",boxShadow:"-8px 0 20px #0008"}}>
    <div style=${{padding:"12px 16px",borderBottom:`1px solid ${T.border}`,display:"flex",alignItems:"center",gap:8}}>
      <span style=${{fontSize:12,fontWeight:700,color:T.text}}>FINDINGS</span>
      <span style=${{fontSize:10,color:T.dim,flex:1}}>${ruleHeaderDisplay}</span>
      <button onClick=${()=>setOpen(false)} style=${{background:"transparent",border:"none",
        color:T.mute,cursor:"pointer",fontSize:15}}>✕</button>
    </div>
    <div style=${{flex:1,overflowY:"auto",padding:"8px 0"}}>
      ${["error","warn","info"].flatMap(sev=>bySev[sev]).map((f,fi)=>{
        const expanded=expandedId===f.id;
        const headline=narrateOne(f,fi);
        return html`<div key=${fi} style=${{borderBottom:`1px solid ${T.border}33`,padding:"10px 16px",
          background:f.new?"#fbbf2406":"transparent",
          borderLeft:f.severity==="error"?`3px solid ${T.red}`:f.new?`3px solid ${T.yellow}`:"3px solid transparent"}}>
          <div style=${{display:"flex",alignItems:"flex-start",gap:6,cursor:"pointer"}}
            onClick=${()=>setExpandedId(expanded?null:f.id)}>
            <span style=${{fontSize:10.5,color:T.text,flex:1,lineHeight:1.45}}>${headline}</span>
            <div style=${{display:"flex",gap:4,flexShrink:0,alignItems:"center"}}>
              <span style=${{fontSize:8,fontWeight:700,color:T.mute,border:`1px solid ${T.border}`,
                borderRadius:4,padding:"0 4px"}}>${f.rule}</span>
              ${f.new?html`<span style=${{fontSize:8,fontWeight:700,color:T.yellow,border:`1px solid ${T.yellow}`,
                borderRadius:4,padding:"0 4px"}}>NEW</span>`:null}
              ${f.sources&&f.sources.length?html`<span style=${{fontSize:9,color:T.mute}}>${f.sources.length} src</span>`:null}
            </div>
          </div>
          ${expanded?html`<div style=${{marginTop:8,paddingTop:8,borderTop:`1px solid ${T.border}33`}}>
            ${(f.sources||[]).slice(0,10).map((s,si)=>html`<div key=${si}
              style=${{fontSize:9.5,fontFamily:"ui-monospace,monospace",color:T.dim,padding:"2px 0",
                display:"flex",alignItems:"center",gap:8}}>
              <span style=${{color:T.text}}>${s.file}:${s.line}</span>
              <span title="Copy read command" onClick=${ev=>{ev.stopPropagation();
                navigator.clipboard.writeText("Read "+s.file+" "+s.line);}}
                style=${{cursor:"pointer",color:T.mute,fontSize:9}}>⧉</span>
            </div>`)}
            ${(f.sources||[]).length>10?html`<div style=${{fontSize:9.5,color:T.mute,marginTop:4}}>+${f.sources.length-10} more</div>`:null}
          </div>`:null}
        </div>`;
      })}
      ${!findings.length?html`<div style=${{padding:"24px",textAlign:"center",color:T.mute,fontSize:11}}>no daemon findings yet</div>`:null}
    </div>
  </div>`;}
function Sidebar({estate,scopes,simEstate,scope,goScope,level,go,open,setOpen,collapsed,setCollapsed,last,journeys,startJourney}){
  const[copied,setCopied]=useState(null);
  const[stdExpanded,setStdExpanded]=useState(false);
  const CATALOG=dynamicCatalog(scopes[scope]||{views:{}});
  if(collapsed) return html`<div style=${{width:44,borderRight:`1px solid ${T.border}`,background:T.chrome,
    display:"flex",flexDirection:"column",alignItems:"center",paddingTop:10,flexShrink:0}}>
    <button onClick=${()=>setCollapsed(false)} title="Show views"
      style=${{background:"transparent",border:`1px solid ${T.border}`,color:T.dim,
      borderRadius:6,padding:"4px 8px",cursor:"pointer",fontSize:13}}>›</button>
  </div>`;
  return html`<div style=${{width:300,borderRight:`1px solid ${T.border}`,overflowY:"auto",background:T.chrome,
    flexShrink:0,display:"flex",flexDirection:"column"}}>
    <div style=${{padding:"11px 14px",display:"flex",alignItems:"center",borderBottom:`1px solid ${T.border}`}}>
      <span style=${{fontSize:11,fontWeight:700,letterSpacing:1.2,color:T.dim}}>CAPABILITIES</span>
      <span style=${{fontSize:8.5,fontWeight:700,marginLeft:7,letterSpacing:.6,
        color:simEstate?T.yellow:T.green,border:`1px solid ${simEstate?T.yellow:T.green}`,
        borderRadius:4,padding:"0 5px"}}>${simEstate?"SIMULATED":"LOCAL"}</span>
      <button onClick=${()=>setCollapsed(true)} title="Hide"
        style=${{marginLeft:"auto",background:"transparent",border:"none",color:T.mute,cursor:"pointer",fontSize:14}}>‹</button>
    </div>
    ${Object.entries(scopes).map(([sid,sv])=>html`<div key=${sid} onClick=${()=>goScope(sid)}
      style=${{display:"flex",alignItems:"flex-start",gap:8,padding:"7px 14px",cursor:"pointer",
        background:sid===scope?"#60a5fa14":"transparent",
        borderLeft:sid===scope?`2px solid ${T.blue}`:"2px solid transparent"}}>
      <${Ico} k="sys" c=${sid===scope?T.blue:T.dim} s=${13}/>
      <div style=${{minWidth:0}}>
        <div style=${{fontSize:12.5,fontWeight:sid===scope?700:500,color:sid===scope?T.text:T.dim}}>${sv.label}</div>
        <div style=${{fontSize:9.5,color:T.mute,marginTop:1}}>${sv.tech}</div>
      </div>
    </div>`)}
    ${journeys&&journeys.length?html`<${React.Fragment}>
      <div style=${{padding:"10px 14px 4px",borderTop:`1px solid ${T.border}`,marginTop:4}}>
        <span style=${{fontSize:11,fontWeight:700,letterSpacing:1.2,color:T.dim}}>JOURNEYS</span>
        <span style=${{fontSize:9,color:T.mute,marginLeft:6}}>· flows across the estate</span>
      </div>
      ${journeys.map(j=>html`<div key=${j.id} onClick=${()=>startJourney(j,0)}
        style=${{display:"flex",alignItems:"flex-start",gap:8,padding:"6px 14px",cursor:"pointer",
          borderLeft:"2px solid transparent"}}>
        <span style=${{color:T.purple,fontSize:10,lineHeight:"16px"}}>▶</span>
        <div style=${{minWidth:0}}>
          <div style=${{fontSize:12,fontWeight:500,color:T.text,whiteSpace:"nowrap",overflow:"hidden",textOverflow:"ellipsis"}}>${j.label}</div>
          <div style=${{fontSize:9.5,color:T.mute,marginTop:1}}>${j.kind} · ${j.steps} steps${j.prov&&j.prov.kind==="simulated"?" · ◌ simulated":""}</div>
        </div>
      </div>`)}
    <//>`:null}
    <div style=${{padding:"10px 14px 4px",borderTop:`1px solid ${T.border}`,marginTop:4}}>
      <span style=${{fontSize:11,fontWeight:700,letterSpacing:1.2,color:T.dim}}>VIEWS</span>
      <span style=${{fontSize:9,color:T.mute,marginLeft:6}}>· ${(scopes[scope]||{}).label} only</span>
    </div>
    <div style=${{flex:1,padding:"6px 0"}}>
    ${(()=>{
      // Two-tier sidebar: Tier 1 = DERIVED (live/sim views from manifest); Tier 2 = collapsed "N more"
      const allItems=CATALOG.flatMap(g=>g.items.map(it=>({...it,grp:g.grp,tag:g.tag})));
      const derivedItems=allItems.filter(it=>it.status==="live"||it.status==="sim");
      const plannedItems=allItems.filter(it=>it.status==="planned");
      // Group derived items by grp for display
      const derivedGroups=[...new Map(derivedItems.map(it=>[it.grp,{grp:it.grp,tag:it.tag,items:[]}])).values()];
      derivedItems.forEach(it=>{const g=derivedGroups.find(x=>x.grp===it.grp);if(g)g.items.push(it);});
      const renderItem=(it,ix)=>{const st=STATUS[it.status];const active=it.id&&it.id===level&&!it.alias;
        const aliasActive=it.id&&it.id===level&&it.alias;
        const clickable=!!it.id;
        return html`<div key=${ix} class=${"vrow"+(it.aka?" hv":"")} onClick=${clickable?()=>go(it.id):null}
          style=${{display:"flex",alignItems:"flex-start",gap:8,padding:"5px 14px 5px 29px",
            cursor:clickable?"pointer":"default",
            background:(active||aliasActive)?"#60a5fa14":"transparent",
            borderLeft:(active||aliasActive)?`2px solid ${T.blue}`:"2px solid transparent"}}>
          <span style=${{color:st.col,fontSize:10,lineHeight:"17px"}}>${st.dot}</span>
          <div style=${{minWidth:0,flex:1}}>
            <div style=${{fontSize:12,fontWeight:active?650:500,
              color:clickable?T.text:T.dim,whiteSpace:"nowrap",overflow:"hidden",textOverflow:"ellipsis"}}>${it.label}</div>
            ${it.note?html`<div style=${{fontSize:9.5,color:T.mute}}>${it.note}</div>`:null}
          </div>
          ${it.aka?html`<${HoverCard} title=${it.label} rows=${[["aka",it.aka]]}/>`:null}
          ${it.id&&last&&last[estate+":"+scope+":"+it.id]?html`<span class="ago" style=${{fontSize:8.5,color:T.mute,lineHeight:"17px",flexShrink:0}}>${ago(last[estate+":"+scope+":"+it.id])}</span>`:null}
          ${it.status==="planned"&&estate!=="local"&&it.vid0?html`<span title="Copy AI-generation prompt for this view"
            onClick=${ev=>{ev.stopPropagation();
              navigator.clipboard.writeText(genPrompt(estate,(scopes[scope]||{}).label||scope,it.vid0,it.label));
              setCopied(it.label);setTimeout(()=>setCopied(null),1400);}}
            style=${{fontSize:10,color:copied===it.label?T.green:T.mute,cursor:"pointer",flexShrink:0,lineHeight:"17px"}}>${copied===it.label?"✓":"⧉"}</span>`:null}
        </div>`;};
      return html`<${React.Fragment}>
        ${derivedGroups.map(g=>html`<div key=${g.grp}>
          <div style=${{display:"flex",alignItems:"center",gap:7,padding:"8px 14px"}}>
            <span style=${{fontSize:11.5,fontWeight:700,letterSpacing:.6,color:T.text}}>${g.grp}</span>
            ${g.tag?html`<span style=${{fontSize:8.5,color:T.blue,border:`1px solid ${T.blue}`,borderRadius:4,
              padding:"0 4px",letterSpacing:.5,textTransform:"uppercase"}}>${g.tag}</span>`:null}
          </div>
          ${g.items.map(renderItem)}
        </div>`)}
        ${plannedItems.length?html`<div style=${{marginTop:8,borderTop:`1px solid ${T.border}33`}}>
          <div onClick=${()=>setStdExpanded(!stdExpanded)} style=${{display:"flex",alignItems:"center",
            gap:8,padding:"7px 14px",cursor:"pointer",userSelect:"none"}}>
            <span style=${{color:T.mute,fontSize:9,transform:stdExpanded?"rotate(90deg)":"none",transition:"transform .15s",width:8}}>▶</span>
            <span style=${{color:T.mute,fontSize:11}}><span style=${{color:T.mute}}>○</span> ${plannedItems.length} more standard views available</span>
          </div>
          ${stdExpanded?plannedItems.map(renderItem):null}
        </div>`:null}
      <//>`;
    })()}
    </div>
    <div style=${{borderTop:`1px solid ${T.border}`,padding:"9px 14px",fontSize:9.5,color:T.dim,
      display:"flex",flexDirection:"column",gap:3}}>
      ${Object.values(STATUS).map((s,i)=>html`<span key=${i}><span style=${{color:s.col}}>${s.dot}</span> ${s.lbl}</span>`)}
    </div>
  </div>`;}

function Flow(){
  const q=new URLSearchParams(location.search);
  const[estate,setEstate]=useState(q.get("estate")||(ESTATES.local?"local":Object.keys(ESTATES)[0]));
  const e0=q.get("estate")||(ESTATES.local?"local":Object.keys(ESTATES)[0]);
  const[scope,setScope]=useState(q.get("scope")||firstScope(e0));
  const[level,setLevel]=useState(()=>{const sc=q.get("scope")||firstScope(e0);
    // ?expandedGroups= only applies to buckets views — route there when no explicit level given
    // Default landing prefers a buckets view (component) — the code view is
    // listed-not-drawn per ruling R1 and must never be the silent default.
    return q.get("level")||firstBucketsView(e0,sc);});
  const[dirOv,setDirOv]=useState(q.get("dir")||null);
  const[den,setDen]=useState(q.get("density")||"compact");
  const[els,setEls]=useState(null);
  // A6: freshness chip reads MODEL.generated live (module-scope MODEL is reassigned + re-rendered
  // by the manifest poll below), so no threading is needed — this tick JUST forces a re-render
  // between polls so the displayed age advances (12s ago → 1m ago → 1h ago) instead of freezing.
  const[,setGenTick]=useState(0);
  useEffect(()=>{const iv=setInterval(()=>setGenTick(t=>t+1),30000);return()=>clearInterval(iv);},[]);
  const SC=ESTATES[estate].scopes;
  const sys=SC[scope]||SC[firstScope(estate)];
  const view=sys.views[level]||sys.views[Object.keys(sys.views)[0]]||null;
  // No views derived yet — render a placeholder rather than crashing on view.count etc.
  if(!view)return html`<div style=${{height:"100vh",display:"flex",flexDirection:"column",background:T.bg,font:"13px -apple-system,Segoe UI,Inter,Roboto,sans-serif",color:T.text}}>
    ${!EMBED&&html`<div style=${{padding:"9px 18px",borderBottom:`1px solid ${T.border}`,display:"flex",alignItems:"center",gap:10,background:T.chrome}}>
      <div style=${{fontWeight:750,fontSize:15}}>aOa <span style=${{color:T.dim,fontWeight:400}}>· architecture</span></div>
    </div>`}
    <div style=${{flex:1,display:"flex",alignItems:"center",justifyContent:"center",color:T.dim,fontSize:13}}>
      <div style=${{textAlign:"center",lineHeight:1.8}}>
        <div style=${{fontSize:28,marginBottom:12,color:T.arch}}>⬡</div>
        <div style=${{fontWeight:600,fontSize:14,color:T.text,marginBottom:6}}>No architecture views yet</div>
        <div>Run <code style=${{background:T.card,border:`1px solid ${T.border}`,padding:"2px 8px",borderRadius:4}}>aoa</code> to derive views from your imports.</div>
      </div>
    </div>
  </div>`;
  const[ov,setOv]=useState(()=>{const o=(q.get("ov")||"").split(",");
    return {concerns:o.includes("concerns"),changed:o.includes("changed")};});
  // showFindings: THE calm-default master gate (A3). Default OFF — a stranger opens a view and
  // sees a role-colored map, not a wall of alarms. Single source of truth for every alarm surface:
  // canvas red/dashed edge paint, bucket ribbons/badges, header ⚠ pill, banner, dock concern-row,
  // caption findings-clause. Detection stays computed underneath either way — this only gates paint.
  // Persisted like the legend's localStorage idiom; ?findings=1 bookmarks it on (open question 5).
  const[showFindings,setShowFindings]=useState(()=>{
    if(q.get("findings")==="1")return true;
    try{return localStorage.getItem("aoa:showFindings")==="1";}catch{return false;}});
  const toggleShowFindings=useCallback(()=>{
    setShowFindings(v=>{const next=!v;try{localStorage.setItem("aoa:showFindings",next?"1":"0");}catch{}return next;});},[]);
  // Capsule expand state — persisted in localStorage (R5: stable base map)
  // test hook: ?expandedGroups=g_domain,g_app pre-expands groups for screenshot verification
  // Accepts IDs with or without "g_" prefix; also matches on label (case-insensitive).
  const[expandedGroups,setExpandedGroups]=useState(()=>{
    const qGroups=q.get("expandedGroups");
    if(qGroups){
      const raw=new Set(qGroups.split(",").filter(Boolean));
      // Also add "g_"-prefixed variants so "adapters" matches "g_adapters"
      const expanded=new Set(raw);
      raw.forEach(id=>{if(!id.startsWith("g_"))expanded.add("g_"+id);});
      return expanded;}
    try{const s=localStorage.getItem("bp:expanded:"+estate+":"+scope);
      return new Set(s?JSON.parse(s):[]);}catch{return new Set();}});
  const isCapsuleView=!!(view&&view.kind==="buckets");
  const toggleCapsule=useCallback((bid)=>{
    setExpandedGroups(prev=>{
      const next=new Set(prev);
      if(next.has(bid))next.delete(bid);else next.add(bid);
      try{localStorage.setItem("bp:expanded:"+estate+":"+scope,JSON.stringify([...next]));}catch{}
      return next;});},[estate,scope]);
  const[last,setLast]=useState({});
  const[autoDir,setAutoDir]=useState(null);
  const dir=dirOv||autoDir||"DOWN";
  // A capsule click both selects the group AND expands it (toggleCapsule below). Expanding
  // flips expandedGroups, a dep of this effect, which unconditionally wipes sel/selId on ANY
  // dep change (view change clears selection) — including this one. Stash the record here so
  // it can be re-applied once the relayout below finishes and the group re-renders expanded.
  const pendingCapsuleSel=React.useRef(null);
  useEffect(()=>{let on=true;const d=SP[den];
    // Decision 2 (A7 consensus, supersedes the old A5 note here): view change clears the
    // selection AND collapses the dock — an expanded dock with no selection is exactly the
    // "empty expanded panel" the house ruling forbids, so expansion cannot outlive its selection.
    setEls(null);setSelRaw(null);setSelId(null);setHighlightMemberId(null);setExpanded(false);setExpandedFold(null);   // remount canvas with the new layout
    const run=dd=>view.kind==="buckets"?layoutBuckets(view,dd,d,ov,{capsuleMode:true,expandedGroups,showFindings}):layoutSimple(view,dd,d,showFindings);
    // Auto direction: lay out BOTH ways, keep whichever fits the viewport at the larger scale
    const fitScale=r=>{const xs=r.nodes.map(n=>n.position.x),ys=r.nodes.map(n=>n.position.y),
      xe=r.nodes.map(n=>n.position.x+(n.width||208)),ye=r.nodes.map(n=>n.position.y+(n.height||64));
      const w=Math.max(...xe)-Math.min(...xs),h=Math.max(...ye)-Math.min(...ys);
      const aw=Math.max(window.innerWidth-300,400),ah=Math.max(window.innerHeight-130,300);
      return Math.min(aw/Math.max(w,1),ah/Math.max(h,1));};
    (async()=>{try{
      if(view.shard&&!view._loaded){               // lazy shard fetch — manifest stays tiny
        const r=await fetch(BASE+view.shard.path+"?v="+view.shard.hash);
        if(!r.ok)throw new Error("HTTP "+r.status+" loading shard "+view.shard.path);
        Object.assign(view,await r.json());view._loaded=true;
        validateView(estate+"/"+scope+"/"+level,view);}
      if(shardIsEmpty(view)){
        // M6-P1: honest empty shard (Go omitempty dropped the buckets/nodes array entirely) —
        // never hand this to layoutBuckets/layoutSimple, render the calm empty-state card instead.
        if(on){setEls({htmlView:true,isEmpty:true,problems:[]});setAutoDir(null);
          setLast(l=>({...l,[estate+":"+scope+":"+level]:Date.now()}));}
        return;}
      if(view.kind==="table"||view.kind==="matrix"){
        // tables/matrices surface their own concerns: ⚠ rows and mutual (cycle) pairs
        const probs=[];
        if(view.kind==="table")(view.rows||[]).forEach(r=>{
          const w=r.find(c=>String(c).trim().startsWith("⚠"));
          if(w)probs.push("⚠ "+r[0]+": "+String(w).replace(/^\s*⚠\s*/,""));});
        else{const it=view.items||[],M=view.matrix||[];
          for(let i=0;i<it.length;i++)for(let j=i+1;j<it.length;j++)
            if((M[i]||[])[j]&&(M[j]||[])[i])
              probs.push("mutual dependency (cycle): "+it[i]+" ↔ "+it[j]+" ("+M[i][j]+" / "+M[j][i]+")");}
        if(on){setEls({htmlView:true,problems:probs});setAutoDir(null);
          setLast(l=>({...l,[estate+":"+scope+":"+level]:Date.now()}));}
        return;}
      let e,dd;
      if(dirOv){e=await run(dirOv);dd=dirOv;}
      else{const a=await run("DOWN"),b=await run("RIGHT");
        if(fitScale(a)>=fitScale(b)){e=a;dd="DOWN";}else{e=b;dd="RIGHT";}}
      if(on){setEls(e);setAutoDir(dd);setLast(l=>({...l,[estate+":"+scope+":"+level]:Date.now()}));
        const pr=pendingCapsuleSel.current;
        if(pr){pendingCapsuleSel.current=null;
          if(e.nodes&&e.nodes.some(nd=>nd.id===pr.id))select(pr.rec,pr.id);}}
    }catch(err){showFatal("VIEW LOAD FAILED · "+(err&&err.message||err));}})();
    return()=>{on=false;};},[estate,scope,level,dirOv,den,ov,expandedGroups,showFindings]);
  // test hook: ?auto=<view>:<ms> simulates a user click after ms (verifies the CLICK path, not URL load)
  useEffect(()=>{const auto=q.get("auto");
    if(auto){const[lv,ms]=auto.split(":");const t=setTimeout(()=>setLevel(lv),parseInt(ms||"800",10));
      return()=>clearTimeout(t);}},[]);
  // The grammar: hover peeks, click SELECTS (ring + dock), the open ▸ chip drills, Esc unwinds.
  const[sel,setSelRaw]=useState(null);
  const[selId,setSelId]=useState(null);
  const[expanded,setExpanded]=useState(false);
  // A8 FIX 2a: which bucket's "+N more" fold is showing its in-node scrollable overlay of the
  // remaining externals (id of the owning group, or null). Pure UI state — never touches ELK.
  const[expandedFold,setExpandedFold]=useState(null);
  // A5: when a FINDINGS row for a member-grain subject is clicked, the parent GROUP is
  // selected (selId becomes the bucket id) but the specific member row inside the dock's
  // member table still needs to read as "this one" — tracked separately from selId because
  // a direct canvas click on a member node selects the member itself (selId===member id),
  // while a finding-row click on a member subject selects the owning group instead (spec 3a).
  const[highlightMemberId,setHighlightMemberId]=useState(null);
  // Selecting a node/group highlights every edge touching it, both directions (owner-observed
  // delight, gin's BINDING) — an edge is "connected" if it IS the clicked id, or its source/target
  // is; clearing (id=null) never matches a real source/target so all edges de-highlight.
  const mark=id=>setEls(e=>{if(!e||!e.nodes)return e;let ch=false;
    // Decision 1 (A7 consensus): glow the clicked node AND its first-degree connections —
    // both the touching edges (existing) and the NODES at their other end (new — a click used
    // to only bold the edges, leaving the neighbor node visually unselected).
    const neighbors=new Set();
    if(id)(e.edges||[]).forEach(ed=>{if(ed.source===id)neighbors.add(ed.target);
      if(ed.target===id)neighbors.add(ed.source);});
    // Symmetric focus (A8 ruling, owner verbatim): everything NOT in the neighborhood dims
    // ~20% while the neighborhood itself brightens — universal across every color, hue never
    // changes. dim is simply "id is set AND this element isn't wanted" — falls back to false
    // for everyone when id is null (deselect), restoring uniform weight for free.
    const nodes=e.nodes.map(n=>{const want=n.id===id||neighbors.has(n.id);const dim=!!id&&!want;
      if(!!n.data._sel===want&&!!n.data._dim===dim)return n;ch=true;return {...n,data:{...n.data,_sel:want,_dim:dim}};});
    const edges=(e.edges||[]).map(ed=>{const want=ed.id===id||ed.source===id||ed.target===id;const dim=!!id&&!want;
      if(!!(ed.data&&ed.data._sel)===want&&!!(ed.data&&ed.data._dim)===dim)return ed;ch=true;return {...ed,data:{...ed.data,_sel:want,_dim:dim}};});
    return ch?{...e,nodes,edges}:e;});
  // Decision 2 (A7 consensus): selecting always fills the dock, deselecting always settles it
  // back to the 26px bar — symmetric, so an "empty expanded panel" can never rest on screen.
  const select=useCallback((rec,id)=>{setSelRaw(rec);setSelId(rec?id:null);
    setExpanded(!!rec);mark(rec?id:null);},[]);
  // A8 punch (correctness lens): clearing selection must also drop the DOM's native focus —
  // React Flow's node wrapper is a focusable (tabIndex=0) div, and without an explicit blur()
  // the browser's own focus ring lingers on the just-deselected node even after _sel/_dim
  // reset to uniform weight, contradicting "Esc/deselect restores everything." The _sel
  // box-shadow glow (see BoxNode/EntityNode/etc above) is the only focus affordance we want.
  const clearSel=useCallback(()=>{
    if(document.activeElement&&document.activeElement!==document.body&&document.activeElement.blur)
      document.activeElement.blur();
    select(null,null);setHighlightMemberId(null);setExpandedFold(null);},[select]);
  // pending selection: applied once the target view's elements are on screen
  // (seeded by ?sel= for screenshots; driven by journey steps at runtime)
  const[pendingSel,setPendingSel]=useState(q.get("sel"));
  const nameOf=useCallback(id2=>{if(view.kind==="buckets"){const b=(view.buckets||[]).find(x=>x.id===id2);if(b)return b.label;}
    const n2=(view.nodes||[]).find(x=>x.id===id2);return n2?n2.label:id2;},[view]);
  const relationsFor=useCallback(id2=>((view.edges||[]).filter(e=>e.source===id2||e.target===id2)
    .map(e=>({dir:e.source===id2?"out":"in",peer:nameOf(e.source===id2?e.target:e.source),
      verb:e.label||"",count:e.count,viol:!!(e.tag||e._viol)}))
    .sort((a,b)=>(b.viol?1:0)-(a.viol?1:0))),[view,nameOf]);
  const onNodeClick=useCallback((ev,n)=>{if(!n.data)return;
    if(n.type==="spacer")return;
    // Decision 3 (A7 consensus) + A8 FIX 2a: the "+N more" fold row is a placeholder, not a
    // real member — clicking it (a) toggles an in-node scrollable overlay of the remaining
    // members open right on the canvas, so the owner can reach all 45+ externals without
    // leaving the map, and (b) still selects the OWNING group so the dock's full ranked list
    // stays in sync. Both are pure state flips — zero canvas relayout, no expandedGroups/
    // toggleCapsule/ELK invocation involved.
    if(n.data._foldMore){
      setExpandedFold(f=>f===n.data._parentId?null:n.data._parentId);
      const bn=els&&els.nodes&&els.nodes.find(x=>x.id===n.data._parentId);
      if(bn)onNodeClick(ev,bn);
      return;}
    const d=n.data,m=(n.type==="solo"?d.member:d)||{};
    const rows=(m.stats?Object.entries(m.stats):[]).concat(m.sub?[["detail",m.sub]]:[]);
    if(d.layer||d.lay)rows.unshift(["layer",d.layer||d.lay]);
    if(d.tech)rows.push(["tech",d.tech]);
    if(m.concerns)rows.push(["findings",m.concerns+" recon findings"]);
    if(m.changed)rows.push(["recent","touched in last 15 commits"]);
    const isGroup=n.type==="bucket"||n.type==="capsule";
    const chip=isGroup?"group":n.type==="entity"?"entity":n.type==="member"?"member":(ETYPE_NAME[d.type]||"element");
    // AGENT row: B1 — paths not unit IDs (Facts() substring-matches paths, not u_... IDs)
    let agent=null;
    if(isGroup||n.type==="solo"){
      const path=(d.path||(d.id||"").replace(/^g_/,"").replace(/_/g,"/"));
      agent={cmd:"aoa arch facts "+path,path};}
    else if(n.type==="member"&&m.id){
      const path=(m.path||m.id.replace(/^[gu]_/,"").replace(/_/g,"/"));
      agent={cmd:"aoa tree "+path+" -d 2",path};}
    // d._allMembers (Decision 3): the group's FULL ranked member list, preserved when the
    // canvas render was budgeted — the dock shows more than the canvas budget from this.
    const rec={label:m.label||d.label,chip,rows,relations:relationsFor(n.id),
      members:isGroup?(d._allMembers||d.members||[]):null,agent,drillTo:d.drillTo||null};
    // Decision 1 (A7 consensus): click ONLY selects (glow + dock) — the map never relays out on
    // a body click. Expansion is the SEPARATE, explicit ▸/▴ chip gesture, detected below via
    // the data-expand-chip marker (node.data can't carry a toggleCapsule callback — it's built
    // in the async ELK layout pass, outside this component's closure).
    select(rec,n.id);
    const isExpandChip=isCapsuleView&&ev&&ev.target&&ev.target.closest&&ev.target.closest("[data-expand-chip]");
    if(isExpandChip&&(n.type==="capsule"||n.type==="bucket")){
      // toggleCapsule() flips expandedGroups, a dep of the view-layout effect above, which
      // unconditionally clears sel/selId on ANY dep change (view change clears selection) —
      // wiping the select() call just made. Stash the record so the layout effect re-applies
      // it once the relayout finishes and the group re-renders under the same id (expanded
      // "bucket" or re-collapsed "capsule") — NOT the generic pendingSel/?sel= mechanism, which
      // fires eagerly on stale pre-relayout elements and would immediately re-toggle it back.
      pendingCapsuleSel.current={id:n.id,rec};toggleCapsule(n.id);}
  },[select,relationsFor,isCapsuleView,toggleCapsule,els]);
  const onEdgeClick=useCallback((_,ed)=>{const mt=ed.data&&ed.data.meta;if(!mt)return;
    const rows=[["from",mt.s],["to",mt.t]];
    if(mt.verb)rows.push(["flow",mt.verb]);
    if(mt.count)rows.push(["volume","×"+mt.count]);
    if(mt.tag)rows.push(["violation",mt.tag]);
    if(mt.stats)Object.entries(mt.stats).forEach(r=>rows.push(r));
    const rev=(view.edges||[]).find(e2=>e2.source===mt.tid&&e2.target===mt.sid);
    if(rev)rows.push(["mutual",mt.t+" → "+mt.s+(rev.count?" ×"+rev.count:"")+" — cycle"]);
    select({label:mt.s+" → "+mt.t,chip:mt.tag?"violation":"relation",rows,relations:[]},ed.id);},[select,view]);
  const onPaneClick=useCallback(()=>clearSel(),[clearSel]);
  // A5: organic canvas clicks always clear any finding-driven member highlight — only a
  // FINDINGS row click (below) should leave highlightMemberId set.
  const handleCanvasNodeClick=useCallback((e,n)=>{setHighlightMemberId(null);onNodeClick(e,n);},[onNodeClick]);
  // A5 (3a): a FINDINGS row click selects its subject on canvas — reuses the exact same
  // select()/onNodeClick mechanics a real click uses. Group-grain subjects select the
  // group node directly; member-grain subjects select the PARENT group (per spec) and mark
  // highlightMemberId so the dock's member table can highlight the specific row.
  const onFindingRowClick=useCallback(match=>{
    if(!match||!els||!els.nodes)return;
    const targetId=match.kind==="node"?match.nodeId:match.bucketId;
    const n=els.nodes.find(x=>x.id===targetId);
    if(n)onNodeClick(null,n);
    setHighlightMemberId(match.kind==="member"?match.memberId:null);
  },[els,onNodeClick]);
  useEffect(()=>{if(!pendingSel||!els||!els.nodes)return;
    const n=els.nodes.find(x=>x.id===pendingSel);
    if(n)onNodeClick(null,n);
    setPendingSel(null);},[els,pendingSel]);
  // ---- journeys: stepped walkthroughs across views (estate-level flow artifacts) ----
  const[jr,setJr]=useState(null);   // {def, idx}
  const jumpStep=useCallback((def,i)=>{if(!def||i<0||i>=def.steps.length)return;
    const st=def.steps[i];
    setJr({def,idx:i});
    setScope(st.scope);setLevel(st.view);setDirOv(null);
    if(st.sel)setPendingSel(st.sel);
    setExpanded(true);},[]);
  const startJourney=useCallback(async(meta,idx)=>{try{
      const r=await fetch(BASE+meta.shard.path);
      if(!r.ok)throw new Error("HTTP "+r.status+" loading journey "+meta.id);
      const def=await r.json();jumpStep(def,idx||0);
    }catch(e){showFatal("JOURNEY LOAD FAILED · "+(e&&e.message||e));}},[jumpStep]);
  const exitJourney=useCallback(()=>setJr(null),[]);
  // test hook: ?journey=<id>&jstep=<n> starts a journey for screenshot verification
  useEffect(()=>{const jid=q.get("journey");if(!jid)return;
    const meta=((ESTATES[estate]||{}).journeys||[]).find(j=>j.id===jid);
    if(meta)startJourney(meta,parseInt(q.get("jstep")||"0",10));},[]);
  useEffect(()=>{const h=ev=>{
      if(ev.key==="Escape"){if(jr)exitJourney();else if(sel)clearSel();else setExpanded(false);}
      if(jr&&ev.key==="ArrowRight")jumpStep(jr.def,jr.idx+1);
      if(jr&&ev.key==="ArrowLeft")jumpStep(jr.def,jr.idx-1);};
    window.addEventListener("keydown",h);return()=>window.removeEventListener("keydown",h);},[sel,clearSel,jr,jumpStep,exitJourney]);
  const[open,setOpen]=useState({});
  const[collapsed,setCollapsed]=useState(false);
  // Narrated findings from daemon (BE-1 route; B3 fix: never client-computed problems)
  const[findings,setFindings]=useState([]);
  // test hook: ?findingsOpen=1 pre-opens the drawer for screenshot verification
  const[findingsOpen,setFindingsOpen]=useState(q.get("findingsOpen")==="1");
  const[findingsExpandedId,setFindingsExpandedId]=useState(null);
  useEffect(()=>{let on=true;
    fetch("/api/arch/findings?scope="+encodeURIComponent(scope)).then(r=>r.ok?r.json():[])
      .then(d=>{if(on)setFindings(Array.isArray(d)?d:[]);}).catch(()=>{});
    return()=>{on=false;};},[scope]);
  const topFinding=findings.find(f=>f.severity==="error")||findings.find(f=>f.new)||findings[0]||null;
  const hasNewFindings=findings.some(f=>f.new);
  // A5 (3a/3b): resolve each daemon finding's subject(s) against the CURRENT view's own
  // element ids — a bucket-kind view's Member.ID *is* the unit fact ID (render_component.go),
  // the exact same id space Finding.Subjects uses (detect.go), so this is a pure client-side
  // lookup: no server-side subject index needed. Computed at render time off `view`+`findings`
  // (NOT baked into the async ELK layout pass) so a findings fetch that resolves after layout
  // never needs a relayout to become visible/clickable.
  const resolveSubject=useCallback(subj=>{
    if(view.kind==="buckets"){
      const grp=(view.buckets||[]).find(b=>b.id===subj);
      if(grp)return{kind:"group",bucketId:grp.id,label:grp.label};
      const owner=(view.buckets||[]).find(b=>(b.members||[]).some(m=>m.id===subj));
      if(owner){const mem=owner.members.find(m=>m.id===subj);
        return{kind:"member",bucketId:owner.id,memberId:mem.id,label:mem.label};}
      return null;}
    const n=(view.nodes||[]).find(x=>x.id===subj);
    return n?{kind:"node",nodeId:n.id,label:n.label}:null;},[view]);
  // One row per finding (not per subject) — a cycle finding can carry many subjects; the
  // dock shows "rule · message · subject" once, anchored to the first subject that resolves
  // inside the current view.
  const viewFindings=React.useMemo(()=>{
    if(view.kind==="table"||view.kind==="matrix")return[];
    const out=[];
    findings.forEach(f=>{
      if(f.scope&&f.scope!==scope)return;
      for(const subj of f.subjects||[]){
        const match=resolveSubject(subj);
        if(match){out.push({finding:f,match});break;}}});
    return out;},[findings,scope,view,resolveSubject]);
  // A5 (3b): member-grain finding subjects present in this view — drives the small ⚠ marker
  // on MemberNode. Independent of the legacy ov.concerns overlay toggle (separate mechanism).
  const findingSubjectIds=React.useMemo(()=>{
    if(!showFindings||view.kind!=="buckets")return null;
    const s=new Set();
    findings.forEach(f=>{if(f.scope&&f.scope!==scope)return;(f.subjects||[]).forEach(x=>s.add(x));});
    return s;},[findings,scope,showFindings,view]);
  const displayNodes=React.useMemo(()=>{
    if(!els||!els.nodes)return null;
    if((!findingSubjectIds||!findingSubjectIds.size)&&!expandedFold)return els.nodes;
    return els.nodes.map(n=>{
      let nn=n;
      if(findingSubjectIds&&findingSubjectIds.size&&n.type==="member"&&findingSubjectIds.has(n.id))
        nn={...nn,data:{...nn.data,showFinding:true}};
      // A8 FIX 2a: the bucket whose "+N more" fold is open gets bumped ABOVE the member/fold
      // node siblings (zIndex 10, set at layout time) so its in-node scrollable overlay of the
      // remaining members paints on top instead of underneath them — pure display-layer state,
      // no ELK relayout. _closeFold is a live closure (this memo runs in the component's own
      // render, not the async ELK pass) so BucketNode's "show less" row can fold it back.
      if(expandedFold&&n.id===expandedFold&&(n.type==="bucket"||n.type==="capsule"))
        nn={...nn,zIndex:50,data:{...nn.data,_foldOpen:true,_closeFold:()=>setExpandedFold(null)}};
      return nn;});},[els,findingSubjectIds,expandedFold]);
  // manual navigation leaves journey mode — the journey owns scope/view only while followed
  const go=useCallback(id=>{setJr(null);setLevel(id);setDirOv(null);},[]);
  const goScope=useCallback(sid=>{setJr(null);setScope(sid);setLevel(firstView(estate,sid));setDirOv(null);},[estate]);
  const goEstate=useCallback(eid=>{setJr(null);setEstate(eid);const sc=firstScope(eid);
    setScope(sc);setLevel(firstView(eid,sc));setDirOv(null);},[]);
  const btn=a=>({background:a?T.cardH:"transparent",border:`1px solid ${a?T.blue:T.border}`,
    color:a?T.text:T.dim,borderRadius:7,padding:"5px 11px",fontSize:12,cursor:"pointer",fontWeight:550});
  // A3 house ruling: the Findings lens is discoverable but QUIET — even lit, it stays a muted
  // outline (never the alarm red the findings themselves use), so the toolbar itself never shouts.
  const rbtn=a=>({background:"transparent",border:`1px solid ${a?T.dim:T.border}`,
    color:a?T.text:T.mute,borderRadius:7,padding:"5px 11px",fontSize:12,cursor:"pointer",fontWeight:550});
  // A6: goal-line — the one question this view answers, at-a-glance (dock VIEW segment keeps the
  // full question+caption; header shows question only — owner ruling: no empty placeholders).
  const VI=VIEW_INTENT[level]||null;
  // A6: LIVE freshness chip — honest "code as of <age>" reusing the same generated.timestamp the
  // footer already carries (serve-time, injected fresh on every 200 poll response). Green dot only
  // under ~2 minutes old; otherwise neutral — never a fake "live" claim (owner ruling).
  // Server emits "YYYY-MM-DD HH:MM:SS UTC" (arch_handler.go time.Now().UTC().Format), which sits
  // outside the ECMA-262-guaranteed ISO 8601 subset — normalize to a strict ISO string
  // ("YYYY-MM-DDTHH:MM:SSZ") before parsing so freshness works on every engine, not just V8.
  const genTs=(MODEL.generated&&MODEL.generated.timestamp)||"";
  const genIso=genTs.replace(" ","T").replace(/ ?UTC$/,"Z");
  const genMs=Date.parse(genIso);
  const genFresh=Number.isFinite(genMs)&&(Date.now()-genMs)<120000;
  return html`<div style=${{height:"100vh",display:"flex",flexDirection:"column",background:T.bg,
    font:"13px -apple-system,Segoe UI,Inter,Roboto,sans-serif",color:T.text}}>
    ${!EMBED&&html`<div style=${{padding:"9px 18px",borderBottom:`1px solid ${T.border}`,display:"flex",alignItems:"center",gap:10,background:T.chrome}}>
      <div style=${{fontWeight:750,fontSize:15,flexShrink:0}}>aOa <span style=${{color:T.dim,fontWeight:400}}>· architecture</span></div>
      <select value=${estate} onChange=${e=>goEstate(e.target.value)}
        style=${{background:T.card,color:ESTATES[estate].sim?T.yellow:T.green,border:`1px solid ${T.border}`,
        borderRadius:7,padding:"4px 8px",fontSize:11.5,fontWeight:600,cursor:"pointer",flexShrink:0,maxWidth:230}}>
        ${Object.entries(ESTATES).map(([eid,ev])=>html`<option key=${eid} value=${eid}>${ev.sim?"◌ ":"● "}${ev.label}</option>`)}
      </select>
      <div style=${{flex:1,minWidth:0,display:"flex",alignItems:"center",gap:8,overflow:"hidden"}}>
        <span style=${{fontSize:12.5,color:T.blue,fontWeight:650,whiteSpace:"nowrap"}}>${sys.label}</span>
        <span style=${{color:T.mute}}>▸</span>
        <span title=${view.title} style=${{fontSize:12.5,color:T.text,fontWeight:600,whiteSpace:"nowrap",overflow:"hidden",textOverflow:"ellipsis"}}>${view.title}</span>
        ${VI&&VI.question?html`<span title=${VI.question} style=${{fontSize:11,color:T.mute,whiteSpace:"nowrap",
          overflow:"hidden",textOverflow:"ellipsis",flexShrink:2,minWidth:0}}>${VI.question}</span>`:null}
        ${view.prov?html`<span title=${view.prov.label} style=${(pk=>({fontSize:9,fontWeight:700,letterSpacing:.5,whiteSpace:"nowrap",cursor:"help",flexShrink:0,
          color:pk==="derived"?T.green:pk==="simulated"?T.yellow:T.cyan,
          border:`1px solid ${pk==="derived"?T.green:pk==="simulated"?T.yellow:T.cyan}`,
          borderRadius:5,padding:"1px 7px"}))(view.prov.kind)}>${view.prov.kind==="derived"?"REAL":view.prov.kind==="simulated"?"SIMULATED":"MIXED"}</span>`:null}
        ${Number.isFinite(genMs)?html`<span title=${"generated "+MODEL.generated.timestamp} style=${{fontSize:9,fontWeight:700,letterSpacing:.5,whiteSpace:"nowrap",cursor:"help",flexShrink:0,
          color:genFresh?T.green:T.mute,border:`1px solid ${genFresh?T.green:T.border}`,
          borderRadius:5,padding:"1px 7px"}}>${genFresh?"● ":"○ "}current · code as of ${ago(genMs)}</span>`:null}
        ${showFindings&&els&&els.problems&&els.problems.length?html`<span title="Show findings"
          onClick=${()=>setExpanded(true)}
          style=${{fontSize:9.5,fontWeight:700,color:T.red,border:`1px solid ${T.red}`,borderRadius:5,
          padding:"1px 7px",whiteSpace:"nowrap",cursor:"pointer",flexShrink:0}}>⚠ ${els.problems.length}</span>`:null}
        ${ISSUES.length?html`<span title=${ISSUES.join("\n")}
          style=${{fontSize:9.5,fontWeight:700,color:T.yellow,border:`1px solid ${T.yellow}`,borderRadius:5,
          padding:"1px 7px",whiteSpace:"nowrap",cursor:"help",flexShrink:0}}>◌ ${ISSUES.length}</span>`:null}
      </div>
      </div>`}
    <div style=${{padding:"3px 18px",borderBottom:`1px solid ${T.border}`,fontSize:10.5,color:T.dim,
      display:"flex",alignItems:"center",gap:10,background:T.chrome}}>
      ${((fc)=>html`<span style=${{minWidth:0,whiteSpace:"nowrap",overflow:"hidden",textOverflow:"ellipsis"}}
        title=${view.count+fc+(view.prov?" · "+view.prov.label:"")}>
        ${view.count}${fc}${view.prov?html`<span style=${{color:T.mute}}> · ${view.prov.label}</span>`:null}
      </span>`)((showFindings&&!(els&&els.isEmpty)&&view.findingsClause)||"")}
      <div style=${{marginLeft:"auto",display:"flex",gap:7,alignItems:"center",flexShrink:0}}>
        <button style=${rbtn(showFindings)} onClick=${toggleShowFindings}
          title="Findings lens — band violations, cycles, orphans, god components (off by default: calm map first)">Findings</button>
        ${showFindings?html`<${React.Fragment}>
          <button style=${btn(ov.concerns)} onClick=${()=>setOv({...ov,concerns:!ov.concerns})}
            title="Recon findings per package (bitmask)">⚠ Members</button>
          <button style=${btn(ov.changed)} onClick=${()=>setOv({...ov,changed:!ov.changed})}
            title="Touched in last 15 commits (git)">Δ Changed</button>
        <//>`:null}
        <span style=${{width:6}}></span>
        <button style=${btn(den==="compact")} onClick=${()=>setDen("compact")}>Compact</button>
        <button style=${btn(den==="comfort")} onClick=${()=>setDen("comfort")}>Comfort</button>
        ${!(els&&els.htmlView)?html`<${React.Fragment}>
        <span style=${{width:6}}></span>
        <button style=${btn(!dirOv)} onClick=${()=>setDirOv(null)}
          title="Pick the direction that best fits the viewport">Auto${!dirOv&&autoDir?(autoDir==="DOWN"?" ↓":" →"):""}</button>
        <button style=${btn(dirOv==="DOWN")} onClick=${()=>setDirOv("DOWN")} title="Top–Bottom">↓</button>
        <button style=${btn(dirOv==="RIGHT")} onClick=${()=>setDirOv("RIGHT")} title="Left–Right">→</button>
        <//>`:null}
      </div>
    </div>
    ${showFindings&&topFinding?html`<div onClick=${()=>setFindingsOpen(true)}
      style=${{padding:"4px 18px",borderBottom:`1px solid ${T.border}`,fontSize:11,
        background:hasNewFindings?"#fbbf2408":T.chrome,cursor:"pointer",
        display:"flex",alignItems:"center",gap:8,
        borderLeft:hasNewFindings?`2px solid ${T.yellow}`:"2px solid transparent"}}>
      <span style=${{fontSize:9.5,fontWeight:700,color:T.arch,letterSpacing:.5,flexShrink:0}}>FINDINGS</span>
      ${hasNewFindings?html`<span style=${{fontSize:8,fontWeight:700,color:T.yellow,border:`1px solid ${T.yellow}`,
        borderRadius:4,padding:"0 4px",flexShrink:0}}>NEW</span>`:null}
      <span style=${{flex:1,color:T.dim,whiteSpace:"nowrap",overflow:"hidden",textOverflow:"ellipsis"}}>${narrateOne(topFinding,0)}</span>
      <span style=${{fontSize:9.5,color:T.mute,flexShrink:0}}>${findings.length} · click for all</span>
    </div>`:null}
    ${jr&&jr.idx>=0?(st=>html`<div style=${{padding:"6px 18px",borderBottom:`1px solid ${T.border}`,
      background:T.raise,display:"flex",alignItems:"center",gap:10}}>
      <span style=${{fontSize:8.5,fontWeight:700,letterSpacing:1,color:T.purple,
        border:`1px solid ${T.purple}`,borderRadius:4,padding:"1px 6px",flexShrink:0}}>JOURNEY</span>
      <span style=${{fontSize:12.5,fontWeight:700,whiteSpace:"nowrap",flexShrink:0}}>${jr.def.label}</span>
      <button onClick=${()=>jumpStep(jr.def,jr.idx-1)} disabled=${jr.idx===0}
        style=${{background:"transparent",border:`1px solid ${T.border}`,color:jr.idx===0?T.mute:T.text,
        borderRadius:6,padding:"1px 9px",cursor:jr.idx===0?"default":"pointer",fontSize:12,flexShrink:0}}>‹</button>
      <span style=${{fontSize:10.5,color:T.dim,flexShrink:0}}>${jr.idx+1} / ${jr.def.steps.length}</span>
      <button onClick=${()=>jumpStep(jr.def,jr.idx+1)} disabled=${jr.idx===jr.def.steps.length-1}
        style=${{background:"transparent",border:`1px solid ${T.border}`,color:jr.idx===jr.def.steps.length-1?T.mute:T.text,
        borderRadius:6,padding:"1px 9px",cursor:jr.idx===jr.def.steps.length-1?"default":"pointer",fontSize:12,flexShrink:0}}>›</button>
      <span style=${{fontSize:11.5,fontWeight:650,whiteSpace:"nowrap",flexShrink:0}}>${st.label}</span>
      <span title=${st.narrative} style=${{fontSize:11,color:T.dim,minWidth:0,flex:1,
        whiteSpace:"nowrap",overflow:"hidden",textOverflow:"ellipsis"}}>${st.narrative}</span>
      <button onClick=${exitJourney} title="Exit journey (Esc)"
        style=${{background:"transparent",border:"none",color:T.mute,cursor:"pointer",fontSize:13,flexShrink:0}}>✕</button>
    </div>`)(jr.def.steps[jr.idx]):null}
    <div style=${{flex:1,display:"flex",minHeight:0}}>
      <${Sidebar} estate=${estate} scopes=${SC} simEstate=${ESTATES[estate].sim}
        scope=${scope} goScope=${goScope} level=${level} go=${go} open=${open} setOpen=${setOpen}
        collapsed=${collapsed} setCollapsed=${setCollapsed} last=${last}
        journeys=${ESTATES[estate].journeys||[]} startJourney=${startJourney}/>
      <div style=${{flex:1,display:"flex",flexDirection:"column",minWidth:0}}>
        <div style=${{flex:1,position:"relative",minHeight:0}}>
          ${findingsOpen?html`<${FindingsDrawer} findings=${findings} open=${findingsOpen} setOpen=${setFindingsOpen}
            expandedId=${findingsExpandedId} setExpandedId=${setFindingsExpandedId}/>`:null}
          ${els&&els.isEmpty?html`<${EmptyShardView} view=${view} vid=${level}/>`:null}
          ${els&&els.htmlView&&!els.isEmpty?html`<${view.kind==="table"?TableView:DSMView} view=${view} onSel=${select} selId=${selId} vid=${level} den=${den}/>`:null}
          ${els&&!els.htmlView?html`<${ReactFlow} key=${estate+"|"+scope+"|"+level+"|"+dir+"|"+den}
            nodes=${displayNodes||els.nodes} edges=${els.edges} nodeTypes=${nodeTypes} edgeTypes=${edgeTypes}
            onNodeClick=${handleCanvasNodeClick} onEdgeClick=${onEdgeClick} onPaneClick=${onPaneClick}
            fitView fitViewOptions=${{padding:0.12}}
            minZoom=${0.1} proOptions=${{hideAttribution:true}}>
            <${Background} color=${T.border} gap=${24} size=${1}/>
            <${Controls} position="bottom-right"/>
          <//>`:null}
          ${els&&!els.htmlView&&(view._loaded||!view.shard)?html`<${CanvasLegend} view=${view}/>`:null}
        </div>
        <${BottomDock} vid=${level} view=${view} sel=${sel} selId=${selId} clearSel=${clearSel}
          probs=${els&&els.problems||[]} findings=${viewFindings} onFindingClick=${onFindingRowClick}
          highlightMemberId=${highlightMemberId}
          expanded=${expanded} setExpanded=${setExpanded}
          moreFlows=${els&&els.moreFlows||0} showFindings=${showFindings}
          estate=${estate} scopeLabel=${sys.label} setLevel=${setLevel} setDirOv=${setDirOv}/>
        ${els&&(view._loaded||!view.shard)?html`<${Footer} view=${view} ov=${ov}/>`:null}
      </div>
    </div></div>`;}
// L19.20: refresh-on-save — keep a root ref so poll can re-render on 200.
const _root=createRoot(document.getElementById("root"));
_root.render(html`<${ReactFlowProvider}><${Flow}/><//>`)
// Poll every 12s with If-None-Match (ETag = m.Rev set at boot and updated on each 200).
// 304 → no-op (arch unchanged — correct, even for zero-symbol file changes).
// 200 → update MODEL + ESTATES and re-render; React preserves hook state (selected
//       view, density, layout direction) across the re-render.
setInterval(async()=>{try{
  const h=_eTag?{"If-None-Match":_eTag}:{};
  const r=await fetch(MODEL_PATH,{headers:h});
  if(r.status===304)return;  // unchanged — no-op
  if(!r.ok)return;           // transient error — skip silently, retry next interval
  _eTag=r.headers.get("ETag")||"";
  MODEL=await r.json();
  ESTATES=MODEL.estates;
  _root.render(html`<${ReactFlowProvider}><${Flow}/><//>`)
}catch(e){}},12000);
