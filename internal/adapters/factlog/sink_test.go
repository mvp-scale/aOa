package factlog

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// FDN-1 (board #27): FactSink JSONL staging writer
// (playbook/integration/01-facts-substrate.md §3.1, D5).
// =============================================================================

func readLines(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()

	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if line := sc.Text(); line != "" {
			lines = append(lines, line)
		}
	}
	require.NoError(t, sc.Err())
	return lines
}

func TestSink_InterfaceSatisfied(t *testing.T) {
	var _ ports.FactSink = (*Sink)(nil)
	var _ ports.FactSink = NullSink{}
}

func TestSink_EmitFlush_WritesJSONL(t *testing.T) {
	root := t.TempDir()
	s, err := New(root)
	require.NoError(t, err)
	defer s.Close()

	f1 := ports.Fact{
		Kind: ports.FactDep, Subject: "go:internal/app", Object: "go:internal/ports",
		Attrs:  map[string]string{"spec": "github.com/corey/aoa/internal/ports"},
		Source: ports.FactSource{File: "internal/app/app.go", Line: 11, Commit: "147ba46"},
		Prov:   ports.ProvDerived, TS: 1781136000,
	}
	f2 := ports.Fact{
		Kind: ports.FactUnit, Subject: "go:internal/app",
		Attrs:  map[string]string{"lang": "go", "files": "9"},
		Source: ports.FactSource{File: "internal/app/app.go", Line: 1, Commit: "147ba46"},
		Prov:   ports.ProvDerived, TS: 1781136000,
	}

	s.Emit(f1)
	s.Emit(f2)
	require.NoError(t, s.Flush())

	path := filepath.Join(root, ".aoa", "facts", "pending.jsonl")
	lines := readLines(t, path)
	require.Len(t, lines, 2)

	var got1, got2 ports.Fact
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &got1))
	require.NoError(t, json.Unmarshal([]byte(lines[1]), &got2))
	assert.Equal(t, f1, got1)
	assert.Equal(t, f2, got2)
}

func TestSink_JSONL_UsesShortKeys(t *testing.T) {
	root := t.TempDir()
	s, err := New(root)
	require.NoError(t, err)
	defer s.Close()

	s.Emit(ports.Fact{
		Kind: ports.FactDep, Subject: "go:internal/app", Object: "go:internal/ports",
		Source: ports.FactSource{File: "internal/app/app.go", Line: 11},
		Prov:   ports.ProvDerived,
	})
	require.NoError(t, s.Flush())

	path := filepath.Join(root, ".aoa", "facts", "pending.jsonl")
	lines := readLines(t, path)
	require.Len(t, lines, 1)

	var raw map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &raw))
	for _, k := range []string{"k", "s", "o", "src", "p"} {
		_, ok := raw[k]
		assert.True(t, ok, "expected short key %q in %s", k, lines[0])
	}
}

func TestSink_AppendsAcrossReopens(t *testing.T) {
	root := t.TempDir()

	s1, err := New(root)
	require.NoError(t, err)
	s1.Emit(ports.Fact{Kind: ports.FactDep, Subject: "go:a", Prov: ports.ProvDerived})
	require.NoError(t, s1.Close())

	s2, err := New(root)
	require.NoError(t, err)
	s2.Emit(ports.Fact{Kind: ports.FactDep, Subject: "go:b", Prov: ports.ProvDerived})
	require.NoError(t, s2.Close())

	path := filepath.Join(root, ".aoa", "facts", "pending.jsonl")
	lines := readLines(t, path)
	require.Len(t, lines, 2)
}

func TestSink_Truncate_ResetsFile(t *testing.T) {
	root := t.TempDir()
	s, err := New(root)
	require.NoError(t, err)
	defer s.Close()

	s.Emit(ports.Fact{Kind: ports.FactDep, Subject: "go:a", Prov: ports.ProvDerived})
	require.NoError(t, s.Flush())

	path := filepath.Join(root, ".aoa", "facts", "pending.jsonl")
	require.Len(t, readLines(t, path), 1)

	require.NoError(t, s.Truncate())
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Zero(t, info.Size())

	s.Emit(ports.Fact{Kind: ports.FactDep, Subject: "go:b", Prov: ports.ProvDerived})
	require.NoError(t, s.Flush())
	require.Len(t, readLines(t, path), 1)
}

func TestNullSink_NeverErrors(t *testing.T) {
	var s NullSink
	s.Emit(ports.Fact{Kind: ports.FactDep, Subject: "go:a"})
	assert.NoError(t, s.Flush())
}
