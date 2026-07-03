// Package cmd — arch subcommand family (L19.16, E6 CLI + socket MethodArch*).
//
// This file holds:
//   - RegisterArchCommands: C4-gated entry point (called from root.go when arch is on).
//   - archClient: shared daemon-first → direct-RO-fallback client helper.
//   - cliArchQuerier: lightweight ArchQuerier backed by a read-only bbolt store.
//   - archUnitSlug: CLI-local unit-ID converter (mirrors app.unitSlug, no import).
//
// Exit codes follow the grep convention (02-arch-service.md §1.3):
//
//	0 — success (and --new: no new findings)
//	1 — empty result (view not found, no path, --new has findings)
//	2 — operational error (no substrate, bad flags)
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/corey/aoa/internal/adapters/bbolt"
	"github.com/corey/aoa/internal/adapters/socket"
	"github.com/corey/aoa/internal/app"
	"github.com/corey/aoa/internal/domain/arch"
	"github.com/corey/aoa/internal/ports"
	"github.com/spf13/cobra"
)

// archDefaultScope is the scope used when --scope is not provided.
const archDefaultScope = "local"

// RegisterArchCommands adds the arch subcommand tree to the root command.
// Must only be called when arch is enabled (C4 — called from root.go).
func RegisterArchCommands(root *cobra.Command) {
	archCmd := &cobra.Command{
		Use:   "arch",
		Short: "Architectural graph commands (import edges, dependency views)",
		Long: `arch provides subcommands for exploring the import-edge graph
extracted during indexing.

Daemon-first: commands use the running daemon when available; fall back to
direct read-only bbolt access when the daemon is down. Requires AOA_ARCH=on
(default).`,
	}

	archCmd.AddCommand(archViewsCmd)
	archCmd.AddCommand(archViewCmd)
	archCmd.AddCommand(archFindingsCmd)
	archCmd.AddCommand(archJourneyCmd)
	archCmd.AddCommand(archDeriveCmd)
	archCmd.AddCommand(archReachCmd)
	archCmd.AddCommand(archBlastCmd)
	archCmd.AddCommand(archFactsCmd)
	// `arch pack` is L22.5's verb (evidence/compliance packaging). The adopted
	// F2 surface is six MethodArch* + the reach/blast CLI aliases (ADR
	// 2026-07-02); L22.5 re-adds pack when pulled from the pool. The stub is
	// removed here rather than shipped dark (checkpoint-F2 PC7 / ledger T49).

	root.AddCommand(archCmd)
}

// ── Daemon-first / direct-RO shared resolver ─────────────────────────────────

// archExecResult is the outcome of an arch command (socket or direct-RO).
type archExecResult struct {
	socketClient *socket.Client    // non-nil when daemon is up
	querier      ports.ArchQuerier // non-nil when using direct-RO fallback
	closeStore   func()            // close the RO store when done; no-op for socket path
	root         string
	projectID    string
}

// resolveArch resolves an arch data source: daemon socket or direct-RO bbolt.
//
// Fallback order (02-arch-service.md §1.2):
//  1. Socket → if daemon is up, return a socket client.
//  2. Lazy revive → if .aoa exists and daemon is not up, try to revive.
//  3. Direct RO → open bbolt read-only (no lock conflict with running daemon).
//  4. Error → neither substrate nor DB found.
func resolveArch(root string) (*archExecResult, int, error) {
	sockPath := socket.SocketPath(root)
	client := socket.NewClient(sockPath)

	// Try daemon.
	if client.Ping() {
		return &archExecResult{
			socketClient: client,
			closeStore:   func() {},
			root:         root,
			projectID:    filepath.Base(root),
		}, 0, nil
	}

	// Lazy revive.
	if reviveDaemon(root, sockPath) {
		if client.Ping() {
			return &archExecResult{
				socketClient: client,
				closeStore:   func() {},
				root:         root,
				projectID:    filepath.Base(root),
			}, 0, nil
		}
	}

	// Direct RO fallback.
	paths := app.NewPaths(root)
	if _, err := os.Stat(paths.DB); os.IsNotExist(err) {
		return nil, 2, fmt.Errorf("no facts substrate. Run: aoa init && aoa daemon start")
	}
	store, err := bbolt.NewReadOnlyStore(paths.DB)
	if err != nil {
		return nil, 2, fmt.Errorf("open DB read-only: %w", err)
	}
	projectID := filepath.Base(root)
	q := newCLIArchQuerier(store, projectID)
	return &archExecResult{
		querier:    q,
		closeStore: func() { store.Close() },
		root:       root,
		projectID:  projectID,
	}, 0, nil
}

// ── CLI-local ArchQuerier (direct-RO fallback) ────────────────────────────────

// cliArchQuerier implements ports.ArchQuerier backed by a read-only bbolt store.
// Used when the daemon is not running. All methods are read-only (db.View).
type cliArchQuerier struct {
	store     *bbolt.Store
	projectID string
}

func newCLIArchQuerier(store *bbolt.Store, projectID string) ports.ArchQuerier {
	return &cliArchQuerier{store: store, projectID: projectID}
}

func (q *cliArchQuerier) Manifest(scope string) (*ports.ArchManifest, error) {
	data, err := q.store.LoadManifest(q.projectID, scope)
	if err != nil || data == nil {
		return nil, err
	}
	var m ports.ArchManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("arch.Manifest: unmarshal: %w", err)
	}
	return &m, nil
}

func (q *cliArchQuerier) View(scope, id string) ([]byte, error) {
	m, err := q.Manifest(scope)
	if err != nil || m == nil {
		return nil, err
	}
	for _, ve := range m.Views {
		if ve.ID == id {
			return q.store.LoadShard(q.projectID, ve.Key)
		}
	}
	return nil, nil // view not found
}

func (q *cliArchQuerier) Findings(scope string) ([]byte, error) {
	findings, err := q.store.LoadFindings(q.projectID, scope)
	if err != nil || findings == nil {
		return nil, err
	}
	return json.Marshal(findings)
}

func (q *cliArchQuerier) Derive(_ string, from, to string, k int) ([]string, error) {
	edges, err := q.store.LoadAllEdges(q.projectID)
	if err != nil || len(edges) == 0 {
		return nil, err
	}

	// Aggregate to unit-level adjacency (mirrors app.aggregateEdges).
	type depKey struct{ from, to string }
	adj := make(map[string][]string)
	seen := make(map[depKey]bool)
	for _, e := range edges {
		fromDir := filepath.Dir(e.FromFile)
		if fromDir == "." {
			fromDir = "root"
		}
		fromID := archUnitSlug(fromDir)
		toID := archUnitSlug(e.ImportPath)
		if fromID == toID {
			continue
		}
		dk := depKey{fromID, toID}
		if !seen[dk] {
			seen[dk] = true
			adj[fromID] = append(adj[fromID], toID)
		}
	}

	if from == to {
		return []string{from}, nil
	}

	// BFS bounded by k hops.
	type bfsState struct {
		id   string
		path []string
	}
	visited := map[string]bool{from: true}
	queue := []bfsState{{id: from, path: []string{from}}}

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		if len(curr.path) > k {
			break
		}
		for _, next := range adj[curr.id] {
			newPath := make([]string, len(curr.path)+1)
			copy(newPath, curr.path)
			newPath[len(curr.path)] = next
			if next == to {
				return newPath, nil
			}
			if !visited[next] && len(newPath) <= k {
				visited[next] = true
				queue = append(queue, bfsState{id: next, path: newPath})
			}
		}
	}
	return nil, nil // no path within k hops
}

func (q *cliArchQuerier) Facts(_ string, subject string, limit int) ([]byte, error) {
	edges, err := q.store.LoadAllEdges(q.projectID)
	if err != nil || len(edges) == 0 {
		return nil, err
	}
	type factEntry struct {
		FromFile   string `json:"from_file"`
		ImportPath string `json:"import_path"`
		StartLine  uint32 `json:"start_line"`
	}
	var result []factEntry
	for _, e := range edges {
		if strings.Contains(e.FromFile, subject) || strings.Contains(e.ImportPath, subject) {
			result = append(result, factEntry{
				FromFile:   e.FromFile,
				ImportPath: e.ImportPath,
				StartLine:  e.StartLine,
			})
		}
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	if result == nil {
		result = []factEntry{}
	}
	return json.Marshal(result)
}

// ── archUnitSlug — path → unit ID (mirrors app.unitSlug) ──────────────────────

// archUnitSlug converts a directory path or import path to a stable unit ID.
// Deterministic: same path → same slug on every machine.
// Format: "u_" + lowercase-alphanum-with-underscores.
//
// Examples:
//
//	"internal/app"            → "u_internal_app"
//	"ext:go.etcd.io/bbolt"   → "u_ext_go_etcd_io_bbolt"
//	"" or "."                 → "u_root"
func archUnitSlug(path string) string {
	if path == "" || path == "." {
		return "u_root"
	}
	path = strings.ToLower(path)
	var b strings.Builder
	prevUnderscore := false
	for _, r := range path {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			prevUnderscore = false
		} else {
			if !prevUnderscore && b.Len() > 0 {
				b.WriteByte('_')
			}
			prevUnderscore = true
		}
	}
	s := strings.TrimRight(b.String(), "_")
	if s == "" {
		return "u_root"
	}
	return "u_" + s
}

// ── prettyJSON helper ─────────────────────────────────────────────────────────

// prettyPrintJSON writes JSON to stdout, pretty-printing if pretty=true.
func prettyPrintJSON(data json.RawMessage, pretty bool) {
	if pretty {
		var v interface{}
		if err := json.Unmarshal(data, &v); err == nil {
			out, _ := json.MarshalIndent(v, "", "  ")
			fmt.Println(string(out))
			return
		}
	}
	fmt.Println(string(data))
}

// Compile-time check: cliArchQuerier satisfies ports.ArchQuerier.
var _ ports.ArchQuerier = (*cliArchQuerier)(nil)

// Ensure domain/arch import is used (for cliArchQuerier.Findings).
var _ = arch.Finding{}
