// FDN-3 (board #29): App-layer wiring for the facts substrate compactor
// (internal/domain/facts). Two entry points feed it:
//
//   - Bulk build (WarmCaches fresh-build branch, Reindex): raw facts are
//     already in memory (built from the same []ports.ImportEdge the legacy
//     resolveEdgeBatch/BuildIndexWithFacts path already collects), so those
//     callers run facts.CompactWithManifests directly and persist via
//     ReplaceAllFacts + PutResolved + PutFindings — no store round trip.
//   - Incremental (doFlushEdgeBatch, the existing 200ms debounce hook —
//     D11/D12): per-file raw facts are swapped via ReplaceFactsForFile, then
//     compactAndPersistFacts below re-reads the FULL raw fact set from the
//     store and recomputes resolved+findings "in memory from the raw
//     buckets" (01-facts-substrate.md §3, compaction cadence point 2) —
//     never from re-parsing.
package app

import (
	"strconv"

	"github.com/corey/aoa/internal/domain/facts"
	"github.com/corey/aoa/internal/ports"
)

// compactAndPersistFacts re-reads every raw FactDep fact currently in the
// store, recompacts (units + adjacency + findings), and writes the result
// wholesale. C1 compliant: callers must not hold a.mu — this only issues
// Store reads/writes, matching the existing doFlushEdgeBatch shape.
//
// Serialized by a.factsMu (review punch, mirrors deriveArch/archDeriveMu):
// the FactsByKind read happens AFTER acquiring the lock, not before, so
// among overlapping incremental compactions (e.g. two debounce flushes in
// quick succession) the last call to acquire the lock is also the last to
// read — it always reflects the newest facts_raw, the same
// read-fresh-inside-the-critical-section guarantee deriveArch gives arch
// shards. Without this, a slower compaction started on older data could
// finish after a faster/newer one and silently revert the facts substrate.
func (a *App) compactAndPersistFacts(projectID, projectRoot string, fileSet map[string]bool) {
	if a.Store == nil {
		return
	}
	a.factsMu.Lock()
	defer a.factsMu.Unlock()

	raw, err := a.Store.FactsByKind(projectID, ports.FactDep)
	if err != nil {
		a.debugf("compactAndPersistFacts: FactsByKind: %v", err)
		return
	}
	units, adj, findings := facts.Compact(raw, projectRoot, fileSet)
	if err := a.Store.PutResolved(projectID, units, adj); err != nil {
		a.debugf("compactAndPersistFacts: PutResolved: %v", err)
	}
	if err := a.Store.PutFindings(projectID, findings); err != nil {
		a.debugf("compactAndPersistFacts: PutFindings: %v", err)
	}
}

// hasFreshFacts reports whether the facts substrate's last compaction used
// the current FactsSchemaVersion — the D14/t64-style boot freshness check
// (mirrors hasLocalArchManifest, arch.go:250-265). A missing/corrupt/absent
// meta record, or any read error, is treated as "not fresh" so a full
// recompact is forced rather than trusting stale output.
func (a *App) hasFreshFacts() bool {
	if a.Store == nil {
		return false
	}
	meta, err := a.Store.FactsMeta(a.ProjectID)
	if err != nil || meta == nil {
		return false
	}
	v, ok := meta["schema_version"]
	if !ok {
		return false
	}
	n, err := strconv.Atoi(v)
	return err == nil && n == facts.FactsSchemaVersion
}
