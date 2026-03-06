package bbolt

import (
	"encoding/json"
	"fmt"

	"github.com/corey/aoa/internal/ports"
	bolt "go.etcd.io/bbolt"
)

// keyTelemetry is the single key within the telemetry bucket.
var keyTelemetry = []byte("project")

// SaveSessionWithTelemetry atomically saves a session summary and increments
// lifetime telemetry in a single bbolt transaction. Session pruning is applied
// after save (same as SaveSessionSummary). If delta is nil, only the session
// is saved.
func (s *Store) SaveSessionWithTelemetry(projectID string, summary *ports.SessionSummary, delta *ports.ProjectTelemetry) error {
	if summary == nil {
		return fmt.Errorf("nil session summary")
	}

	sessionData, err := json.Marshal(summary)
	if err != nil {
		return fmt.Errorf("marshal session summary: %w", err)
	}

	return s.db.Update(func(tx *bolt.Tx) error {
		proj, err := tx.CreateBucketIfNotExists([]byte(projectID))
		if err != nil {
			return err
		}

		// Save session (same logic as SaveSessionSummary)
		sb, err := proj.CreateBucketIfNotExists(bucketSessions)
		if err != nil {
			return err
		}
		isNew := sb.Get([]byte(summary.SessionID)) == nil
		if err := sb.Put([]byte(summary.SessionID), sessionData); err != nil {
			return err
		}

		// Prune oldest sessions beyond retention limit.
		// Stats().KeyN is stale within the transaction, so adjust for new keys.
		count := sb.Stats().KeyN
		if isNew {
			count++
		}
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

		// Increment telemetry if delta provided
		if delta == nil {
			return nil
		}

		tb, err := proj.CreateBucketIfNotExists(bucketTelemetry)
		if err != nil {
			return err
		}

		// Load current telemetry
		var telem ports.ProjectTelemetry
		if raw := tb.Get(keyTelemetry); raw != nil {
			if err := json.Unmarshal(raw, &telem); err != nil {
				return fmt.Errorf("unmarshal telemetry: %w", err)
			}
		}

		// Apply delta
		telem.TokensSaved += delta.TokensSaved
		telem.TimeSavedMs += delta.TimeSavedMs
		telem.Reads += delta.Reads
		telem.GuidedReads += delta.GuidedReads
		telem.Sessions += delta.Sessions
		telem.Prompts += delta.Prompts
		telem.InputTokens += delta.InputTokens
		telem.OutputTokens += delta.OutputTokens
		telem.CacheReadTokens += delta.CacheReadTokens
		telem.ShadowSaved += delta.ShadowSaved

		// FirstSessionAt: keep earliest non-zero value
		if delta.FirstSessionAt != 0 {
			if telem.FirstSessionAt == 0 || delta.FirstSessionAt < telem.FirstSessionAt {
				telem.FirstSessionAt = delta.FirstSessionAt
			}
		}

		telemData, err := json.Marshal(&telem)
		if err != nil {
			return fmt.Errorf("marshal telemetry: %w", err)
		}
		return tb.Put(keyTelemetry, telemData)
	})
}

// LoadTelemetry retrieves lifetime telemetry for a project.
// If no telemetry record exists but sessions do, performs a one-time backfill
// by summing all existing session summaries within a single transaction.
// Returns a zero-value struct (not nil) for projects with no data.
func (s *Store) LoadTelemetry(projectID string) (*ports.ProjectTelemetry, error) {
	var telem ports.ProjectTelemetry
	var needsBackfill bool

	// First try a read-only path
	err := s.db.View(func(tx *bolt.Tx) error {
		proj := tx.Bucket([]byte(projectID))
		if proj == nil {
			return nil // no project → zero telemetry
		}

		tb := proj.Bucket(bucketTelemetry)
		if tb != nil {
			if raw := tb.Get(keyTelemetry); raw != nil {
				return json.Unmarshal(raw, &telem)
			}
		}

		// Telemetry bucket missing or empty — check if sessions exist for backfill
		sb := proj.Bucket(bucketSessions)
		if sb != nil && sb.Stats().KeyN > 0 {
			needsBackfill = true
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if !needsBackfill {
		return &telem, nil
	}

	// Backfill: sum all sessions and write telemetry in a single write transaction
	err = s.db.Update(func(tx *bolt.Tx) error {
		proj := tx.Bucket([]byte(projectID))
		if proj == nil {
			return nil
		}

		// Double-check: another goroutine might have backfilled between View and Update
		tb, err := proj.CreateBucketIfNotExists(bucketTelemetry)
		if err != nil {
			return err
		}
		if raw := tb.Get(keyTelemetry); raw != nil {
			// Already backfilled — load and return
			return json.Unmarshal(raw, &telem)
		}

		// Sum all sessions
		sb := proj.Bucket(bucketSessions)
		if sb == nil {
			return nil
		}

		var earliest int64
		err = sb.ForEach(func(k, v []byte) error {
			var sess ports.SessionSummary
			if err := json.Unmarshal(v, &sess); err != nil {
				return nil // skip corrupt entries
			}
			telem.TokensSaved += sess.TokensSaved
			telem.TimeSavedMs += sess.TimeSavedMs
			telem.Reads += sess.ReadCount
			telem.GuidedReads += sess.GuidedReadCount
			telem.Sessions++
			telem.Prompts += sess.PromptCount
			telem.InputTokens += int64(sess.InputTokens)
			telem.OutputTokens += int64(sess.OutputTokens)
			telem.CacheReadTokens += int64(sess.CacheReadTokens)
			if sess.StartTime != 0 && (earliest == 0 || sess.StartTime < earliest) {
				earliest = sess.StartTime
			}
			return nil
		})
		if err != nil {
			return err
		}
		telem.FirstSessionAt = earliest

		// Persist backfilled telemetry
		data, err := json.Marshal(&telem)
		if err != nil {
			return fmt.Errorf("marshal backfilled telemetry: %w", err)
		}
		return tb.Put(keyTelemetry, data)
	})
	if err != nil {
		return nil, err
	}

	return &telem, nil
}
