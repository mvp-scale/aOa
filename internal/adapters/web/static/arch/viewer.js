
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
 aoa:{cmd:T.purple,app:T.blue,adapters:T.arch,domain:T.green,ports:T.red,atlas:T.cyan,supporting:T.neutral},
 gf:{cli:T.purple,serve:T.blue,ingest:T.cyan,pipeline:T.green,render:T.yellow,infra:T.arch,supporting:T.neutral},
 dep:{dev:T.blue,ci:T.purple,registry:T.arch,user:T.green},
 retail:{order:T.arch,inventory:T.cyan,customer:T.purple,shared:T.neutral},
 mc:{aws:T.yellow,gcp:T.blue,shared:T.neutral}};
let ESTATES=MODEL.estates; // L19.20: let — poll updates on 200 before re-render
const firstScope=e=>Object.keys(ESTATES[e].scopes)[0];
const firstView=(e,sc)=>Object.keys(ESTATES[e].scopes[sc].views)[0];
// generic catalog for simulated scopes: derived from the views the scope actually has
const VIEWFAM={context:["C4 Model","System Context"],container:["C4 Model","Container"],
 component:["C4 Model","Component"],domains:["C4 Model","Domain map"],
 deployment:["Technology & Ops","Deployment"],dataflow:["Flows & Behavior","Data Flow (DFD)"],
 datamodel:["Data","Data Model / ER"]};
// THE STANDARD CATALOG (from the canonical-documents research) — shown IN FULL for
// every system; items the system has render live/sim, the rest stay listed as planned.
const STD_CATALOG=[
 {grp:"C4 Model",tag:"modern",items:[
   {vid:["context"],label:"System Context"},
   {vid:["container"],label:"Container"},
   {vid:["component","domains"],label:"Component"},
   {vid:["deployment"],label:"Deployment"},
   {vid:["sequence"],label:"Dynamic (sequence)",note:"needs call-edge resolution"},
   {vid:["code"],label:"Code (L4)",note:"symbol table · not drawn by design — needs call-edge resolution"}]},
 {grp:"Flows & Behavior",items:[
   {vid:["dataflow"],label:"Data Flow (DFD)"},
   {vid:["trust"],label:"Trust Boundaries (STRIDE)",note:"DFD overlay · rule-pack"},
   {vid:["statemachine"],label:"State Machine",note:"needs state extraction"}]},
 {grp:"Data",items:[
   {vid:["datamodel"],label:"Data Model / ER"},
   {vid:["glossary"],label:"Glossary",note:"atlas seed + writer"}]},
 {grp:"Technology & Ops",items:[
   {vid:["techportfolio"],label:"Technology Portfolio",note:"config scan"},
   {vid:["sbom"],label:"SBOM (CycloneDX)",note:"document · manifests"}]},
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
  const CMAP={purple:T.purple,blue:T.blue,arch:T.arch,green:T.green,red:T.red,cyan:T.cyan,yellow:T.yellow,neutral:T.neutral,dim:T.dim};
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
      return {id:hit,label:it.label,alias:it.alias,
        status:(v.prov&&v.prov.kind==="simulated")?"sim":"live"};}
    return {label:it.label,status:"planned",note:it.note||"not yet derived for this system",vid0:(it.vid||[])[0]};})}));}
const snap=v=>Math.round(v/8)*8;
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
  dangerouslySetInnerHTML=${{__html:IP[k]||IP.supporting}}/>`;}

const ElkEdge=memo(function ElkEdge({id,data,style,markerEnd}){
  if(!data||!data.section) return null;
  const s=data.section;
  const pts=[s.startPoint,...(s.bendPoints||[]),s.endPoint];
  const d=pts.map((p,i)=>(i===0?"M ":"L ")+p.x+" "+p.y).join(" ");
  const m=data.meta;
  return html`<${React.Fragment}>
    <${BaseEdge} id=${id} path=${d} markerEnd=${markerEnd}
      style=${data._sel?{...style,strokeWidth:3,opacity:1}:style}/>
    ${data.label?html`<${EdgeLabelRenderer}>
      <div className=${"nodrag nopan"+(m?" hv":"")} style=${{position:"absolute",
        transform:`translate(${data.label.x}px,${data.label.y}px)`,
        background:T.bg,border:`1px solid ${data._sel?T.blue:T.border}`,borderRadius:5,
        padding:"0 5px",height:16,lineHeight:"15px",fontSize:10.5,fontWeight:600,
        color:T.text,pointerEvents:"all",zIndex:5,cursor:"pointer"}}>${data.label.text}
        ${m?html`<${HoverCard} title=${m.s+" → "+m.t}
          rows=${[["flow",m.verb],["volume",m.count?"×"+m.count:""],[m.tag?"violation":"",m.tag||""]]}
          hint="click → relation detail"/>`:null}
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
    color:T.text,cursor:"pointer",boxShadow:data._sel?`0 0 0 2px ${T.blue}`:"none"}}>
    <${Handle} type="target" position=${Position.Top} style=${{opacity:0}}/><${Handle} type="target" position=${Position.Left} style=${{opacity:0}}/>
    <div style=${{display:"flex",alignItems:"center",gap:8,height:"100%",opacity:mock?0.88:1}}>
      <${Ico} k=${data.icon} c=${col} s=${17}/>
      <span style=${{fontSize:13.5,fontWeight:600,whiteSpace:"nowrap"}}>${data.label}</span>
      ${drill?html`<span style=${{marginLeft:"auto",fontSize:10,color:col,border:`1px solid ${col}`,borderRadius:5,padding:"0 5px"}}>open ▸</span>`:null}
    </div>
    <${HoverCard} title=${data.label} rows=${(data.stats?Object.entries(data.stats):[["type",NAME[t]||t],["detail",data.sub]]).concat(mock?[["status","inferred · sourceable"]]:[])}
      hint=${drill?"click → opens "+data.drillTo+" view":"click → details"}/>
    <${Handle} type="source" position=${Position.Bottom} style=${{opacity:0}}/><${Handle} type="source" position=${Position.Right} style=${{opacity:0}}/>
  </div>`;}
function EntityNode({data}){
  const c=T.green;
  return html`<div class=${hvc(data.id)} style=${{width:data.w-2,cursor:"pointer",
    boxShadow:data._sel?`0 0 0 2px ${T.blue}`:"none",borderRadius:6}}>
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
  return html`<div style=${{width:"100%",height:"100%",background:T.band,
    border:`1.5px ${dash?"dashed":"solid"} ${data._god?T.red:c}`,borderRadius:4,boxSizing:"border-box",
    opacity:data._dead?0.45:1,cursor:"pointer",
    boxShadow:data._sel?`0 0 0 2px ${T.blue}`:data._god?`0 0 0 2px ${T.red}55, 0 0 22px ${T.red}44`:"none"}}>
    <${Handle} type="target" position=${Position.Top} style=${{opacity:0}}/><${Handle} type="target" position=${Position.Left} style=${{opacity:0}}/>
    <div style=${{display:"flex",alignItems:"center",gap:7,padding:"7px 12px",height:data.head,boxSizing:"border-box"}}>
      <${Ico} k=${data.ico||data.layer} c=${c} s=${14}/>
      <span style=${{fontSize:11,fontWeight:700,color:c,textTransform:"uppercase",letterSpacing:1.1}}>${data.label}</span>
      ${data._cyc?html`<span style=${{fontSize:8.5,fontWeight:700,color:T.red,border:`1px solid ${T.red}`,borderRadius:4,padding:"0 4px"}}>⟳ CYCLE</span>`:null}
      ${data._dead?html`<span style=${{fontSize:8.5,fontWeight:700,color:T.yellow,border:`1px solid ${T.yellow}`,borderRadius:4,padding:"0 4px"}}>ORPHAN</span>`:null}
      ${data._over?html`<span style=${{fontSize:8.5,fontWeight:700,color:T.yellow,border:`1px solid ${T.yellow}`,borderRadius:4,padding:"0 4px"}}>${data._over} · COLLAPSED</span>`:null}
      <span style=${{marginLeft:"auto",fontSize:10.5,color:T.dim}}>${data.members.length}</span>
    </div>
    <${Handle} type="source" position=${Position.Bottom} style=${{opacity:0}}/><${Handle} type="source" position=${Position.Right} style=${{opacity:0}}/>
  </div>`;}
function SoloNode({data}){const c=data.col;const dash=data.layer==="supporting";
  return html`<div class=${hvc(data.member.id)} style=${{background:T.band,border:`1.5px ${dash?"dashed":"solid"} ${c}`,
    borderRadius:4,padding:"8px 12px",width:data.w-2,height:data.h-2,boxSizing:"border-box",color:T.text,
    cursor:"pointer",boxShadow:data._sel?`0 0 0 2px ${T.blue}`:"none"}}>
    <${Handle} type="target" position=${Position.Top} style=${{opacity:0}}/><${Handle} type="target" position=${Position.Left} style=${{opacity:0}}/>
    <div style=${{display:"flex",alignItems:"center",gap:7}}>
      <${Ico} k=${data.ico||data.layer} c=${c} s=${14}/>
      <span style=${{fontSize:11,fontWeight:700,color:c,textTransform:"uppercase",letterSpacing:1.1,whiteSpace:"nowrap"}}>${data.label}</span>
      <span style=${{fontSize:12.5,fontWeight:600,marginLeft:6,whiteSpace:"nowrap"}}>${data.member.label}</span>
    </div>
    <${HoverCard} title=${data.member.label} rows=${data.member.stats?Object.entries(data.member.stats):[["layer",data.label],["detail",data.member.sub]]} hint="click → details"/>
    <${Handle} type="source" position=${Position.Bottom} style=${{opacity:0}}/><${Handle} type="source" position=${Position.Right} style=${{opacity:0}}/>
  </div>`;}
function MemberNode({data}){const c=data.col;
  return html`<div class=${hvc(data.id)} style=${{background:T.card,border:`1px solid ${T.border}`,borderLeft:`3px solid ${c}`,
    borderRadius:6,padding:data.wrap?"4px 10px":"5px 10px",width:data.w,height:data.h,boxSizing:"border-box",color:T.text,
    display:"flex",alignItems:"center",cursor:"pointer",boxShadow:data._sel?`0 0 0 2px ${T.blue}`:"none"}}>
    <div style=${{display:"flex",alignItems:"center",gap:6,width:"100%"}}>
      <${Ico} k=${data.lay} c=${c} s=${12}/>
      <span style=${{fontSize:data.wrap?10.5:11.5,fontWeight:600,whiteSpace:data.wrap?"normal":"nowrap",lineHeight:1.25}}>${data.label}</span>
      ${(data.showC||data.showH)?html`<span style=${{marginLeft:"auto",display:"flex",gap:3,flexShrink:0}}>
        ${data.showC?html`<span style=${{fontSize:8.5,fontWeight:700,color:T.red,border:`1px solid ${T.red}`,
          borderRadius:7,padding:"0 4px",lineHeight:"12px"}}>⚠${data.concerns>99?"99+":data.concerns}</span>`:null}
        ${data.showH?html`<span style=${{fontSize:8.5,fontWeight:700,color:T.yellow,border:`1px solid ${T.yellow}`,
          borderRadius:7,padding:"0 4px",lineHeight:"12px"}}>Δ</span>`:null}
      </span>`:null}
    </div>
    <${HoverCard} title=${data.label} rows=${(data.stats?Object.entries(data.stats):[["detail",data.sub]]).concat([
      [data.concerns?"findings":"",data.concerns?data.concerns+" recon findings":""],
      [data.changed?"recent":"",data.changed?"touched in last 15 commits":""]])} hint="click → details"/>
  </div>`;}
function TableView({view,onSel,selId,vid}){
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
  return html`<div style=${{position:"absolute",inset:0,overflow:"auto",padding:"56px 40px 40px"}}>
    <table style=${{borderCollapse:"collapse",minWidth:560,fontSize:12.5,color:T.text}}>
      <thead><tr>${(view.columns||[]).map((c,i)=>html`<th key=${i} style=${{textAlign:"left",
        padding:"8px 16px",borderBottom:`2px solid ${T.border}`,color:T.dim,fontSize:10.5,
        textTransform:"uppercase",letterSpacing:1}}>${c}</th>`)}</tr></thead>
      <tbody>${(view.rows||[]).map((r,ri)=>html`<tr key=${ri} onClick=${()=>pick(r,ri)}
        style=${{cursor:"pointer",background:selId==="row:"+ri?T.cardH:"transparent"}}>
        ${r.map((cell,ci)=>html`<td key=${ci} style=${{padding:"7px 16px",
          borderBottom:`1px solid ${T.border}55`,
          color:String(cell).startsWith("⚠")?T.red:ci===0?T.text:T.dim,
          fontWeight:ci===0?600:400,fontFamily:ci===0?"inherit":"ui-monospace,monospace",
          fontSize:ci===0?12.5:11.5}}>${cell}</td>`)}</tr>`)}</tbody>
    </table></div>`;}
function MatrixView({view,onSel,selId}){
  const items=view.items||[],M=view.matrix||[];
  // cell click = the pair record (both directions, MUTUAL flagged); header click = the module's fan profile
  const pickCell=(i,j)=>{const v=(M[i]||[])[j];if(i===j||!v||!onSel)return;
    const rv=(M[j]||[])[i];
    onSel({label:items[i]+" → "+items[j],chip:rv?"mutual":"dependency",
      rows:[["row → col",items[i]+" → "+items[j]+" · "+v]]
        .concat(rv?[["col → row",items[j]+" → "+items[i]+" · "+rv],["status","MUTUAL — cycle"]]:[]),
      relations:[]},"mx:"+i+","+j);};
  const pickMod=i=>{if(!onSel)return;
    const outs=(M[i]||[]).reduce((a,v)=>a+(v||0),0);
    const ins=items.reduce((a,_,r)=>a+((M[r]||[])[i]||0),0);
    onSel({label:items[i],chip:"module",
      rows:[["fan-out",outs+" dependencies"],["fan-in",ins+" dependents"]],
      relations:items.map((it,j)=>({dir:"out",peer:it,verb:"",count:(M[i]||[])[j],
        viol:!!((M[i]||[])[j]&&(M[j]||[])[i])})).filter(r=>r.count)
       .concat(items.map((it,j)=>({dir:"in",peer:it,verb:"",count:(M[j]||[])[i],
        viol:!!((M[i]||[])[j]&&(M[j]||[])[i])})).filter(r=>r.count&&r.peer!==items[i]))
       .sort((a,b)=>(b.viol?1:0)-(a.viol?1:0))},"mx:"+i);};
  return html`<div style=${{position:"absolute",inset:0,overflow:"auto",padding:"56px 40px 40px"}}>
    <table style=${{borderCollapse:"collapse",fontSize:11,color:T.text}}>
      <thead><tr><th style=${{padding:"6px 10px"}}></th>
        ${items.map((it,i)=>html`<th key=${i} onClick=${()=>pickMod(i)} style=${{padding:"6px 10px",color:T.dim,fontSize:10,
          textTransform:"uppercase",letterSpacing:.5,cursor:"pointer",
          background:selId==="mx:"+i?T.cardH:"transparent"}}>${it}</th>`)}</tr></thead>
      <tbody>${items.map((row,i)=>html`<tr key=${i}>
        <th onClick=${()=>pickMod(i)} style=${{padding:"6px 12px",textAlign:"right",color:T.dim,fontSize:10,
          textTransform:"uppercase",letterSpacing:.5,cursor:"pointer",
          background:selId==="mx:"+i?T.cardH:"transparent"}}>${row}</th>
        ${items.map((_,j)=>{const v=(M[i]||[])[j];const cyc=v&&(M[j]||[])[i];
          return html`<td key=${j} onClick=${()=>pickCell(i,j)} style=${{width:44,height:32,textAlign:"center",
            border:`1px solid ${selId==="mx:"+i+","+j?T.blue:T.border+"55"}`,
            cursor:i!==j&&v?"pointer":"default",
            background:i===j?T.cardH:v?(cyc?T.red+"33":T.blue+"22"):"transparent",
            color:cyc?T.red:v?T.text:T.mute,fontWeight:v?700:400}}>${i===j?"·":(v||"")}</td>`;})}
      </tr>`)}</tbody></table>
    <div style=${{marginTop:14,fontSize:10.5,color:T.dim}}>cell = dependencies row → column · <span style=${{color:T.red}}>red = mutual (cycle)</span></div>
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
  return html`<div style=${{width:"100%",height:"100%",background:T.band,
    border:`1.5px ${dash?"dashed":"solid"} ${ext?T.mute:c}`,borderRadius:4,boxSizing:"border-box",
    cursor:"pointer",display:"flex",alignItems:"center",gap:7,padding:"7px 12px",
    boxShadow:data._sel?`0 0 0 2px ${T.blue}`:"none"}}>
    <${Handle} type="target" position=${Position.Top} style=${{opacity:0}}/><${Handle} type="target" position=${Position.Left} style=${{opacity:0}}/>
    <${Ico} k=${ext?"ext":data.ico||data.layer} c=${ext?T.mute:c} s=${14}/>
    <span style=${{fontSize:11,fontWeight:700,color:ext?T.mute:c,textTransform:"uppercase",letterSpacing:1.1,
      whiteSpace:"nowrap",flex:1,overflow:"hidden",textOverflow:"ellipsis"}}>${data.label}</span>
    <span style=${{fontSize:10.5,fontWeight:700,color:ext?T.mute:T.dim,flexShrink:0,
      background:T.chrome,borderRadius:8,padding:"1px 6px"}}>${data.memberCount!==undefined?data.memberCount:data.members&&data.members.length||0}</span>
    <span style=${{fontSize:9,color:T.mute,flexShrink:0}}>▸</span>
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

async function layoutSimple(view,dir,d){
  const ent=view.kind==="entity";
  const sizes={};
  view.nodes.forEach(n=>{
    if(ent){sizes[n.id]={w:Math.min(400,Math.max(264,snap((n.label||"").length*7.6+62))),h:34+n.fields.length*21+2};}
    else {const base=n.type==="sys"?248:208;
      // canvas carries identity only (sub moved to hover) — width follows the label, never ellipsis
      sizes[n.id]={w:Math.min(360,Math.max(base,snap((n.label||"").length*7.8+(n.drillTo?118:66)))),h:44};}});
  const g={id:"root",layoutOptions:EOPT(dir,d),
    children:view.nodes.map(n=>({id:n.id,width:sizes[n.id].w,height:sizes[n.id].h})),
    edges:view.edges.map(e=>({id:e.id,sources:[e.source],targets:[e.target],
      labels:[{text:e.label||"",width:lblW(e.label||""),height:16}]}))};
  const r=await elk.layout(g);
  const pos={};r.children.forEach(c=>pos[c.id]={x:snap(c.x),y:snap(c.y)});
  const laidById={};(r.edges||[]).forEach(e=>laidById[e.id]=e);
  const problems=[];const nById={};view.nodes.forEach(n=>nById[n.id]=n);
  view.edges.forEach(e=>{if(e.tag){e._viol=true;
    problems.push(e.tag+": "+((nById[e.source]||{}).label)+" → "+((nById[e.target]||{}).label));}});
  return {nodes:view.nodes.map(n=>({id:n.id,type:ent?"entity":"box",position:pos[n.id],
      width:sizes[n.id].w,height:sizes[n.id].h,
      data:{...n,w:sizes[n.id].w,h:sizes[n.id].h}})).concat(labelSpacers(laidById)),
    edges:view.edges.map(e=>{const r=rfEdge(e,laidById,()=>T.dim,id2=>(nById[id2]||{}).label||id2);
      if(e._viol){r.style={...r.style,stroke:T.red,strokeDasharray:"6 3"};
        r.markerEnd={...r.markerEnd,color:T.red};
        if(r.data.label)r.data.label.text="⚠ "+r.data.label.text;}
      return r;}),problems};}

// Color is meaning: same layer name => same color, app-wide. Resolution order:
// view palette -> canonical layer pin -> stable name hash. Red/yellow are RESERVED
// (violations / warnings) and never appear in the rotation.
const CYCLE=[T.arch,T.cyan,T.purple,T.green,T.blue,T.neutral];
const LAYER_PIN={core:T.blue,channel:T.purple,integration:T.arch,data:T.green,
  external:T.dim,supporting:T.neutral,platform:T.cyan,edge:T.cyan};
const lhash=s=>{let h=0;for(let i=0;i<s.length;i++)h=(h*31+s.charCodeAt(i))>>>0;return h;};
// Per-view color map: palette/pinned names are absolute; hashed names probe to a free
// color on collision so one view never shows two layers in the same color.
const viewLayerColors=view=>{const pal=PALETTES[view.palette||"aoa"]||{};
  const used=new Set(),map={};
  const layers=[...new Set((view.buckets||[]).map(b=>b.layer))];
  layers.forEach(l=>{const c=pal[l]||LAYER_PIN[l];if(c){map[l]=c;used.add(c);}});
  layers.forEach(l=>{if(map[l])return;
    let i=lhash(l||"")%CYCLE.length,n=0;
    while(n<CYCLE.length&&used.has(CYCLE[i])){i=(i+1)%CYCLE.length;n++;}
    map[l]=CYCLE[i];used.add(CYCLE[i]);});
  return map;};
const layerColor=(view,l)=>viewLayerColors(view)[l]||LAYER_PIN[l]||CYCLE[lhash(l||"")%CYCLE.length];
// mergeExternalBuckets: fold all g_ext_* buckets into ONE EXTERNALS capsule (R3).
// Returns [internals[], externals_capsule_or_null].
function mergeExternalBuckets(buckets){
  const internal=buckets.filter(b=>!b.id.startsWith("g_ext_"));
  const external=buckets.filter(b=>b.id.startsWith("g_ext_"));
  if(!external.length)return[buckets,null];
  const allExtMembers=external.flatMap(b=>b.members||[]);
  const extCapsule={id:"g_EXTERNALS",layer:"supporting",boundary:true,
    label:"EXTERNALS",members:allExtMembers,
    memberCount:allExtMembers.length,part:99,
    _external:true};
  return[internal,extCapsule];}
async function layoutBuckets(view,dir,d,ov,opts){
  ov=ov||{};opts=opts||{};
  const capsuleMode=!!opts.capsuleMode;
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
  const remappedEdges=view.edges.map(e=>({...e,source:remapId(e.source),target:remapId(e.target)}));
  // Deduplicate remapped edges by (source,target) so merged externals don't produce parallel ELK edges
  const edgeSeen=new Set();
  const elkEdges=remappedEdges.filter(e=>{if(e.source===e.target)return false;
    const k=e.source+"\x00"+e.target;if(edgeSeen.has(k))return false;edgeSeen.add(k);return true;});
  elkEdges.forEach(e=>{if(deg[e.source])deg[e.source].o++;if(deg[e.target])deg[e.target].i++;
    const sp=bById[e.source],tp=bById[e.target];
    if(sp&&tp&&sp.part>tp.part){e._viol=true;problems.push("band violation: "+nameOfBucket(e.source)+" → "+nameOfBucket(e.target));}
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
  B.forEach(b=>{if(!capsuleMode&&(b.members||[]).length>40){b._over=b.members.length;
    b.members=b.members.slice(0,23).concat([{id:b.id+"_more",label:"+"+(b._over-23)+" more…",sub:"over budget"}]);}});
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
        data:{...b,head:d.head,col,expanded:false}});
      return;}
    if(b.solo){
      nodes.push({id:b.id,type:"solo",position:bp,width:b.w,height:b.h,
        zIndex:0,draggable:false,selectable:false,
        data:{layer:b.layer,label:b.label,member:b.members[0],w:b.w,h:b.h,col}});
      return;}
    nodes.push({id:b.id,type:"bucket",position:bp,width:b.w,height:b.h,
      style:{width:b.w,height:b.h},zIndex:0,draggable:false,selectable:false,
      data:{...b,head:d.head,col}});
    b.members.forEach((m,i)=>{const c2=i%b.cols,row=Math.floor(i/b.cols);
      nodes.push({id:m.id,type:"member",draggable:false,selectable:false,zIndex:10,
        width:b.iw,height:b.ih,
        position:{x:bp.x+d.px+c2*(b.iw+d.gx),y:bp.y+d.head+d.py+row*(b.ih+d.gy)},
        data:{...m,lay:b.layer,col,w:b.iw,h:b.ih,wrap:b.wrap,
          showC:!!(ov.concerns&&m.concerns>0),showH:!!(ov.changed&&m.changed)}});});});
  const bcol={};B.forEach(b=>bcol[b.id]=colOf(b.layer));
  const edges2=elkEdges.map(e=>{const r=rfEdge(e,laidById,ee=>bcol[ee.source]||T.dim,id2=>nameOfBucket(id2));
    if(e._viol){r.style={...r.style,stroke:T.red,strokeDasharray:"6 3",opacity:.95};
      r.markerEnd={...r.markerEnd,color:T.red};
      if(r.data.label)r.data.label.text="⚠ "+r.data.label.text;}
    return r;});
  return {nodes:nodes.concat(labelSpacers(laidById)),edges:edges2,problems};}

// Docked footer: ONE derived legend — shows only what the current view actually renders —
// plus the provenance stamp anchored right.
// Bottom dock — the drill tier. THREE FIXED SEGMENTS, always present, content changes:
//   VIEW (this view's record: question · pass · sources · counts)
//   SELECTION (the clicked element/edge: stat table + relations table, violations first)
//   CONCERNS (this view's diagnostics; rows touching the selection highlight)
// Persistent: 26px collapsed bar <-> 208px expanded. Never unmounts; the nav rail never reflows.
const DK={th:{fontSize:8.5,fontWeight:700,letterSpacing:.8,textTransform:"uppercase",color:T.mute,
    textAlign:"left",padding:"2px 10px 4px 0",borderBottom:`1px solid ${T.border}`},
  td:{fontSize:11,color:T.text,padding:"3px 10px 3px 0",borderBottom:`1px solid ${T.border}55`,
    verticalAlign:"top",lineHeight:1.45}};
function DockTable({cols,rows}){
  return html`<table style=${{borderCollapse:"collapse",width:"100%"}}>
    <thead><tr>${cols.map((c,i)=>html`<th key=${i} style=${DK.th}>${c}</th>`)}</tr></thead>
    <tbody>${rows.map((r,ri)=>html`<tr key=${ri}>${r.map((cell,ci)=>html`<td key=${ci}
      style=${{...DK.td,color:r._viol?T.red:ci===0?T.dim:T.text,
        fontFamily:ci>0&&String(cell).match(/^[≈×~\\d]/)?"ui-monospace,monospace":"inherit"}}>${cell}</td>`)}</tr>`)}
  </tbody></table>`;}
// The caption: the view ANSWERS its own question, derived at render time from data
// already on screen — counts, heaviest edge, mutual pairs, flagged rows, findings.
function caption(view,probs){
  const fin=probs&&probs.length?` · ⚠ ${probs.length} finding${probs.length>1?"s":""}`:"";
  if(view.kind==="buckets"){
    const B=view.buckets||[],members=B.reduce((a,b)=>a+(b.members||[]).length,0);
    const he=(view.edges||[]).reduce((m,e)=>((e.count||0)>(m.count||0)?e:m),{});
    const bn=id=>{const b=B.find(x=>x.id===id);return b?b.label:id;};
    return `${B.length} groups · ${members} members`+
      (he.id?` — heaviest: ${bn(he.source)} → ${bn(he.target)} ×${he.count}`:"")+fin;}
  if(view.kind==="matrix"){
    const it=view.items||[],M=view.matrix||[];let sum=0;const mut=[];
    for(let i=0;i<it.length;i++)for(let j=0;j<it.length;j++){sum+=(M[i]||[])[j]||0;
      if(i<j&&(M[i]||[])[j]&&(M[j]||[])[i])mut.push(`${it[i]} ↔ ${it[j]} (${M[i][j]}/${M[j][i]})`);}
    return `${sum} dependencies · ${it.length} modules · ${mut.length} mutual pair${mut.length===1?"":"s"}`+
      (mut.length?`: ${mut[0]}`:"");}
  if(view.kind==="table"){
    const rows=view.rows||[];const fl=rows.filter(r=>r.some(c=>String(c).trim().startsWith("⚠")));
    return `${rows.length} rows`+(fl.length?` · ⚠ ${fl.length} flagged — first: ${fl[0][0]}`:" · none flagged");}
  if(view.kind==="entity"){
    const deg={};(view.edges||[]).forEach(e=>{deg[e.source]=(deg[e.source]||0)+1;deg[e.target]=(deg[e.target]||0)+1;});
    const top=Object.entries(deg).sort((a,b)=>b[1]-a[1])[0];
    const nm=id=>{const n=(view.nodes||[]).find(x=>x.id===id);return n?n.label:id;};
    return `${(view.nodes||[]).length} entities · ${(view.edges||[]).length} relationships`+
      (top?` — spine: ${nm(top[0])} (${top[1]} relations)`:"")+fin;}
  const tag=(view.edges||[]).find(e=>e.tag);
  return `${(view.nodes||[]).length} elements · ${(view.edges||[]).length} labeled flows`+
    (tag?` · ⚠ ${tag.tag}: ${(tag.label||"").slice(0,48)}`:fin);}
function BottomDock({vid,view,sel,clearSel,probs,expanded,setExpanded}){
  const VI=VIEW_INTENT[vid]||null;
  const cap=caption(view,probs);
  const hl=p=>sel&&sel.label&&String(p).includes(String(sel.label).slice(0,24));
  const sortedProbs=sel?[...probs].sort((a,b)=>(hl(b)?1:0)-(hl(a)?1:0)):probs;
  const Seg=({title,col,flex,wash,children})=>html`<div style=${{flex,minWidth:0,padding:"9px 16px",
    borderRight:`1px solid ${T.border}`,overflowY:"auto",position:"relative",background:wash||"transparent"}}>
    <div style=${{fontSize:9.5,fontWeight:700,letterSpacing:1.2,color:col,marginBottom:7,
      position:"sticky",top:0,background:T.raise,paddingBottom:3}}>${title}</div>
    ${children}</div>`;
  if(!expanded)return html`<div onClick=${()=>setExpanded(true)}
    style=${{borderTop:`1px solid ${T.border}`,background:T.chrome,height:26,display:"flex",
    alignItems:"center",gap:0,flexShrink:0,cursor:"pointer",userSelect:"none"}}>
    <div style=${{flex:1.4,minWidth:0,padding:"0 16px",display:"flex",gap:7,alignItems:"center",overflow:"hidden"}}>
      <span style=${{fontSize:8.5,fontWeight:700,letterSpacing:1,color:T.blue,flexShrink:0}}>VIEW</span>
      <span style=${{fontSize:10.5,color:T.dim,whiteSpace:"nowrap",overflow:"hidden",textOverflow:"ellipsis"}}>${cap}</span>
    </div>
    <div style=${{flex:1,minWidth:0,padding:"0 16px",display:"flex",gap:7,alignItems:"center",borderLeft:`1px solid ${T.border}`,overflow:"hidden"}}>
      <span style=${{fontSize:8.5,fontWeight:700,letterSpacing:1,color:T.text,flexShrink:0}}>SELECTION</span>
      <span style=${{fontSize:10.5,color:sel?T.text:T.mute,whiteSpace:"nowrap",overflow:"hidden",textOverflow:"ellipsis"}}>${sel?sel.label:"none — click an element or edge"}</span>
    </div>
    <div style=${{flex:.6,padding:"0 16px",display:"flex",gap:7,alignItems:"center",borderLeft:`1px solid ${T.border}`,
      background:probs.length?"#f8717110":"transparent"}}>
      <span style=${{fontSize:8.5,fontWeight:700,letterSpacing:1,color:probs.length?T.red:T.mute}}>FINDINGS</span>
      <span style=${{fontSize:10.5,color:probs.length?T.red:T.green}}>${probs.length||"✓"}</span>
    </div>
    <span style=${{padding:"0 12px",color:T.mute,fontSize:11}}>⌃</span>
  </div>`;
  return html`<div style=${{borderTop:`1px solid ${T.border}`,background:T.raise,height:208,
    display:"flex",flexShrink:0,position:"relative",boxShadow:"0 -6px 16px #0009"}}>
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
        <div style=${{display:"flex",alignItems:"center",gap:8,marginBottom:7}}>
          <span style=${{fontSize:13,fontWeight:700,color:T.text}}>${sel.label}</span>
          ${sel.chip?html`<span style=${{fontSize:8.5,fontWeight:700,letterSpacing:.6,color:T.dim,
            border:`1px solid ${T.border}`,borderRadius:4,padding:"0 5px",textTransform:"uppercase"}}>${sel.chip}</span>`:null}
          <button onClick=${clearSel} title="Clear selection"
            style=${{marginLeft:"auto",background:"transparent",border:"none",color:T.mute,cursor:"pointer",fontSize:13}}>×</button>
        </div>
        <div style=${{display:"flex",gap:24,alignItems:"flex-start"}}>
          <div style=${{flex:1,minWidth:0}}>
            <${DockTable} cols=${["stat","value"]} rows=${sel.rows||[]}/>
          </div>
          ${(sel.relations&&sel.relations.length)?html`<div style=${{flex:1.2,minWidth:0}}>
            <${DockTable} cols=${["dir","peer","flow"]}
              rows=${sel.relations.map(r=>Object.assign([r.dir==="out"?"→":"←",r.peer,
                (r.viol?"⚠ ":"")+(r.verb||"")+(r.count?" ×"+r.count:"")],{_viol:r.viol}))}/>
          </div>`:null}
          ${(sel.members&&sel.members.length)?html`<div style=${{flex:1.2,minWidth:0}}>
            <${DockTable} cols=${["member ("+sel.members.length+")","detail"]}
              rows=${sel.members.slice(0,24).map(m=>[m.label,
                m.stats?Object.values(m.stats)[0]:(m.sub||"")])}/>
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
      <//>`:html`<div style=${{color:T.mute,fontSize:11,marginTop:28,textAlign:"center"}}>none — click an element or edge</div>`}
    <//>
    <${Seg} title=${"FINDINGS · "+probs.length} col=${probs.length?T.red:T.mute} flex=${1}
      wash=${probs.length?"#f8717108":null}>
      ${probs.length?sortedProbs.map((p,i)=>html`<div key=${i} style=${{fontSize:10.5,lineHeight:1.5,marginBottom:6,
        color:hl(p)?T.text:T.dim,borderLeft:`2px solid ${T.red}`,paddingLeft:9,
        background:hl(p)?T.cardH:"transparent"}}>${p}</div>`)
      :html`<div style=${{color:T.green,fontSize:11}}>✓ no findings in this view</div>`}
    <//>
    <button onClick=${()=>setExpanded(false)} title="Collapse"
      style=${{position:"absolute",top:5,right:8,background:"transparent",border:"none",
      color:T.mute,cursor:"pointer",fontSize:13}}>⌄</button>
  </div>`;}
const ETYPE_NAME={sys:"system",ext:"external",container:"container",store:"store",proc:"process"};
const ETYPE_COLR={sys:T.blue,ext:T.dim,container:T.arch,store:T.green,proc:T.blue};
// In-context legend: pinned to the canvas corner so color meaning sits where you look.
// Derived from what is actually on screen — same resolution the nodes use, by construction.
function CanvasLegend({view}){
  let items=[];
  if(view.kind==="buckets"){
    const seen=new Set();
    (view.buckets||[]).forEach(b=>{if(!seen.has(b.layer)){seen.add(b.layer);
      items.push({txt:b.layer,c:layerColor(view,b.layer)});}});}
  else if(view.kind==="simple"){
    const seen=new Set();
    (view.nodes||[]).forEach(n=>{if(!seen.has(n.type)){seen.add(n.type);
      items.push({txt:ETYPE_NAME[n.type]||n.type,c:ETYPE_COLR[n.type]||T.dim});}});}
  if(items.length<2)return null;
  return html`<div style=${{position:"absolute",top:10,right:14,zIndex:6,background:"#18181bf0",
    border:`1px solid ${T.borderR}`,borderRadius:8,padding:"7px 11px",
    display:"flex",flexDirection:"column",gap:4}}>
    ${items.map((it,i)=>html`<div key=${i} style=${{display:"flex",alignItems:"center",gap:7,fontSize:10.5,color:T.dim}}>
      <span style=${{width:10,height:10,borderRadius:3,background:it.c+"30",border:`1.5px solid ${it.c}`,flexShrink:0}}></span>
      ${it.txt}</div>`)}
  </div>`;}
function Footer({view,ov}){
  const groups=[];
  if(view.kind==="buckets"){
    // layer colors live in the canvas legend (where the eye is) — footer owns edge grammar only
    const ed=[{txt:"dependency · color = source layer",glyph:"━"},{txt:"bundled count",glyph:"×N"}];
    if((view.buckets||[]).some(b=>b.layer==="supporting"))ed.push({txt:"supporting / inferred",glyph:"┄"});
    groups.push({label:"EDGES",items:ed});
  } else if(view.kind==="table"||view.kind==="matrix"){
    groups.push({label:"DOCUMENT",items:[{txt:(view.rows?view.rows.length+" rows":((view.items||[]).length+"×"+(view.items||[]).length+" matrix")),glyph:"≣"}]});
  } else if(view.kind==="entity"){
    groups.push({label:"ELEMENTS",items:[{txt:"bucket table",c:T.green,ico:"store"}]});
    groups.push({label:"EDGES",items:[{txt:"contains",glyph:"━"}]});
  } else {
    const seen={};(view.nodes||[]).forEach(n=>{if(!seen[n.type])seen[n.type]=n.icon;});
    groups.push({label:"ELEMENTS",items:Object.keys(seen).map(t=>({txt:ETYPE_NAME[t]||t,c:ETYPE_COLR[t]||T.dim,ico:seen[t]}))});
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
    <span style=${{marginLeft:"auto",color:T.mute,flexShrink:0}}>generated ${MODEL.generated.timestamp} · always current</span>
  </div>`;}

// The model catalog — industry-standard families, each item with render status:
// live = derived from real data now · sim = simulated, sourceable · planned = gated on an extractor
// Per-system catalogs — each system's views stay contained within it
const CATALOGS={
 aoa:[
  {grp:"C4 Model",tag:"modern",items:[
    {id:"context",label:"System Context",status:"live"},
    {id:"container",label:"Container",status:"live"},
    {id:"component",label:"Component",status:"live"},
    {id:"deployment",label:"Deployment",status:"sim",note:"sourceable · CI/IaC"},
    {label:"Dynamic (sequence)",status:"planned",note:"needs call-edge resolution"},
    {label:"Code (L4)",status:"planned",note:"symbol table · not drawn by design"}]},
  {grp:"Flows & Behavior",items:[
    {id:"dataflow",label:"Data Flow (DFD)",status:"live"},
    {label:"Trust Boundaries (STRIDE)",status:"planned",note:"DFD overlay · rule-pack"},
    {label:"State Machine",status:"planned",note:"needs state extraction"}]},
  {grp:"Journeys",tag:"streaming",items:[
    {label:"Query path · A → B",status:"planned",note:"capture along the flow · call edges"},
    {label:"Indexing path · watch → persist",status:"planned",note:"capture along the flow"}]},
  {grp:"Data",items:[
    {id:"datamodel",label:"Data Model / ER",status:"live"},
    {label:"Glossary",status:"planned",note:"atlas seed + writer"}]},
  {grp:"Technology & Ops",items:[
    {label:"Technology Portfolio",status:"planned",note:"config scan"},
    {label:"SBOM (CycloneDX)",status:"planned",note:"document · manifests"}]},
  {grp:"Classical structure",items:[
    {id:"component",label:"Layered Architecture",status:"live",alias:true},
    {label:"Dependency Matrix (DSM)",status:"planned",note:"matrix renderer"},
    {label:"Cycle / Tangle Report",status:"planned",note:"SCC pass"}]}],
 graphify:[
  {grp:"C4 Model",tag:"modern",items:[
    {label:"System Context",status:"planned",note:"not yet derived for this system"},
    {label:"Container",status:"planned",note:"not yet derived for this system"},
    {id:"component",label:"Component",status:"live"},
    {label:"Deployment",status:"planned",note:"sourceable · CI config"}]},
  {grp:"Flows & Behavior",items:[
    {label:"Data Flow (DFD)",status:"planned",note:"not yet derived for this system"}]},
  {grp:"Data",items:[
    {label:"Data Model / ER",status:"planned",note:"not yet derived for this system"}]},
  {grp:"Classical structure",items:[
    {id:"component",label:"Layered Architecture",status:"live",alias:true},
    {label:"Cycle / Tangle Report",status:"planned",note:"SCC pass"}]}]};
const FIRST={aoa:"context",graphify:"component"};
const STATUS={live:{dot:"●",col:T.green,lbl:"derived live"},
              sim:{dot:"◌",col:T.yellow,lbl:"simulated · sourceable"},
              planned:{dot:"○",col:T.mute,lbl:"planned · extractor gated"}};

const ago=t=>{const s=(Date.now()-t)/1000;return s<5?"now":s<60?Math.floor(s)+"s ago":Math.floor(s/60)+"m ago";};
// narrateOne: converts a Finding to a plain-language headline per §4.2 patterns
function narrateOne(f){
  if(!f)return"";
  const msg=f.message||"";
  if(f.rule==="god"){
    const m=msg.match(/in (\d+).*out (\d+)/);
    const inN=m?m[1]:"?",outN=m?m[2]:"?";
    const subj=f.subjects&&f.subjects[0]?f.subjects[0].replace(/^[gu]_/,""):msg.split(":")[1]||"this package";
    return `${subj} is load-bearing — ${inN} packages lean on it and it reaches into ${outN}. Changes here ripple widest.`;}
  if(f.rule==="cycle"){
    const m=msg.match(/×(\d+)/);const cuts=m?m[1]:"?";
    const parts=msg.replace("dependency cycle: ","").split(" → ");
    if(parts.length>=2){const a=parts[0].replace(/^[gu]_/,""),b=parts[1].replace(/^[gu]_/,"");
      return `${a} and ${b} depend on each other — a cycle. Cheapest cut: the ${a} → ${b} edge (${f.cheapestCut||"unknown"}).`;}
    return msg;}
  if(f.rule==="dead-candidate"||f.rule==="dead"||f.rule==="orphan"){
    const subj=f.subjects&&f.subjects[0]?f.subjects[0].replace(/^[gu]_/,""):msg.split(":")[1]||"this package";
    return `${subj} looks dead — nothing imports it and search has never touched it. Removal candidate.`;}
  if(f.rule==="budget"){
    const subj=f.subjects&&f.subjects[0]?f.subjects[0].replace(/^[gu]_/,""):msg;
    return `${subj} exceeds the member budget. Consider splitting the group.`;}
  return msg;}
// FindingsDrawer: slide-over panel showing all daemon findings with narration + sources
function FindingsDrawer({findings,open,setOpen,expandedId,setExpandedId}){
  if(!open)return null;
  const bySev={error:[],warn:[],info:[]};
  findings.forEach(f=>{(bySev[f.severity]||bySev.warn).push(f);});
  const ruleCounts={};
  findings.forEach(f=>{ruleCounts[f.rule]=(ruleCounts[f.rule]||0)+1;});
  const ruleHeader=Object.entries(ruleCounts).sort((a,b)=>b[1]-a[1]).map(([r,n])=>r+" ×"+n).join(" · ");
  return html`<div style=${{position:"absolute",top:0,right:0,width:460,height:"100%",
    background:T.chrome,borderLeft:`1px solid ${T.border}`,zIndex:20,display:"flex",
    flexDirection:"column",boxShadow:"-8px 0 20px #0008"}}>
    <div style=${{padding:"12px 16px",borderBottom:`1px solid ${T.border}`,display:"flex",alignItems:"center",gap:8}}>
      <span style=${{fontSize:12,fontWeight:700,color:T.text}}>FINDINGS</span>
      <span style=${{fontSize:10,color:T.dim,flex:1}}>${ruleHeader}</span>
      <button onClick=${()=>setOpen(false)} style=${{background:"transparent",border:"none",
        color:T.mute,cursor:"pointer",fontSize:15}}>✕</button>
    </div>
    <div style=${{flex:1,overflowY:"auto",padding:"8px 0"}}>
      ${["error","warn","info"].flatMap(sev=>bySev[sev]).map((f,fi)=>{
        const expanded=expandedId===f.id;
        const headline=narrateOne(f);
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
        return html`<div key=${ix} class="vrow" onClick=${clickable?()=>go(it.id):null}
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
    return q.get("level")||firstView(e0,sc);});
  const[dirOv,setDirOv]=useState(q.get("dir")||null);
  const[den,setDen]=useState(q.get("density")||"compact");
  const[els,setEls]=useState(null);
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
  // Capsule expand state — persisted in localStorage (R5: stable base map)
  const[expandedGroups,setExpandedGroups]=useState(()=>{
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
  useEffect(()=>{let on=true;const d=SP[den];
    setEls(null);setSelRaw(null);setSelId(null);   // view change clears selection but PRESERVES dock expansion; remount canvas with the new layout
    const run=dd=>view.kind==="buckets"?layoutBuckets(view,dd,d,ov,{capsuleMode:true,expandedGroups}):layoutSimple(view,dd,d);
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
      if(on){setEls(e);setAutoDir(dd);setLast(l=>({...l,[estate+":"+scope+":"+level]:Date.now()}));}
    }catch(err){showFatal("VIEW LOAD FAILED · "+(err&&err.message||err));}})();
    return()=>{on=false;};},[estate,scope,level,dirOv,den,ov,expandedGroups]);
  // test hook: ?auto=<view>:<ms> simulates a user click after ms (verifies the CLICK path, not URL load)
  useEffect(()=>{const auto=q.get("auto");
    if(auto){const[lv,ms]=auto.split(":");const t=setTimeout(()=>setLevel(lv),parseInt(ms||"800",10));
      return()=>clearTimeout(t);}},[]);
  // The grammar: hover peeks, click SELECTS (ring + dock), the open ▸ chip drills, Esc unwinds.
  const[sel,setSelRaw]=useState(null);
  const[selId,setSelId]=useState(null);
  const[expanded,setExpanded]=useState(false);
  const mark=id=>setEls(e=>{if(!e||!e.nodes)return e;let ch=false;
    const nodes=e.nodes.map(n=>{const want=n.id===id;
      if(!!n.data._sel===want)return n;ch=true;return {...n,data:{...n.data,_sel:want}};});
    const edges=(e.edges||[]).map(ed=>{const want=ed.id===id;
      if(!!(ed.data&&ed.data._sel)===want)return ed;ch=true;return {...ed,data:{...ed.data,_sel:want}};});
    return ch?{...e,nodes,edges}:e;});
  const select=useCallback((rec,id)=>{setSelRaw(rec);setSelId(rec?id:null);
    if(rec)setExpanded(true);mark(rec?id:null);},[]);
  const clearSel=useCallback(()=>select(null,null),[select]);
  const nameOf=useCallback(id2=>{if(view.kind==="buckets"){const b=(view.buckets||[]).find(x=>x.id===id2);if(b)return b.label;}
    const n2=(view.nodes||[]).find(x=>x.id===id2);return n2?n2.label:id2;},[view]);
  const relationsFor=useCallback(id2=>((view.edges||[]).filter(e=>e.source===id2||e.target===id2)
    .map(e=>({dir:e.source===id2?"out":"in",peer:nameOf(e.source===id2?e.target:e.source),
      verb:e.label||"",count:e.count,viol:!!(e.tag||e._viol)}))
    .sort((a,b)=>(b.viol?1:0)-(a.viol?1:0))),[view,nameOf]);
  const onNodeClick=useCallback((_,n)=>{if(!n.data)return;
    if(n.data.drillTo){setLevel(n.data.drillTo);setDirOv(null);return;}
    if(n.type==="spacer")return;
    // Capsule toggle: click expands/collapses the group (R5: in-place, no global relayout)
    if(n.type==="capsule"&&isCapsuleView){toggleCapsule(n.id);return;}
    const d=n.data,m=(n.type==="solo"?d.member:d)||{};
    const rows=(m.stats?Object.entries(m.stats):[]).concat(m.sub?[["detail",m.sub]]:[]);
    if(d.layer||d.lay)rows.unshift(["layer",d.layer||d.lay]);
    if(d.tech)rows.push(["tech",d.tech]);
    if(m.concerns)rows.push(["findings",m.concerns+" recon findings"]);
    if(m.changed)rows.push(["recent","touched in last 15 commits"]);
    const chip=n.type==="bucket"?"group":n.type==="entity"?"entity":n.type==="member"?"member":(ETYPE_NAME[d.type]||"element");
    // AGENT row: B1 — paths not unit IDs (Facts() substring-matches paths, not u_... IDs)
    let agent=null;
    if(n.type==="bucket"||n.type==="solo"){
      const path=(d.path||(d.id||"").replace(/^g_/,"").replace(/_/g,"/"));
      agent={cmd:"aoa arch facts "+path,path};}
    else if(n.type==="member"&&m.id){
      const path=(m.path||m.id.replace(/^[gu]_/,"").replace(/_/g,"/"));
      agent={cmd:"aoa tree "+path+" -d 2",path};}
    select({label:m.label||d.label,chip,rows,relations:relationsFor(n.id),
      members:n.type==="bucket"?(d.members||[]):null,agent},n.id);},[select,relationsFor,isCapsuleView]);
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
  // pending selection: applied once the target view's elements are on screen
  // (seeded by ?sel= for screenshots; driven by journey steps at runtime)
  const[pendingSel,setPendingSel]=useState(q.get("sel"));
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
  const[findingsOpen,setFindingsOpen]=useState(false);
  const[findingsExpandedId,setFindingsExpandedId]=useState(null);
  useEffect(()=>{let on=true;
    fetch("/api/arch/findings").then(r=>r.ok?r.json():[]).then(d=>{if(on)setFindings(Array.isArray(d)?d:[]);}).catch(()=>{});
    return()=>{on=false;};},[]);
  const topFinding=findings.find(f=>f.severity==="error")||findings.find(f=>f.new)||findings[0]||null;
  const hasNewFindings=findings.some(f=>f.new);
  // manual navigation leaves journey mode — the journey owns scope/view only while followed
  const go=useCallback(id=>{setJr(null);setLevel(id);setDirOv(null);},[]);
  const goScope=useCallback(sid=>{setJr(null);setScope(sid);setLevel(firstView(estate,sid));setDirOv(null);},[estate]);
  const goEstate=useCallback(eid=>{setJr(null);setEstate(eid);const sc=firstScope(eid);
    setScope(sc);setLevel(firstView(eid,sc));setDirOv(null);},[]);
  const btn=a=>({background:a?T.cardH:"transparent",border:`1px solid ${a?T.blue:T.border}`,
    color:a?T.text:T.dim,borderRadius:7,padding:"5px 11px",fontSize:12,cursor:"pointer",fontWeight:550});
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
        ${view.prov?html`<span title=${view.prov.label} style=${(pk=>({fontSize:9,fontWeight:700,letterSpacing:.5,whiteSpace:"nowrap",cursor:"help",flexShrink:0,
          color:pk==="derived"?T.green:pk==="simulated"?T.yellow:T.cyan,
          border:`1px solid ${pk==="derived"?T.green:pk==="simulated"?T.yellow:T.cyan}`,
          borderRadius:5,padding:"1px 7px"}))(view.prov.kind)}>${view.prov.kind==="derived"?"REAL":view.prov.kind==="simulated"?"SIMULATED":"MIXED"}</span>`:null}
        ${els&&els.problems&&els.problems.length?html`<span title="Show findings"
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
      <span style=${{minWidth:0,whiteSpace:"nowrap",overflow:"hidden",textOverflow:"ellipsis"}}
        title=${view.count+(view.prov?" · "+view.prov.label:"")}>
        ${view.count}${view.prov?html`<span style=${{color:T.mute}}> · ${view.prov.label}</span>`:null}
      </span>
      <div style=${{marginLeft:"auto",display:"flex",gap:7,alignItems:"center",flexShrink:0}}>
        <button style=${btn(ov.concerns)} onClick=${()=>setOv({...ov,concerns:!ov.concerns})}
          title="Recon findings per package (bitmask)">⚠ Findings</button>
        <button style=${btn(ov.changed)} onClick=${()=>setOv({...ov,changed:!ov.changed})}
          title="Touched in last 15 commits (git)">Δ Changed</button>
        <span style=${{width:6}}></span>
        <button style=${btn(den==="compact")} onClick=${()=>setDen("compact")}>Compact</button>
        <button style=${btn(den==="comfort")} onClick=${()=>setDen("comfort")}>Comfort</button>
        <span style=${{width:6}}></span>
        <button style=${btn(!dirOv)} onClick=${()=>setDirOv(null)}
          title="Pick the direction that best fits the viewport">Auto${!dirOv&&autoDir?(autoDir==="DOWN"?" ↓":" →"):""}</button>
        <button style=${btn(dirOv==="DOWN")} onClick=${()=>setDirOv("DOWN")} title="Top–Bottom">↓</button>
        <button style=${btn(dirOv==="RIGHT")} onClick=${()=>setDirOv("RIGHT")} title="Left–Right">→</button>
      </div>
    </div>
    ${topFinding?html`<div onClick=${()=>setFindingsOpen(true)}
      style=${{padding:"4px 18px",borderBottom:`1px solid ${T.border}`,fontSize:11,
        background:hasNewFindings?"#fbbf2408":T.chrome,cursor:"pointer",
        display:"flex",alignItems:"center",gap:8,
        borderLeft:hasNewFindings?`2px solid ${T.yellow}`:"2px solid transparent"}}>
      <span style=${{fontSize:9.5,fontWeight:700,color:T.arch,letterSpacing:.5,flexShrink:0}}>FINDINGS</span>
      ${hasNewFindings?html`<span style=${{fontSize:8,fontWeight:700,color:T.yellow,border:`1px solid ${T.yellow}`,
        borderRadius:4,padding:"0 4px",flexShrink:0}}>NEW</span>`:null}
      <span style=${{flex:1,color:T.dim,whiteSpace:"nowrap",overflow:"hidden",textOverflow:"ellipsis"}}>${narrateOne(topFinding)}</span>
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
          ${els&&els.htmlView?html`<${view.kind==="table"?TableView:MatrixView} view=${view} onSel=${select} selId=${selId} vid=${level}/>`:null}
          ${els&&!els.htmlView?html`<${ReactFlow} key=${estate+"|"+scope+"|"+level+"|"+dir+"|"+den}
            nodes=${els.nodes} edges=${els.edges} nodeTypes=${nodeTypes} edgeTypes=${edgeTypes}
            onNodeClick=${onNodeClick} onEdgeClick=${onEdgeClick} onPaneClick=${onPaneClick}
            fitView fitViewOptions=${{padding:0.12}}
            minZoom=${0.1} proOptions=${{hideAttribution:true}}>
            <${Background} color=${T.border} gap=${24} size=${1}/>
            <${Controls} position="bottom-right"/>
          <//>`:null}
          ${els&&!els.htmlView&&(view._loaded||!view.shard)?html`<${CanvasLegend} view=${view}/>`:null}
        </div>
        <${BottomDock} vid=${level} view=${view} sel=${sel} clearSel=${clearSel}
          probs=${els&&els.problems||[]} expanded=${expanded} setExpanded=${setExpanded}/>
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
