// Package arch — footprint.go: the deterministic, network-never repo footprint
// detector (consensus 2026-07-08, §2 Layers 0–1b; v1 grain-fix scope, ruling B).
//
// A single-tree-walk pass that reads a repo's directory footprint FIRST and
// emits project KIND + anchor(s) + per-anchor grouping GRAIN. It answers the
// scrapy §10 failure: the top-level-directory grouper collapsed scrapy's 25
// subpackages into one box because scrapy's architecture lives one level down
// under scrapy/. The footprint detects that boundary and roots the grouping
// there.
//
// v1 CUT-LINE (ruling B): Layers 0–1b only — Layer-0 noise excision + Layer-1a/1b
// manifest/unit markers. NO Layer-2 shape fallback, NO implicit-monorepo
// (≥2 1b markers → MULTI), NO multi-anchor UI. Every non-workspace repo yields
// exactly ONE anchor. Absent footprint → grouping is byte-identical to today.
//
// PROVENANCE (ruling A): the footprint grain stamps "derived" (REAL) — consistent
// with kickoff-F2 §7 D1 ("deterministic name/group/annotate of extracted facts is
// REAL"). "mixed" is reserved strictly for Haiku-touched anchors (the --refine
// seam below) and systems-overview aggregate edges (deferred).
//
// NETWORK POLICY (ruling D): this pass is network-never. The only path that would
// leave the machine is the optional Haiku --refine layer, which is NOT built in
// v1 — only the FootprintRefiner interface seam is present (see bottom of file).
package arch

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// FootprintSchema is the on-disk schema identifier for footprint.json.
const FootprintSchema = "aoa.arch-footprint/v1"

// Footprint is the deterministic detection result, written to
// {ProjectRoot}/.aoa/arch/footprint.json. It carries NO timestamp so the file
// is byte-stable across re-runs (a re-run on an unchanged tree produces
// identical bytes).
type Footprint struct {
	Schema string `json:"$schema"`
	// Kind is the dominant project kind (go-app, python-app, node-app,
	// rust-app, cloudflare-worker, …). Refined by 1c framework markers.
	Kind string `json:"kind"`
	// Engine records how the footprint was produced. "deterministic" always in
	// v1; "haiku" only when a future --refine pass touches it (ruling D).
	Engine string `json:"engine"`
	// Prov is the provenance stamp for the whole footprint. "derived" (REAL) in
	// v1 (ruling A). Becomes "mixed" only if Haiku refines an anchor.
	Prov string `json:"prov"`
	// Anchors is the scope enumerator. v1 always has exactly ONE anchor
	// (ruling B); the slice shape is future-proof for multi-anchor.
	Anchors []Anchor `json:"anchors"`
	// Excluded lists Layer-0 excised top-level dirs (noise + satellites).
	Excluded []string `json:"excluded,omitempty"`
}

// Anchor is one architectural root within the repo.
type Anchor struct {
	// Path is the repo-relative anchor directory. "" means the repo root.
	Path string `json:"path"`
	// Kind is this anchor's project kind (may differ per anchor in multi-anchor;
	// equals Footprint.Kind in v1 single-anchor).
	Kind string `json:"kind"`
	// Marker is the filename that anchored it (go.mod, setup.py, package.json…)
	// or "" for a root/no-marker anchor.
	Marker string `json:"marker,omitempty"`
	// Confidence: "high" for a marker-driven anchor; "low" reserved for the
	// deferred Layer-2 shape fallback (never emitted in v1).
	Confidence string `json:"confidence"`
	// Grain is how deep pathPrefixGroup groups paths under this anchor.
	// nil → default top-level-dir grain (byte-identical fallback).
	Grain *Grain `json:"grain,omitempty"`
}

// Grain controls the grouping depth for an anchor (the scrapy fix, §3).
//
//   - Mode "segment" (or nil Grain): group at the first meaningful segment —
//     today's default. Correct for a flat packages/* member or a root-anchored
//     repo like aOa.
//   - Mode "descend" (Under P, Depth N): for paths whose first segment is P,
//     skip that prefix and group at the Nth segment below it. Generalizes the
//     historical `internal` special-case into a data-driven rule.
type Grain struct {
	Mode  string `json:"mode"`            // "segment" | "descend"
	Under string `json:"under,omitempty"` // descend: the prefix to skip
	Depth int    `json:"depth,omitempty"` // descend: segment index below Under (≥1)
}

// ── Layer 0: excision sets ──────────────────────────────────────────────────

// footprintIgnoreDirs are hard-excluded by name (Layer 0 — never walked, never
// anchored). Mirrors the indexer's skipDirs plus build-artifact names from §2.
var footprintIgnoreDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, "dist": true,
	"build": true, "target": true, "__pycache__": true, ".next": true,
	".venv": true, ".idea": true, ".vscode": true, ".aoa": true,
	".claude": true, "bin": true, "obj": true, ".tox": true, ".pytest_cache": true,
}

// footprintSatelliteDirs are classified-out as satellites — real code but never
// architectural anchors (Linguist-derived, §2 Layer 0). They land in Excluded
// and are skipped when scanning for the dominant code subtree.
var footprintSatelliteDirs = map[string]bool{
	"docs": true, "doc": true, "test": true, "tests": true, "__tests__": true,
	"examples": true, "example": true, "spec": true, "specs": true, "extras": true,
	"testdata": true, "fixtures": true, "benchmarks": true, "scripts": true,
}

// ── Layer 1 markers ─────────────────────────────────────────────────────────

// unitMarker maps a 1b unit-marker filename to the project kind it stamps.
// v1 handles unit markers (single anchor); workspace markers (1a) are detected
// but MULTI expansion is deferred (ruling B) — a workspace repo still yields one
// root anchor in v1.
var unitMarkers = map[string]string{
	"go.mod":         "go-app",
	"package.json":   "node-app",
	"pyproject.toml": "python-app",
	"setup.py":       "python-app",
	"setup.cfg":      "python-app",
	"Cargo.toml":     "rust-app",
	"pom.xml":        "java-app",
	"build.gradle":   "java-app",
	"Gemfile":        "ruby-app",
	"composer.json":  "php-app",
	"mix.exs":        "elixir-app",
	"pubspec.yaml":   "dart-app",
}

// DetectFootprint runs the deterministic Layers 0–1b pass over the repo rooted
// at root and returns a single-anchor Footprint (ruling B). Network-never;
// single tree walk. Errors only on unreadable roots.
func DetectFootprint(root string) (*Footprint, error) {
	scan, err := walkRepo(root)
	if err != nil {
		return nil, err
	}

	// Layer 1b: find the nearest-enclosing unit marker. Prefer the shallowest
	// marker (outermost boundary); a root marker wins over a nested one.
	anchorPath, marker, kind := scan.pickUnitAnchor()

	// Layer 1b anchor-grain decision (§2.1b): decide where the architecture
	// actually lives and root the anchor + grain there.
	//
	//   - NAMED anchor (marker at depth>0, e.g. app/go.mod): the anchor's own
	//     subpackages are the architecture — descend into it.
	//   - ROOT anchor (marker at repo root): if a SINGLE dominant code subtree
	//     holds the architecture (scrapy → everything under scrapy/), re-root
	//     the anchor to that subtree and descend into it. If the architecture is
	//     spread across several top-level dirs (aOa → internal/, cmd/, ports/),
	//     keep the root anchor with default (segment) grain → byte-identical.
	var grain *Grain
	if anchorPath != "" {
		grain = &Grain{Mode: "descend", Under: firstSeg(anchorPath), Depth: 1}
	} else if dom := scan.dominantSubtree(); dom != "" {
		// Re-root the root anchor at the dominant subtree (scrapy case).
		anchorPath = dom
		grain = &Grain{Mode: "descend", Under: dom, Depth: 1}
	}

	kind = scan.refineKind(kind) // Layer 1c framework markers

	fp := &Footprint{
		Schema: FootprintSchema,
		Kind:   kind,
		Engine: "deterministic",
		Prov:   "derived", // ruling A
		Anchors: []Anchor{{
			Path:       anchorPath,
			Kind:       kind,
			Marker:     marker,
			Confidence: "high",
			Grain:      grain,
		}},
		Excluded: scan.excluded(),
	}
	return fp, nil
}

// repoScan is the single-walk result: which markers were found where, and the
// per-top-level-dir source file counts (for the grain decision).
type repoScan struct {
	root string
	// markers: repo-relative dir → set of marker filenames present there.
	markers map[string]map[string]bool
	// topDirCodeFiles: top-level dir name → count of source files beneath it.
	topDirCodeFiles map[string]int
	// satellites/ignored top-level dirs seen (for Excluded).
	excludedDirs map[string]bool
	// frameworkMarkers seen anywhere (1c).
	frameworkMarkers map[string]bool
}

// walkRepo performs the single deterministic scan (Layer 0 excision inline).
// Provider order per §2 Layer 0: prefer `git ls-files` when .git is present —
// tracked + untracked-unignored files, dropping build artifacts (gitignored
// vendor trees, grammar forests) with zero heuristics. WalkDir is the fallback
// for non-git roots.
func walkRepo(root string) (*repoScan, error) {
	s := &repoScan{
		root:             root,
		markers:          map[string]map[string]bool{},
		topDirCodeFiles:  map[string]int{},
		excludedDirs:     map[string]bool{},
		frameworkMarkers: map[string]bool{},
	}

	if files, ok := gitLsFiles(root); ok {
		// ls-files never surfaces ignored/dot dirs — record top-level ones so
		// Excluded stays in parity with the walk provider's output.
		if entries, err := os.ReadDir(root); err == nil {
			for _, e := range entries {
				n := e.Name()
				if e.IsDir() && (footprintIgnoreDirs[n] || strings.HasPrefix(n, ".")) {
					s.excludedDirs[n] = true
				}
			}
		}
		for _, rel := range files {
			s.noteFile(rel)
		}
		return s, nil
	}

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries, keep walking
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return nil
		}
		if rel == "." {
			return nil
		}
		name := d.Name()

		if d.IsDir() {
			// Layer 0: hard-exclude by name.
			if footprintIgnoreDirs[name] || strings.HasPrefix(name, ".") {
				if topLevel(rel) {
					s.excludedDirs[name] = true
				}
				return filepath.SkipDir
			}
			// Satellites are classified out (top-level only for Excluded list);
			// still walked so we don't crash, but excluded from anchor scan.
			if topLevel(rel) && footprintSatelliteDirs[name] {
				s.excludedDirs[name] = true
			}
			return nil
		}

		s.noteFile(rel)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("footprint: walk %s: %w", root, err)
	}
	return s, nil
}

// noteFile processes one repo-relative file path: Layer-0 excision by path
// segment, marker collection, and per-top-dir source counting. Shared by the
// git ls-files provider and the WalkDir fallback (which additionally prunes
// ignored dirs during the walk, making the segment guard redundant there).
func (s *repoScan) noteFile(rel string) {
	segs := strings.Split(filepath.ToSlash(rel), "/")

	// Layer 0 by segment: ignore-named or hidden dirs anywhere above the file.
	// The git provider needs this guard for files that are tracked or
	// untracked-unignored under such dirs (e.g. a committed vendor/).
	for i, seg := range segs[:len(segs)-1] {
		if footprintIgnoreDirs[seg] || strings.HasPrefix(seg, ".") {
			if i == 0 {
				s.excludedDirs[seg] = true
			}
			return
		}
	}
	if len(segs) > 1 && footprintSatelliteDirs[segs[0]] {
		s.excludedDirs[segs[0]] = true
	}

	name := segs[len(segs)-1]
	dir := ""
	if len(segs) > 1 {
		dir = strings.Join(segs[:len(segs)-1], "/")
	}

	// Record markers by their containing dir (repo-relative).
	if _, ok := unitMarkers[name]; ok || isWorkspaceMarker(name) {
		if s.markers[dir] == nil {
			s.markers[dir] = map[string]bool{}
		}
		s.markers[dir][name] = true
	}
	if isFrameworkMarker(name) {
		s.frameworkMarkers[name] = true
	}

	// Count source files per top-level dir (for grain decision).
	// Root-level files bucket under "" — they count toward the dominance
	// denominator (dominantSubtree) but can never anchor: a filename is
	// not a subtree (merge-consensus F1).
	//
	// Test files are excluded: every real file typically grows a same-named
	// test twin (a_test.go, a.test.ts, …), so counting twins double-weights
	// ordinary growth and can flip the 80% dominance threshold on churn that
	// adds no new architecture (aOa self-detection flake, VL-3).
	if isSourceFile(name) && !isTestFile(name) {
		top := ""
		if len(segs) > 1 {
			top = segs[0]
		}
		if !footprintSatelliteDirs[top] && !footprintIgnoreDirs[top] {
			s.topDirCodeFiles[top]++
		}
	}
}

// gitLsFiles returns the repo's in-scope file list — tracked plus
// untracked-unignored (`git ls-files -co --exclude-standard`) — sorted, when
// root is a git repo and git is runnable. This is Layer 0's preferred provider
// (§2): gitignored build artifacts drop out with zero heuristics. ok=false
// (no .git, git missing, or git error) → caller falls back to WalkDir.
// Deterministic: output is sorted; no timestamps, no network.
func gitLsFiles(root string) ([]string, bool) {
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		return nil, false
	}
	out, err := exec.Command("git", "-C", root, "ls-files", "-co", "--exclude-standard", "-z").Output()
	if err != nil {
		return nil, false
	}
	var files []string
	for _, f := range strings.Split(string(out), "\x00") {
		if f != "" {
			files = append(files, f)
		}
	}
	sort.Strings(files)
	return files, true
}

// pickUnitAnchor returns the anchor dir, its marker filename, and kind for the
// nearest-enclosing (shallowest-wins) 1b unit marker. "" dir means repo root.
// v1: a single anchor (ruling B) — the shallowest marker; if none, root anchor.
func (s *repoScan) pickUnitAnchor() (dir, marker, kind string) {
	type cand struct {
		dir, marker, kind string
		depth             int
	}
	var cands []cand
	for d, ms := range s.markers {
		for m := range ms {
			if k, ok := unitMarkers[m]; ok {
				cands = append(cands, cand{dir: d, marker: m, kind: k, depth: pathDepth(d)})
			}
		}
	}
	if len(cands) == 0 {
		return "", "", "unknown"
	}
	// Shallowest wins; ties broken deterministically by (dir, marker).
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].depth != cands[j].depth {
			return cands[i].depth < cands[j].depth
		}
		if cands[i].dir != cands[j].dir {
			return cands[i].dir < cands[j].dir
		}
		return cands[i].marker < cands[j].marker
	})
	c := cands[0]
	return c.dir, c.marker, c.kind
}

// dominantSubtree implements the Layer-1b anchor-grain decision for a ROOT
// anchor. It returns the name of the single top-level code dir that holds the
// architecture (scrapy → "scrapy"), or "" when the architecture is spread
// across multiple top-level dirs (aOa → internal/, cmd/, ports/), in which case
// the root anchor keeps default (segment) grain and views stay byte-identical.
//
// Rule: one top-level code dir holding ≥80% of ALL in-scope source files
// (root-level files included in the denominator) → that dominant dir.
// Otherwise "" — multi-dir layouts (aOa) and root-heavy flat repos keep the
// default segment grain, byte-identical views.
func (s *repoScan) dominantSubtree() string {
	total := 0
	best := ""
	bestN := 0
	for dir, n := range s.topDirCodeFiles {
		total += n
		if dir == "" {
			continue // root-level files count in total but never anchor (F1)
		}
		if n > bestN || (n == bestN && dir < best) {
			bestN, best = n, dir
		}
	}
	if total == 0 || best == "" {
		return ""
	}
	if float64(bestN) >= 0.80*float64(total) {
		return best // one dir dominates the code
	}
	return "" // spread layout (aOa) or root-heavy flat repo → default grain
}

// refineKind applies Layer 1c framework markers to sharpen kind (never boundary).
func (s *repoScan) refineKind(kind string) string {
	if s.frameworkMarkers["wrangler.toml"] || s.frameworkMarkers["wrangler.json"] || s.frameworkMarkers["wrangler.jsonc"] {
		return "cloudflare-worker"
	}
	return kind
}

// excluded returns the sorted list of Layer-0 excised top-level dirs.
func (s *repoScan) excluded() []string {
	out := make([]string, 0, len(s.excludedDirs))
	for d := range s.excludedDirs {
		out = append(out, d)
	}
	sort.Strings(out)
	if len(out) == 0 {
		return nil
	}
	return out
}

// ── small helpers ───────────────────────────────────────────────────────────

func topLevel(rel string) bool { return !strings.Contains(rel, string(filepath.Separator)) }

func firstSeg(rel string) string {
	rel = filepath.ToSlash(rel)
	if i := strings.IndexByte(rel, '/'); i >= 0 {
		return rel[:i]
	}
	return rel
}

func pathDepth(rel string) int {
	if rel == "" {
		return 0
	}
	return strings.Count(filepath.ToSlash(rel), "/") + 1
}

func isWorkspaceMarker(name string) bool {
	switch name {
	case "pnpm-workspace.yaml", "go.work", "lerna.json", "nx.json", "turbo.json",
		"MODULE.bazel", "WORKSPACE":
		return true
	}
	return false
}

func isFrameworkMarker(name string) bool {
	switch name {
	case "wrangler.toml", "wrangler.json", "wrangler.jsonc",
		"serverless.yml", "serverless.yaml", "Dockerfile", "docker-compose.yml":
		return true
	}
	return strings.HasPrefix(name, "next.config.")
}

// isSourceFile reports whether a filename is a code source file for the grain
// count. A conservative superset of common languages (extension match).
func isSourceFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".go", ".py", ".pyw", ".js", ".jsx", ".mjs", ".cjs", ".ts", ".tsx", ".mts",
		".rs", ".java", ".c", ".h", ".cpp", ".hpp", ".cc", ".cxx", ".cs", ".rb",
		".php", ".swift", ".kt", ".scala", ".ex", ".exs", ".erl", ".hs", ".ml",
		".clj", ".lua", ".dart", ".sh":
		return true
	}
	return false
}

// isTestFile reports whether name is a test file by common cross-language
// naming convention (a same-named twin of a real source file). Test twins are
// excluded from the dominance count (dominantSubtree) so ordinary growth —
// which adds a test file alongside every real file — never asymmetrically
// tips which top-level dir "dominates" the repo.
func isTestFile(name string) bool {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	switch {
	case strings.HasSuffix(name, "_test.go"),
		strings.HasSuffix(name, "_test.py"),
		strings.HasSuffix(base, ".test"),
		strings.HasSuffix(base, ".spec"),
		strings.HasSuffix(base, "_spec"):
		return true
	}
	return false
}

// ── Marshal / Parse / Load / Save ───────────────────────────────────────────

// MarshalFootprint encodes a Footprint to byte-stable JSON (no timestamp).
func MarshalFootprint(fp *Footprint) ([]byte, error) {
	data, err := json.MarshalIndent(fp, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("footprint: marshal: %w", err)
	}
	return data, nil
}

// ParseFootprint decodes footprint.json bytes, validating the schema tag.
func ParseFootprint(data []byte) (*Footprint, error) {
	var fp Footprint
	if err := json.Unmarshal(data, &fp); err != nil {
		return nil, fmt.Errorf("footprint: parse: %w", err)
	}
	if fp.Schema != FootprintSchema {
		return nil, fmt.Errorf("footprint: unsupported schema %q — expected %s", fp.Schema, FootprintSchema)
	}
	return &fp, nil
}

// FootprintPath returns the on-disk path for a project root's footprint.json.
func FootprintPath(projectRoot string) string {
	return filepath.Join(projectRoot, ".aoa", "arch", "footprint.json")
}

// LoadFootprint reads and parses footprint.json for a project root.
// Returns (nil, nil) when the file does not exist (absent → default grain).
func LoadFootprint(projectRoot string) (*Footprint, error) {
	data, err := os.ReadFile(FootprintPath(projectRoot))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("footprint: read: %w", err)
	}
	return ParseFootprint(data)
}

// SaveFootprint writes footprint.json under {root}/.aoa/arch/ (created as needed).
func SaveFootprint(projectRoot string, fp *Footprint) error {
	dir := filepath.Join(projectRoot, ".aoa", "arch")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("footprint: mkdir %s: %w", dir, err)
	}
	data, err := MarshalFootprint(fp)
	if err != nil {
		return err
	}
	if err := os.WriteFile(FootprintPath(projectRoot), data, 0o644); err != nil {
		return fmt.Errorf("footprint: write: %w", err)
	}
	return nil
}

// PrimaryGrain returns the v1 single-anchor's grain (or nil). v1 applies one
// grain globally in pathPrefixGroup; multi-anchor grain-per-scope is deferred.
func (fp *Footprint) PrimaryGrain() *Grain {
	if fp == nil || len(fp.Anchors) == 0 {
		return nil
	}
	return fp.Anchors[0].Grain
}

// Summary returns a one-line human description for `aoa arch recon`:
//   "kind: python-app · anchor: scrapy/ · grain: subpackage".
func (fp *Footprint) Summary() string {
	if fp == nil || len(fp.Anchors) == 0 {
		return "no footprint"
	}
	a := fp.Anchors[0]
	anchor := "<root>/"
	if a.Path != "" {
		anchor = a.Path + "/"
	}
	grain := "top-level"
	if a.Grain != nil && a.Grain.Mode == "descend" {
		grain = fmt.Sprintf("subpackage (descend under %s/ depth %d)", a.Grain.Under, a.Grain.Depth)
	}
	return fmt.Sprintf("kind: %s · anchor: %s · grain: %s", fp.Kind, anchor, grain)
}

// ── Haiku refinement seam (ruling D — NOT built in v1) ──────────────────────

// FootprintRefiner is the single interface point the optional Haiku --refine
// layer drops into later (consensus §4, ruling D). It takes a completed
// deterministic footprint and may relabel anchors + tie-break low-confidence
// grain — it may NEVER invent an anchor the deterministic pass did not surface,
// and any anchor it touches must be stamped Prov "mixed". In v1 the ONLY
// implementation is DeterministicRefiner (a no-op); no network code exists and
// no --refine flag is wired live. When the Haiku layer ships, it satisfies this
// same interface, degrades to DeterministicRefiner on any network failure, and
// its egress is dir names + @domain histograms only — NEVER file contents.
type FootprintRefiner interface {
	RefineFootprint(fp *Footprint) (*Footprint, error)
}

// DeterministicRefiner is the v1 no-op refiner: it returns the deterministic
// footprint unchanged. This is the seam; the Haiku implementation is deferred.
type DeterministicRefiner struct{}

// RefineFootprint returns fp unchanged (v1 no-op — the deterministic footprint
// is already complete and usable; Haiku is quality, never correctness, §4).
func (DeterministicRefiner) RefineFootprint(fp *Footprint) (*Footprint, error) {
	return fp, nil
}

// compile-time seam check.
var _ FootprintRefiner = DeterministicRefiner{}
