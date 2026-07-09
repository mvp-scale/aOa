package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/corey/aoa/internal/adapters/bbolt"
	"github.com/corey/aoa/internal/adapters/socket"
	"github.com/corey/aoa/internal/app"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Pre-recon v1 (bridge 2026-07-09) — `aoa arch recon` must invalidate derived
// arch views so the next derive rebuilds with the fresh footprint grain:
//   - daemon up  → socket Reindex (daemon holds the DB lock; its re-derive
//     picks up footprint.json from disk). Six-MethodArch* ADR intact — Reindex
//     is an existing admin method, no new protocol constant.
//   - daemon down → direct RW DeleteShardsForScope; the widened boot-derive
//     trigger (manifest absent) rebuilds on next daemon start.
//   - no DB      → nothing to invalidate; recon must stay DB-less and never
//     create a database.

// fakeReconDaemon implements reconDaemon for tests.
type fakeReconDaemon struct {
	up        bool
	reindexed int
}

func (f *fakeReconDaemon) Ping() bool { return f.up }
func (f *fakeReconDaemon) Reindex() (*socket.ReindexResult, error) {
	f.reindexed++
	return &socket.ReindexResult{}, nil
}

// seedArchDB creates {root}/.aoa/aoa.db holding one shard + manifest for the
// project (projectID = base of root, scope "local") and returns the projectID.
func seedArchDB(t *testing.T, root string) string {
	t.Helper()
	pid := filepath.Base(root)
	paths := app.NewPaths(root)
	require.NoError(t, os.MkdirAll(filepath.Dir(paths.DB), 0755))
	store, err := bbolt.NewStore(paths.DB)
	require.NoError(t, err)
	require.NoError(t, store.SaveShards(pid, map[string][]byte{
		"local/component@abc123": []byte(`{"kind":"buckets"}`),
	}))
	require.NoError(t, store.SaveManifest(pid, "local", []byte(`{"rev":"r1"}`)))
	require.NoError(t, store.Close())
	return pid
}

func TestReconInvalidate_DaemonDown_DeletesShardsAndManifest(t *testing.T) {
	root := t.TempDir()
	pid := seedArchDB(t, root)

	_, err := invalidateArchShards(root, &fakeReconDaemon{up: false})
	require.NoError(t, err)

	store, err := bbolt.NewReadOnlyStore(app.NewPaths(root).DB)
	require.NoError(t, err)
	defer store.Close()

	m, err := store.LoadManifest(pid, "local")
	require.NoError(t, err)
	assert.Empty(t, m, "manifest must be deleted when daemon is down")

	shard, err := store.LoadShard(pid, "local/component@abc123")
	require.NoError(t, err)
	assert.Empty(t, shard, "stale shard must be deleted when daemon is down")
}

func TestReconInvalidate_NoDB_CreatesNothing(t *testing.T) {
	root := t.TempDir() // no .aoa at all — recon is DB-less by design

	_, err := invalidateArchShards(root, &fakeReconDaemon{up: false})
	require.NoError(t, err)

	assert.NoFileExists(t, app.NewPaths(root).DB,
		"invalidation must never create a database where none exists")
}

func TestReconInvalidate_DaemonUp_TriggersReindex_NoDirectDelete(t *testing.T) {
	root := t.TempDir()
	pid := seedArchDB(t, root)

	daemon := &fakeReconDaemon{up: true}
	_, err := invalidateArchShards(root, daemon)
	require.NoError(t, err)

	assert.Equal(t, 1, daemon.reindexed,
		"daemon-up path must trigger exactly one Reindex over the socket")

	// The daemon owns the rebuild; the CLI must NOT touch the DB directly
	// (it couldn't anyway — the daemon holds the bbolt lock).
	store, err := bbolt.NewReadOnlyStore(app.NewPaths(root).DB)
	require.NoError(t, err)
	defer store.Close()
	m, err := store.LoadManifest(pid, "local")
	require.NoError(t, err)
	assert.NotEmpty(t, m, "daemon-up path must not delete shards directly")
}
