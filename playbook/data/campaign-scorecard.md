# 10-Estate Campaign Scorecard

## 1. Summary

| Estate | Views Authored | Clean Render | Faults Injected (A/B) | Detected + Highlighted (A) | Contained (B) | Missed | Render Failures |
|---|---|---|---|---|---|---|---|
| retail-monolith | context, domains, datamodel | ✅ (context) | 2A / 2B | 2/2 | 2/2 | 0 | 0 |
| streaming-platform | component, dataflow, deployment | ✅ (component) | 3A / 1B | 3/3 | 1/1 | 0 | 0 |
| cloudflare-edge | container, component, dataflow | ✅ (container) | 2A / 1B | 2/2 | 1/1 | 0 | 0 |
| hybrid-cloud | context, container, component, dataflow | ✅ (context) | 2A / 1B | 2/2 | 1/1 | 0 | 0 |
| multi-cloud | context, deployment, dataflow | ✅ (context, deployment) | 2A / 1B | 2/2 ⚠️ caveat | 1/1 | 0 | 0 |
| web-app | container, component, datamodel | ✅ (container) | 2A / 0B | 2/2 | — | 0 | 0 |
| mobile-bff | context, container, dataflow, component | ✅ (context) | 2A / 1B | 2/2 | 1/1 | 0 | 0 |
| event-platform | context, dataflow, component, datamodel | ✅ (context) | 2A / 1B | 2/2 | 1/1 | 0 | 0 |
| ml-platform | component, dataflow, deployment | ✅ (component) | 2A / 1B | 2/2 | 1/1 | 0 | 0 |
| legacy-soa | context, component, dataflow | ✅ (context) | 3A / 1B | 3/3 | 1/1 | 0 | 0 |
| **Totals** | — | **11/11 clean shots** | **22A / 10B** | **22/22** | **10/10** | **0** | **0** |

## 2. Detection Detail

### retail-monolith
| Fault | Class | Verdict | Matched Indicator |
|---|---|---|---|
| direct-db e_ctx5 (context) | A | **DETECTED** | Red dashed edge POS Terminals → Oracle 12c, label "⚠ queries prices directly"; ⚠ 1 pattern problem chip |
| cycle b_order→b_inventory→b_customer (domains) | A | **DETECTED** | ⟳ CYCLE badges on all three domain groups |
| budget-overflow b_shared 46 members (domains) | B | **CONTAINED + VISIBLE** | "46 · COLLAPSED" badge + "+23 more / over budget" truncation tile; ◌ 2 model issues chip |
| dangling-edge e_dm7 → t_carriers (datamodel) | B | **CONTAINED** | Edge dropped from canvas; surfaced only via ◌ 2 model issues chip; diagram usable (chip-only signal — see Gaps) |

### streaming-platform
| Fault | Class | Verdict | Matched Indicator |
|---|---|---|---|
| god b-play in4/out3 (component) | A | **DETECTED** | PLAYBACK bucket red dashed border / red halo; ⚠ 3 pattern problems chip |
| shared-db e11 b-bill→b-data (component) | A | **DETECTED** | Red dashed edge "⚠ ledger ×14" BILLING → DATA PLANE |
| band-violation bv1 b-bill→b-play (component) | A | **DETECTED** | Red dashed edge "⚠ entitlement push ×7" BILLING → PLAYBACK |
| duplicate-id m-cat-3 in b-cat (component) | B | **CONTAINED** | ◌ 1 model issue chip; no per-node badge on Catalog (chip-only signal) |

### cloudflare-edge
| Fault | Class | Verdict | Matched Indicator |
|---|---|---|---|
| bypass-api e8 gateway→catalog-d1 (dataflow) | A | **DETECTED** | Red dashed edge "⚠ env.DB direct query"; ⚠ 1 pattern problem chip |
| orphan b4 Observability (component) | A | **DETECTED** | Yellow ORPHAN badge on OBSERVABILITY bucket, no edges |
| empty-bucket b5 Legacy Bindings (component) | B | **CONTAINED** | Bucket renders empty (0 members), stays connected via "binds ×2", correctly NOT double-badged orphan (chips cropped in capture — see Gaps) |

### hybrid-cloud
| Fault | Class | Verdict | Matched Indicator |
|---|---|---|---|
| cycle b_intbus→b_cloud→b_shared (component) | A | **DETECTED** | ⟳ CYCLE badges on all three buckets; return edge "pushes golden rec…" visible; ⚠ chip |
| cross-version e_df4 d_erp→d_einv (dataflow) | A | **DETECTED** | Red dashed edge "⚠ issues invoice", only red edge in flow; ⚠ 1 pattern problem chip (label omits the v1/v2 detail — see Gaps) |
| dangling-edge e_con7 → c_legacy_mq (container) | B | **CONTAINED** | Edge dropped; ◌ 1 model issue chip; 7 containers fully usable |

### multi-cloud
| Fault | Class | Verdict | Matched Indicator |
|---|---|---|---|
| band-violation e13 b_azure→b_cp (deployment) | A | **DETECTED** | Red dashed upward edge "⚠ failover state write ×5" |
| god b_cp Shared Control Plane (deployment) | A | **DETECTED (caveat)** | Red halo on SHARED CONTROL PLANE — but badge reads "⟳ CYCLE", not GOD; chip says 3 pattern problems vs 2 in manifest (see Gaps) |
| duplicate-id aws1 ×2 in b_aws (deployment) | B | **CONTAINED (lossy)** | ◌ 1 model issue chip; count badge 4 vs 3 rendered chips — EKS Cluster silently collapsed by id collision; diagram usable |

### web-app
| Fault | Class | Verdict | Matched Indicator |
|---|---|---|---|
| direct-db tag e6 spa→postgres (container) | A | **DETECTED** | Red dashed edge with red arrowhead, label "…s directly (SQL)"; ⚠ 1 pattern problem chip |
| orphan b5 Legacy Helpers (component) | A | **DETECTED** | ORPHAN badge on LEGACY HELPERS, 2 members, no edges |

### mobile-bff
| Fault | Class | Verdict | Matched Indicator |
|---|---|---|---|
| bypass-api e12 atlas-ios→core-api (container) | A | **DETECTED** | Red dashed edge "⚠ calls core directly"; ⚠ 1 pattern problem chip |
| cycle b2→b3→b4→b2 (component) | A | **DETECTED** | ⟳ CYCLE badges on SYNC ENGINE, CONFLICT SERVICE, PUSH FANOUT |
| empty-bucket b5 Data Layer (component) | B | **CONTAINED** | Bucket rendered with 0 members while incoming edges (persists ops ×22, etc.) still terminate; ◌ 1 model issue chip |

### event-platform
| Fault | Class | Verdict | Matched Indicator |
|---|---|---|---|
| god b4 Kafka Bus in3/out3 (component) | A | **DETECTED** | Red halo on Kafka Bus with fan-in/fan-out edges visible; ⚠ 2 pattern problems chip |
| band-violation e10 b5→b8 (component) | A | **DETECTED** | Red dashed edge "⚠ writes lineage directly ×3" Stream Processors → Governance |
| budget-overflow b4 46 members (component) | B | **CONTAINED + VISIBLE** | "46 · COLLAPSED" badge + "+23 more / over budget" tile; ◌ 1 model issue chip |

### ml-platform
| Fault | Class | Verdict | Matched Indicator |
|---|---|---|---|
| cross-version pe2 art→srv (deployment) | A | **DETECTED** | Red/pink dashed edge "⚠ pulls model v2 (fleet pinned v1) ×6"; pods show "T4 · model v1" — best-in-campaign cross-version rendering |
| cycle tr→rg→inf→tr (component) | A | **DETECTED** | ⟳ CYCLE badges on Training, Model Registry, Inference; back-edge ×3 drawn |
| dangling-edge de9 n8→n99 (dataflow) | B | **CONTAINED** | Edge dropped, no phantom node; ◌ 1 model issue chip; flow fully usable |

### legacy-soa
| Fault | Class | Verdict | Matched Indicator |
|---|---|---|---|
| god b-esb ESB Hub in3/out3 (component) | A | **DETECTED** | Red/pink halo border on WEBSPHERE ESB HUB |
| shared-db tag ec6 b-batch→b-db2 (component) | A | **DETECTED** | Red dashed edge "⚠ bulk load ×12"; DB2 bucket outlined red |
| orphan b-uddi (component) | A | **DETECTED** | Explicit ORPHAN badge on UDDI REGISTRY (RETIRED), no edges |
| duplicate-id m-soap-1 ×2 in b-soap (component) | B | **CONTAINED** | ◌ 1 model issue chip; SOAP bucket renders without break (chip-only signal) |

## 3. Totals

| Metric | Score | Rate |
|---|---|---|
| Class A detection (visible highlight) | 22 / 22 | **100%** |
| Class B containment (no render break, surfaced as model issue) | 10 / 10 | **100%** |
| Clean-render rate (clean variants, no false-positive chips/badges) | 11 / 11 shots, 10 / 10 estates | **100%** |
| Render failures across all 30 shots | 0 / 30 | **0%** |
| Class B with a *localized* visual indicator (beyond header chip) | 6 / 10 | 60% |

## 4. Gaps & Caveats

No outright misses. Strict review surfaces these weaknesses:

1. **God-badge taxonomy gap (multi-cloud)** — b_cp's god fault is surfaced via red halo only; the badge text reads "⟳ CYCLE", not a GOD/HUB label. Hypothesis: injected e13 (b_azure→b_cp) plus pre-existing e8 (b_cp→b_azure) forms an incidental 2-node cycle, and the cycle detector's badge wins placement over a god indicator. Also explains the chip reading "3 pattern problems" against 2 manifest faults — a true incidental finding, not a false positive, but it muddies fault accounting. Note legacy-soa's god rendered halo-only with no badge text reported either; the god pattern may have no dedicated badge at all.
2. **Chip-only Class B signal for duplicate-id and dangling-edge** — 4 of 10 B-faults (streaming m-cat-3, legacy m-soap-1, retail e_dm7, ml de9) have zero localized affordance: dropped edges vanish silently and duplicate members get no per-bucket marker. The header chip says *something* is wrong but not *where*. Empty-bucket and budget-overflow, by contrast, have strong local indicators.
3. **Lossy duplicate-id containment (multi-cloud)** — id collision silently swallows the EKS Cluster member (count badge 4, 3 chips rendered). Contained, but a real member disappears from the canvas; an inline "id conflict" marker would be safer.
4. **Inconsistent cross-version edge labeling** — ml-platform explains the mismatch in the label ("pulls model v2 (fleet pinned v1)"); hybrid-cloud shows only "⚠ issues invoice" with the v1/v2 detail invisible. Hypothesis: label comes from authored edge text, not synthesized from the tag — detection quality depends on author discipline.
5. **Capture/viewport issues, not viewer issues** — cloudflare-edge component shot crops the header chips (chip counts unverifiable in that frame); multi-cloud deployment needed a retry after two blank ~9KB PNGs (screenshot timing flakiness). Neither affects detection scoring but both degrade evidence quality.
6. **Unexercised authored views** — several authored views were never screenshotted in faulted form (streaming dataflow/deployment, web-app datamodel, event-platform dataflow/datamodel, retail-monolith clean domains/datamodel, etc.). No faults were planted there, so nothing was missed, but "no false positives on faulted files in non-faulted views" is unverified for those.

## 5. Verdict

**Yes — the system credibly handles complex and poorly-designed architectures.** Across 10 estates, 7 fault types, and 5 view kinds, every Class A architectural smell (cycles, god components, band violations, bypass/direct-db/shared-db tags, orphans, cross-version drift) produced a visible, correctly-located highlight, and every Class B model defect (dangling edges, duplicate ids, empty buckets, budget overflows) was contained without a single render failure — including a 46-member bucket auto-collapse and edges pointing at nonexistent nodes. Clean variants produced zero false positives. The credible weaknesses are diagnostic precision, not robustness: god components lack a dedicated badge, and chip-only surfacing for dropped edges and id collisions means a user knows the model is dirty but must hunt for where. Recommended hardening: a GOD/HUB badge, localized markers for dropped/duplicate elements, and tag-derived cross-version labels.