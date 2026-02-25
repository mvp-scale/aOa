// Package bbolt implements the ports.Storage interface using bbolt (embedded B+ tree).
// Each project gets its own top-level bucket. Within that bucket, "index" and "learner"
// sub-buckets hold JSON-serialized data. Writes are transactional — a crash mid-write
// cannot corrupt previously committed data.
package bbolt

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/corey/aoa/internal/domain/analyzer"
	"github.com/corey/aoa/internal/ports"
	bolt "go.etcd.io/bbolt"
)

// Bucket keys
var (
	bucketIndex      = []byte("index")
	bucketLearner    = []byte("learner")
	bucketSessions   = []byte("sessions")
	bucketDimensions = []byte("dimensions")
	keyTokens        = []byte("tokens")
	keyMetadata      = []byte("metadata")
	keyFiles         = []byte("files")
	keyState         = []byte("state")
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

	// Unmarshal the three blobs concurrently — each writes to its own data,
	// no shared state between goroutines.
	var wg sync.WaitGroup
	var tokErr, metaErr, filesErr error
	var rawMeta map[string]*ports.SymbolMeta
	var rawFiles map[string]*ports.FileMeta

	if tokensJSON != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tokErr = json.Unmarshal(tokensJSON, &idx.Tokens)
		}()
	}

	if metaJSON != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			metaErr = json.Unmarshal(metaJSON, &rawMeta)
		}()
	}

	if filesJSON != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			filesErr = json.Unmarshal(filesJSON, &rawFiles)
		}()
	}

	wg.Wait()

	if tokErr != nil {
		return nil, fmt.Errorf("unmarshal tokens: %w", tokErr)
	}
	if metaErr != nil {
		return nil, fmt.Errorf("unmarshal metadata: %w", metaErr)
	}
	if filesErr != nil {
		return nil, fmt.Errorf("unmarshal files: %w", filesErr)
	}

	// Convert string-keyed maps to typed keys
	for k, sym := range rawMeta {
		var fileID uint32
		var line uint16
		if _, err := fmt.Sscanf(k, "%d:%d", &fileID, &line); err != nil {
			return nil, fmt.Errorf("parse metadata key %q: %w", k, err)
		}
		idx.Metadata[ports.TokenRef{FileID: fileID, Line: line}] = sym
	}

	for k, fm := range rawFiles {
		var fid uint32
		if _, err := fmt.Sscanf(k, "%d", &fid); err != nil {
			return nil, fmt.Errorf("parse file key %q: %w", k, err)
		}
		idx.Files[fid] = fm
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

// maxSessionRetention is the maximum number of session summaries to keep per project.
const maxSessionRetention = 200

// SaveSessionSummary persists a session summary for a project.
// If more than maxSessionRetention sessions exist, the oldest are pruned.
func (s *Store) SaveSessionSummary(projectID string, summary *ports.SessionSummary) error {
	if summary == nil {
		return fmt.Errorf("nil session summary")
	}

	data, err := json.Marshal(summary)
	if err != nil {
		return fmt.Errorf("marshal session summary: %w", err)
	}

	return s.db.Update(func(tx *bolt.Tx) error {
		proj, err := tx.CreateBucketIfNotExists([]byte(projectID))
		if err != nil {
			return err
		}
		sb, err := proj.CreateBucketIfNotExists(bucketSessions)
		if err != nil {
			return err
		}
		if err := sb.Put([]byte(summary.SessionID), data); err != nil {
			return err
		}

		// Prune oldest sessions beyond retention limit.
		// bbolt cursors iterate keys in sorted order; Claude session IDs are
		// chronologically sortable, so earliest keys are the oldest sessions.
		count := sb.Stats().KeyN
		if count > maxSessionRetention {
			excess := count - maxSessionRetention
			c := sb.Cursor()
			for k, _ := c.First(); k != nil && excess > 0; k, _ = c.Next() {
				if err := sb.Delete(k); err != nil {
					return err
				}
				excess--
			}
		}

		return nil
	})
}

// LoadSessionSummary retrieves a session summary by session ID.
// Returns nil, nil if no summary exists.
func (s *Store) LoadSessionSummary(projectID string, sessionID string) (*ports.SessionSummary, error) {
	var data []byte

	err := s.db.View(func(tx *bolt.Tx) error {
		proj := tx.Bucket([]byte(projectID))
		if proj == nil {
			return nil
		}
		sb := proj.Bucket(bucketSessions)
		if sb == nil {
			return nil
		}
		if v := sb.Get([]byte(sessionID)); v != nil {
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

	var summary ports.SessionSummary
	if err := json.Unmarshal(data, &summary); err != nil {
		return nil, fmt.Errorf("unmarshal session summary: %w", err)
	}
	return &summary, nil
}

// ListSessionSummaries returns all session summaries for a project.
func (s *Store) ListSessionSummaries(projectID string) ([]*ports.SessionSummary, error) {
	var summaries []*ports.SessionSummary

	err := s.db.View(func(tx *bolt.Tx) error {
		proj := tx.Bucket([]byte(projectID))
		if proj == nil {
			return nil
		}
		sb := proj.Bucket(bucketSessions)
		if sb == nil {
			return nil
		}
		return sb.ForEach(func(k, v []byte) error {
			var summary ports.SessionSummary
			if err := json.Unmarshal(v, &summary); err != nil {
				return nil // skip corrupt entries
			}
			summaries = append(summaries, &summary)
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return summaries, nil
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

// SaveAllDimensions persists all dimensional analysis results for a project.
// Keys are relative file paths, values are JSON-serialized FileAnalysis.
// Overwrites any prior dimensions for this projectID.
func (s *Store) SaveAllDimensions(projectID string, analyses map[string]*analyzer.FileAnalysis) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		proj, err := tx.CreateBucketIfNotExists([]byte(projectID))
		if err != nil {
			return err
		}
		// Delete existing dimensions bucket to replace all data
		_ = proj.DeleteBucket(bucketDimensions)
		db, err := proj.CreateBucket(bucketDimensions)
		if err != nil {
			return err
		}
		for path, analysis := range analyses {
			data, err := json.Marshal(analysis)
			if err != nil {
				return fmt.Errorf("marshal dimensions for %s: %w", path, err)
			}
			if err := db.Put([]byte(path), data); err != nil {
				return err
			}
		}
		return nil
	})
}

// LoadAllDimensions retrieves all dimensional analysis results for a project.
// Returns nil, nil if no dimensions exist.
func (s *Store) LoadAllDimensions(projectID string) (map[string]*analyzer.FileAnalysis, error) {
	var results map[string]*analyzer.FileAnalysis

	err := s.db.View(func(tx *bolt.Tx) error {
		proj := tx.Bucket([]byte(projectID))
		if proj == nil {
			return nil
		}
		db := proj.Bucket(bucketDimensions)
		if db == nil {
			return nil
		}
		results = make(map[string]*analyzer.FileAnalysis)
		return db.ForEach(func(k, v []byte) error {
			var analysis analyzer.FileAnalysis
			if err := json.Unmarshal(v, &analysis); err != nil {
				return nil // skip corrupt entries
			}
			results[string(k)] = &analysis
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}
