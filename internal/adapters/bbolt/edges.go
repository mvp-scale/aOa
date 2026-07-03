package bbolt

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/corey/aoa/internal/ports"
	bolt "go.etcd.io/bbolt"
)

// edgesVersion is the C3 schema version for the edges, arch_shards, and
// facts_unresolved buckets. Bump this when the serialisation format changes.
// Old binaries (no EdgeStore code) simply ignore these buckets; a version
// mismatch triggers a drop-and-re-create (buckets are pure cache).
const edgesVersion byte = 1

// fileIDKey encodes a uint32 fileID as a 4-byte big-endian key.
// Big-endian ensures lexicographic order == numeric order in bbolt cursors.
// Keys are 4 bytes; _version key is 8 bytes ("_version") — no length collision.
func fileIDKey(fileID uint32) []byte {
	key := make([]byte, 4)
	binary.BigEndian.PutUint32(key, fileID)
	return key
}

// openEdgesBucket opens the edges sub-bucket inside proj for reading.
// Returns nil if the bucket does not exist or carries a wrong _version (C3/T38).
// A nil return is not an error: callers treat it as "no edges stored".
// Version check on the read path prevents silently returning corrupt or
// future-format data when a new binary reads a bucket it has not yet rewritten.
func openEdgesBucket(proj *bolt.Bucket) *bolt.Bucket {
	if proj == nil {
		return nil
	}
	eb := proj.Bucket(bucketEdges)
	if eb == nil {
		return nil
	}
	v := eb.Get(keyVersion)
	if len(v) == 0 || v[0] != edgesVersion {
		return nil // missing or wrong version — treat as empty (C3: stale/future data)
	}
	return eb
}

// ensureEdgesBucket opens or creates the edges sub-bucket inside proj for
// writing, enforcing the C3 version contract:
//
//   - New bucket (no _version key) → write edgesVersion, return bucket.
//   - Existing bucket, correct version → return as-is.
//   - Existing bucket, wrong version → DeleteBucket + CreateBucket + write
//     edgesVersion (edges are cache — safe to drop and re-derive).
func ensureEdgesBucket(proj *bolt.Bucket) (*bolt.Bucket, error) {
	eb, err := proj.CreateBucketIfNotExists(bucketEdges)
	if err != nil {
		return nil, fmt.Errorf("create edges bucket: %w", err)
	}

	v := eb.Get(keyVersion)
	if len(v) == 0 {
		// Fresh bucket — write the version byte.
		if err := eb.Put(keyVersion, []byte{edgesVersion}); err != nil {
			return nil, fmt.Errorf("write edges version: %w", err)
		}
		return eb, nil
	}

	if v[0] == edgesVersion {
		return eb, nil // already correct version
	}

	// Version mismatch — drop and re-create (C3: cache-only, always re-derivable).
	if err := proj.DeleteBucket(bucketEdges); err != nil {
		return nil, fmt.Errorf("delete mismatched edges bucket: %w", err)
	}
	eb, err = proj.CreateBucket(bucketEdges)
	if err != nil {
		return nil, fmt.Errorf("recreate edges bucket: %w", err)
	}
	if err := eb.Put(keyVersion, []byte{edgesVersion}); err != nil {
		return nil, fmt.Errorf("write edges version after recreate: %w", err)
	}
	return eb, nil
}

// SaveEdgesBatch writes all accumulated per-file edge deltas in a single bbolt
// write transaction — C2 burst coalescing (L19.12).
//
// For each entry in fileEdges:
//   - len(edges) > 0  → bucket.Put  (save / overwrite)
//   - len(edges) == 0 → bucket.Delete (remove stale entry for a deleted file)
//
// The entire map is written atomically: any error aborts the whole tx.
// Implements ports.EdgeStore. C1: caller must NOT hold App.mu.
func (s *Store) SaveEdgesBatch(projectID string, fileEdges map[uint32][]ImportEdge) error {
	if len(fileEdges) == 0 {
		return nil
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		proj, err := tx.CreateBucketIfNotExists([]byte(projectID))
		if err != nil {
			return err
		}
		eb, err := ensureEdgesBucket(proj)
		if err != nil {
			return err
		}
		for fileID, edges := range fileEdges {
			key := fileIDKey(fileID)
			if len(edges) == 0 {
				// Empty slice signals "delete this file's edges".
				if err := eb.Delete(key); err != nil {
					return fmt.Errorf("batch delete fileID %d: %w", fileID, err)
				}
			} else {
				data, err := json.Marshal(edges)
				if err != nil {
					return fmt.Errorf("batch marshal fileID %d: %w", fileID, err)
				}
				if err := eb.Put(key, data); err != nil {
					return fmt.Errorf("batch put fileID %d: %w", fileID, err)
				}
			}
		}
		return nil
	})
}

// SaveEdgesForFile replaces all edges for a single file.
// Implements ports.EdgeStore. C1: caller must NOT hold App.mu.
func (s *Store) SaveEdgesForFile(projectID string, fileID uint32, edges []ImportEdge) error {
	data, err := json.Marshal(edges)
	if err != nil {
		return fmt.Errorf("marshal edges for file %d: %w", fileID, err)
	}

	return s.db.Update(func(tx *bolt.Tx) error {
		proj, err := tx.CreateBucketIfNotExists([]byte(projectID))
		if err != nil {
			return err
		}
		eb, err := ensureEdgesBucket(proj)
		if err != nil {
			return err
		}
		return eb.Put(fileIDKey(fileID), data)
	})
}

// LoadEdgesForFile returns all edges for a single file.
// Returns nil, nil if no edges exist for that file or if the bucket is absent.
// Implements ports.EdgeStore. C3: missing bucket → empty result, no panic.
func (s *Store) LoadEdgesForFile(projectID string, fileID uint32) ([]ImportEdge, error) {
	var result []ImportEdge

	err := s.db.View(func(tx *bolt.Tx) error {
		proj := tx.Bucket([]byte(projectID))
		if proj == nil {
			return nil
		}
		eb := openEdgesBucket(proj)
		if eb == nil {
			return nil // no bucket → empty (C3: new binary opens old DB)
		}
		v := eb.Get(fileIDKey(fileID))
		if v == nil {
			return nil // file absent
		}
		// Copy bytes out of the transaction before json.Unmarshal.
		cp := make([]byte, len(v))
		copy(cp, v)
		var edges []ImportEdge
		if err := json.Unmarshal(cp, &edges); err != nil {
			// T38: surface corrupt entries — never silently return zero-values to F2.
			// Return the error so callers know the row was unreadable; they can
			// decide whether to treat it as empty or propagate (G7: no silent zeros).
			return fmt.Errorf("LoadEdgesForFile: corrupt data for fileID %d: %w", fileID, err)
		}
		result = edges
		return nil
	})
	return result, err
}

// DeleteEdgesForFile removes all edges for a single file.
// O(1) bbolt Delete per fileID — no full scan. Idempotent.
// Implements ports.EdgeStore. C1: caller must NOT hold App.mu.
func (s *Store) DeleteEdgesForFile(projectID string, fileID uint32) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		proj := tx.Bucket([]byte(projectID))
		if proj == nil {
			return nil // no project → nothing to delete (idempotent)
		}
		eb := openEdgesBucket(proj)
		if eb == nil {
			return nil // no bucket → nothing to delete (idempotent)
		}
		return eb.Delete(fileIDKey(fileID))
	})
}

// LoadAllEdges returns every edge stored for the project, across all files.
// Iterates the edges bucket in key order, skipping the _version metadata key.
// Returns nil, nil if no edges bucket or project exists.
// T38: corrupt rows are skipped but counted — if any were skipped the returned
// error carries the count so callers (and F2's input pipeline) are never handed
// silent zero-values from unreadable rows (G7: never silent zeros).
// Implements ports.EdgeStore.
func (s *Store) LoadAllEdges(projectID string) ([]ImportEdge, error) {
	var all []ImportEdge
	var skipped int

	err := s.db.View(func(tx *bolt.Tx) error {
		proj := tx.Bucket([]byte(projectID))
		if proj == nil {
			return nil
		}
		eb := openEdgesBucket(proj)
		if eb == nil {
			return nil
		}

		return eb.ForEach(func(k, v []byte) error {
			// Skip metadata keys: _version is 8 bytes; fileID keys are 4 bytes.
			if len(k) != 4 {
				return nil
			}
			// Copy value out of the transaction.
			cp := make([]byte, len(v))
			copy(cp, v)
			var edges []ImportEdge
			if err := json.Unmarshal(cp, &edges); err != nil {
				// T38: count corrupt entries — never silently feed zeros to F2.
				// Continue iterating so valid rows are still returned.
				skipped++
				return nil
			}
			all = append(all, edges...)
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	if skipped > 0 {
		return all, fmt.Errorf("LoadAllEdges: %d corrupt entry/entries skipped", skipped)
	}
	return all, nil
}

// ensureUnresolvedBucket opens or creates the facts_unresolved sub-bucket inside
// proj for writing, enforcing the same C3 version contract as ensureEdgesBucket.
func ensureUnresolvedBucket(proj *bolt.Bucket) (*bolt.Bucket, error) {
	ub, err := proj.CreateBucketIfNotExists(bucketFactsUnresolved)
	if err != nil {
		return nil, fmt.Errorf("create facts_unresolved bucket: %w", err)
	}
	v := ub.Get(keyVersion)
	if len(v) == 0 {
		if err := ub.Put(keyVersion, []byte{edgesVersion}); err != nil {
			return nil, fmt.Errorf("write facts_unresolved version: %w", err)
		}
		return ub, nil
	}
	if v[0] == edgesVersion {
		return ub, nil
	}
	// Version mismatch — drop and re-create (C3: cache-only, always re-derivable).
	if err := proj.DeleteBucket(bucketFactsUnresolved); err != nil {
		return nil, fmt.Errorf("delete mismatched facts_unresolved bucket: %w", err)
	}
	ub, err = proj.CreateBucket(bucketFactsUnresolved)
	if err != nil {
		return nil, fmt.Errorf("recreate facts_unresolved bucket: %w", err)
	}
	if err := ub.Put(keyVersion, []byte{edgesVersion}); err != nil {
		return nil, fmt.Errorf("write facts_unresolved version after recreate: %w", err)
	}
	return ub, nil
}

// SaveUnresolved persists import specs that looked intra-repo but did not
// resolve to any known file in the current index. Keyed by ImportPath+"\x00"+
// FromFile+"\x00"+StartLine for per-site deduplication. Idempotent.
// Implements ports.EdgeStore. C1: caller must NOT hold App.mu.
func (s *Store) SaveUnresolved(projectID string, entries []ImportEdge) error {
	if len(entries) == 0 {
		return nil
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		proj, err := tx.CreateBucketIfNotExists([]byte(projectID))
		if err != nil {
			return err
		}
		ub, err := ensureUnresolvedBucket(proj)
		if err != nil {
			return err
		}
		for _, e := range entries {
			// keyed by NUL-separated spec, fromFile, and line for per-site deduplication.
			key := []byte(e.ImportPath + "\x00" + e.FromFile + "\x00" +
				strconv.FormatUint(uint64(e.StartLine), 10))
			data, err := json.Marshal(e)
			if err != nil {
				continue // skip corrupt; individual failures are non-fatal
			}
			if err := ub.Put(key, data); err != nil {
				return fmt.Errorf("put unresolved %q: %w", e.ImportPath, err)
			}
		}
		return nil
	})
}

// ReplaceAllEdges atomically clears the edges and facts_unresolved buckets for
// the project and writes the new file→edges map in a single bbolt write
// transaction.  The drop-and-recreate pattern eliminates phantom rows from
// deleted or renumbered files on WarmCaches / Reindex rebuilds (finding 9 /
// T34).  Clearing facts_unresolved on the same rebuild prevents stale broken-
// import records from accumulating across Reindex cycles (finding R8 / T42).
// Passing nil or empty fileEdges still clears both buckets (safe reset).
// All-or-nothing: any error aborts the whole tx.
// Implements ports.EdgeStore. C1: caller must NOT hold App.mu.
func (s *Store) ReplaceAllEdges(projectID string, fileEdges map[uint32][]ImportEdge) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		proj, err := tx.CreateBucketIfNotExists([]byte(projectID))
		if err != nil {
			return err
		}
		// Drop the existing edges bucket (and all its rows) to eliminate stale
		// file IDs from the previous build.  Atomically within this tx.
		if proj.Bucket(bucketEdges) != nil {
			if err := proj.DeleteBucket(bucketEdges); err != nil {
				return fmt.Errorf("replace: delete edges bucket: %w", err)
			}
		}
		// T42: clear facts_unresolved on full rebuild — stale broken-import
		// records from deleted files accumulate forever without this reset.
		// Both buckets are pure cache (always re-derivable); drop-and-recreate
		// is the correct atomic pattern (C3: cache-only, safe to drop).
		if proj.Bucket(bucketFactsUnresolved) != nil {
			if err := proj.DeleteBucket(bucketFactsUnresolved); err != nil {
				return fmt.Errorf("replace: delete facts_unresolved bucket: %w", err)
			}
		}
		if len(fileEdges) == 0 {
			return nil // clear only — nothing to write
		}
		eb, err := ensureEdgesBucket(proj)
		if err != nil {
			return err
		}
		for fileID, edges := range fileEdges {
			if len(edges) == 0 {
				continue // skip empty entries: fresh bucket, nothing to delete
			}
			data, err := json.Marshal(edges)
			if err != nil {
				return fmt.Errorf("replace marshal fileID %d: %w", fileID, err)
			}
			if err := eb.Put(fileIDKey(fileID), data); err != nil {
				return fmt.Errorf("replace put fileID %d: %w", fileID, err)
			}
		}
		return nil
	})
}

// ImportEdge is an alias so the test file can reference the concrete type
// without importing ports directly. The bbolt package uses ports.ImportEdge
// everywhere; this alias exists only for the internal JSON encode/decode helpers.
type ImportEdge = ports.ImportEdge
