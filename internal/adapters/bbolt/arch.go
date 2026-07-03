package bbolt

import (
	"bytes"
	"fmt"
	"strings"

	bolt "go.etcd.io/bbolt"
)

// archShardsVersion is the C3 schema version for the arch_shards bucket.
// Bump when the on-disk serialisation format changes (key shape, encoding, etc.).
// Both-direction rollback:
//   - New binary + old DB (no arch_shards bucket) → CreateBucketIfNotExists; silent.
//   - New binary + DB with wrong version → DeleteBucket + CreateBucket + re-derive.
//   - Old binary + DB with arch_shards bucket → bucket is ignored (no arch code).
const archShardsVersion byte = 1

// manifestPrefix is the special key prefix used to store scope manifests
// inside the arch_shards bucket. Manifests are stored under "_manifest/{scope}".
// This prefix is chosen to never collide with shard keys ("{scope}/{view}@{hash}").
const manifestPrefix = "_manifest/"

// openArchBucket opens the arch_shards sub-bucket inside proj for reading.
// Returns nil if the bucket does not exist or carries a wrong _version (C3).
// A nil return is not an error: callers treat it as "no shards stored".
func openArchBucket(proj *bolt.Bucket) *bolt.Bucket {
	if proj == nil {
		return nil
	}
	ab := proj.Bucket(bucketArchShards)
	if ab == nil {
		return nil
	}
	v := ab.Get(keyVersion)
	if len(v) == 0 || v[0] != archShardsVersion {
		return nil // missing or wrong version — treat as empty (C3: stale/future data)
	}
	return ab
}

// ensureArchBucket opens or creates the arch_shards sub-bucket, enforcing C3:
//   - New bucket → write archShardsVersion and return.
//   - Existing bucket, correct version → return as-is.
//   - Existing bucket, wrong version → DeleteBucket + CreateBucket + write version.
func ensureArchBucket(proj *bolt.Bucket) (*bolt.Bucket, error) {
	ab, err := proj.CreateBucketIfNotExists(bucketArchShards)
	if err != nil {
		return nil, fmt.Errorf("create arch_shards bucket: %w", err)
	}

	v := ab.Get(keyVersion)
	if len(v) == 0 {
		// Fresh bucket — write the version byte.
		if err := ab.Put(keyVersion, []byte{archShardsVersion}); err != nil {
			return nil, fmt.Errorf("write arch_shards version: %w", err)
		}
		return ab, nil
	}
	if v[0] == archShardsVersion {
		return ab, nil // correct version
	}

	// Version mismatch — drop and re-create (C3: shards are pure cache).
	if err := proj.DeleteBucket(bucketArchShards); err != nil {
		return nil, fmt.Errorf("delete mismatched arch_shards bucket: %w", err)
	}
	ab, err = proj.CreateBucket(bucketArchShards)
	if err != nil {
		return nil, fmt.Errorf("recreate arch_shards bucket: %w", err)
	}
	if err := ab.Put(keyVersion, []byte{archShardsVersion}); err != nil {
		return nil, fmt.Errorf("write arch_shards version after recreate: %w", err)
	}
	return ab, nil
}

// SaveShards atomically writes a batch of rendered shard JSON blobs.
// Each key must be formatted as "{scope}/{view}@{hash}".
// C1: caller must NOT hold App.mu.
// Implements ports.ArchStore.
func (s *Store) SaveShards(projectID string, shards map[string][]byte) error {
	if len(shards) == 0 {
		return nil
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		proj, err := tx.CreateBucketIfNotExists([]byte(projectID))
		if err != nil {
			return err
		}
		ab, err := ensureArchBucket(proj)
		if err != nil {
			return err
		}
		for key, data := range shards {
			if err := ab.Put([]byte(key), data); err != nil {
				return fmt.Errorf("SaveShards put %q: %w", key, err)
			}
		}
		return nil
	})
}

// LoadShard returns the raw JSON bytes for a shard by key.
// Key format: "{scope}/{view}@{hash}". Returns nil, nil if not found.
// Implements ports.ArchStore.
func (s *Store) LoadShard(projectID, key string) ([]byte, error) {
	var result []byte
	err := s.db.View(func(tx *bolt.Tx) error {
		proj := tx.Bucket([]byte(projectID))
		ab := openArchBucket(proj)
		if ab == nil {
			return nil // no bucket or wrong version → empty (C3)
		}
		v := ab.Get([]byte(key))
		if v == nil {
			return nil // key absent
		}
		// Copy bytes out of transaction scope.
		cp := make([]byte, len(v))
		copy(cp, v)
		result = cp
		return nil
	})
	return result, err
}

// SaveManifest writes the manifest JSON for a scope.
// Stored under key "_manifest/{scope}" in the arch_shards bucket.
// C1: caller must NOT hold App.mu.
// Implements ports.ArchStore.
func (s *Store) SaveManifest(projectID, scope string, data []byte) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		proj, err := tx.CreateBucketIfNotExists([]byte(projectID))
		if err != nil {
			return err
		}
		ab, err := ensureArchBucket(proj)
		if err != nil {
			return err
		}
		key := manifestPrefix + scope
		return ab.Put([]byte(key), data)
	})
}

// LoadManifest returns the manifest JSON for a scope.
// Returns nil, nil if no manifest has been written yet.
// Implements ports.ArchStore.
func (s *Store) LoadManifest(projectID, scope string) ([]byte, error) {
	var result []byte
	err := s.db.View(func(tx *bolt.Tx) error {
		proj := tx.Bucket([]byte(projectID))
		ab := openArchBucket(proj)
		if ab == nil {
			return nil
		}
		key := manifestPrefix + scope
		v := ab.Get([]byte(key))
		if v == nil {
			return nil
		}
		cp := make([]byte, len(v))
		copy(cp, v)
		result = cp
		return nil
	})
	return result, err
}

// DeleteShardsForScope removes all shard entries whose key starts with
// "{scope}/" AND the manifest for that scope. Idempotent.
// C1: caller must NOT hold App.mu.
// Implements ports.ArchStore.
func (s *Store) DeleteShardsForScope(projectID, scope string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		proj := tx.Bucket([]byte(projectID))
		if proj == nil {
			return nil // nothing to delete
		}
		ab := openArchBucket(proj)
		if ab == nil {
			return nil // bucket absent or wrong version — treat as empty
		}

		scopePrefix := []byte(scope + "/")
		manifestKey := []byte(manifestPrefix + scope)

		// Collect keys to delete (avoid modifying cursor during iteration).
		var toDelete [][]byte
		c := ab.Cursor()
		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			if bytes.HasPrefix(k, scopePrefix) || bytes.Equal(k, manifestKey) {
				cp := make([]byte, len(k))
				copy(cp, k)
				toDelete = append(toDelete, cp)
			}
		}
		for _, k := range toDelete {
			if err := ab.Delete(k); err != nil {
				return fmt.Errorf("DeleteShardsForScope delete %q: %w", k, err)
			}
		}
		return nil
	})
}

// HasArchBucket reports whether the arch_shards bucket exists and carries
// the correct schema version for the project. Read-only (db.View). C1 n/a.
// Implements ports.ArchStore.
func (s *Store) HasArchBucket(projectID string) bool {
	var found bool
	_ = s.db.View(func(tx *bolt.Tx) error {
		proj := tx.Bucket([]byte(projectID))
		ab := openArchBucket(proj)
		found = ab != nil
		return nil
	})
	return found
}

// scopeOf extracts the scope portion from a shard key like "{scope}/{view}@{hash}".
// Returns "" if the key does not contain "/".
func scopeOf(key string) string {
	if idx := strings.Index(key, "/"); idx >= 0 {
		return key[:idx]
	}
	return ""
}
