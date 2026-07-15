// FDN-1 (board #27): the FactStore adapter for the universal facts substrate
// (playbook/integration/01-facts-substrate.md §3). Same DB file as the rest
// of the store, new project-scoped sub-buckets. Additive only — EdgeStore
// (edges.go) and the scope-keyed Finding store (findings.go) are untouched.
package bbolt

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/corey/aoa/internal/ports"
	bolt "go.etcd.io/bbolt"
)

// factsVersion is the C3 schema version shared by every bucket in this file
// (D10). Bump when any serialisation format below changes. Old binaries (no
// FactStore code) simply ignore these buckets; a version mismatch on a
// newer binary triggers drop+recreate — facts are pure re-derivable cache.
const factsVersion byte = 1

// Fact-substrate bucket keys (spec §3).
//
// bucketFactUnresolved and bucketFactFindings deliberately do NOT reuse the
// spec's literal bucket names "facts_unresolved"/"facts_findings": those
// names are already live bbolt buckets holding differently-shaped data —
// L19.9's EdgeStore.SaveUnresolved bucket (bucketFactsUnresolved,
// []ports.ImportEdge JSON, edges.go:253-282) and L19.15's Finding-by-scope
// bucket (bucketFactsFindings, []ports.Finding JSON, findings.go:17-20).
// Reusing those exact names here would have two unrelated features silently
// overwrite/corrupt each other's rows under one bucket with incompatible
// codecs. Singular "fact_" keeps this new substrate namespaced apart while
// the pre-existing EdgeStore/Finding code stays untouched, per FDN-1 scope.
var (
	bucketFactsMeta      = []byte("facts_meta")
	bucketFactsRaw       = []byte("facts_raw")
	bucketFactsByFile    = []byte("facts_byfile")
	bucketFactsUnits     = []byte("facts_units")
	bucketFactsDepFwd    = []byte("facts_dep_fwd")
	bucketFactsDepRev    = []byte("facts_dep_rev")
	bucketFactUnresolved = []byte("fact_unresolved") // NOT bucketFactsUnresolved — see note above
	bucketFactFindings   = []byte("fact_findings")   // NOT bucketFactsFindings — see note above
	bucketFactsBaseline  = []byte("facts_baseline")
)

// factsBucketNames lists every bucket owned by the fact substrate, used by
// DeleteProjectFacts and by ensure/open helpers' callers.
var factsBucketNames = [][]byte{
	bucketFactsMeta, bucketFactsRaw, bucketFactsByFile, bucketFactsUnits,
	bucketFactsDepFwd, bucketFactsDepRev, bucketFactUnresolved, bucketFactFindings,
	bucketFactsBaseline,
}

// openFactsBucket opens a fact-substrate sub-bucket inside proj for reading.
// Returns nil if the bucket is absent or carries a wrong _version (C3) — a
// nil return means "treat as empty", never a panic.
func openFactsBucket(proj *bolt.Bucket, name []byte) *bolt.Bucket {
	if proj == nil {
		return nil
	}
	b := proj.Bucket(name)
	if b == nil {
		return nil
	}
	v := b.Get(keyVersion)
	if len(v) == 0 || v[0] != factsVersion {
		return nil
	}
	return b
}

// ensureFactsBucket opens or creates a fact-substrate sub-bucket inside proj
// for writing, enforcing the same C3 version contract as ensureEdgesBucket /
// ensureFindingsBucket: fresh bucket gets the version byte; matching version
// is returned as-is; mismatched version is dropped and re-created (facts are
// cache, always re-derivable, D10).
func ensureFactsBucket(proj *bolt.Bucket, name []byte) (*bolt.Bucket, error) {
	b, err := proj.CreateBucketIfNotExists(name)
	if err != nil {
		return nil, fmt.Errorf("create %s bucket: %w", name, err)
	}
	v := b.Get(keyVersion)
	if len(v) == 0 {
		if err := b.Put(keyVersion, []byte{factsVersion}); err != nil {
			return nil, fmt.Errorf("write %s version: %w", name, err)
		}
		return b, nil
	}
	if v[0] == factsVersion {
		return b, nil
	}
	if err := proj.DeleteBucket(name); err != nil {
		return nil, fmt.Errorf("delete mismatched %s bucket: %w", name, err)
	}
	b, err = proj.CreateBucket(name)
	if err != nil {
		return nil, fmt.Errorf("recreate %s bucket: %w", name, err)
	}
	if err := b.Put(keyVersion, []byte{factsVersion}); err != nil {
		return nil, fmt.Errorf("write %s version after recreate: %w", name, err)
	}
	return b, nil
}

// deleteFactsBuckets drops every fact-substrate bucket inside proj. Missing
// buckets are skipped (idempotent). Callers run this inside their own
// db.Update transaction.
func deleteFactsBuckets(proj *bolt.Bucket) error {
	for _, name := range factsBucketNames {
		if proj.Bucket(name) != nil {
			if err := proj.DeleteBucket(name); err != nil {
				return fmt.Errorf("delete %s: %w", name, err)
			}
		}
	}
	return nil
}

// isMetaKey reports whether k is the bucket-internal _version marker, so
// ForEach scans over fact buckets can skip it.
func isMetaKey(k []byte) bool { return bytes.Equal(k, keyVersion) }

// factRawKey builds the facts_raw key: file \x00 line \x00 seq (spec §3).
// seq is the fact's index within its ReplaceFactsForFile call, guaranteeing
// uniqueness even when several facts share the same file:line.
func factRawKey(file string, line uint32, seq int) []byte {
	return []byte(file + "\x00" + strconv.FormatUint(uint64(line), 10) + "\x00" + strconv.Itoa(seq))
}

// encodeKeyList / decodeKeyList encode the facts_byfile incremental-delete
// index: a binary list of facts_raw keys backing one file (little-endian:
// count:uint32, then per key keyLen:uint16 + key bytes).
func encodeKeyList(keys [][]byte) []byte {
	total := 4
	for _, k := range keys {
		total += 2 + len(k)
	}
	buf := make([]byte, total)
	offset := 0
	binary.LittleEndian.PutUint32(buf[offset:], uint32(len(keys)))
	offset += 4
	for _, k := range keys {
		binary.LittleEndian.PutUint16(buf[offset:], uint16(len(k)))
		offset += 2
		copy(buf[offset:], k)
		offset += len(k)
	}
	return buf
}

func decodeKeyList(data []byte) ([][]byte, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("key list too short: %d bytes", len(data))
	}
	offset := 0
	count := binary.LittleEndian.Uint32(data[offset:])
	offset += 4
	keys := make([][]byte, 0, count)
	for i := uint32(0); i < count; i++ {
		if offset+2 > len(data) {
			return nil, fmt.Errorf("truncated at key %d length (offset %d)", i, offset)
		}
		kl := int(binary.LittleEndian.Uint16(data[offset:]))
		offset += 2
		if offset+kl > len(data) {
			return nil, fmt.Errorf("truncated at key %d (offset %d, need %d)", i, offset, kl)
		}
		k := make([]byte, kl)
		copy(k, data[offset:offset+kl])
		offset += kl
		keys = append(keys, k)
	}
	return keys, nil
}

// encodeDepEdges / decodeDepEdges implement the posting-list adjacency
// encoding from spec §3: little-endian edgeCount:uint32, then per edge
// unitLen:uint16 + unit + count:uint16. Mirrors encodePostingLists /
// decodePostingLists in encoding.go.
func encodeDepEdges(edges []ports.DepEdge) ([]byte, error) {
	total := 4
	for _, e := range edges {
		total += 2 + len(e.Unit) + 2
	}
	buf := make([]byte, total)
	offset := 0
	binary.LittleEndian.PutUint32(buf[offset:], uint32(len(edges)))
	offset += 4
	for _, e := range edges {
		ub := []byte(e.Unit)
		if len(ub) > 65535 {
			return nil, fmt.Errorf("unit id too long: %d bytes", len(ub))
		}
		binary.LittleEndian.PutUint16(buf[offset:], uint16(len(ub)))
		offset += 2
		copy(buf[offset:], ub)
		offset += len(ub)
		binary.LittleEndian.PutUint16(buf[offset:], e.Count)
		offset += 2
	}
	return buf, nil
}

func decodeDepEdges(data []byte) ([]ports.DepEdge, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("dep edge posting list too short: %d bytes", len(data))
	}
	offset := 0
	count := binary.LittleEndian.Uint32(data[offset:])
	offset += 4
	edges := make([]ports.DepEdge, 0, count)
	for i := uint32(0); i < count; i++ {
		if offset+2 > len(data) {
			return nil, fmt.Errorf("truncated at edge %d unit length (offset %d)", i, offset)
		}
		ul := int(binary.LittleEndian.Uint16(data[offset:]))
		offset += 2
		if offset+ul > len(data) {
			return nil, fmt.Errorf("truncated at edge %d unit (offset %d, need %d)", i, offset, ul)
		}
		unit := string(data[offset : offset+ul])
		offset += ul
		if offset+2 > len(data) {
			return nil, fmt.Errorf("truncated at edge %d count (offset %d)", i, offset)
		}
		cnt := binary.LittleEndian.Uint16(data[offset:])
		offset += 2
		edges = append(edges, ports.DepEdge{Unit: unit, Count: cnt})
	}
	return edges, nil
}

// putFactsForFileTx is the shared per-file swap body used by both
// ReplaceFactsForFile (one file, its own tx) and ReplaceAllFacts (every
// file, one tx). Prior facts for path are located via the facts_byfile
// index and deleted before the new set is written; empty facts is a pure
// delete. Caller owns the surrounding db.Update transaction.
func putFactsForFileTx(rawB, byFileB *bolt.Bucket, path string, facts []ports.Fact) error {
	fileKey := []byte(path)
	if prev := byFileB.Get(fileKey); prev != nil {
		cp := append([]byte(nil), prev...)
		if oldKeys, derr := decodeKeyList(cp); derr == nil {
			for _, k := range oldKeys {
				if err := rawB.Delete(k); err != nil {
					return fmt.Errorf("delete stale raw key for %q: %w", path, err)
				}
			}
		}
		// A corrupt key-list index is non-fatal: new facts still get
		// written below; any orphaned rows are pure cache garbage,
		// cleared by the next full rebuild (facts are re-derivable).
	}

	if len(facts) == 0 {
		return byFileB.Delete(fileKey)
	}

	newKeys := make([][]byte, 0, len(facts))
	for i, f := range facts {
		key := factRawKey(path, f.Source.Line, i)
		data, merr := json.Marshal(f)
		if merr != nil {
			return fmt.Errorf("marshal fact %d for %q: %w", i, path, merr)
		}
		if err := rawB.Put(key, data); err != nil {
			return fmt.Errorf("put fact %d for %q: %w", i, path, err)
		}
		newKeys = append(newKeys, key)
	}
	return byFileB.Put(fileKey, encodeKeyList(newKeys))
}

// ReplaceFactsForFile atomically swaps all raw facts attributed to one file
// (§4.1's incremental unit of work). Implements ports.FactStore.
// C1: caller must NOT hold App.mu.
func (s *Store) ReplaceFactsForFile(projectID, path string, facts []ports.Fact) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		proj, err := tx.CreateBucketIfNotExists([]byte(projectID))
		if err != nil {
			return err
		}
		rawB, err := ensureFactsBucket(proj, bucketFactsRaw)
		if err != nil {
			return err
		}
		byFileB, err := ensureFactsBucket(proj, bucketFactsByFile)
		if err != nil {
			return err
		}
		if err := putFactsForFileTx(rawB, byFileB, path, facts); err != nil {
			return fmt.Errorf("ReplaceFactsForFile: %w", err)
		}
		return nil
	})
}

// ReplaceAllFacts atomically clears facts_raw + facts_byfile for the project
// and writes every file's facts in one tx (bulk counterpart to
// ReplaceFactsForFile, mirroring EdgeStore.ReplaceAllEdges). Used by
// full-build paths (WarmCaches, Reindex) so stale rows from a previous build
// never linger. Implements ports.FactStore. C1: caller must NOT hold App.mu.
func (s *Store) ReplaceAllFacts(projectID string, fileFacts map[string][]ports.Fact) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		proj, err := tx.CreateBucketIfNotExists([]byte(projectID))
		if err != nil {
			return err
		}
		if proj.Bucket(bucketFactsRaw) != nil {
			if err := proj.DeleteBucket(bucketFactsRaw); err != nil {
				return fmt.Errorf("ReplaceAllFacts: delete facts_raw: %w", err)
			}
		}
		if proj.Bucket(bucketFactsByFile) != nil {
			if err := proj.DeleteBucket(bucketFactsByFile); err != nil {
				return fmt.Errorf("ReplaceAllFacts: delete facts_byfile: %w", err)
			}
		}
		rawB, err := ensureFactsBucket(proj, bucketFactsRaw)
		if err != nil {
			return err
		}
		byFileB, err := ensureFactsBucket(proj, bucketFactsByFile)
		if err != nil {
			return err
		}
		for path, facts := range fileFacts {
			if len(facts) == 0 {
				continue // nothing to write against a freshly-cleared bucket
			}
			if err := putFactsForFileTx(rawB, byFileB, path, facts); err != nil {
				return fmt.Errorf("ReplaceAllFacts: %w", err)
			}
		}
		return nil
	})
}

// replaceAdjacencyBucket drops and rewrites one dep_fwd/dep_rev bucket
// wholesale from adjMap. Used by PutResolved (§3: "resolved is rewritten
// wholesale by the compactor").
func replaceAdjacencyBucket(proj *bolt.Bucket, name []byte, adjMap map[string][]ports.DepEdge) error {
	if proj.Bucket(name) != nil {
		if err := proj.DeleteBucket(name); err != nil {
			return fmt.Errorf("delete %s: %w", name, err)
		}
	}
	b, err := ensureFactsBucket(proj, name)
	if err != nil {
		return err
	}
	for unit, edges := range adjMap {
		data, err := encodeDepEdges(edges)
		if err != nil {
			return fmt.Errorf("encode %s edges for %q: %w", name, unit, err)
		}
		if err := b.Put([]byte(unit), data); err != nil {
			return fmt.Errorf("put %s %q: %w", name, unit, err)
		}
	}
	return nil
}

// PutResolved writes compactor output: unit records + adjacency, overwriting
// any prior resolved state in one tx (§3, §4). Also stamps facts_meta's
// compacted_at/counts so substrate health is inspectable. Implements
// ports.FactStore. C1: caller must NOT hold App.mu.
func (s *Store) PutResolved(projectID string, units []ports.Fact, adj *ports.DepAdjacency) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		proj, err := tx.CreateBucketIfNotExists([]byte(projectID))
		if err != nil {
			return err
		}

		if proj.Bucket(bucketFactsUnits) != nil {
			if err := proj.DeleteBucket(bucketFactsUnits); err != nil {
				return fmt.Errorf("PutResolved: delete facts_units: %w", err)
			}
		}
		unitsB, err := ensureFactsBucket(proj, bucketFactsUnits)
		if err != nil {
			return err
		}
		for _, u := range units {
			data, merr := json.Marshal(u)
			if merr != nil {
				return fmt.Errorf("PutResolved: marshal unit %q: %w", u.Subject, merr)
			}
			if err := unitsB.Put([]byte(u.Subject), data); err != nil {
				return fmt.Errorf("PutResolved: put unit %q: %w", u.Subject, err)
			}
		}

		var fwd, rev map[string][]ports.DepEdge
		if adj != nil {
			fwd, rev = adj.Fwd, adj.Rev
		}
		if err := replaceAdjacencyBucket(proj, bucketFactsDepFwd, fwd); err != nil {
			return fmt.Errorf("PutResolved: dep_fwd: %w", err)
		}
		if err := replaceAdjacencyBucket(proj, bucketFactsDepRev, rev); err != nil {
			return fmt.Errorf("PutResolved: dep_rev: %w", err)
		}

		metaB, err := ensureFactsBucket(proj, bucketFactsMeta)
		if err != nil {
			return err
		}
		edgeCount := 0
		for _, es := range fwd {
			edgeCount += len(es)
		}
		counts := fmt.Sprintf(`{"units":%d,"edges":%d}`, len(units), edgeCount)
		if err := metaB.Put([]byte("compacted_at"), []byte(strconv.FormatInt(time.Now().Unix(), 10))); err != nil {
			return fmt.Errorf("PutResolved: meta compacted_at: %w", err)
		}
		if err := metaB.Put([]byte("counts"), []byte(counts)); err != nil {
			return fmt.Errorf("PutResolved: meta counts: %w", err)
		}
		if err := metaB.Put([]byte("schema_version"), []byte(strconv.Itoa(factsCompactSchemaVersion))); err != nil {
			return fmt.Errorf("PutResolved: meta schema_version: %w", err)
		}
		return nil
	})
}

// factsCompactSchemaVersion mirrors domain/facts.FactsSchemaVersion, kept as
// a literal (not an import) so this adapter stays domain-independent
// (hexagonal law, CLAUDE.md: adapters depend on ports, not domain
// packages). Bump in lockstep with facts.FactsSchemaVersion whenever the
// compactor's output shape changes.
const factsCompactSchemaVersion = 1

// PutFindings writes compact-time detector output (FDN-3, D27): FactFinding
// facts keyed by rule\x00subject (§3 bucket layout), overwriting the entire
// fact_findings bucket wholesale — findings are recomputed fresh every
// compaction (internal/domain/facts/detect.go), never merged with a prior
// run's stale rows. Implements ports.FactStore. C1: caller must NOT hold
// App.mu.
func (s *Store) PutFindings(projectID string, findings []ports.Fact) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		proj, err := tx.CreateBucketIfNotExists([]byte(projectID))
		if err != nil {
			return err
		}
		if proj.Bucket(bucketFactFindings) != nil {
			if err := proj.DeleteBucket(bucketFactFindings); err != nil {
				return fmt.Errorf("PutFindings: delete fact_findings: %w", err)
			}
		}
		b, err := ensureFactsBucket(proj, bucketFactFindings)
		if err != nil {
			return err
		}
		for _, f := range findings {
			rule := f.Attrs["rule"]
			key := []byte(rule + "\x00" + f.Subject)
			data, merr := json.Marshal(f)
			if merr != nil {
				return fmt.Errorf("PutFindings: marshal %q/%q: %w", rule, f.Subject, merr)
			}
			if err := b.Put(key, data); err != nil {
				return fmt.Errorf("PutFindings: put %q/%q: %w", rule, f.Subject, err)
			}
		}
		return nil
	})
}

// FactsMeta returns facts_meta's recorded keys (schema_version,
// compacted_at, counts) as a string map, or nil if this project has never
// been compacted. Implements ports.FactStore.
func (s *Store) FactsMeta(projectID string) (map[string]string, error) {
	var result map[string]string
	err := s.db.View(func(tx *bolt.Tx) error {
		proj := tx.Bucket([]byte(projectID))
		if proj == nil {
			return nil
		}
		b := openFactsBucket(proj, bucketFactsMeta)
		if b == nil {
			return nil
		}
		result = make(map[string]string)
		return b.ForEach(func(k, v []byte) error {
			if isMetaKey(k) {
				return nil
			}
			result[string(k)] = string(v)
			return nil
		})
	})
	return result, err
}

// collectFactsBucket appends every Fact value in b (a single-Fact-per-key
// bucket such as facts_units or fact_findings) to out, skipping the
// _version marker. Corrupt rows are skipped, not fatal — a scan should not
// abort because one row is unreadable.
func collectFactsBucket(b *bolt.Bucket, out *[]ports.Fact) error {
	if b == nil {
		return nil
	}
	return b.ForEach(func(k, v []byte) error {
		if isMetaKey(k) {
			return nil
		}
		var f ports.Fact
		cp := append([]byte(nil), v...)
		if err := json.Unmarshal(cp, &f); err != nil {
			return nil
		}
		*out = append(*out, f)
		return nil
	})
}

// FactsByKind returns every fact of the given kind. FactUnit reads
// facts_units; FactFinding reads the fact-substrate findings bucket; every
// other kind (dep/route/schema/deploy/owner) scans facts_raw — the parser
// only ever emits raw facts at file grain (§2.1: unit facts are synthesized
// by the compactor, not the parser). FactDelta facts are transient (§4.2,
// never persisted) so always return empty. Implements ports.FactStore.
func (s *Store) FactsByKind(projectID string, kind ports.FactKind) ([]ports.Fact, error) {
	var result []ports.Fact
	err := s.db.View(func(tx *bolt.Tx) error {
		proj := tx.Bucket([]byte(projectID))
		if proj == nil {
			return nil
		}
		switch kind {
		case ports.FactUnit:
			return collectFactsBucket(openFactsBucket(proj, bucketFactsUnits), &result)
		case ports.FactFinding:
			return collectFactsBucket(openFactsBucket(proj, bucketFactFindings), &result)
		default:
			rb := openFactsBucket(proj, bucketFactsRaw)
			if rb == nil {
				return nil
			}
			return rb.ForEach(func(k, v []byte) error {
				if isMetaKey(k) {
					return nil
				}
				var f ports.Fact
				cp := append([]byte(nil), v...)
				if err := json.Unmarshal(cp, &f); err != nil {
					return nil // skip corrupt row
				}
				if f.Kind == kind {
					result = append(result, f)
				}
				return nil
			})
		}
	})
	return result, err
}

// FactsForSubject returns every fact whose Subject equals subject, scanning
// facts_raw (evidence), facts_units (resolved unit record), and the
// fact-substrate findings bucket. This is the evidence-pack query
// (`aoa arch facts <subject>`, §3/§5). Implements ports.FactStore.
func (s *Store) FactsForSubject(projectID, subject string) ([]ports.Fact, error) {
	var result []ports.Fact
	err := s.db.View(func(tx *bolt.Tx) error {
		proj := tx.Bucket([]byte(projectID))
		if proj == nil {
			return nil
		}
		for _, name := range [][]byte{bucketFactsRaw, bucketFactsUnits, bucketFactFindings} {
			b := openFactsBucket(proj, name)
			if b == nil {
				continue
			}
			if err := b.ForEach(func(k, v []byte) error {
				if isMetaKey(k) {
					return nil
				}
				var f ports.Fact
				cp := append([]byte(nil), v...)
				if err := json.Unmarshal(cp, &f); err != nil {
					return nil // skip corrupt row
				}
				if f.Subject == subject {
					result = append(result, f)
				}
				return nil
			}); err != nil {
				return err
			}
		}
		return nil
	})
	return result, err
}

// readDepEdges is the shared O(1) bucket-get + posting-list-decode path for
// Dependencies/Dependents (§3, §5: ≤50µs warm).
func (s *Store) readDepEdges(projectID string, bucketName []byte, unit string) ([]ports.DepEdge, error) {
	var result []ports.DepEdge
	err := s.db.View(func(tx *bolt.Tx) error {
		proj := tx.Bucket([]byte(projectID))
		if proj == nil {
			return nil
		}
		b := openFactsBucket(proj, bucketName)
		if b == nil {
			return nil
		}
		v := b.Get([]byte(unit))
		if v == nil {
			return nil
		}
		cp := append([]byte(nil), v...)
		edges, err := decodeDepEdges(cp)
		if err != nil {
			return fmt.Errorf("readDepEdges: unit %q: %w", unit, err)
		}
		result = edges
		return nil
	})
	return result, err
}

// Dependencies returns unit's resolved outbound edges (what it imports).
// Implements ports.FactStore.
func (s *Store) Dependencies(projectID, unit string) ([]ports.DepEdge, error) {
	return s.readDepEdges(projectID, bucketFactsDepFwd, unit)
}

// Dependents returns unit's resolved inbound edges (who imports it).
// Implements ports.FactStore.
func (s *Store) Dependents(projectID, unit string) ([]ports.DepEdge, error) {
	return s.readDepEdges(projectID, bucketFactsDepRev, unit)
}

// SaveBaseline persists a frozen FactBaseline as gzip-compressed JSON (§3
// bucket table: "facts_baseline ... (gzip JSON)"). Implements ports.FactStore.
// C1: caller must NOT hold App.mu.
func (s *Store) SaveBaseline(projectID, name string, b *ports.FactBaseline) error {
	raw, err := json.Marshal(b)
	if err != nil {
		return fmt.Errorf("SaveBaseline: marshal %q: %w", name, err)
	}
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	if _, err := gw.Write(raw); err != nil {
		return fmt.Errorf("SaveBaseline: gzip %q: %w", name, err)
	}
	if err := gw.Close(); err != nil {
		return fmt.Errorf("SaveBaseline: gzip close %q: %w", name, err)
	}
	data := buf.Bytes()

	return s.db.Update(func(tx *bolt.Tx) error {
		proj, err := tx.CreateBucketIfNotExists([]byte(projectID))
		if err != nil {
			return err
		}
		bb, err := ensureFactsBucket(proj, bucketFactsBaseline)
		if err != nil {
			return err
		}
		return bb.Put([]byte(name), data)
	})
}

// LoadBaseline returns the named baseline, or nil, nil if absent.
// Implements ports.FactStore.
func (s *Store) LoadBaseline(projectID, name string) (*ports.FactBaseline, error) {
	var result *ports.FactBaseline
	err := s.db.View(func(tx *bolt.Tx) error {
		proj := tx.Bucket([]byte(projectID))
		if proj == nil {
			return nil
		}
		bb := openFactsBucket(proj, bucketFactsBaseline)
		if bb == nil {
			return nil
		}
		v := bb.Get([]byte(name))
		if v == nil {
			return nil
		}
		cp := append([]byte(nil), v...)
		gr, err := gzip.NewReader(bytes.NewReader(cp))
		if err != nil {
			return fmt.Errorf("LoadBaseline: gzip reader %q: %w", name, err)
		}
		raw, err := io.ReadAll(gr)
		if err != nil {
			return fmt.Errorf("LoadBaseline: gzip read %q: %w", name, err)
		}
		var fb ports.FactBaseline
		if err := json.Unmarshal(raw, &fb); err != nil {
			return fmt.Errorf("LoadBaseline: corrupt data for %q: %w", name, err)
		}
		result = &fb
		return nil
	})
	return result, err
}

// DeleteProjectFacts removes every fact-substrate bucket for a project.
// Idempotent: deleting facts for a nonexistent project is not an error.
// Wired into the project-cleanup path (Store.DeleteProject calls the same
// deleteFactsBuckets helper) so `aoa remove`/`aoa reset` clean the substrate
// explicitly, even though DeleteProject's own DeleteBucket on the whole
// project bucket already recursively removes everything nested inside it.
// Implements ports.FactStore. C1: caller must NOT hold App.mu.
func (s *Store) DeleteProjectFacts(projectID string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		proj := tx.Bucket([]byte(projectID))
		if proj == nil {
			return nil // idempotent — nothing to delete
		}
		return deleteFactsBuckets(proj)
	})
}
