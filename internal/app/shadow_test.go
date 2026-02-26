package app

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestShadowRing_Push(t *testing.T) {
	var ring ShadowRing

	ring.Push(ToolShadow{
		Source:      "session_log",
		ToolID:      "tool_1",
		ActualChars: 1000,
		ShadowChars: 200,
		CharsSaved:  800,
	})

	assert.Equal(t, 1, ring.Count())
	assert.Equal(t, int64(800), ring.TotalCharsSaved())
}

func TestShadowRing_Entries(t *testing.T) {
	var ring ShadowRing

	for i := 0; i < 5; i++ {
		ring.Push(ToolShadow{
			Timestamp:  time.Unix(int64(i), 0),
			ToolID:     "tool_" + string(rune('0'+i)),
			CharsSaved: (i + 1) * 100,
		})
	}

	entries := ring.Entries()
	assert.Len(t, entries, 5)
	// Oldest first
	assert.Equal(t, time.Unix(0, 0), entries[0].Timestamp)
	assert.Equal(t, time.Unix(4, 0), entries[4].Timestamp)
}

func TestShadowRing_Wrap(t *testing.T) {
	var ring ShadowRing

	// Push 105 entries — wraps at 100
	for i := 0; i < 105; i++ {
		ring.Push(ToolShadow{
			Timestamp:  time.Unix(int64(i), 0),
			CharsSaved: 10,
		})
	}

	assert.Equal(t, 100, ring.Count())
	entries := ring.Entries()
	assert.Len(t, entries, 100)
	// Oldest surviving: entry 5
	assert.Equal(t, time.Unix(5, 0), entries[0].Timestamp)
	// Newest: entry 104
	assert.Equal(t, time.Unix(104, 0), entries[99].Timestamp)
}

func TestShadowRing_FindByToolID(t *testing.T) {
	var ring ShadowRing

	ring.Push(ToolShadow{ToolID: "t1", CharsSaved: 100})
	ring.Push(ToolShadow{ToolID: "t2", CharsSaved: 200})
	ring.Push(ToolShadow{ToolID: "t3", CharsSaved: 300})

	found := ring.FindByToolID("t2")
	assert.NotNil(t, found)
	assert.Equal(t, 200, found.CharsSaved)

	// Not found
	assert.Nil(t, ring.FindByToolID("t99"))
	assert.Nil(t, ring.FindByToolID(""))
}

func TestShadowRing_TotalCharsSaved(t *testing.T) {
	var ring ShadowRing

	ring.Push(ToolShadow{CharsSaved: 100})
	ring.Push(ToolShadow{CharsSaved: -50}) // negative = no savings
	ring.Push(ToolShadow{CharsSaved: 200})

	// Only positive values accumulate
	assert.Equal(t, int64(300), ring.TotalCharsSaved())
}

func TestShadowRing_Reset(t *testing.T) {
	var ring ShadowRing
	ring.Push(ToolShadow{CharsSaved: 100})
	ring.Reset()

	assert.Equal(t, 0, ring.Count())
	assert.Equal(t, int64(0), ring.TotalCharsSaved())
	assert.Nil(t, ring.Entries())
}
