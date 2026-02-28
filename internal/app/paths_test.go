package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPaths(t *testing.T) {
	p := NewPaths("/project")
	assert.Equal(t, filepath.Join("/project", ".aoa"), p.Root)
	assert.Equal(t, filepath.Join("/project", ".aoa", "aoa.db"), p.DB)
	assert.Equal(t, filepath.Join("/project", ".aoa", "status.json"), p.Status)
	assert.Equal(t, filepath.Join("/project", ".aoa", "log"), p.LogDir)
	assert.Equal(t, filepath.Join("/project", ".aoa", "log", "daemon.log"), p.DaemonLog)
	assert.Equal(t, filepath.Join("/project", ".aoa", "run"), p.RunDir)
	assert.Equal(t, filepath.Join("/project", ".aoa", "run", "daemon.pid"), p.PIDFile)
	assert.Equal(t, filepath.Join("/project", ".aoa", "run", "http.port"), p.PortFile)
	assert.Equal(t, filepath.Join("/project", ".aoa", "recon"), p.ReconDir)
	assert.Equal(t, filepath.Join("/project", ".aoa", "recon", "enabled"), p.ReconEnabled)
	assert.Equal(t, filepath.Join("/project", ".aoa", "recon", "investigated.json"), p.ReconInvestigated)
	assert.Equal(t, filepath.Join("/project", ".aoa", "hook"), p.HookDir)
	assert.Equal(t, filepath.Join("/project", ".aoa", "hook", "context.jsonl"), p.ContextJSONL)
	assert.Equal(t, filepath.Join("/project", ".aoa", "hook", "usage.txt"), p.UsageTxt)
	assert.Equal(t, filepath.Join("/project", ".aoa", "bin"), p.BinDir)
	assert.Equal(t, filepath.Join("/project", ".aoa", "grammars"), p.GrammarsDir)
}

func TestEnsureDirs(t *testing.T) {
	dir := t.TempDir()
	p := NewPaths(dir)

	// First call creates directories.
	require.NoError(t, p.EnsureDirs())
	for _, d := range []string{p.Root, p.LogDir, p.RunDir, p.ReconDir, p.HookDir, p.BinDir, p.GrammarsDir} {
		info, err := os.Stat(d)
		require.NoError(t, err, "dir %s should exist", d)
		assert.True(t, info.IsDir())
	}

	// Second call is idempotent — no error.
	require.NoError(t, p.EnsureDirs())
}

func TestMigrate_FreshInstall(t *testing.T) {
	dir := t.TempDir()
	p := NewPaths(dir)
	require.NoError(t, p.EnsureDirs())

	count, err := p.Migrate()
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestMigrate_OldLayout(t *testing.T) {
	dir := t.TempDir()
	p := NewPaths(dir)
	require.NoError(t, p.EnsureDirs())

	// Create old-layout files in .aoa/ root.
	oldFiles := map[string]string{
		"daemon.log":               "log data",
		"daemon.pid":               "12345",
		"http.port":                "8080",
		"recon.enabled":            "/usr/bin/aoa-recon",
		"recon-investigated.json":  `{"file.go":1}`,
		"context.jsonl":            `{"ts":1}`,
		"usage.txt":                "usage data",
	}
	for name, content := range oldFiles {
		require.NoError(t, os.WriteFile(filepath.Join(p.Root, name), []byte(content), 0644))
	}

	count, err := p.Migrate()
	require.NoError(t, err)
	assert.Equal(t, 7, count)

	// Verify files moved to new locations.
	data, err := os.ReadFile(p.DaemonLog)
	require.NoError(t, err)
	assert.Equal(t, "log data", string(data))

	data, err = os.ReadFile(p.PIDFile)
	require.NoError(t, err)
	assert.Equal(t, "12345", string(data))

	data, err = os.ReadFile(p.PortFile)
	require.NoError(t, err)
	assert.Equal(t, "8080", string(data))

	data, err = os.ReadFile(p.ReconEnabled)
	require.NoError(t, err)
	assert.Equal(t, "/usr/bin/aoa-recon", string(data))

	data, err = os.ReadFile(p.ReconInvestigated)
	require.NoError(t, err)
	assert.Equal(t, `{"file.go":1}`, string(data))

	data, err = os.ReadFile(p.ContextJSONL)
	require.NoError(t, err)
	assert.Equal(t, `{"ts":1}`, string(data))

	data, err = os.ReadFile(p.UsageTxt)
	require.NoError(t, err)
	assert.Equal(t, "usage data", string(data))

	// Old files should be gone.
	for name := range oldFiles {
		_, err := os.Stat(filepath.Join(p.Root, name))
		assert.True(t, os.IsNotExist(err), "old file %s should be removed", name)
	}
}

func TestMigrate_Idempotent(t *testing.T) {
	dir := t.TempDir()
	p := NewPaths(dir)
	require.NoError(t, p.EnsureDirs())

	// Create one old file.
	require.NoError(t, os.WriteFile(filepath.Join(p.Root, "daemon.pid"), []byte("99"), 0644))

	count1, err := p.Migrate()
	require.NoError(t, err)
	assert.Equal(t, 1, count1)

	// Second call: source gone, dest exists -> count=0.
	count2, err := p.Migrate()
	require.NoError(t, err)
	assert.Equal(t, 0, count2)
}

func TestMigrate_NoOverwrite(t *testing.T) {
	dir := t.TempDir()
	p := NewPaths(dir)
	require.NoError(t, p.EnsureDirs())

	// Create both old and new files with different content.
	require.NoError(t, os.WriteFile(filepath.Join(p.Root, "daemon.pid"), []byte("old"), 0644))
	require.NoError(t, os.WriteFile(p.PIDFile, []byte("new"), 0644))

	count, err := p.Migrate()
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	// New file should be unchanged.
	data, err := os.ReadFile(p.PIDFile)
	require.NoError(t, err)
	assert.Equal(t, "new", string(data))

	// Old file should still exist (not deleted when dest exists).
	data, err = os.ReadFile(filepath.Join(p.Root, "daemon.pid"))
	require.NoError(t, err)
	assert.Equal(t, "old", string(data))
}

func TestMigrate_DomainsCleanup(t *testing.T) {
	dir := t.TempDir()
	p := NewPaths(dir)
	require.NoError(t, p.EnsureDirs())

	// Create dead domains/ directory with a file inside.
	domainsDir := filepath.Join(p.Root, "domains")
	require.NoError(t, os.MkdirAll(domainsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(domainsDir, "old.json"), []byte("{}"), 0644))

	count, err := p.Migrate()
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	// domains/ directory should be removed.
	_, err = os.Stat(domainsDir)
	assert.True(t, os.IsNotExist(err), "domains/ dir should be removed")
}
