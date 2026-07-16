// COL-3 (board M6): app-layer wiring for the Ownership (view id "ownership")
// view — two readers joined at unit grain, tried in this order per unit:
//
//  1. CODEOWNERS (internal/adapters/codeowners) — Provenance "declared": an
//     extensionless disk read (D30, mirrors vl7.go's readDockerfile), probed
//     at root/CODEOWNERS, root/.github/CODEOWNERS, root/docs/CODEOWNERS.
//  2. Bounded git authorship — Provenance "derived": a single bounded `git
//     log` subprocess (same commit-depth/time-window discipline as VL-2's
//     churnSinceWindow/churnMaxCommits, vl2.go:101-114), giving each unit its
//     top commit-author within the bounded window.
//
// Deliberate deviation from the work-order's suggested "git shortlog -sne
// per unit-dir": running one `git shortlog`/`git log` subprocess per unit
// directory would itself be unbounded (a repo with N unit directories spawns
// N subprocesses per derive pass) — the exact class of risk VL-2's bounded-
// subprocess law exists to prevent. Instead this reader runs the SAME single
// bounded `git log --name-only --pretty=format:%H|%an` call VL-2 already
// established the pattern for, and aggregates per-unit author commit counts
// from its one pass of output. Recorded here per the vl3.go/vl6.go/vl7.go
// precedent of noting deliberate deviations rather than silently diverging
// from a work order's stated implementation route.
//
// A missing CODEOWNERS file is not an error (an absent file is COL-3's
// signal to fall back to git authorship for every unit); a non-git root with
// no CODEOWNERS produces a nil result — the view's honest "0 units with
// defined owners" empty state, never a phantom shard.
package app

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/corey/aoa/internal/adapters/codeowners"
	"github.com/corey/aoa/internal/domain/arch"
)

// ownershipSinceWindow bounds the git-log read by calendar time (MEASURED
// guard, same class as churnSinceWindow). Ownership benefits from a longer
// look-back than change-risk churn (authorship is a slower-moving signal
// than "what changed recently"), so this window is wider than VL-2's — but
// it is still bounded, never a full-history walk.
const ownershipSinceWindow = "365 days ago"

// ownershipMaxCommits bounds the git-log read by commit depth (the second
// half of the MEASURED guard), belt-and-suspenders with ownershipSinceWindow
// exactly as churnMaxCommits is with churnSinceWindow.
const ownershipMaxCommits = 1000

// buildOwnershipEntries assembles COL-3's Ownership view rows for one
// derive pass. root is the project root; units is the current fact set
// (unit ID -> path/file/line), used to join both readers back to a known
// unit and to supply G7 source pointers for the git-authorship fallback.
//
// Per unit, CODEOWNERS is tried first (declared); a unit CODEOWNERS doesn't
// cover falls back to bounded git authorship (derived) when git history is
// available. A unit matched by neither produces no row (never fabricated).
func buildOwnershipEntries(root string, units []arch.UnitFact) []arch.OwnershipEntry {
	rules, err := codeowners.Read(root)
	hasRules := err == nil && len(rules) > 0

	gitOwner := buildGitAuthorship(root)

	var out []arch.OwnershipEntry
	for _, u := range units {
		if hasRules {
			if owners, rule, ok := codeowners.Match(rules, u.Path); ok {
				out = append(out, arch.OwnershipEntry{
					Path:       u.Path,
					Owners:     owners,
					Provenance: "declared",
					File:       rule.File,
					Line:       rule.Line,
				})
				continue
			}
		}
		if gitOwner != nil {
			if author, ok := gitOwner[u.ID]; ok {
				out = append(out, arch.OwnershipEntry{
					Path:       u.Path,
					Owners:     []string{author},
					Provenance: "derived",
					File:       u.File,
					Line:       u.Line,
				})
			}
		}
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Path < out[j].Path
	})
	return out
}

// buildGitAuthorship maps unit ID -> its top commit-author within the
// bounded window (ownershipSinceWindow/ownershipMaxCommits). Returns nil
// when root has no git history reachable within the bound (not a git repo,
// or no commits in range) — the honest "no signal" state buildOwnershipEntries
// treats as "nothing to fall back to" for every unit.
func buildGitAuthorship(root string) map[string]string {
	raw, err := runGitAuthorshipLog(root)
	if err != nil || len(raw) == 0 {
		return nil
	}
	counts := parseGitAuthorshipLog(raw)
	if len(counts) == 0 {
		return nil
	}
	top := make(map[string]string, len(counts))
	for uid, authors := range counts {
		top[uid] = topAuthor(authors)
	}
	return top
}

// runGitAuthorshipLog runs the bounded git-log subprocess and returns its
// raw stdout: one block per commit — a "<hash>|<author>" header line,
// followed by that commit's changed file paths, blocks separated by a blank
// line (mirrors runGitChurnLog's shape, vl2.go). Any error (not a git repo,
// git missing, no commits) is returned to the caller as "nothing to show",
// not surfaced as an app-level failure — ownership is best-effort enrichment.
func runGitAuthorshipLog(root string) ([]byte, error) {
	cmd := exec.Command("git", "-C", root, "log",
		"--since="+ownershipSinceWindow,
		"--max-count="+strconv.Itoa(ownershipMaxCommits),
		"--name-only",
		"--pretty=format:%H|%an")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// parseGitAuthorshipLog parses runGitAuthorshipLog's raw stdout into unit ID
// -> author name -> commit count. Pure (no I/O) so it's testable independent
// of a real git subprocess. Grain matches parseGitChurnLog exactly:
// filepath.Dir of each changed file path, then unitSlug.
func parseGitAuthorshipLog(raw []byte) map[string]map[string]int {
	counts := make(map[string]map[string]int)

	var curAuthor string
	inCommit := false
	lines := bytes.Split(raw, []byte("\n"))
	for _, lineB := range lines {
		line := string(bytes.TrimRight(lineB, "\r"))
		if line == "" {
			continue
		}
		if hash, author, ok := splitCommitAuthorLine(line); ok {
			_ = hash
			curAuthor = author
			inCommit = true
			continue
		}
		if !inCommit {
			continue
		}
		dir := filepath.Dir(line)
		if dir == "." {
			dir = "root"
		}
		uid := unitSlug(dir)
		if counts[uid] == nil {
			counts[uid] = make(map[string]int)
		}
		counts[uid][curAuthor]++
	}
	return counts
}

// splitCommitAuthorLine splits a "<hash>|<author>" header line (this
// reader's `--pretty=format:%H|%an`) from a changed-file-path line. The
// first '|'-delimited field must be a valid 40-hex commit hash
// (isGitCommitHash, vl2.go) — a real file path never contains '|' followed
// by a hash-shaped prefix, so this is an unambiguous discriminator, same
// role as parseGitChurnLog's bare-hash-line check.
func splitCommitAuthorLine(line string) (hash, author string, ok bool) {
	idx := strings.IndexByte(line, '|')
	if idx < 0 {
		return "", "", false
	}
	h := line[:idx]
	if !isGitCommitHash(h) {
		return "", "", false
	}
	return h, line[idx+1:], true
}

// topAuthor returns the author with the highest commit count in authors,
// ties broken alphabetically (deterministic regardless of map iteration
// order).
func topAuthor(authors map[string]int) string {
	best := ""
	bestCount := -1
	for a, c := range authors {
		if c > bestCount || (c == bestCount && a < best) {
			best = a
			bestCount = c
		}
	}
	return best
}
