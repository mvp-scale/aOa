package bbolt

import (
	"encoding/binary"
	"encoding/json"
	"fmt"

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
// Returns nil if the bucket does not exist (no error — C3: old DB).
func openEdgesBucket(proj *bolt.Bucket) *bolt.Bucket {
	if proj == nil {
		return nil
	}
	return proj.Bucket(bucketEdges)
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
			return nil // corrupt entry — return empty (graceful degradation)
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
// Implements ports.EdgeStore.
func (s *Store) LoadAllEdges(projectID string) ([]ImportEdge, error) {
	var all []ImportEdge

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
				return nil // skip corrupt entries (graceful degradation)
			}
			all = append(all, edges...)
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return all, nil
}

// ImportEdge is an alias so the test file can reference the concrete type
// without importing ports directly. The bbolt package uses ports.ImportEdge
// everywhere; this alias exists only for the internal JSON encode/decode helpers.
type ImportEdge = ports.ImportEdge
