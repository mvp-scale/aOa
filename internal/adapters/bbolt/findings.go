package bbolt

import (
	"encoding/json"
	"fmt"

	"github.com/corey/aoa/internal/domain/arch"
	bolt "go.etcd.io/bbolt"
)

// findingsVersion is the C3 schema version for the facts_findings bucket.
// Bump this when the serialisation format changes.
// Old binaries (no FindingsStore code) simply ignore this bucket; a version
// mismatch triggers a drop-and-re-create (findings are pure cache, always re-derivable).
const findingsVersion byte = 1

// bucketFactsFindings is the sub-bucket key for persisted arch findings.
// Declared alongside the other bucket keys in store.go; defined here to avoid
// splitting related constants across files.
var bucketFactsFindings = []byte("facts_findings") // L19.15: finding facts (C3 versioned)

// openFindingsBucket opens the facts_findings sub-bucket inside proj for reading.
// Returns nil if the bucket does not exist or carries a wrong _version (C3).
// A nil return is not an error: callers treat it as "no findings stored".
func openFindingsBucket(proj *bolt.Bucket) *bolt.Bucket {
	if proj == nil {
		return nil
	}
	fb := proj.Bucket(bucketFactsFindings)
	if fb == nil {
		return nil
	}
	v := fb.Get(keyVersion)
	if len(v) == 0 || v[0] != findingsVersion {
		return nil // missing or wrong version — treat as empty (C3: stale/future data)
	}
	return fb
}

// ensureFindingsBucket opens or creates the facts_findings sub-bucket inside proj
// for writing, enforcing the C3 version contract:
//
//   - New bucket (no _version key) → write findingsVersion, return bucket.
//   - Existing bucket, correct version → return as-is.
//   - Existing bucket, wrong version → DeleteBucket + CreateBucket + write
//     findingsVersion (findings are cache — safe to drop and re-derive).
func ensureFindingsBucket(proj *bolt.Bucket) (*bolt.Bucket, error) {
	fb, err := proj.CreateBucketIfNotExists(bucketFactsFindings)
	if err != nil {
		return nil, fmt.Errorf("create facts_findings bucket: %w", err)
	}

	v := fb.Get(keyVersion)
	if len(v) == 0 {
		// Fresh bucket — write the version byte.
		if err := fb.Put(keyVersion, []byte{findingsVersion}); err != nil {
			return nil, fmt.Errorf("write facts_findings version: %w", err)
		}
		return fb, nil
	}

	if v[0] == findingsVersion {
		return fb, nil // already correct version
	}

	// Version mismatch — drop and re-create (C3: cache-only, always re-derivable).
	if err := proj.DeleteBucket(bucketFactsFindings); err != nil {
		return nil, fmt.Errorf("delete mismatched facts_findings bucket: %w", err)
	}
	fb, err = proj.CreateBucket(bucketFactsFindings)
	if err != nil {
		return nil, fmt.Errorf("recreate facts_findings bucket: %w", err)
	}
	if err := fb.Put(keyVersion, []byte{findingsVersion}); err != nil {
		return nil, fmt.Errorf("write facts_findings version after recreate: %w", err)
	}
	return fb, nil
}

// SaveFindings persists findings for a given (projectID, scope) pair.
// The entire slice is serialised as a JSON array and stored under the scope key.
// C1: caller must NOT hold App.mu (snapshot-release-write pattern).
// C3: bucket carries _version byte; version mismatch → drop-and-re-derive.
func (s *Store) SaveFindings(projectID, scope string, findings []arch.Finding) error {
	data, err := json.Marshal(findings)
	if err != nil {
		return fmt.Errorf("SaveFindings: marshal scope %q: %w", scope, err)
	}

	return s.db.Update(func(tx *bolt.Tx) error {
		proj, err := tx.CreateBucketIfNotExists([]byte(projectID))
		if err != nil {
			return err
		}
		fb, err := ensureFindingsBucket(proj)
		if err != nil {
			return err
		}
		return fb.Put([]byte(scope), data)
	})
}

// LoadFindings returns the findings stored for a given (projectID, scope) pair.
// Returns nil, nil if no findings exist for that scope or if the bucket is absent.
// C3: missing bucket or version mismatch → empty result, no panic.
func (s *Store) LoadFindings(projectID, scope string) ([]arch.Finding, error) {
	var result []arch.Finding

	err := s.db.View(func(tx *bolt.Tx) error {
		proj := tx.Bucket([]byte(projectID))
		if proj == nil {
			return nil
		}
		fb := openFindingsBucket(proj)
		if fb == nil {
			return nil // no bucket or wrong version → empty (C3)
		}
		v := fb.Get([]byte(scope))
		if v == nil {
			return nil // scope absent
		}
		// Copy bytes out of the transaction before json.Unmarshal.
		cp := make([]byte, len(v))
		copy(cp, v)
		var findings []arch.Finding
		if err := json.Unmarshal(cp, &findings); err != nil {
			return fmt.Errorf("LoadFindings: corrupt data for scope %q: %w", scope, err)
		}
		result = findings
		return nil
	})
	return result, err
}
