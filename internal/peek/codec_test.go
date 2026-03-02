package peek

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoundTrip(t *testing.T) {
	cases := []struct {
		fileID    uint32
		startLine uint16
	}{
		{0, 0},
		{1, 15},
		{42, 300},
		{1000, 65535},
		{0xFFFF, 0xFFFF},
		{100000, 1},
	}

	for _, tc := range cases {
		code := Encode(tc.fileID, tc.startLine)
		gotFile, gotLine, err := Decode(code)
		require.NoError(t, err, "code=%s", code)
		assert.Equal(t, tc.fileID, gotFile, "fileID mismatch for code=%s", code)
		assert.Equal(t, tc.startLine, gotLine, "startLine mismatch for code=%s", code)
	}
}

func TestDecodeInvalid(t *testing.T) {
	_, _, err := Decode("!!invalid!!")
	assert.Error(t, err)

	_, _, err = Decode("")
	assert.Error(t, err)
}

func TestEncodeLength(t *testing.T) {
	// Typical codes should be compact (4-7 chars)
	code := Encode(42, 100)
	assert.LessOrEqual(t, len(code), 7)
	assert.GreaterOrEqual(t, len(code), 1)
}
