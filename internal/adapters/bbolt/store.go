// Package bbolt implements the ports.Storage interface using bbolt (embedded B+ tree).
// Each project gets its own top-level bucket. Within that bucket, "index" and "learner"
// sub-buckets hold JSON-serialized data. Writes are transactional — a crash mid-write
// cannot corrupt previously committed data.
package bbolt

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/corey/aoa/internal/ports"
	bolt "go.etcd.io/bbolt"
)

// Bucket keys
var (
	bucketIndex   = []byte("index")
	bucketLearner = []byte("learner")
	keyTokens     = []byte("tokens")
	keyMetadata   = []byte("metadata")
	keyFiles      = []byte("files")
	keyState      = []byte("state")
)

// Store implements ports.Storage backed by bbolt.
type Store struct {
	db *bolt.DB
}

// NewStore opens (or creates) a bbolt database at the given path.
func NewStore(path string) (*Store, error) {
	db, err := bolt.Open(path, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("bbolt open: %w", err)
	}
	return &Store{db: db}, nil
}

// Close closes the underlying bbolt database.
func (s *Store) Close() error {
	return s.db.Close()
}

// metadataKey encodes a TokenRef as a string key for JSON map serialization.
func metadataKey(ref ports.TokenRef) string {
	return fmt.Sprintf("%d:%d", ref.FileID, ref.Line)
}

// tokenRefKey encodes a TokenRef as a string for JSON map keys.
// Same format as metadataKey — "fileID:line".
type tokenRefKey = string

// indexJSON is the JSON-serializable form of ports.Index.
// Map keys must be strings in JSON, so TokenRef keys are encoded as "fileID:line"
// and uint32 file IDs are encoded as decimal strings.
type indexJSON struct {
	Tokens   map[string][]ports.TokenRef    `json:"tokens"`
	Metadata map[tokenRefKey]*ports.SymbolMeta `json:"metadata"`
	Files    map[string]*ports.FileMeta     `json:"files"`
}

// SaveIndex persists the full search index for a project.
func (s *Store) SaveIndex(projectID string, idx *ports.Index) error {
	if idx == nil {
		return fmt.Errorf("nil index")
	}

	// Convert to JSON-serializable form
	ij := indexJSON{
		Tokens:   idx.Tokens,
		Metadata: make(map[tokenRefKey]*ports.SymbolMeta, len(idx.Metadata)),
		Files:    make(map[string]*ports.FileMeta, len(idx.Files)),
	}

	for ref, sym := range idx.Metadata {
		ij.Metadata[metadataKey(ref)] = sym
	}
	for fid, fm := range idx.Files {
		ij.Files[fmt.Sprintf("%d", fid)] = fm
	}

	tokensJSON, err := json.Marshal(ij.Tokens)
	if err != nil {
		return fmt.Errorf("marshal tokens: %w", err)
	}
	metaJSON, err := json.Marshal(ij.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}
	filesJSON, err := json.Marshal(ij.Files)
	if err != nil {
		return fmt.Errorf("marshal files: %w", err)
	}

	return s.db.Update(func(tx *bolt.Tx) error {
		proj, err := tx.CreateBucketIfNotExists([]byte(projectID))
		if err != nil {
			return err
		}
		ib, err := proj.CreateBucketIfNotExists(bucketIndex)
		if err != nil {
			return err
		}
		if err := ib.Put(keyTokens, tokensJSON); err != nil {
			return err
		}
		if err := ib.Put(keyMetadata, metaJSON); err != nil {
			return err
		}
		return ib.Put(keyFiles, filesJSON)
	})
}

// LoadIndex retrieves the search index for a project.
// Returns nil, nil if no index exists (fresh project).
func (s *Store) LoadIndex(projectID string) (*ports.Index, error) {
	var tokensJSON, metaJSON, filesJSON []byte

	err := s.db.View(func(tx *bolt.Tx) error {
		proj := tx.Bucket([]byte(projectID))
		if proj == nil {
			return nil
		}
		ib := proj.Bucket(bucketIndex)
		if ib == nil {
			return nil
		}
		// Copy bytes out of the transaction (bbolt slices are only valid within tx)
		if v := ib.Get(keyTokens); v != nil {
			tokensJSON = make([]byte, len(v))
			copy(tokensJSON, v)
		}
		if v := ib.Get(keyMetadata); v != nil {
			metaJSON = make([]byte, len(v))
			copy(metaJSON, v)
		}
		if v := ib.Get(keyFiles); v != nil {
			filesJSON = make([]byte, len(v))
			copy(filesJSON, v)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if tokensJSON == nil && metaJSON == nil && filesJSON == nil {
		return nil, nil
	}

	idx := &ports.Index{
		Tokens:   make(map[string][]ports.TokenRef),
		Metadata: make(map[ports.TokenRef]*ports.SymbolMeta),
		Files:    make(map[uint32]*ports.FileMeta),
	}

	if tokensJSON != nil {
		if err := json.Unmarshal(tokensJSON, &idx.Tokens); err != nil {
			return nil, fmt.Errorf("unmarshal tokens: %w", err)
		}
	}

	if metaJSON != nil {
		var rawMeta map[string]*ports.SymbolMeta
		if err := json.Unmarshal(metaJSON, &rawMeta); err != nil {
			return nil, fmt.Errorf("unmarshal metadata: %w", err)
		}
		for k, sym := range rawMeta {
			var fileID uint32
			var line uint16
			if _, err := fmt.Sscanf(k, "%d:%d", &fileID, &line); err != nil {
				return nil, fmt.Errorf("parse metadata key %q: %w", k, err)
			}
			idx.Metadata[ports.TokenRef{FileID: fileID, Line: line}] = sym
		}
	}

	if filesJSON != nil {
		var rawFiles map[string]*ports.FileMeta
		if err := json.Unmarshal(filesJSON, &rawFiles); err != nil {
			return nil, fmt.Errorf("unmarshal files: %w", err)
		}
		for k, fm := range rawFiles {
			var fid uint32
			if _, err := fmt.Sscanf(k, "%d", &fid); err != nil {
				return nil, fmt.Errorf("parse file key %q: %w", k, err)
			}
			idx.Files[fid] = fm
		}
	}

	return idx, nil
}

// SaveLearnerState persists the full learner state for a project.
func (s *Store) SaveLearnerState(projectID string, state *ports.LearnerState) error {
	if state == nil {
		return fmt.Errorf("nil learner state")
	}

	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal learner state: %w", err)
	}

	return s.db.Update(func(tx *bolt.Tx) error {
		proj, err := tx.CreateBucketIfNotExists([]byte(projectID))
		if err != nil {
			return err
		}
		lb, err := proj.CreateBucketIfNotExists(bucketLearner)
		if err != nil {
			return err
		}
		return lb.Put(keyState, data)
	})
}

// LoadLearnerState retrieves learner state for a project.
// Returns nil, nil if no state exists (fresh project).
func (s *Store) LoadLearnerState(projectID string) (*ports.LearnerState, error) {
	var data []byte

	err := s.db.View(func(tx *bolt.Tx) error {
		proj := tx.Bucket([]byte(projectID))
		if proj == nil {
			return nil
		}
		lb := proj.Bucket(bucketLearner)
		if lb == nil {
			return nil
		}
		if v := lb.Get(keyState); v != nil {
			data = make([]byte, len(v))
			copy(data, v)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if data == nil {
		return nil, nil
	}

	var state ports.LearnerState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("unmarshal learner state: %w", err)
	}
	return &state, nil
}

// DeleteProject removes all data (index + learner state) for a project.
// Idempotent: deleting a nonexistent project is not an error.
func (s *Store) DeleteProject(projectID string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		if err := tx.DeleteBucket([]byte(projectID)); err == bolt.ErrBucketNotFound {
			return nil // idempotent
		} else {
			return err
		}
	})
}
