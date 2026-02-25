// Binary encoding for search index blobs.
//
// Format v1 replaces JSON with compact binary posting lists for the tokens map
// (the dominant blob) and gob encoding for metadata and files maps.
//
// Binary posting list format (little-endian):
//
//	tokenCount: uint32
//	per token:
//	  keyLen:   uint16
//	  key:      [keyLen]byte
//	  refCount: uint32
//	  refs:     [refCount]× (FileID:uint32 + Line:uint16)
package bbolt

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"sort"

	"github.com/corey/aoa/internal/ports"
)

// refSize is the byte size of a single encoded TokenRef (uint32 + uint16).
const refSize = 6

// encodePostingLists encodes a tokens map to compact binary format.
// Token keys are sorted for deterministic output. A single buffer is
// pre-allocated to avoid repeated growth.
func encodePostingLists(tokens map[string][]ports.TokenRef) ([]byte, error) {
	// Pre-calculate total size for single allocation.
	// Header: 4 bytes (tokenCount)
	// Per token: 2 (keyLen) + len(key) + 4 (refCount) + refCount*6
	totalSize := 4
	for key, refs := range tokens {
		totalSize += 2 + len(key) + 4 + len(refs)*refSize
	}

	buf := make([]byte, totalSize)
	offset := 0

	// Sort keys for deterministic output.
	keys := make([]string, 0, len(tokens))
	for k := range tokens {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Token count header.
	binary.LittleEndian.PutUint32(buf[offset:], uint32(len(keys)))
	offset += 4

	for _, key := range keys {
		refs := tokens[key]

		// Key length + key bytes.
		keyBytes := []byte(key)
		if len(keyBytes) > 65535 {
			return nil, fmt.Errorf("token key too long: %d bytes", len(keyBytes))
		}
		binary.LittleEndian.PutUint16(buf[offset:], uint16(len(keyBytes)))
		offset += 2
		copy(buf[offset:], keyBytes)
		offset += len(keyBytes)

		// Ref count + refs.
		binary.LittleEndian.PutUint32(buf[offset:], uint32(len(refs)))
		offset += 4
		for _, ref := range refs {
			binary.LittleEndian.PutUint32(buf[offset:], ref.FileID)
			offset += 4
			binary.LittleEndian.PutUint16(buf[offset:], ref.Line)
			offset += 2
		}
	}

	return buf, nil
}

// decodePostingLists decodes binary posting lists back to a tokens map.
// Every read is bounds-checked to avoid panics on corrupt data.
func decodePostingLists(data []byte) (map[string][]ports.TokenRef, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("posting list too short: %d bytes", len(data))
	}

	offset := 0
	tokenCount := binary.LittleEndian.Uint32(data[offset:])
	offset += 4

	tokens := make(map[string][]ports.TokenRef, tokenCount)

	for i := uint32(0); i < tokenCount; i++ {
		// Read key length.
		if offset+2 > len(data) {
			return nil, fmt.Errorf("truncated at token %d key length (offset %d)", i, offset)
		}
		keyLen := int(binary.LittleEndian.Uint16(data[offset:]))
		offset += 2

		// Read key bytes.
		if offset+keyLen > len(data) {
			return nil, fmt.Errorf("truncated at token %d key (offset %d, need %d)", i, offset, keyLen)
		}
		key := string(data[offset : offset+keyLen])
		offset += keyLen

		// Read ref count.
		if offset+4 > len(data) {
			return nil, fmt.Errorf("truncated at token %d ref count (offset %d)", i, offset)
		}
		refCount := binary.LittleEndian.Uint32(data[offset:])
		offset += 4

		// Read refs.
		refsBytes := int(refCount) * refSize
		if offset+refsBytes > len(data) {
			return nil, fmt.Errorf("truncated at token %d refs (offset %d, need %d)", i, offset, refsBytes)
		}

		refs := make([]ports.TokenRef, refCount)
		for j := uint32(0); j < refCount; j++ {
			refs[j].FileID = binary.LittleEndian.Uint32(data[offset:])
			offset += 4
			refs[j].Line = binary.LittleEndian.Uint16(data[offset:])
			offset += 2
		}

		tokens[key] = refs
	}

	return tokens, nil
}

// encodeGob encodes a value using gob. Used for metadata and files blobs
// which are small (~0.6MB combined) and benefit from gob's ~2-3x compression
// over JSON without needing a custom binary format.
func encodeGob(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// decodeGob decodes gob-encoded data into target. Target must be a pointer.
func decodeGob(data []byte, target interface{}) error {
	return gob.NewDecoder(bytes.NewReader(data)).Decode(target)
}
