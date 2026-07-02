#!/usr/bin/env python3
"""Architecture mockup v5 — DIAGRAM VARIETY on one rendering engine:
C4 trio (context/container/component) + Deployment topology (simulated/sourceable)
+ Data Flow (derived from pipeline) + Data Model (bbolt, real) + a FOREIGN repo
(graphify, real parsed Python imports) to prove generality.
Keeps: partitioning bands, verbatim ELK routes, label chips, compact/comfort,
LR/TB, title block + legend. elkjs 0.11.1."""
import json, subprocess, re, datetime, glob, os
from collections import Counter

ROOT, MOD = "/home/corey/aOa-go", "github.com/corey/aoa"
def run(c): return subprocess.run(c, cwd=ROOT, capture_output=True, text=True).stdout
def layer(p):
    for pre,l in [("/internal/domain","domain"),("/internal/ports","ports"),
                  ("/internal/adapters","adapters"),("/internal/app","app"),
                  ("/cmd","cmd"),("/atlas","atlas")]:
        if p.startswith(pre): return l
    return "supporting"
PART={"cmd":0,"app":1,"adapters":2,"domain":3,"ports":4,"atlas":4,"supporting":5}

# ---------- aOa component data (real, go list) ----------
raw=run(["go","list","-json","./internal/...","./cmd/...","./atlas/..."])
dec=json.JSONDecoder(); i=0; objs=[]
while i<len(raw):
    while i<len(raw) and raw[i] in " \n\t\r": i+=1
    if i>=len(raw): break
    o,j=dec.raw_decode(raw,i); objs.append(o); i=j
pkgs={}; edges=[]
for o in objs:
    ip=o.get("ImportPath","")
    if not ip.startswith(MOD): continue
    rel=ip[len(MOD):] or "/"; pkgs.setdefault(rel,{"layer":layer(rel),"fanin":0,"fanout":0})
    for imp in (o.get("Imports") or []):
        if imp.startswith(MOD):
            rt=imp[len(MOD):] or "/"; edges.append((rel,rt))
for a,b in edges:
    pkgs.setdefault(b,{"layer":layer(b),"fanin":0,"fanout":0})
    pkgs[a]["fanout"]+=1; pkgs[b]["fanin"]+=1
def short(p):
    s=p.lstrip("/"); return s[9:] if s.startswith("internal/") else s
# ---- overlays (REAL data): concerns = recon bitmask findings per package (dimensions
# proxy); changed = packages touched in the last 15 commits (git history) ----
concerns={}
try:
    with open(ROOT+"/playbook/data/arch_proxy.jsonl") as f:
        for line in f:
            d=json.loads(line); p=d.get("path","")
            if not p.endswith(".go"): continue
            pk="/"+os.path.dirname(p)
            n=len(d.get("findings") or [])
            if n: concerns[pk]=concerns.get(pk,0)+n
except FileNotFoundError: pass
changed=set()
for ln in run(["git","log","-15","--name-only","--pretty=format:"]).splitlines():
    ln=ln.strip()
    if ln.endswith(".go"): changed.add("/"+os.path.dirname(ln))
print(f"overlays: concern pkgs={len(concerns)} changed pkgs={len(changed&set(pkgs))}")

LAYERS=["cmd","app","adapters","domain","ports","atlas","supporting"]
aoa_buckets=[]
for l in LAYERS:
    mem=[{"id":"m_"+re.sub(r'[^A-Za-z0-9]','_',p.strip('/')),
          "label":short(p).split("/")[-1],"sub":"in "+str(m["fanin"]),
          "concerns":concerns.get(p,0),"changed":p in changed}
         for p,m in sorted(pkgs.items(),key=lambda kv:(-kv[1]["fanin"],kv[0])) if m["layer"]==l]
    if mem: aoa_buckets.append({"id":"b_"+l,"layer":l,"label":l,"part":PART[l],"members":mem})
agg=Counter()
for a,b in edges:
    la,lb=pkgs[a]["layer"],pkgs[b]["layer"]
    if la!=lb: agg[(la,lb)]+=1
aoa_edges=[{"id":f"a_{la}_{lb}","source":"b_"+la,"target":"b_"+lb,"count":c}
           for (la,lb),c in sorted(agg.items())]

# ---------- graphify component data (real, parsed Python imports; grouping inferred) ----------
GF_GROUPS={
 "cli":["__main__"],
 "serve":["serve","querylog","watch","hooks","benchmark","diagnostics"],
 "ingest":["ingest","scip_ingest","mcp_ingest","transcribe","google_workspace","pg_introspect","prs","wiki"],
 "pipeline":["detect","extract","build","cluster","analyze","report","export","dedup",
             "semantic_cleanup","symbol_resolution","global_graph","multigraph_compat","affected"],
 "render":["tree_html","callflow_html"],
 "infra":["cache","security","validate","manifest","llm"],
}
GF_PART={"cli":0,"serve":1,"ingest":2,"pipeline":3,"render":4,"infra":5,"supporting":6}
def gf_group(m):
    for g,ms in GF_GROUPS.items():
        if m in ms: return g
    return "supporting"
gf_mods=set(); gf_raw=Counter()
for f in sorted(glob.glob(ROOT+"/repo/graphify/*.py")):
    m=os.path.basename(f)[:-3]
    if m=="__init__": continue
    gf_mods.add(m)
for f in sorted(glob.glob(ROOT+"/repo/graphify/*.py")):
    m=os.path.basename(f)[:-3]
    if m=="__init__": continue
    src=open(f,encoding="utf-8",errors="replace").read()
    tgts=set(re.findall(r'^\s*from\s+(?:graphify\.|\.)(\w+)\s+import',src,re.M))
    for blob in re.findall(r'^\s*from\s+graphify\s+import\s+([\w,\s]+)',src,re.M):
        tgts.update(x for x in re.split(r'[,\s]+',blob) if x)
    for t in tgts:
        if t in gf_mods and t!=m: gf_raw[(m,t)]+=1
gf_fanin=Counter(); gf_fanout=Counter()
for (a,b),_ in gf_raw.items(): gf_fanout[a]+=1; gf_fanin[b]+=1
gf_buckets=[]
for g in ["cli","serve","ingest","pipeline","render","infra","supporting"]:
    mem=[{"id":"g_"+m,"label":m,"sub":"in "+str(gf_fanin[m])}
         for m in sorted(gf_mods,key=lambda x:(-gf_fanin[x],x)) if gf_group(m)==g]
    if mem: gf_buckets.append({"id":"gb_"+g,"layer":g,"label":g,"part":GF_PART[g],"members":mem})
gf_agg=Counter()
for (a,b),_ in gf_raw.items():
    ga,gb=gf_group(a),gf_group(b)
    if ga!=gb: gf_agg[(ga,gb)]+=1
gf_edges=[{"id":f"ge_{a}_{b}","source":"gb_"+a,"target":"gb_"+b,"count":c}
          for (a,b),c in sorted(gf_agg.items())]
gf_total=sum(gf_raw.values())
print(f"graphify: modules={len(gf_mods)} raw_edges={gf_total} bundled={len(gf_edges)}")

# ---------- C4 context & container (as before) ----------
context_nodes=[
 {"id":"aoa","type":"sys","icon":"sys","label":"aOa","sub":"Code-intelligence engine","drillTo":"container","real":True},
 {"id":"logs","type":"ext","icon":"ext","label":"Claude Session Logs","sub":"[sourceable · config]","real":False},
 {"id":"repo","type":"ext","icon":"ext","label":"Source Repository","sub":"[sourceable · fs]","real":False},
 {"id":"grammars","type":"ext","icon":"ext","label":"Tree-sitter Grammars","sub":"[sourceable · .so]","real":False}]
context_edges=[{"id":"x1","source":"logs","target":"aoa","label":"tails"},
 {"id":"x2","source":"repo","target":"aoa","label":"parses"},
 {"id":"x3","source":"aoa","target":"grammars","label":"loads"}]
container_nodes=[
 {"id":"bin","type":"container","icon":"container","label":"aoa","sub":"[Go · single binary]","drillTo":"component","real":True},
 {"id":"db","type":"store","icon":"store","label":"bbolt","sub":"[embedded KV]","drillTo":"datamodel","real":True},
 {"id":"logs2","type":"ext","icon":"ext","label":"Session Logs","sub":"[sourceable]","real":False},
 {"id":"repo2","type":"ext","icon":"ext","label":"Repository","sub":"[28 languages]","real":False}]
container_edges=[{"id":"k1","source":"bin","target":"db","label":"reads/writes"},
 {"id":"k2","source":"logs2","target":"bin","label":"tails"},
 {"id":"k3","source":"repo2","target":"bin","label":"parses"}]

# ---------- Data Flow (derived from documented pipeline; flow labels) ----------
dfd_nodes=[
 {"id":"jsonl","type":"ext","icon":"ext","label":"Claude JSONL","sub":"~/.claude/projects/*.jsonl","real":True},
 {"id":"tailer","type":"proc","icon":"app","label":"tailer","sub":"defensive JSONL tail","real":True},
 {"id":"parser","type":"proc","icon":"app","label":"parser","sub":"raw → events","real":True},
 {"id":"reader","type":"proc","icon":"app","label":"claude.Reader","sub":"canonical events","real":True},
 {"id":"app","type":"proc","icon":"app","label":"app.onSessionEvent","sub":"range gate · file_hits","real":True},
 {"id":"learner","type":"proc","icon":"domain","label":"learner.observe()","sub":"bigrams · autotune","real":True},
 {"id":"db2","type":"store","icon":"store","label":"aoa.db","sub":"bbolt · project-scoped","real":True},
 {"id":"status","type":"store","icon":"store","label":"status.json","sub":".aoa/status.json","real":True},
 {"id":"dash","type":"proc","icon":"app","label":"dashboard","sub":"localhost · 5 tabs","real":True},
 {"id":"agent","type":"ext","icon":"ext","label":"AI agent / CLI","sub":"grep · egrep · peek","real":True}]
dfd_edges=[
 {"id":"f1","source":"jsonl","target":"tailer","label":"raw lines"},
 {"id":"f2","source":"tailer","target":"parser","label":"JSONL"},
 {"id":"f3","source":"parser","target":"reader","label":"raw events"},
 {"id":"f4","source":"reader","target":"app","label":"SessionEvent"},
 {"id":"f5","source":"app","target":"learner","label":"observe()"},
 {"id":"f6","source":"learner","target":"db2","label":"state"},
 {"id":"f7","source":"app","target":"status","label":"status line"},
 {"id":"f8","source":"db2","target":"dash","label":"metrics"},
 {"id":"f9","source":"agent","target":"app","label":"search"},
 {"id":"f10","source":"app","target":"agent","label":"results <1ms"}]

# ---------- Data Model (bbolt buckets, real per store.go) ----------
dm_nodes=[
 {"id":"proj","type":"entity","label":"project","tech":"bucket · keyed by projectID",
  "fields":["projectID  (root key)"],"real":True},
 {"id":"index","type":"entity","label":"index","tech":"bucket · format v1",
  "fields":["_version  byte","tokens  posting lists (bin)","metadata  SymbolMeta (gob)","files  FileMeta (gob)"],"real":True},
 {"id":"learnerB","type":"entity","label":"learner","tech":"bucket",
  "fields":["state  LearnerState (json)"],"real":True},
 {"id":"sessions","type":"entity","label":"sessions","tech":"bucket",
  "fields":["sessionID → metrics (json)"],"real":True},
 {"id":"dims","type":"entity","label":"dimensions","tech":"bucket",
  "fields":["path → FileAnalysis (json)","  bitmask [6]uint64","  methods[] + findings[]"],"real":True},
 {"id":"telem","type":"entity","label":"telemetry","tech":"bucket",
  "fields":["project rollup (json)","lifetime tokens/savings"],"real":True}]
dm_edges=[
 {"id":"d1","source":"proj","target":"index","label":"contains"},
 {"id":"d2","source":"proj","target":"learnerB","label":"contains"},
 {"id":"d3","source":"proj","target":"sessions","label":"contains"},
 {"id":"d4","source":"proj","target":"dims","label":"contains"},
 {"id":"d5","source":"proj","target":"telem","label":"contains"}]

# ---------- Deployment topology (simulated · sourceable from CI/IaC config) ----------
dep_buckets=[
 {"id":"db_dev","layer":"dev","label":"developer machine","part":0,"members":[
   {"id":"p_src","label":"source repo","sub":"git"},
   {"id":"p_build","label":"build.sh","sub":"guarded"},
   {"id":"p_tag","label":"git tag","sub":"vX.Y.Z"}]},
 {"id":"db_gh","layer":"ci","label":"github actions","part":1,"members":[
   {"id":"p_ci","label":"CI vet+test","sub":"20m"},
   {"id":"p_rel","label":"goreleaser","sub":"matrix"},
   {"id":"p_npm","label":"npm publish","sub":"on tag"}]},
 {"id":"db_reg","layer":"registry","label":"registries","part":2,"members":[
   {"id":"p_ghr","label":"gh releases","sub":"binaries"},
   {"id":"p_npmr","label":"@mvpscale/aoa","sub":"npm"}]},
 {"id":"db_user","layer":"user","label":"user machine","part":3,"members":[
   {"id":"p_bin","label":"aoa binary","sub":"~/.local/bin"},
   {"id":"p_daemon","label":"daemon","sub":"socket+http"},
   {"id":"p_aoa","label":".aoa/","sub":"db · grammars"},
   {"id":"p_dash","label":"dashboard","sub":"localhost"}]}]
dep_edges=[
 {"id":"dp1","source":"db_dev","target":"db_gh","count":1,"label":"push tag"},
 {"id":"dp2","source":"db_gh","target":"db_reg","count":2,"label":"publish"},
 {"id":"dp3","source":"db_reg","target":"db_user","count":2,"label":"install"}]
DEP_PARTC={"dev":"#60a5fa","ci":"#c084fc","registry":"#fb923c","user":"#34d399"}

GEN=datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%d %H:%M UTC")
# provenance per view — the fake-vs-real contract, displayed in the header
PROV_REAL={"kind":"derived","label":"REAL · derived from code"}
PROV_MIX={"kind":"mixed","label":"MIXED · system real · externals inferred"}
PROV_SIM={"kind":"simulated","label":"SIMULATED · sourceable, not yet extracted"}
# ---------- SIMULATED ESTATES (capability demos; every view marked SIMULATED with
# the source it WOULD be collected from) ----------
def sim(what,src): return {"kind":"simulated","label":f"SIMULATED · would derive from: {src}"}

mono_domains_buckets=[
 {"id":"rb_order","layer":"order","label":"order","part":0,"ico":"container","members":[
   {"id":"ro1","label":"pricing","sub":"≈2,100"},{"id":"ro2","label":"checkout","sub":"≈1,800"},
   {"id":"ro3","label":"fulfillment","sub":"≈3,200"},{"id":"ro4","label":"returns","sub":"≈900"}]},
 {"id":"rb_inv","layer":"inventory","label":"inventory","part":0,"ico":"store","members":[
   {"id":"ri1","label":"stock","sub":"≈2,400"},{"id":"ri2","label":"warehouse","sub":"≈1,600"},
   {"id":"ri3","label":"replenishment","sub":"≈1,100"}]},
 {"id":"rb_cust","layer":"customer","label":"customer","part":0,"ico":"user","members":[
   {"id":"rc1","label":"profiles","sub":"≈1,900"},{"id":"rc2","label":"auth","sub":"≈1,400"},
   {"id":"rc3","label":"loyalty","sub":"≈800"}]},
 {"id":"rb_shared","layer":"shared","label":"shared kernel","part":1,"ico":"infra","members":[
   {"id":"rs1","label":"persistence","sub":"≈2,200"},{"id":"rs2","label":"messaging","sub":"≈1,300"},
   {"id":"rs3","label":"utils","sub":"≈3,000"}]}]
mono_domains_edges=[
 {"id":"re1","source":"rb_order","target":"rb_shared","count":212},
 {"id":"re2","source":"rb_inv","target":"rb_shared","count":167},
 {"id":"re3","source":"rb_cust","target":"rb_shared","count":143},
 {"id":"re4","source":"rb_order","target":"rb_inv","count":38},
 {"id":"re5","source":"rb_order","target":"rb_cust","count":21},
 {"id":"re6","source":"rb_inv","target":"rb_cust","count":9}]
mono_context_nodes=[
 {"id":"mono","type":"sys","icon":"sys","label":"Retail Monolith","sub":"Java · ≈480k LOC","real":False},
 {"id":"pos","type":"ext","icon":"ext","label":"POS Terminals","sub":"stores · 2,400 sites","real":False},
 {"id":"pay","type":"ext","icon":"ext","label":"Payment Gateway","sub":"PCI boundary","real":False},
 {"id":"wms","type":"ext","icon":"ext","label":"Warehouse Mgmt","sub":"3PL EDI","real":False}]
mono_context_edges=[{"id":"me1","source":"pos","target":"mono","label":"sells"},
 {"id":"me2","source":"mono","target":"pay","label":"charges"},
 {"id":"me3","source":"mono","target":"wms","label":"ships"}]
mono_dm_nodes=[
 {"id":"t_ord","type":"entity","label":"orders","tech":"table","fields":["id PK","customer_id FK","status","total"],"real":False},
 {"id":"t_oi","type":"entity","label":"order_items","tech":"table","fields":["order_id FK","sku","qty","price"],"real":False},
 {"id":"t_inv","type":"entity","label":"inventory","tech":"table","fields":["sku PK","qty_on_hand","warehouse_id FK"],"real":False},
 {"id":"t_cust","type":"entity","label":"customers","tech":"table","fields":["id PK","email","tier"],"real":False}]
mono_dm_edges=[{"id":"md1","source":"t_ord","target":"t_oi","label":"1:N"},
 {"id":"md2","source":"t_cust","target":"t_ord","label":"1:N"},
 {"id":"md3","source":"t_oi","target":"t_inv","label":"sku"}]

cf_container_nodes=[
 {"id":"cfb","type":"ext","icon":"ext","label":"Browser","sub":"global users","real":False},
 {"id":"cfw1","type":"container","icon":"container","label":"api-worker","sub":"[Worker · TS]","real":False},
 {"id":"cfw2","type":"container","icon":"container","label":"auth-worker","sub":"[Worker · TS]","real":False},
 {"id":"cfkv","type":"store","icon":"store","label":"KV","sub":"[sessions]","real":False},
 {"id":"cfd1","type":"store","icon":"store","label":"D1","sub":"[sqlite]","real":False},
 {"id":"cfq","type":"store","icon":"store","label":"Queues","sub":"[events]","real":False},
 {"id":"cfo","type":"ext","icon":"ext","label":"Origin API","sub":"[Node · fallback]","real":False}]
cf_container_edges=[
 {"id":"cf1","source":"cfb","target":"cfw1","label":"https"},
 {"id":"cf2","source":"cfw1","target":"cfw2","label":"service binding"},
 {"id":"cf3","source":"cfw2","target":"cfkv","label":"sessions"},
 {"id":"cf4","source":"cfw1","target":"cfd1","label":"sql"},
 {"id":"cf5","source":"cfw1","target":"cfq","label":"enqueue"},
 {"id":"cf6","source":"cfw1","target":"cfo","label":"fallback"}]

mc_dep_buckets=[
 {"id":"mcb_aws","layer":"aws","label":"aws · us-east-1","part":0,"ico":"sys","members":[
   {"id":"ma1","label":"eks-cluster","sub":"42 pods"},{"id":"ma2","label":"rds-postgres","sub":"primary"},
   {"id":"ma3","label":"s3-assets","sub":"cdn"},{"id":"ma4","label":"lambda-jobs","sub":"12 fns"}]},
 {"id":"mcb_gcp","layer":"gcp","label":"gcp · europe-west1","part":0,"ico":"sys","members":[
   {"id":"mg1","label":"gke-cluster","sub":"28 pods"},{"id":"mg2","label":"bigquery","sub":"analytics"},
   {"id":"mg3","label":"pubsub","sub":"events"}]},
 {"id":"mcb_sh","layer":"shared","label":"shared control plane","part":1,"ico":"infra","members":[
   {"id":"ms1","label":"terraform","sub":"IaC"},{"id":"ms2","label":"vault","sub":"secrets"},
   {"id":"ms3","label":"gh-actions","sub":"CI/CD"}]}]
mc_dep_edges=[
 {"id":"mc1","source":"mcb_aws","target":"mcb_sh","count":3},
 {"id":"mc2","source":"mcb_gcp","target":"mcb_sh","count":3},
 {"id":"mc3","source":"mcb_aws","target":"mcb_gcp","count":2}]

# estates: LOCAL = real systems derived from this workspace (sidebar);
# simulated estates = header-selector capability demos, fully contained
MODEL={
 "schema":"aoa.archmodel/v1-mock",
 "generated":{"tool":"build_c4_mockup.py","timestamp":GEN,
              "inputs":["go list","repo/graphify/*.py imports","store.go","arch_proxy.jsonl","git log"]},
 "estates":{
  "local":{"label":"Local · this workspace","sim":False,"scopes":{
   "aoa":{"label":"aOa","tech":"Go · single binary","views":{
    "context":{"kind":"simple","title":"System Context","count":"4 elements","dir":"DOWN",
              "prov":PROV_MIX,"nodes":context_nodes,"edges":context_edges},
    "container":{"kind":"simple","title":"Container diagram","count":"4 containers","dir":"DOWN",
              "prov":PROV_MIX,"nodes":container_nodes,"edges":container_edges},
    "component":{"kind":"buckets","title":"Component diagram","dir":"DOWN",
              "count":f"{len(pkgs)} packages · {len(edges)} dependencies",
              "prov":PROV_REAL,"buckets":aoa_buckets,"edges":aoa_edges},
    "dataflow":{"kind":"simple","title":"Data Flow — signal pipeline","dir":"RIGHT",
              "count":"10 elements","prov":PROV_REAL,"nodes":dfd_nodes,"edges":dfd_edges},
    "datamodel":{"kind":"entity","title":"Data Model — aoa.db (bbolt)","dir":"DOWN",
              "count":"6 buckets · from store.go","prov":PROV_REAL,"nodes":dm_nodes,"edges":dm_edges},
    "deployment":{"kind":"buckets","title":"Deployment — build → release → install","dir":"RIGHT",
              "count":"4 environments","prov":PROV_SIM,
              "buckets":dep_buckets,"edges":dep_edges,"palette":"dep","labeled":True}}},
   "graphify":{"label":"graphify","tech":"Python · CLI","views":{
    "component":{"kind":"buckets","title":"Component diagram","dir":"DOWN",
              "count":f"{len(gf_mods)} modules · {gf_total} imports",
              "prov":{"kind":"derived","label":"REAL · imports parsed · grouping inferred"},
              "buckets":gf_buckets,"edges":gf_edges,"palette":"gf"}}}}},
  "monolith":{"label":"Retail Monolith","sim":True,"scopes":{
   "mono":{"label":"Retail Monolith","tech":"Java · ≈480k LOC","views":{
    "context":{"kind":"simple","title":"System Context","count":"4 elements","dir":"DOWN",
              "prov":sim("system","EDI/API configs + reflexion"),"nodes":mono_context_nodes,"edges":mono_context_edges},
    "domains":{"kind":"buckets","title":"Domain map","dir":"DOWN",
              "count":"≈14,800 elements · 3 domains + shared kernel",
              "prov":sim("domains","package roots + reflexion mapping"),
              "buckets":mono_domains_buckets,"edges":mono_domains_edges,"palette":"retail"},
    "datamodel":{"kind":"entity","title":"Data Model — core tables","dir":"DOWN",
              "count":"4 of ≈310 tables","prov":sim("schema","DB migrations / DDL"),
              "nodes":mono_dm_nodes,"edges":mono_dm_edges}}}}},
  "cloudflare":{"label":"Cloudflare Edge","sim":True,"scopes":{
   "edge":{"label":"Edge","tech":"Workers · TypeScript","views":{
    "container":{"kind":"simple","title":"Container diagram","count":"7 elements","dir":"DOWN",
              "prov":sim("bindings","wrangler.toml + service bindings"),
              "nodes":cf_container_nodes,"edges":cf_container_edges}}}}},
  "multicloud":{"label":"Multi-Cloud","sim":True,"scopes":{
   "platform":{"label":"Platform","tech":"AWS + GCP","views":{
    "deployment":{"kind":"buckets","title":"Deployment topology","dir":"DOWN",
              "count":"2 clouds + shared control plane",
              "prov":sim("topology","terraform state + cloud APIs"),
              "buckets":mc_dep_buckets,"edges":mc_dep_edges,"palette":"mc","labeled":True}}}}}}}
# ---- fold campaign estate fixtures into the main model (dropdown access) ----
# the 3 hand-coded sims are superseded by the richer campaign versions
for k in ["monolith","cloudflare","multicloud"]: MODEL["estates"].pop(k,None)
for base in sorted({os.path.basename(f)[:-5].rsplit("-",1)[0]
                    for f in glob.glob("playbook/mockups/estates/*-clean.json")
                    if not os.path.basename(f).startswith("smoke")}):
    for variant in ("clean","faulted"):
        f=f"playbook/mockups/estates/{base}-{variant}.json"
        if not os.path.exists(f): continue
        try: data=json.load(open(f))
        except Exception as ex:
            print(f"skip {f}: {ex}"); continue
        for eid,ev in (data.get("estates") or {}).items():
            ev=dict(ev); ev["sim"]=True
            ev["label"]=ev.get("label",eid)+(" ⚠ faulted" if variant=="faulted" else " · clean")
            MODEL["estates"][f"{base}-{variant}"]=ev
# toy retail-monolith is superseded by the Hartwell enterprise estate;
# fixture files stay on disk as test fixtures but leave the dropdown
for variant in ("clean","faulted"): MODEL["estates"].pop(f"retail-monolith-{variant}",None)
print(f"estates in dropdown: {len(MODEL['estates'])}")

# ---- journeys: estate-level flow artifacts (sidecar per estate base name) ----
# a journey is a stepped walkthrough across existing views; step anchors reference
# ids that exist in BOTH variants. This contract is the TARGET the future facts
# substrate backs into — authored/AI-generated today, derived tomorrow.
for jf in glob.glob("playbook/mockups/estates/journeys/*.json"):
    jbase=os.path.basename(jf)[:-5]
    try: jdata=json.load(open(jf))
    except Exception as ex:
        print(f"skip journeys {jf}: {ex}"); continue
    for variant in ("clean","faulted"):
        jeid=f"{jbase}-{variant}"
        if jeid in MODEL["estates"]:
            MODEL["estates"][jeid]["journeys"]=jdata.get("journeys",[])
_nj=sum(len(ev.get("journeys",[])) for ev in MODEL["estates"].values())
if _nj: print(f"journeys folded: {_nj}")

# ---- decomposed contract: tiny manifest + one shard per architectural document ----
import hashlib, shutil
OUT="playbook/mockups/archmodel"
shutil.rmtree(OUT,ignore_errors=True)
manifest={"schema":"aoa.archmodel/v1-mock","sharded":True,
          "generated":MODEL["generated"],"estates":{}}
n_shards=0; biggest=(0,"")
for eid,ev in MODEL["estates"].items():
    me={"label":ev["label"],"sim":ev["sim"],"scopes":{}}
    for sid,sv in ev["scopes"].items():
        ms={"label":sv["label"],"tech":sv["tech"],"views":{}}
        for vid,v in sv["views"].items():
            payload=json.dumps(v,separators=(",",":"),ensure_ascii=False).encode()
            h=hashlib.sha256(payload).hexdigest()[:12]
            rel=f"{eid}/{sid}/{vid}.json"
            os.makedirs(f"{OUT}/{eid}/{sid}",exist_ok=True)
            open(f"{OUT}/{rel}","wb").write(payload)
            n_shards+=1
            if len(payload)>biggest[0]: biggest=(len(payload),rel)
            # manifest carries only the summary the sidebar/catalog needs + the shard ref
            ms["views"][vid]={"kind":v["kind"],"title":v["title"],"count":v["count"],
                "dir":v.get("dir"),"prov":v.get("prov"),
                "shard":{"path":rel,"hash":h,"bytes":len(payload)}}
        me["scopes"][sid]=ms
    if ev.get("journeys"):
        me["journeys"]=[]
        for j in ev["journeys"]:
            payload=json.dumps(j,separators=(",",":"),ensure_ascii=False).encode()
            rel=f"{eid}/journeys/{j['id']}.json"
            os.makedirs(f"{OUT}/{eid}/journeys",exist_ok=True)
            open(f"{OUT}/{rel}","wb").write(payload)
            n_shards+=1
            me["journeys"].append({"id":j["id"],"label":j["label"],"kind":j.get("kind"),
                "steps":len(j.get("steps",[])),"prov":j.get("prov"),"shard":{"path":rel}})
    manifest["estates"][eid]=me
mbytes=open(f"{OUT}/manifest.json","w").write(json.dumps(manifest,indent=1,ensure_ascii=False))
print(f"contract: manifest {mbytes/1024:.1f}KB + {n_shards} shards (largest {biggest[0]/1024:.1f}KB {biggest[1]})")

# Viewer JS is the canonical source in internal/adapters/web/static/arch/viewer.js
# This generator is a consumer of that file, not its owner.
# After reading, __VIEW_INTENT__ is injected at build time (V2 will move this to runtime fetch).
JS=open("internal/adapters/web/static/arch/viewer.js").read()
HTML="""<!doctype html><html><head><meta charset="utf-8"><title>aOa — Architecture</title>
<link rel="stylesheet" href="https://esm.sh/@xyflow/react@12.3.5/dist/style.css">
<style>html,body,#root{margin:0;height:100%;background:#0a0a0c}
.vrow .ago{display:none}.vrow:hover .ago{display:inline}
.react-flow__controls-button{background:#161618!important;border-color:#252528!important;fill:#8b8b96!important}
/* raise hovered LEAF nodes only — a raised bucket would paint over its own members */
.react-flow__node-member:hover,.react-flow__node-box:hover,
.react-flow__node-solo:hover,.react-flow__node-entity:hover{z-index:1200!important}
.hv{position:relative}
.hv .hovercard{visibility:hidden;transition:visibility 0s linear 0s;position:absolute;left:0;top:calc(100% + 7px);
 min-width:200px;max-width:300px;background:#18181b;border:1px solid #34343a;border-radius:8px;padding:9px 12px;
 z-index:1300;box-shadow:0 10px 28px #000c;pointer-events:none;white-space:normal;text-transform:none;letter-spacing:0}
.hv:hover .hovercard{visibility:visible;transition-delay:.15s}  /* hover-intent: delayed open, instant dismiss */
.hv.force .hovercard{visibility:visible;transition-delay:0s}
.react-flow__edge{cursor:pointer}
.react-flow__edge:hover .react-flow__edge-path{stroke-width:3!important;opacity:1!important}</style>
</head><body><div id="root"></div><script type="module">__JS__</script></body></html>"""
# build-time injection: view intent (question/vital/hover) from the standards file
_std=json.load(open("playbook/standards/view-standards.json"))
_intent={vid:{"question":v["question"],"vital":v["vital"],"hover":v["hover"],"pass":v["pass"]}
         for vid,v in _std["views"].items()}
JS=JS.replace("__VIEW_INTENT__",json.dumps(_intent,ensure_ascii=False))
open("playbook/mockups/architecture-c4.html","w").write(HTML.replace("__JS__",JS))
print("wrote playbook/mockups/architecture-c4.html")
