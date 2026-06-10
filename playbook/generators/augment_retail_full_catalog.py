#!/usr/bin/env python3
"""Augment the retail-monolith estate (clean + faulted) so EVERY row of the
standard catalog renders — the full-catalog proof estate."""
import json

def sim(src): return {"kind":"simulated","label":f"SIMULATED · would derive from: {src}"}

VIEWS={
 "container":{"kind":"simple","title":"Container view","count":"6 containers","dir":"DOWN",
  "prov":sim("deploy descriptors (EAR/WAR) + JVM topology"),
  "nodes":[
   {"id":"was","type":"container","icon":"container","label":"WebSphere EAR","sub":"[Java 8 · 480k LOC]","real":False},
   {"id":"web","type":"container","icon":"container","label":"Storefront","sub":"[AngularJS 1.x]","real":False},
   {"id":"db","type":"store","icon":"store","label":"Oracle 12c","sub":"[≈310 tables]","real":False},
   {"id":"mq","type":"store","icon":"store","label":"IBM MQ","sub":"[orders · inventory]","real":False},
   {"id":"batch","type":"container","icon":"container","label":"Batch JCL","sub":"[nightly]","real":False},
   {"id":"cdn","type":"ext","icon":"ext","label":"Akamai","sub":"[static]","real":False}],
  "edges":[
   {"id":"c1","source":"web","target":"was","label":"REST"},
   {"id":"c2","source":"was","target":"db","label":"JDBC"},
   {"id":"c3","source":"was","target":"mq","label":"JMS"},
   {"id":"c4","source":"batch","target":"db","label":"sqlldr"},
   {"id":"c5","source":"cdn","target":"web","label":"serves"}]},
 "deployment":{"kind":"buckets","title":"Deployment — DC + DR","count":"2 sites · 9 nodes","dir":"RIGHT",
  "prov":sim("F5 pools + WebSphere cell topology + Veritas DR runbook"),
  "buckets":[
   {"id":"dc","layer":"dc","label":"primary DC","part":0,"ico":"sys","members":[
     {"id":"d1","label":"was-cell-1","sub":"4 JVMs"},{"id":"d2","label":"was-cell-2","sub":"4 JVMs"},
     {"id":"d3","label":"oracle-rac","sub":"2 nodes"},{"id":"d4","label":"mq-pair","sub":"active"}]},
   {"id":"dr","layer":"dr","label":"DR site","part":1,"ico":"infra","members":[
     {"id":"d5","label":"was-standby","sub":"cold"},{"id":"d6","label":"oracle-dg","sub":"DataGuard"},
     {"id":"d7","label":"mq-standby","sub":"passive"}]}],
  "edges":[{"id":"dp1","source":"dc","target":"dr","count":3,"label":"replication"}],"labeled":True},
 "sequence":{"kind":"simple","title":"Dynamic — submit order (happy path)","count":"6 participants · 7 calls","dir":"RIGHT",
  "prov":sim("call-edge resolution (P4) — until then: APM traces"),
  "nodes":[
   {"id":"s0","type":"ext","icon":"ext","label":"POS","sub":"terminal","real":False},
   {"id":"s1","type":"proc","icon":"app","label":"api-gateway","sub":"","real":False},
   {"id":"s2","type":"proc","icon":"app","label":"OrderService","sub":"","real":False},
   {"id":"s3","type":"proc","icon":"app","label":"InventoryService","sub":"","real":False},
   {"id":"s4","type":"proc","icon":"app","label":"PaymentFacade","sub":"","real":False},
   {"id":"s5","type":"store","icon":"store","label":"Oracle","sub":"","real":False}],
  "edges":[
   {"id":"q1","source":"s0","target":"s1","label":"1 · submitOrder"},
   {"id":"q2","source":"s1","target":"s2","label":"2 · createOrder"},
   {"id":"q3","source":"s2","target":"s3","label":"3 · reserveStock"},
   {"id":"q4","source":"s2","target":"s4","label":"4 · authorize"},
   {"id":"q5","source":"s2","target":"s5","label":"5 · INSERT order"},
   {"id":"q6","source":"s4","target":"s5","label":"6 · INSERT auth"},
   {"id":"q7","source":"s2","target":"s1","label":"7 · 201 Created"}]},
 "code":{"kind":"entity","title":"Code (L4) — order checkout classes","count":"4 classes","dir":"DOWN",
  "prov":sim("symbol index (SymbolMeta) — class/method table"),
  "nodes":[
   {"id":"k1","type":"entity","label":"OrderService","tech":"class · 2,140 loc",
    "fields":["+ createOrder(cart): Order","+ cancel(id)","- validate(cart)","- priceOf(sku)"],"real":False},
   {"id":"k2","type":"entity","label":"OrderRepository","tech":"class · 890 loc",
    "fields":["+ save(Order)","+ findById(id)","+ findByCustomer(cid)"],"real":False},
   {"id":"k3","type":"entity","label":"Order","tech":"entity",
    "fields":["id: long","items: List<Line>","status: OrderStatus","total: Money"],"real":False},
   {"id":"k4","type":"entity","label":"InventoryClient","tech":"class · 410 loc",
    "fields":["+ reserve(sku, qty)","+ release(token)"],"real":False}],
  "edges":[
   {"id":"kc1","source":"k1","target":"k2","label":"uses"},
   {"id":"kc2","source":"k1","target":"k4","label":"uses"},
   {"id":"kc3","source":"k2","target":"k3","label":"persists"}]},
 "trust":{"kind":"buckets","title":"Trust boundaries (STRIDE)","count":"4 zones · 7 flows","dir":"DOWN",
  "prov":sim("boundary rule-pack over call edges + network segments"),
  "buckets":[
   {"id":"z1","layer":"internet","label":"untrusted · internet","part":0,"boundary":True,"ico":"ext","members":[
     {"id":"t1","label":"POS clients","sub":"2,400 sites"},{"id":"t2","label":"partner EDI","sub":"3PL"}]},
   {"id":"z2","layer":"dmz","label":"DMZ","part":1,"boundary":True,"ico":"infra","members":[
     {"id":"t3","label":"WAF","sub":"F5"},{"id":"t4","label":"api-gateway","sub":"authn"}]},
   {"id":"z3","layer":"internal","label":"internal · app zone","part":2,"boundary":True,"ico":"app","members":[
     {"id":"t5","label":"order-services","sub":"EAR"},{"id":"t6","label":"batch","sub":"JCL"}]},
   {"id":"z4","layer":"data","label":"restricted · data zone","part":3,"boundary":True,"ico":"store","members":[
     {"id":"t7","label":"oracle","sub":"PII"},{"id":"t8","label":"pci vault","sub":"tokens"}]}],
  "edges":[
   {"id":"z_1","source":"z1","target":"z2","count":2,"label":"TLS"},
   {"id":"z_2","source":"z2","target":"z3","count":2,"label":"authz ticket"},
   {"id":"z_3","source":"z3","target":"z4","count":3,"label":"JDBC · vault API"}],"labeled":True},
 "statemachine":{"kind":"simple","title":"State machine — Order lifecycle","count":"6 states · 8 transitions","dir":"RIGHT",
  "prov":sim("state-field extraction (OrderStatus enum + transitions)"),
  "nodes":[
   {"id":"st1","type":"proc","icon":"domain","label":"Created","sub":"","real":False},
   {"id":"st2","type":"proc","icon":"domain","label":"Authorized","sub":"","real":False},
   {"id":"st3","type":"proc","icon":"domain","label":"Picking","sub":"","real":False},
   {"id":"st4","type":"proc","icon":"domain","label":"Shipped","sub":"","real":False},
   {"id":"st5","type":"proc","icon":"domain","label":"Delivered","sub":"","real":False},
   {"id":"st6","type":"proc","icon":"ext","label":"Cancelled","sub":"terminal","real":False}],
  "edges":[
   {"id":"m1","source":"st1","target":"st2","label":"payment ok"},
   {"id":"m2","source":"st2","target":"st3","label":"stock reserved"},
   {"id":"m3","source":"st3","target":"st4","label":"manifest"},
   {"id":"m4","source":"st4","target":"st5","label":"POD"},
   {"id":"m5","source":"st1","target":"st6","label":"timeout"},
   {"id":"m6","source":"st2","target":"st6","label":"fraud"},
   {"id":"m7","source":"st3","target":"st6","label":"oos"}]},
 "glossary":{"kind":"table","title":"Glossary — ubiquitous language","count":"8 terms",
  "prov":sim("atlas identifier mining + human definitions"),
  "columns":["Term","Definition","Owning domain"],
  "rows":[
   ["SKU","Stock-keeping unit — sellable item identity","inventory"],
   ["ATP","Available-to-promise — sellable qty after reservations","inventory"],
   ["Basket","Pre-checkout cart, POS-side state","order"],
   ["Authorization","Payment hold prior to capture","order"],
   ["Replenishment","Warehouse → store stock movement","inventory"],
   ["Loyalty tier","Customer discount band (bronze…platinum)","customer"],
   ["POD","Proof of delivery — closes the order lifecycle","order"],
   ["Golden record","Deduplicated master customer identity","customer"]]},
 "techportfolio":{"kind":"table","title":"Technology portfolio","count":"8 platforms",
  "prov":sim("manifests + CMDB + version scan"),
  "columns":["Platform","Version","Category","Status"],
  "rows":[
   ["Java","8 (EOL)","runtime","⚠ migrate"],
   ["WebSphere ND","8.5.5","app server","⚠ extended support"],
   ["Oracle DB","12c","database","⚠ upgrade to 19c"],
   ["AngularJS","1.6 (EOL)","frontend","⚠ replace"],
   ["IBM MQ","9.0","messaging","supported"],
   ["Spring","4.3","framework","⚠ CVE backlog"],
   ["Akamai","—","CDN","supported"],
   ["Control-M","9","batch scheduler","supported"]]},
 "sbom":{"kind":"table","title":"SBOM — top components","count":"10 of ≈340 components",
  "prov":sim("lockfiles/poms → CycloneDX (Syft-style scan)"),
  "columns":["Component","Version","License","Risk"],
  "rows":[
   ["log4j","1.2.17","Apache-2.0","⚠ EOL · CVE-2019-17571"],
   ["spring-core","4.3.30","Apache-2.0","⚠ 3 CVEs"],
   ["commons-collections","3.2.1","Apache-2.0","⚠ deserialization"],
   ["struts (legacy module)","2.3.x","Apache-2.0","⚠ critical"],
   ["hibernate","5.2","LGPL-2.1","ok"],
   ["jackson-databind","2.9","Apache-2.0","⚠ 2 CVEs"],
   ["guava","20.0","Apache-2.0","ok"],
   ["angularjs","1.6.9","MIT","⚠ EOL"],
   ["ojdbc8","12.2","Oracle FUTC","review license"],
   ["ibm-mq-client","9.0","IBM ILAN","ok"]]},
 "dsm":{"kind":"matrix","title":"Dependency Structure Matrix","count":"5 modules",
  "prov":sim("import graph → N×N matrix"),
  "items":["order","inventory","customer","shared","batch"],
  "matrix":[[None,38,21,212,0],[9,None,9,167,0],[0,0,None,143,0],[0,0,0,None,0],[24,31,0,88,None]]},
 "cycles":{"kind":"table","title":"Cycle / tangle report","count":"2 cycles · tangle 14%",
  "prov":sim("Tarjan SCC over import graph"),
  "columns":["Cycle","Path","Edges","Verdict"],
  "rows":[
   ["#1","order → inventory → order","38 ⇄ 9","⚠ break via order-events topic"],
   ["#2","batch ↔ shared (config feedback)","88 ⇄ 12","⚠ extract batch-config"],
   ["tangle","4 of 5 modules in one SCC neighborhood","—","⚠ 14% of edges in cycles"]]},
}

for variant in ("clean","faulted"):
    f=f"playbook/mockups/estates/retail-monolith-{variant}.json"
    d=json.load(open(f))
    (eid,ev),=d["estates"].items()
    (sid,sv),=ev["scopes"].items()
    for vid,v in VIEWS.items():
        nv=json.loads(json.dumps(v))
        if variant=="faulted" and vid=="trust":
            nv["edges"].append({"id":"z_bad","source":"z1","target":"z4","count":1,
                "label":"legacy report pull","tag":"boundary-bypass"})
            nv["count"]="4 zones · 8 flows"
        sv["views"][vid]=nv
    json.dump(d,open(f,"w"),indent=1,ensure_ascii=False)
    print(f"{variant}: {len(sv['views'])} views")
