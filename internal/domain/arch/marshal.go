package arch

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
)

// MarshalShard encodes a Shard to byte-stable JSON.
// Rules (§2.6 byte-stability):
//   - Uses json.Encoder with SetEscapeHTML(false).
//   - No trailing newline (Encoder appends one; we strip it).
//   - No timestamps inside shards (timestamps live only in the manifest).
//   - Struct field order is deterministic (Go encodes in declaration order).
//   - Callers must sort all slices before calling MarshalShard.
func MarshalShard(s *Shard) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		return nil, fmt.Errorf("arch.MarshalShard: %w", err)
	}
	b := buf.Bytes()
	// Strip the trailing newline that Encode appends.
	if len(b) > 0 && b[len(b)-1] == '\n' {
		b = b[:len(b)-1]
	}
	return b, nil
}

// ContentHash returns the first 12 hex characters of SHA-256(b).
// Matches the generator: sha256(shard_bytes)[:12] (build_c4_mockup.py:383).
func ContentHash(b []byte) string {
	h := sha256.Sum256(b)
	return fmt.Sprintf("%x", h[:])[:12]
}
