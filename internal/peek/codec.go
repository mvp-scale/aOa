// Package peek provides stateless encode/decode for peek codes.
// A peek code is a compact base36 encoding of (fileID, startLine)
// that agents pass to "aoa peek" to retrieve method bodies.
package peek

import (
	"fmt"
	"strconv"
)

// MaxRange is the symbol size limit for peek codes. Symbols spanning
// more than MaxRange lines get no peek code (too large to be useful).
const MaxRange = 500

// Encode packs a fileID and startLine into a compact base36 string.
// Layout: fileID << 16 | startLine → base36.
func Encode(fileID uint32, startLine uint16) string {
	v := uint64(fileID)<<16 | uint64(startLine)
	return strconv.FormatUint(v, 36)
}

// Decode unpacks a peek code back to (fileID, startLine).
func Decode(code string) (uint32, uint16, error) {
	v, err := strconv.ParseUint(code, 36, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid peek code %q: %w", code, err)
	}
	fileID := uint32(v >> 16)
	startLine := uint16(v & 0xFFFF)
	return fileID, startLine, nil
}
