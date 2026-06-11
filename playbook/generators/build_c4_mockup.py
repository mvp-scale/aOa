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
    manifest["estates"][eid]=me
mbytes=open(f"{OUT}/manifest.json","w").write(json.dumps(manifest,indent=1,ensure_ascii=False))
print(f"contract: manifest {mbytes/1024:.1f}KB + {n_shards} shards (largest {biggest[0]/1024:.1f}KB {biggest[1]})")

JS=r"""
import React,{useState,useEffect,useCallback,memo} from "https://esm.sh/react@18.3.1";
import {createRoot} from "https://esm.sh/react-dom@18.3.1/client";
import {ReactFlow,Background,Controls,Handle,Position,BaseEdge,EdgeLabelRenderer,useReactFlow,ReactFlowProvider}
 from "https://esm.sh/@xyflow/react@12.3.5?deps=react@18.3.1,react-dom@18.3.1";
import ELK from "https://esm.sh/elkjs@0.11.1/lib/elk.bundled.js";
import htm from "https://esm.sh/htm@3.1.1";
const html=htm.bind(React.createElement); const elk=new ELK();
// the contract file IS the data source — anything that emits a valid archmodel gets every view
const MQ=new URLSearchParams(location.search);
function showFatal(msg){let d=document.getElementById("fatal");if(!d){d=document.createElement("div");d.id="fatal";
  d.style.cssText="position:fixed;top:0;left:0;right:0;z-index:9999;background:#7f1d1d;color:#fff;font:600 12px ui-monospace,monospace;padding:8px 14px;border-bottom:2px solid #f87171";
  document.body.appendChild(d);}if(!d._set){d._set=true;d.textContent="RENDER FAILURE · "+msg;}}
window.addEventListener("error",ev=>showFatal(ev.message));
window.addEventListener("unhandledrejection",ev=>showFatal(String(ev.reason&&ev.reason.message||ev.reason)));
const MODEL_PATH=MQ.get("model")||"archmodel/manifest.json";
const BASE=MODEL_PATH.includes("/")?MODEL_PATH.slice(0,MODEL_PATH.lastIndexOf("/")+1):"";
let MODEL;
try{MODEL=await fetch(MODEL_PATH).then(r=>{
  if(!r.ok)throw new Error("HTTP "+r.status+" loading "+MODEL_PATH);
  return r.json();});}
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
const T={bg:"#0c0c0e",card:"#161618",cardH:"#1c1c1f",band:"#121215",border:"#252528",text:"#e8e8ec",
 dim:"#8b8b96",mute:"#55555f",green:"#34d399",blue:"#60a5fa",purple:"#c084fc",cyan:"#22d3ee",
 yellow:"#fbbf24",red:"#f87171",arch:"#fb923c",neutral:"#94a3b8"};
const PALETTES={
 aoa:{cmd:T.purple,app:T.blue,adapters:T.arch,domain:T.green,ports:T.red,atlas:T.cyan,supporting:T.neutral},
 gf:{cli:T.purple,serve:T.blue,ingest:T.cyan,pipeline:T.green,render:T.yellow,infra:T.arch,supporting:T.neutral},
 dep:{dev:T.blue,ci:T.purple,registry:T.arch,user:T.green},
 retail:{order:T.arch,inventory:T.cyan,customer:T.purple,shared:T.neutral},
 mc:{aws:T.yellow,gcp:T.blue,shared:T.neutral}};
const ESTATES=MODEL.estates;
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
   {vid:["code"],label:"Code (L4)",note:"symbol table"}]},
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
// per-view intent from playbook/standards/view-standards.json (injected at build time):
// the question each view answers, what is canvas-vital, what stays hover-tier
const VIEW_INTENT=__VIEW_INTENT__;
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
function TableView({view,onSel,selId}){
  // row click = select: the dock shows the full record (long prose cells live there untruncated)
  const pick=(r,ri)=>onSel&&onSel({label:String(r[0]),chip:"record",
    rows:(view.columns||[]).map((c,ci)=>[c,r[ci]]),relations:[]},"row:"+ri);
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
function labelSpacers(laidById){const sp=[];let i=0;
  Object.values(laidById).forEach(le=>{(le.labels||[]).forEach(l=>{
    if(l.x===undefined)return;
    sp.push({id:"_lsp"+(i++),type:"spacer",position:{x:l.x,y:l.y},
      width:l.width||40,height:l.height||16,draggable:false,selectable:false,data:{}});});});
  return sp;}
const nodeTypes={box:BoxNode,bucket:BucketNode,solo:SoloNode,member:MemberNode,entity:EntityNode,spacer:SpacerNode};
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
      labels:[{text:e.label,width:lblW(e.label),height:16}]}))};
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
async function layoutBuckets(view,dir,d,ov){
  ov=ov||{};
  const B=view.buckets;
  const colOf=l=>layerColor(view,l);
  const problems=[];const deg={};B.forEach(b=>deg[b.id]={i:0,o:0});
  const bById={};B.forEach(b=>bById[b.id]=b);
  view.edges.forEach(e=>{deg[e.source].o++;deg[e.target].i++;
    const sp=bById[e.source],tp=bById[e.target];
    if(sp&&tp&&sp.part>tp.part){e._viol=true;problems.push("band violation: "+sp.label+" → "+tp.label);}
    if(e.tag){e._viol=true;problems.push(e.tag+": "+sp.label+" → "+tp.label);}});
  B.forEach(b=>{const dg=deg[b.id];
    if(dg.i+dg.o===0&&B.length>1){b._dead=true;problems.push("orphan: "+b.label+" — no connections");}
    if(dg.i>=3&&dg.o>=3){b._god=true;problems.push("god component: "+b.label+" (in "+dg.i+" · out "+dg.o+")");}});
  (function(){const adj={};view.edges.forEach(e=>{(adj[e.source]=adj[e.source]||[]).push(e.target);});
   const st={};let cyc=null;
   function dfs(u,stk){st[u]=1;stk.push(u);
     for(const v2 of adj[u]||[]){if(st[v2]===1){cyc=stk.slice(stk.indexOf(v2)).concat(v2);return true;}
       if(!st[v2]&&dfs(v2,stk))return true;}
     st[u]=2;stk.pop();return false;}
   for(const b of B){if(!st[b.id]&&dfs(b.id,[]))break;}
   if(cyc){problems.push("dependency cycle: "+cyc.map(id=>bById[id]?bById[id].label:id).join(" → "));
     cyc.forEach(id=>{if(bById[id])bById[id]._cyc=true;});}})();
  B.forEach(b=>{if((b.members||[]).length>40){b._over=b.members.length;
    b.members=b.members.slice(0,23).concat([{id:b.id+"_more",label:"+"+(b._over-23)+" more…",sub:"over budget"}]);}});
  B.forEach(b=>{const n=b.members.length;
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
    edges:view.edges.map(e=>({id:e.id,sources:[e.source],targets:[e.target],
      labels:[{text:(view.labeled&&e.label)?e.label+" ×"+e.count:"×"+e.count,
               width:lblW((view.labeled&&e.label)?e.label+" x"+e.count:"x"+e.count),height:16}]}))};
  const r=await elk.layout(g);
  const pos={};r.children.forEach(c=>pos[c.id]={x:snap(c.x),y:snap(c.y)});
  const laidById={};(r.edges||[]).forEach(e=>laidById[e.id]=e);
  const nodes=[];
  B.forEach(b=>{const bp=pos[b.id];const col=colOf(b.layer);
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
  const edges2=view.edges.map(e=>{const r=rfEdge(e,laidById,ee=>bcol[ee.source]||T.dim,id2=>(bById[id2]||{}).label||id2);
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
function BottomDock({vid,view,sel,clearSel,probs,expanded,setExpanded}){
  const VI=VIEW_INTENT[vid]||null;
  const counts=view.kind==="buckets"
    ?`${(view.buckets||[]).length} groups · ${(view.buckets||[]).reduce((a,b)=>a+(b.members||[]).length,0)} members · ${(view.edges||[]).length} edges`
    :view.kind==="table"?`${(view.rows||[]).length} rows`
    :view.kind==="matrix"?`${(view.items||[]).length}×${(view.items||[]).length} matrix`
    :`${(view.nodes||[]).length} elements · ${(view.edges||[]).length} edges`;
  const hl=p=>sel&&sel.label&&String(p).includes(String(sel.label).slice(0,24));
  const sortedProbs=sel?[...probs].sort((a,b)=>(hl(b)?1:0)-(hl(a)?1:0)):probs;
  const Seg=({title,col,flex,children})=>html`<div style=${{flex,minWidth:0,padding:"9px 16px",
    borderRight:`1px solid ${T.border}`,overflowY:"auto",position:"relative"}}>
    <div style=${{fontSize:9.5,fontWeight:700,letterSpacing:1.2,color:col,marginBottom:7,
      position:"sticky",top:0,background:T.band,paddingBottom:3}}>${title}</div>
    ${children}</div>`;
  if(!expanded)return html`<div onClick=${()=>setExpanded(true)}
    style=${{borderTop:`1px solid ${T.border}`,background:T.band,height:26,display:"flex",
    alignItems:"center",gap:0,flexShrink:0,cursor:"pointer",userSelect:"none"}}>
    <div style=${{flex:1.4,minWidth:0,padding:"0 16px",display:"flex",gap:7,alignItems:"center",overflow:"hidden"}}>
      <span style=${{fontSize:8.5,fontWeight:700,letterSpacing:1,color:T.blue,flexShrink:0}}>VIEW</span>
      <span style=${{fontSize:10.5,color:T.dim,whiteSpace:"nowrap",overflow:"hidden",textOverflow:"ellipsis"}}>${VI?VI.question:view.title}</span>
    </div>
    <div style=${{flex:1,minWidth:0,padding:"0 16px",display:"flex",gap:7,alignItems:"center",borderLeft:`1px solid ${T.border}`,overflow:"hidden"}}>
      <span style=${{fontSize:8.5,fontWeight:700,letterSpacing:1,color:T.text,flexShrink:0}}>SELECTION</span>
      <span style=${{fontSize:10.5,color:sel?T.text:T.mute,whiteSpace:"nowrap",overflow:"hidden",textOverflow:"ellipsis"}}>${sel?sel.label:"none — click an element or edge"}</span>
    </div>
    <div style=${{flex:.6,padding:"0 16px",display:"flex",gap:7,alignItems:"center",borderLeft:`1px solid ${T.border}`}}>
      <span style=${{fontSize:8.5,fontWeight:700,letterSpacing:1,color:probs.length?T.red:T.mute}}>CONCERNS</span>
      <span style=${{fontSize:10.5,color:probs.length?T.red:T.green}}>${probs.length||"✓"}</span>
    </div>
    <span style=${{padding:"0 12px",color:T.mute,fontSize:11}}>⌃</span>
  </div>`;
  return html`<div style=${{borderTop:`1px solid ${T.border}`,background:T.band,height:208,
    display:"flex",flexShrink:0,position:"relative"}}>
    <${Seg} title="VIEW" col=${T.blue} flex=${1}>
      <div style=${{fontSize:12,fontWeight:650,color:T.text,lineHeight:1.45}}>${VI?VI.question:view.title}</div>
      ${VI&&VI.pass?html`<div style=${{fontSize:10.5,color:T.dim,lineHeight:1.5,marginTop:6}}>
        <span style=${{color:T.mute,fontSize:8.5,fontWeight:700,letterSpacing:.8}}>PASS </span>${VI.pass}</div>`:null}
      ${view.prov?html`<div style=${{fontSize:10.5,color:T.dim,lineHeight:1.5,marginTop:6}}>
        <span style=${{color:T.mute,fontSize:8.5,fontWeight:700,letterSpacing:.8}}>SOURCE </span>${view.prov.label}</div>`:null}
      <div style=${{fontSize:10.5,color:T.dim,marginTop:6}}>
        <span style=${{color:T.mute,fontSize:8.5,fontWeight:700,letterSpacing:.8}}>ON SCREEN </span>${counts}</div>
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
      <//>`:html`<div style=${{color:T.mute,fontSize:11,marginTop:28,textAlign:"center"}}>none — click an element or edge</div>`}
    <//>
    <${Seg} title=${"CONCERNS · "+probs.length} col=${probs.length?T.red:T.mute} flex=${1}>
      ${probs.length?sortedProbs.map((p,i)=>html`<div key=${i} style=${{fontSize:10.5,lineHeight:1.5,marginBottom:6,
        color:hl(p)?T.text:T.dim,borderLeft:`2px solid ${T.red}`,paddingLeft:9,
        background:hl(p)?T.cardH:"transparent"}}>${p}</div>`)
      :html`<div style=${{color:T.green,fontSize:11}}>✓ no concerns detected in this view</div>`}
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
  return html`<div style=${{position:"absolute",top:10,right:14,zIndex:6,background:"#0c0c0ef0",
    border:`1px solid ${T.border}`,borderRadius:8,padding:"7px 11px",
    display:"flex",flexDirection:"column",gap:4}}>
    ${items.map((it,i)=>html`<div key=${i} style=${{display:"flex",alignItems:"center",gap:7,fontSize:10.5,color:T.dim}}>
      <span style=${{width:10,height:10,borderRadius:3,background:it.c+"30",border:`1.5px solid ${it.c}`,flexShrink:0}}></span>
      ${it.txt}</div>`)}
  </div>`;}
function Footer({view,ov}){
  const groups=[];
  if(view.kind==="buckets"){
    groups.push({label:"LAYERS",items:(view.buckets||[]).map(b=>({txt:b.layer,c:layerColor(view,b.layer),ico:b.ico||b.layer}))});
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
    gap:14,padding:"7px 16px",fontSize:10.5,color:T.dim,flexWrap:"wrap",background:T.bg}}>
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
function Sidebar({estate,scopes,simEstate,scope,goScope,level,go,open,setOpen,collapsed,setCollapsed,last}){
  const[copied,setCopied]=useState(null);
  const CATALOG=(estate==="local"&&CATALOGS[scope])?CATALOGS[scope]:dynamicCatalog(scopes[scope]||{views:{}});
  if(collapsed) return html`<div style=${{width:44,borderRight:`1px solid ${T.border}`,
    display:"flex",flexDirection:"column",alignItems:"center",paddingTop:10,flexShrink:0}}>
    <button onClick=${()=>setCollapsed(false)} title="Show models"
      style=${{background:"transparent",border:`1px solid ${T.border}`,color:T.dim,
      borderRadius:6,padding:"4px 8px",cursor:"pointer",fontSize:13}}>›</button>
  </div>`;
  return html`<div style=${{width:300,borderRight:`1px solid ${T.border}`,overflowY:"auto",
    flexShrink:0,display:"flex",flexDirection:"column"}}>
    <div style=${{padding:"11px 14px",display:"flex",alignItems:"center",borderBottom:`1px solid ${T.border}`}}>
      <span style=${{fontSize:11,fontWeight:700,letterSpacing:1.2,color:T.dim}}>SYSTEMS</span>
      <span style=${{fontSize:8.5,fontWeight:700,marginLeft:7,letterSpacing:.6,
        color:simEstate?T.yellow:T.green,border:`1px solid ${simEstate?T.yellow:T.green}`,
        borderRadius:4,padding:"0 5px"}}>${simEstate?"SIMULATED":"LOCAL"}</span>
      <button onClick=${()=>setCollapsed(true)} title="Hide"
        style=${{marginLeft:"auto",background:"transparent",border:"none",color:T.mute,cursor:"pointer",fontSize:14}}>‹</button>
    </div>
    ${Object.entries(scopes).map(([sid,sv])=>html`<div key=${sid} onClick=${()=>goScope(sid)}
      style=${{display:"flex",alignItems:"flex-start",gap:8,padding:"7px 14px",cursor:"pointer",
        background:sid===scope?T.cardH:"transparent",
        borderLeft:sid===scope?`2px solid ${T.blue}`:"2px solid transparent"}}>
      <${Ico} k="sys" c=${sid===scope?T.blue:T.dim} s=${13}/>
      <div style=${{minWidth:0}}>
        <div style=${{fontSize:12.5,fontWeight:sid===scope?700:500,color:sid===scope?T.text:T.dim}}>${sv.label}</div>
        <div style=${{fontSize:9.5,color:T.mute,marginTop:1}}>${sv.tech}</div>
      </div>
    </div>`)}
    <div style=${{padding:"10px 14px 4px",borderTop:`1px solid ${T.border}`,marginTop:4}}>
      <span style=${{fontSize:11,fontWeight:700,letterSpacing:1.2,color:T.dim}}>MODELS</span>
      <span style=${{fontSize:9,color:T.mute,marginLeft:6}}>· ${(scopes[scope]||{}).label} only</span>
    </div>
    <div style=${{flex:1,padding:"6px 0"}}>
    ${CATALOG.map(g=>{const isOpen=open[g.grp]!==false;
      return html`<div key=${g.grp}>
        <div onClick=${()=>setOpen({...open,[g.grp]:!isOpen})}
          style=${{display:"flex",alignItems:"center",gap:7,padding:"8px 14px",cursor:"pointer",userSelect:"none"}}>
          <span style=${{color:T.mute,fontSize:9,transform:isOpen?"rotate(90deg)":"none",transition:"transform .15s",width:8}}>▶</span>
          <span style=${{fontSize:11.5,fontWeight:700,letterSpacing:.6,color:T.text}}>${g.grp}</span>
          ${g.tag?html`<span style=${{fontSize:8.5,color:T.blue,border:`1px solid ${T.blue}`,borderRadius:4,
            padding:"0 4px",letterSpacing:.5,textTransform:"uppercase"}}>${g.tag}</span>`:null}
        </div>
        ${isOpen?g.items.map((it,ix)=>{const st=STATUS[it.status];const active=it.id&&it.id===level&&!it.alias;
          const aliasActive=it.id&&it.id===level&&it.alias;
          const clickable=!!it.id;
          return html`<div key=${ix} onClick=${clickable?()=>go(it.id):null}
            style=${{display:"flex",alignItems:"flex-start",gap:8,padding:"5px 14px 5px 29px",
              cursor:clickable?"pointer":"default",
              background:(active||aliasActive)?T.cardH:"transparent",
              borderLeft:(active||aliasActive)?`2px solid ${T.blue}`:"2px solid transparent"}}>
            <span style=${{color:st.col,fontSize:10,lineHeight:"17px"}}>${st.dot}</span>
            <div style=${{minWidth:0,flex:1}}>
              <div style=${{fontSize:12,fontWeight:active?650:500,
                color:clickable?T.text:T.dim,whiteSpace:"nowrap",overflow:"hidden",textOverflow:"ellipsis"}}>${it.label}</div>
              ${it.note?html`<div style=${{fontSize:9.5,color:T.mute}}>${it.note}</div>`:null}
            </div>
            ${it.id&&last&&last[estate+":"+scope+":"+it.id]?html`<span style=${{fontSize:8.5,color:T.mute,lineHeight:"17px",flexShrink:0}}>${ago(last[estate+":"+scope+":"+it.id])}</span>`:null}
            ${it.status==="planned"&&estate!=="local"&&it.vid0?html`<span title="Copy AI-generation prompt for this view"
              onClick=${ev=>{ev.stopPropagation();
                navigator.clipboard.writeText(genPrompt(estate,(scopes[scope]||{}).label||scope,it.vid0,it.label));
                setCopied(it.label);setTimeout(()=>setCopied(null),1400);}}
              style=${{fontSize:10,color:copied===it.label?T.green:T.mute,cursor:"pointer",flexShrink:0,lineHeight:"17px"}}>${copied===it.label?"✓":"⧉"}</span>`:null}
          </div>`;}):null}
      </div>`;})}
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
  const view=sys.views[level]||sys.views[Object.keys(sys.views)[0]];
  const[ov,setOv]=useState(()=>{const o=(q.get("ov")||"").split(",");
    return {concerns:o.includes("concerns"),changed:o.includes("changed")};});
  const[last,setLast]=useState({});
  const[autoDir,setAutoDir]=useState(null);
  const dir=dirOv||autoDir||"DOWN";
  useEffect(()=>{let on=true;const d=SP[den];
    setEls(null);setSelRaw(null);setSelId(null);   // view change clears selection but PRESERVES dock expansion; remount canvas with the new layout
    const run=dd=>view.kind==="buckets"?layoutBuckets(view,dd,d,ov):layoutSimple(view,dd,d);
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
    return()=>{on=false;};},[estate,scope,level,dirOv,den,ov]);
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
    const d=n.data,m=(n.type==="solo"?d.member:d)||{};
    const rows=(m.stats?Object.entries(m.stats):[]).concat(m.sub?[["detail",m.sub]]:[]);
    if(d.layer||d.lay)rows.unshift(["layer",d.layer||d.lay]);
    if(d.tech)rows.push(["tech",d.tech]);
    if(m.concerns)rows.push(["findings",m.concerns+" recon findings"]);
    if(m.changed)rows.push(["recent","touched in last 15 commits"]);
    const chip=n.type==="bucket"?"group":n.type==="entity"?"entity":n.type==="member"?"member":(ETYPE_NAME[d.type]||"element");
    select({label:m.label||d.label,chip,rows,relations:relationsFor(n.id),
      members:n.type==="bucket"?(d.members||[]):null},n.id);},[select,relationsFor]);
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
  useEffect(()=>{const h=ev=>{if(ev.key==="Escape"){if(sel)clearSel();else setExpanded(false);}};
    window.addEventListener("keydown",h);return()=>window.removeEventListener("keydown",h);},[sel,clearSel]);
  // test hook: ?sel=<nodeId> selects for screenshot verification (mark() is idempotent — no loop)
  useEffect(()=>{const sid=q.get("sel");if(!sid||!els||!els.nodes)return;
    const n=els.nodes.find(x=>x.id===sid);if(n&&selId!==sid)onNodeClick(null,n);},[els]);
  const[open,setOpen]=useState({});
  const[collapsed,setCollapsed]=useState(false);
  const go=useCallback(id=>{setLevel(id);setDirOv(null);},[]);
  const goScope=useCallback(sid=>{setScope(sid);setLevel(firstView(estate,sid));setDirOv(null);},[estate]);
  const goEstate=useCallback(eid=>{setEstate(eid);const sc=firstScope(eid);
    setScope(sc);setLevel(firstView(eid,sc));setDirOv(null);},[]);
  const btn=a=>({background:a?T.cardH:"transparent",border:`1px solid ${a?T.blue:T.border}`,
    color:a?T.text:T.dim,borderRadius:7,padding:"5px 11px",fontSize:12,cursor:"pointer",fontWeight:550});
  return html`<div style=${{height:"100vh",display:"flex",flexDirection:"column",background:T.bg,
    font:"13px -apple-system,Segoe UI,Inter,Roboto,sans-serif",color:T.text}}>
    <div style=${{padding:"9px 18px",borderBottom:`1px solid ${T.border}`,display:"flex",alignItems:"center",gap:10}}>
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
        ${els&&els.problems&&els.problems.length?html`<span title="Show concerns"
          onClick=${()=>setExpanded(true)}
          style=${{fontSize:9.5,fontWeight:700,color:T.red,border:`1px solid ${T.red}`,borderRadius:5,
          padding:"1px 7px",whiteSpace:"nowrap",cursor:"pointer",flexShrink:0}}>⚠ ${els.problems.length}</span>`:null}
        ${ISSUES.length?html`<span title=${ISSUES.join("\n")}
          style=${{fontSize:9.5,fontWeight:700,color:T.yellow,border:`1px solid ${T.yellow}`,borderRadius:5,
          padding:"1px 7px",whiteSpace:"nowrap",cursor:"help",flexShrink:0}}>◌ ${ISSUES.length}</span>`:null}
      </div>
      <div style=${{display:"flex",gap:8,alignItems:"center",flexShrink:0}}>
        <button style=${btn(ov.concerns)} onClick=${()=>setOv({...ov,concerns:!ov.concerns})}
          title="Recon findings per package (bitmask)">⚠ Concerns</button>
        <button style=${btn(ov.changed)} onClick=${()=>setOv({...ov,changed:!ov.changed})}
          title="Touched in last 15 commits (git)">Δ Changed</button>
        <span style=${{width:8}}></span>
        <button style=${btn(den==="compact")} onClick=${()=>setDen("compact")}>Compact</button>
        <button style=${btn(den==="comfort")} onClick=${()=>setDen("comfort")}>Comfort</button>
        <span style=${{width:8}}></span>
        <button style=${btn(!dirOv)} onClick=${()=>setDirOv(null)}
          title="Pick the direction that best fits the viewport">Auto${!dirOv&&autoDir?(autoDir==="DOWN"?" ↓":" →"):""}</button>
        <button style=${btn(dirOv==="DOWN")} onClick=${()=>setDirOv("DOWN")} title="Top–Bottom">↓</button>
        <button style=${btn(dirOv==="RIGHT")} onClick=${()=>setDirOv("RIGHT")} title="Left–Right">→</button>
      </div></div>
    <div style=${{padding:"3px 18px",borderBottom:`1px solid ${T.border}`,fontSize:10.5,color:T.dim,
      whiteSpace:"nowrap",overflow:"hidden",textOverflow:"ellipsis"}}
      title=${view.count+(view.prov?" · "+view.prov.label:"")}>
      ${view.count}${view.prov?html`<span style=${{color:T.mute}}> · ${view.prov.label}</span>`:null}
    </div>
    <div style=${{flex:1,display:"flex",minHeight:0}}>
      <${Sidebar} estate=${estate} scopes=${SC} simEstate=${ESTATES[estate].sim}
        scope=${scope} goScope=${goScope} level=${level} go=${go} open=${open} setOpen=${setOpen}
        collapsed=${collapsed} setCollapsed=${setCollapsed} last=${last}/>
      <div style=${{flex:1,display:"flex",flexDirection:"column",minWidth:0}}>
        <div style=${{flex:1,position:"relative",minHeight:0}}>
          ${els&&els.htmlView?html`<${view.kind==="table"?TableView:MatrixView} view=${view} onSel=${select} selId=${selId}/>`:null}
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
createRoot(document.getElementById("root")).render(html`<${ReactFlowProvider}><${Flow}/><//>`);
"""
HTML="""<!doctype html><html><head><meta charset="utf-8"><title>aOa — Architecture</title>
<link rel="stylesheet" href="https://esm.sh/@xyflow/react@12.3.5/dist/style.css">
<style>html,body,#root{margin:0;height:100%;background:#0c0c0e}
.react-flow__controls-button{background:#161618!important;border-color:#252528!important;fill:#8b8b96!important}
/* raise hovered LEAF nodes only — a raised bucket would paint over its own members */
.react-flow__node-member:hover,.react-flow__node-box:hover,
.react-flow__node-solo:hover,.react-flow__node-entity:hover{z-index:1200!important}
.hv{position:relative}
.hv .hovercard{visibility:hidden;transition:visibility 0s linear 0s;position:absolute;left:0;top:calc(100% + 7px);
 min-width:200px;max-width:300px;background:#1c1c1f;border:1px solid #3a3a40;border-radius:8px;padding:9px 12px;
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
