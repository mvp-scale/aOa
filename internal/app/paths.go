package app

import (
	"os"
	"path/filepath"
)

// Paths holds all resolved filesystem paths for the .aoa/ project directory.
// All fields are pre-computed strings — zero-alloc access after construction.
type Paths struct {
	Root   string // .aoa/
	DB     string // .aoa/aoa.db
	Status string // .aoa/status.json

	LogDir    string // .aoa/log/
	DaemonLog string // .aoa/log/daemon.log

	RunDir  string // .aoa/run/
	PIDFile  string // .aoa/run/daemon.pid
	PortFile string // .aoa/run/http.port

	ReconDir          string // .aoa/recon/
	ReconEnabled      string // .aoa/recon/enabled
	ReconInvestigated string // .aoa/recon/investigated.json

	HookDir      string // .aoa/hook/
	ContextJSONL string // .aoa/hook/context.jsonl
	UsageTxt     string // .aoa/hook/usage.txt

	BinDir   string // .aoa/bin/

	GrammarsDir string // .aoa/grammars/
}

// NewPaths constructs all resolved paths from a project root directory.
func NewPaths(projectRoot string) *Paths {
	root := filepath.Join(projectRoot, ".aoa")
	return &Paths{
		Root:   root,
		DB:     filepath.Join(root, "aoa.db"),
		Status: filepath.Join(root, "status.json"),

		LogDir:    filepath.Join(root, "log"),
		DaemonLog: filepath.Join(root, "log", "daemon.log"),

		RunDir:  filepath.Join(root, "run"),
		PIDFile:  filepath.Join(root, "run", "daemon.pid"),
		PortFile: filepath.Join(root, "run", "http.port"),

		ReconDir:          filepath.Join(root, "recon"),
		ReconEnabled:      filepath.Join(root, "recon", "enabled"),
		ReconInvestigated: filepath.Join(root, "recon", "investigated.json"),

		HookDir:      filepath.Join(root, "hook"),
		ContextJSONL: filepath.Join(root, "hook", "context.jsonl"),
		UsageTxt:     filepath.Join(root, "hook", "usage.txt"),

		BinDir:   filepath.Join(root, "bin"),

		GrammarsDir: filepath.Join(root, "grammars"),
	}
}

// EnsureDirs creates all subdirectories under .aoa/. Idempotent.
func (p *Paths) EnsureDirs() error {
	dirs := []string{
		p.Root,
		p.LogDir,
		p.RunDir,
		p.ReconDir,
		p.HookDir,
		p.BinDir,
		p.GrammarsDir,
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return err
		}
	}
	return nil
}

// migration defines a single file move from old flat layout to new subdirectory layout.
type migration struct {
	oldName string // relative to .aoa/ (flat)
	newPath string // absolute destination path
}

// Migrate moves files from old flat .aoa/ locations to the new subdirectory layout.
// Returns the number of files moved. Idempotent: skips if source is missing or
// destination already exists. Also removes the dead domains/ directory if present.
func (p *Paths) Migrate() (int, error) {
	moves := []migration{
		{"daemon.log", p.DaemonLog},
		{"daemon.pid", p.PIDFile},
		{"http.port", p.PortFile},
		{"recon.enabled", p.ReconEnabled},
		{"recon-investigated.json", p.ReconInvestigated},
		{"context.jsonl", p.ContextJSONL},
		{"usage.txt", p.UsageTxt},
	}

	count := 0
	for _, m := range moves {
		oldPath := filepath.Join(p.Root, m.oldName)

		// Skip if source doesn't exist.
		if _, err := os.Stat(oldPath); err != nil {
			continue
		}

		// Don't overwrite existing destination.
		if _, err := os.Stat(m.newPath); err == nil {
			continue
		}

		if err := os.Rename(oldPath, m.newPath); err != nil {
			return count, err
		}
		count++
	}

	// Remove dead domains/ directory if present (legacy artifact).
	domainsDir := filepath.Join(p.Root, "domains")
	if info, err := os.Stat(domainsDir); err == nil && info.IsDir() {
		os.RemoveAll(domainsDir)
	}

	return count, nil
}

// CleanEphemeral removes ephemeral runtime files (PID file and port file).
// Called on clean daemon shutdown.
func (p *Paths) CleanEphemeral() {
	os.Remove(p.PIDFile)
	os.Remove(p.PortFile)
}
