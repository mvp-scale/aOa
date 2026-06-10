#!/usr/bin/env python3
"""Lint archmodel view shards against playbook/standards/view-standards.json.

Mechanical half of the model-standard skill: label budgets, vital-field
presence, tier discipline. Reports WARN per finding; never blocks (exit 0)
unless --strict. The judgment half (blind judge) lives in the skill.

Usage: python3 playbook/generators/lint_views.py [estate-prefix] [--strict]
"""
import json, os, sys, glob

STD = json.load(open("playbook/standards/view-standards.json"))
G = STD["global"]["budgets"]
VIEWS = STD["views"]

findings = []
def warn(path, msg): findings.append((path, msg))

def lint_labels(path, view):
    if view["kind"] == "buckets":
        for b in view.get("buckets", []):
            if not b.get("label"): warn(path, f"bucket {b.get('id')}: no label")
            if "layer" not in b: warn(path, f"bucket {b.get('id')}: no layer")
            for m in b.get("members", []):
                L = m.get("label", "")
                if not L: warn(path, f"member {m.get('id')}: no label")
                elif len(L) > G["member_label_chars"]:
                    warn(path, f"member label over budget ({len(L)}>{G['member_label_chars']}): \"{L}\"")
            if len(b.get("members", [])) > G["bucket_members_max"]:
                warn(path, f"bucket {b['id']}: {len(b['members'])} members over budget {G['bucket_members_max']}")
    elif view["kind"] in ("simple", "entity"):
        nodes = view.get("nodes", [])
        if view["kind"] == "simple" and len(nodes) > G["simple_view_nodes_max"]:
            warn(path, f"{len(nodes)} nodes over budget {G['simple_view_nodes_max']}")
        for n in nodes:
            L = n.get("label", "")
            if not L: warn(path, f"node {n.get('id')}: no label")
            elif len(L) > G["node_label_chars"]:
                warn(path, f"node label over budget ({len(L)}>{G['node_label_chars']}): \"{L}\"")
            if view["kind"] == "entity" and not n.get("fields"):
                warn(path, f"entity {n.get('id')}: no fields (key fields are vital tier)")
    for e in view.get("edges", []):
        L = e.get("label", "")
        if len(L) > G["edge_label_chars"]:
            warn(path, f"edge label over budget ({len(L)}>{G['edge_label_chars']}): \"{L}\"")

def lint_vital(path, vid, view):
    std = VIEWS.get(vid)
    if not std: return
    # verb-on-every-edge views: the edge label IS the vital signal
    if vid in ("context", "container", "dataflow", "statemachine", "datamodel", "sequence", "deployment"):
        if view["kind"] in ("simple", "entity"):
            for e in view.get("edges", []):
                if not e.get("label"):
                    warn(path, f"edge {e.get('id')}: unlabeled — '{std['question']}' needs the verb")
    if vid == "trust" and view["kind"] == "buckets":
        if not any("part" in b for b in view.get("buckets", [])):
            warn(path, "trust view: no zone has a part (band ordering is vital)")
    if vid in ("glossary", "techportfolio", "sbom") and view["kind"] == "table":
        if not view.get("rows"): warn(path, "table view: no rows")
    if vid == "dsm" and view["kind"] == "matrix":
        items, M = view.get("items", []), view.get("matrix", [])
        if len(M) != len(items): warn(path, "matrix not square against items")

def main():
    prefix = next((a for a in sys.argv[1:] if not a.startswith("--")), "")
    strict = "--strict" in sys.argv
    n = 0
    for shard in sorted(glob.glob("playbook/mockups/archmodel/*/*/*.json")):
        rel = os.path.relpath(shard, "playbook/mockups/archmodel")
        if prefix and not rel.startswith(prefix): continue
        estate, scope, vfile = rel.split(os.sep)
        vid = vfile[:-5]
        try: view = json.load(open(shard))
        except Exception as ex:
            warn(rel, f"unparseable: {ex}"); continue
        n += 1
        path = f"{estate}/{scope}/{vid}"
        lint_labels(path, view)
        lint_vital(path, vid, view)
    by_view = {}
    for p, m in findings: by_view.setdefault(p, []).append(m)
    for p in sorted(by_view):
        print(f"WARN {p}")
        for m in by_view[p]: print(f"     · {m}")
    print(f"\n{n} views linted · {len(by_view)} below standard · {len(findings)} findings")
    sys.exit(1 if strict and findings else 0)

main()
