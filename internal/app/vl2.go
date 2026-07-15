// VL-2 (board #36): app-layer wiring for the Change Map (view id "change")
// view — a bounded git-churn reader joined with an indexed complexity proxy
// (symbol count).
//
// Bounded by design (spec: "churn is superlinear in commits×files"): the git
// read caps BOTH commit depth (churnMaxCommits) and calendar window
// (churnSinceWindow) so a large, old repo cannot turn one derive pass into a
// full-history scan. Reads run via `git log` subprocess — no new go.mod
// dependency — mirroring the "snapshot-release" shape of buildRefHits/
// buildCodeSymbolIndex (idx is read directly at derive time, never through
// FactStore; see vl1.go's package doc for the analogous SBOM/TechStack
// rationale). A missing/non-git root is not an error: buildChurnEntries
// returns nil and the Change Map view renders its honest "0 units changed"
// empty state (never a phantom shard).
package app

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/corey/aoa/internal/domain/arch"
	"github.com/corey/aoa/internal/ports"
)

// churnSinceWindow bounds the git-log read by calendar time (MEASURED
// guard — see package doc). 90 days is a v1 80/20 default: long enough to
// surface real hot spots, short enough that a decade-old repo's full history
// is never walked.
const churnSinceWindow = "90 days ago"

// churnMaxCommits bounds the git-log read by commit depth (the second half
// of the MEASURED guard) — belt-and-suspenders with churnSinceWindow so a
// repo with an unusually high commit rate inside the window still can't turn
// one derive pass into an unbounded scan.
const churnMaxCommits = 500

// buildChurnEntries assembles VL-2's Change Map rows for one derive pass.
// root is the project root (git commands run with `-C root`). units is the
// current fact set (unit ID → path/file/line), used to join churn back to a
// known unit and to supply G7 source pointers. idx may be nil (mirrors
// buildRefHits/buildCodeSymbolIndex's contract) — complexity is then simply
// absent (0) for every row, churn rows still render.
//
// Returns nil when root has no git history reachable within the bounded
// window (not a git repo, or no commits in range) — the honest empty state.
func buildChurnEntries(root string, units []arch.UnitFact, idx *ports.Index) []arch.ChurnEntry {
	raw, err := runGitChurnLog(root)
	if err != nil || len(raw) == 0 {
		return nil
	}

	changedFiles, commits := parseGitChurnLog(raw)
	if len(changedFiles) == 0 {
		return nil
	}

	complexity := buildComplexity(idx)

	unitByID := make(map[string]arch.UnitFact, len(units))
	for _, u := range units {
		unitByID[u.ID] = u
	}

	entries := make([]arch.ChurnEntry, 0, len(changedFiles))
	for uid, files := range changedFiles {
		u, ok := unitByID[uid]
		if !ok {
			// Churn landed in a directory with no current unit fact (e.g. a
			// since-deleted package). Not this view's job to invent a unit
			// record — skip, same "known unit only" discipline as
			// buildRefHits's dead-candidate join.
			continue
		}
		comp := complexity[uid]
		entries = append(entries, arch.ChurnEntry{
			Path:         u.Path,
			ChangedFiles: len(files),
			Commits:      len(commits[uid]),
			Complexity:   comp,
			Risk:         len(files) * comp,
			File:         u.File,
			Line:         u.Line,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Path < entries[j].Path
	})
	return entries
}

// runGitChurnLog runs the bounded git-log subprocess and returns its raw
// stdout: one block per commit — a line holding just the commit hash,
// followed by that commit's changed file paths, blocks separated by a blank
// line (git log --name-only's native shape). Any error (not a git repo, git
// missing, no commits) is returned to the caller as "nothing to show", not
// surfaced as an app-level failure — churn is best-effort enrichment.
func runGitChurnLog(root string) ([]byte, error) {
	cmd := exec.Command("git", "-C", root, "log",
		"--since="+churnSinceWindow,
		"--max-count="+strconv.Itoa(churnMaxCommits),
		"--name-only",
		"--pretty=format:%H")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// parseGitChurnLog parses runGitChurnLog's raw stdout into two maps: unit ID
// → set of distinct changed file paths, and unit ID → set of distinct commit
// hashes that touched it. Pure (no I/O) so it's testable independent of a
// real git subprocess.
//
// Grain: filepath.Dir of each changed file path, then unitSlug — identical
// to aggregateEdges/buildRefHits's unit keyspace, so churn agrees with the
// current fact set's unit IDs by construction.
func parseGitChurnLog(raw []byte) (changedFiles map[string]map[string]bool, commits map[string]map[string]bool) {
	changedFiles = make(map[string]map[string]bool)
	commits = make(map[string]map[string]bool)

	var curCommit string
	lines := bytes.Split(raw, []byte("\n"))
	for _, lineB := range lines {
		line := string(bytes.TrimRight(lineB, "\r"))
		if line == "" {
			continue
		}
		if isGitCommitHash(line) {
			curCommit = line
			continue
		}
		if curCommit == "" {
			continue
		}
		dir := filepath.Dir(line)
		if dir == "." {
			dir = "root"
		}
		uid := unitSlug(dir)

		if changedFiles[uid] == nil {
			changedFiles[uid] = make(map[string]bool)
		}
		changedFiles[uid][line] = true

		if commits[uid] == nil {
			commits[uid] = make(map[string]bool)
		}
		commits[uid][curCommit] = true
	}
	return changedFiles, commits
}

// isGitCommitHash reports whether line looks like a `%H` full commit hash
// (40 lowercase hex chars) — the only way parseGitChurnLog distinguishes a
// commit-header line from a changed-file-path line in `--name-only` output.
func isGitCommitHash(line string) bool {
	if len(line) != 40 {
		return false
	}
	for _, r := range line {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}

// buildComplexity maps unit ID → indexed symbol count (the complexity
// proxy). Grain and iteration shape mirror buildRefHits exactly (same
// filepath.Dir + unitSlug keyspace) but counts idx.Metadata entries — one
// per named symbol (function/type/etc) — rather than idx.Tokens' raw
// reference hits, since "how many named symbols live here" is the more
// direct complexity signal.
//
// Returns an empty (non-nil) map when idx is nil, so callers can index it
// unconditionally; every lookup then yields 0 complexity.
func buildComplexity(idx *ports.Index) map[string]int {
	complexity := make(map[string]int)
	if idx == nil {
		return complexity
	}
	for ref := range idx.Metadata {
		fm, ok := idx.Files[ref.FileID]
		if !ok || fm.Path == "" {
			continue
		}
		dir := filepath.Dir(fm.Path)
		if dir == "." {
			dir = "root"
		}
		complexity[unitSlug(dir)]++
	}
	return complexity
}
