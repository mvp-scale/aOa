//go:build compliance

// Package compliance validates aOa's integration contract with Claude Code's
// on-disk session format. See CONTRACT.md for the full spec.
//
// This file deliberately does NOT import any aOa packages — the test encodes
// the expected schema directly so that it validates the contract, not the
// implementation. If parser.go drifts, this test still tells you what
// Claude Code actually emits.
//
// Run:
//
//	go test -tags compliance ./compliance/claude-session/...
//	AOA_COMPLIANCE_SESSION_DIR=/path/to/dir go test -tags compliance ./compliance/claude-session/...
//
// Without the build tag, this file is invisible to `go test ./...`.
package compliance

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Recognized top-level event types — the set our parser actively translates.
// Any type outside this set is reported as drift.
var recognizedTypes = map[string]bool{
	"user":            true,
	"assistant":       true,
	"system":          true,
	"permission-mode": true, // translated by translatePermissionMode
	"attachment":      true, // translated by translateAttachment
	"last-prompt":     true, // translated by translateLastPrompt
	"ai-title":        true, // translated by translateAITitle
}

// Known-but-unhandled top-level types — silently dropped by the parser.
// Listed here so they are not flagged as "new drift" repeatedly.
// To raise the bar (treat unhandled as drift), remove from this set.
var knownUnhandledTypes = map[string]bool{
	"progress":              true, // mentioned in reader.go but not actively consumed
	"queue-operation":       true, // CONFIRMED v2.1.172 — message-queue lifecycle; enqueue.content carries queued input. Acknowledge-drop (L20.1).
	"file-history-snapshot": true, // mentioned in reader.go but not actively consumed
	"mode":                  true, // NEW v2.1.172 — interaction-mode control-plane event, sibling of permission-mode. Acknowledge-drop (L20.1).
}

// Top-level envelope fields the parser consumes. New fields beyond this set
// are reported as informational drift (signal we are dropping).
var consumedEnvelopeFields = map[string]bool{
	// Identity envelope (always-consumed)
	"type":        true,
	"uuid":        true,
	"id":          true, // alternate uuid key
	"timestamp":   true,
	"version":     true,
	"sessionId":   true,
	"session_id":  true,
	"isMeta":      true,
	"subtype":     true,
	"parentUuid":  true,
	"parent_uuid": true,
	"parentId":    true,
	"durationMs":  true,
	"message":     true, // body, recursed into separately

	// Envelope context (v2.1.126+) — all consumed by parser into tailer.SessionEvent
	"cwd":                     true,
	"entrypoint":              true,
	"gitBranch":               true,
	"isSidechain":             true,
	"userType":                true,
	"requestId":               true,
	"promptId":                true,
	"sourceToolAssistantUUID": true,
	"toolUseResult":           true,
	"messageCount":            true,
	"permissionMode":          true,
	"origin":                  true,
	// Type-scoped envelope fields on new event types
	"aiTitle":    true,
	"lastPrompt": true,
	"leafUuid":   true,
	"attachment": true,
	"content":    true, // system events with subtype=away_summary carry resume text here
}

// Known-but-unconsumed envelope fields — signal Claude emits that the parser
// sees but does not read. New fields land here pending a processing decision.
var knownUnconsumedEnvelopeFields = map[string]bool{
	"promptSource": true, // NEW v2.1.172 (user) — how the prompt entered the turn ("typed" / queued / ...). Acknowledge-drop (L20.1).
	"level":        true, // NEW v2.1.172 (system) — severity level ("info"). Acknowledge-drop (L20.1).
	"slug":         true, // NEW v2.1.172 (user/assistant/attachment) — human-readable session slug (e.g. "staged-floating-candy"). Acknowledge-drop (L20.1).
	"agentId":             true, // NEW v2.1.178 (user/assistant/attachment) — links a top-level event to a spawned subagent; value = subagent file shortid. Acknowledge-drop; HIGH-value L18 attribution link.
	"attributionAgent":    true, // NEW v2.1.178 (assistant) — subagent TYPE that produced the event ("general-purpose"). Pairs with agentId. Acknowledge-drop (L20.1).
	"pendingWorkflowCount": true, // NEW v2.1.173 (system) — per-session pending-workflow counter. Acknowledge-drop (L20.1).
	"messageCount":        true, // NEW v2.1.173 (system) — per-session message counter. Acknowledge-drop (L20.1).
}

// Known-but-unhandled system subtypes — observed but not branched on.
// system events with these subtypes are passed through with subtype only.
var knownUnhandledSystemSubtypes = map[string]bool{
	"turn_duration": true, // handled (we read durationMs)
	"away_summary":  true, // observed v2.1.126 — carries `content` field with resume summary
	"local_command": true, // NEW v2.1.172 — local slash-command echo; carries `content` + `level`. content captured into SystemContent but only surfaced for away_summary. Acknowledge-drop (L20.1).
}

// Recognized message-content block types.
var recognizedContentBlockTypes = map[string]bool{
	"text":        true,
	"thinking":    true,
	"tool_use":    true,
	"tool_result": true,
}

// Known content block types observed but not actively consumed.
var knownUnhandledContentBlockTypes = map[string]bool{
	"image": true,
}

// Reference sets used by the non-compliance coverage report (Pass 5).
// These mirror what parser.go actually consumes.

// Usage fields the parser reads. All v2.1.126 usage fields are consumed.
var consumedUsageFields = map[string]bool{
	"input_tokens":                true,
	"output_tokens":               true,
	"cache_read_input_tokens":     true,
	"cache_creation_input_tokens": true,
	"service_tier":                true,
	"cache_creation":              true, // nested TTL bucket breakdown
	"server_tool_use":             true, // nested web search/fetch counts
	"inference_geo":               true,
	"iterations":                  true,
	"speed":                       true,
}

// Message-level fields the parser reads.
var consumedMessageFields = map[string]bool{
	"role":          true,
	"model":         true,
	"content":       true,
	"usage":         true,
	"id":            true, // Anthropic API message ID
	"stop_reason":   true,
	"stop_sequence": true,
	"stop_details":  true,
	// `type` (literal "message") is intentionally [SKIP] — see CONTRACT.md §3.6
}

// Message fields explicitly skipped (pure API echo, no value on canonical event).
// Listed here so the report distinguishes intentional skips from drift.
var skippedMessageFields = map[string]bool{
	"type": true, // Anthropic API response type literal
}

// System subtypes whose payload the parser consumes. The parser does not
// branch on subtype value during translation, but each subtype's relevant
// payload is extracted: turn_duration -> durationMs, away_summary -> content.
var consumedSystemSubtypes = map[string]bool{
	"turn_duration": true, // durationMs is read
	"away_summary":  true, // content is read into AwaySummary
}

// Tools whose toolUseResult shape we read into a typed struct.
// Per the integration plan: all four discriminated shapes are integrated.
// Processing differs (Bash/Agent USED; Edit/other on HOLD) but integration
// coverage is 100%.
var consumedToolUseResultShapes = map[string]bool{
	"Bash":  true,
	"Edit":  true,
	"Agent": true,
	"other": true,
	// NEW v2.1.172 — these shapes do not match the Bash/Edit/Agent signature
	// keys, so the parser absorbs them via the "other" catch-all (Raw preserved,
	// no crash/drop). Counted consumed on the same basis as "other". If a typed
	// branch is ever added, move the relevant fields into the contract.
	"ToolSearch": true,
	"TaskCreate": true,
	"TaskUpdate": true,
}

// Required fields per recognized type. These MUST be present; absence is FAIL.
// (Required from aOa's perspective — i.e., what the parser would silently miss.)
var requiredFieldsByType = map[string][]string{
	"user":      {"type", "uuid", "timestamp", "message"},
	"assistant": {"type", "uuid", "timestamp", "message"},
	"system":    {"type", "uuid", "timestamp"},
}

// observedManifest holds the highest-versioned validated baseline.
type observedManifest struct {
	Version  string         // e.g., "2.1.126"
	Dir      string         // e.g., "versions/v2.1.126-observed"
	Manifest map[string]any // parsed manifest.json
}

// loadBaseline scans versions/v*-observed/ and returns the highest-versioned
// manifest. This is the "last validated" contract.
func loadBaseline(t *testing.T) observedManifest {
	t.Helper()
	entries, err := os.ReadDir("versions")
	if err != nil {
		t.Skipf("versions/ not readable: %v", err)
	}
	type cand struct {
		name string
		ver  []int
	}
	var best cand
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		n := e.Name()
		if !strings.HasPrefix(n, "v") || !strings.HasSuffix(n, "-observed") {
			continue
		}
		verStr := strings.TrimSuffix(strings.TrimPrefix(n, "v"), "-observed")
		ver := parseSemver(verStr)
		if ver == nil {
			continue
		}
		if best.name == "" || semverLess(best.ver, ver) {
			best = cand{name: n, ver: ver}
		}
	}
	if best.name == "" {
		t.Skipf("no v*-observed/ directories under versions/")
	}
	dir := filepath.Join("versions", best.name)
	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Skipf("cannot read %s/manifest.json: %v", dir, err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Skipf("bad manifest.json in %s: %v", dir, err)
	}
	v, _ := m["claude_code_version"].(string)
	return observedManifest{Version: v, Dir: dir, Manifest: m}
}

// parseSemver converts "2.1.126" → [2,1,126]. Returns nil on malformed input.
func parseSemver(s string) []int {
	parts := strings.Split(s, ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil
		}
		out = append(out, n)
	}
	return out
}

// semverLess returns true if a < b (component-wise).
func semverLess(a, b []int) bool {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return len(a) < len(b)
}

// observedVersionsInSession scans the latest JSONL and returns the set of
// `version` field values seen. In a healthy session there should be exactly one.
func observedVersionsInSession(t *testing.T, jsonlPath string) []string {
	t.Helper()
	data, err := os.ReadFile(jsonlPath)
	if err != nil {
		t.Skipf("cannot read %s: %v", jsonlPath, err)
	}
	seen := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || len(line) > 512*1024 {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			continue
		}
		if v, _ := obj["version"].(string); v != "" {
			seen[v] = true
		}
	}
	out := make([]string, 0, len(seen))
	for v := range seen {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

// sessionDir resolves the Claude Code project session dir to validate.
// Override via AOA_COMPLIANCE_SESSION_DIR; otherwise compute from this repo.
func sessionDir(t *testing.T) string {
	t.Helper()
	if v := os.Getenv("AOA_COMPLIANCE_SESSION_DIR"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("cannot resolve home dir: %v", err)
	}
	// Use this repo's encoded path as the default subject of validation.
	repo, err := os.Getwd()
	if err != nil {
		t.Skipf("cannot resolve cwd: %v", err)
	}
	// compliance_test.go runs in compliance/claude-session — walk up to repo root.
	for i := 0; i < 4 && filepath.Base(repo) != "aOa-go"; i++ {
		repo = filepath.Dir(repo)
	}
	encoded := strings.ReplaceAll(repo, "/", "-")
	return filepath.Join(home, ".claude", "projects", encoded)
}

// latestJSONL returns the most recently modified .jsonl file in dir.
func latestJSONL(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skipf("session dir not readable %q: %v", dir, err)
	}
	var newest os.DirEntry
	var newestTime int64
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if t := info.ModTime().UnixNano(); t > newestTime {
			newestTime = t
			newest = e
		}
	}
	if newest == nil {
		t.Skipf("no .jsonl files found in %q", dir)
	}
	return filepath.Join(dir, newest.Name())
}

// ---------- Pass 0: Version Header ----------

// TestPass0_Versions reports the validated baseline vs the live session
// version. Runs first by alphabetical ordering. Drift between baseline and
// live triggers a FAIL — that's the signal that the contract needs to be
// re-captured against the new version.
func TestPass0_Versions(t *testing.T) {
	dir := sessionDir(t)
	latest := latestJSONL(t, dir)
	baseline := loadBaseline(t)
	observed := observedVersionsInSession(t, latest)

	t.Logf("─── Claude Code Session Contract ───────────────────────────────")
	t.Logf("  Repo dir under test:    %s", dir)
	t.Logf("  Active session file:    %s", filepath.Base(latest))
	t.Logf("  Baseline (validated):   v%s   (from %s)", baseline.Version, baseline.Dir)
	if len(observed) == 0 {
		t.Logf("  Observed (live):        <no version field found in session>")
	} else if len(observed) == 1 {
		t.Logf("  Observed (live):        v%s", observed[0])
	} else {
		t.Logf("  Observed (live):        %v   (mixed — session spans Claude Code upgrade)", observed)
	}

	switch {
	case len(observed) == 0:
		t.Logf("  Status:                 UNKNOWN — cannot determine live version")
	case len(observed) == 1 && observed[0] == baseline.Version:
		t.Logf("  Status:                 MATCH — contract validated at this exact version")
	case len(observed) == 1:
		liveVer := parseSemver(observed[0])
		baseVer := parseSemver(baseline.Version)
		direction := "newer than"
		if liveVer != nil && baseVer != nil && semverLess(liveVer, baseVer) {
			direction = "older than"
		}
		t.Errorf("DRIFT: live version v%s is %s baseline v%s — re-capture versions/v%s-observed/ and re-run", observed[0], direction, baseline.Version, observed[0])
	default:
		t.Errorf("DRIFT: live session contains multiple versions %v — baseline is v%s", observed, baseline.Version)
	}
	t.Logf("─────────────────────────────────────────────────────────────────")
}

// ---------- Pass 1: Topology ----------

func TestPass1_Topology(t *testing.T) {
	dir := sessionDir(t)

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("FAIL: session directory missing: %s (%v)", dir, err)
	}
	if !info.IsDir() {
		t.Fatalf("FAIL: %s is not a directory", dir)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("FAIL: cannot read %s: %v", dir, err)
	}

	// Verify the {S}.jsonl + {S}/ co-located shape.
	jsonls := map[string]bool{}
	subdirs := map[string]bool{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			subdirs[name] = true
			continue
		}
		if strings.HasSuffix(name, ".jsonl") {
			jsonls[strings.TrimSuffix(name, ".jsonl")] = true
		}
	}

	pairCount := 0
	for s := range jsonls {
		if subdirs[s] {
			pairCount++
		}
	}
	if pairCount == 0 {
		t.Errorf("FAIL: no {S}.jsonl + {S}/ co-located pairs found in %s", dir)
	} else {
		t.Logf("OK: %d co-located {S}.jsonl + {S}/ pairs", pairCount)
	}

	// Verify subagents/ and tool-results/ shape on the latest session's directory.
	latest := latestJSONL(t, dir)
	sessionID := strings.TrimSuffix(filepath.Base(latest), ".jsonl")
	artifactsDir := filepath.Join(dir, sessionID)
	if _, err := os.Stat(artifactsDir); err != nil {
		t.Logf("INFO: no artifacts dir for latest session %s (acceptable for new sessions)", sessionID)
		return
	}
	for _, sub := range []string{"subagents", "tool-results"} {
		path := filepath.Join(artifactsDir, sub)
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			t.Logf("OK: %s/ exists", sub)
		} else {
			t.Logf("INFO: %s/ not present in latest session (acceptable if no triggers fired)", sub)
		}
	}
}

// ---------- Pass 2: File format ----------

func TestPass2_FileFormat(t *testing.T) {
	dir := sessionDir(t)
	latest := latestJSONL(t, dir)

	data, err := os.ReadFile(latest)
	if err != nil {
		t.Fatalf("FAIL: cannot read %s: %v", latest, err)
	}

	lines := strings.Split(string(data), "\n")
	parsed := 0
	bad := 0
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// 512KB cap matches tailer.go behavior.
		if len(line) > 512*1024 {
			t.Logf("INFO: line %d exceeds 512KB cap (%d bytes) — would be skipped by tailer", i+1, len(line))
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			bad++
			if bad <= 3 {
				t.Errorf("FAIL: line %d is not valid JSON object: %v", i+1, err)
			}
			continue
		}
		parsed++
	}

	if parsed == 0 {
		t.Errorf("FAIL: no valid JSON objects found in %s", latest)
	}
	t.Logf("OK: parsed %d JSON objects (%d failures) in %s", parsed, bad, filepath.Base(latest))
}

// ---------- Pass 3: Schema ----------

func TestPass3_Schema(t *testing.T) {
	dir := sessionDir(t)
	latest := latestJSONL(t, dir)

	data, err := os.ReadFile(latest)
	if err != nil {
		t.Fatalf("FAIL: cannot read %s: %v", latest, err)
	}

	observedTypes := map[string]int{}
	observedEnvelopeFields := map[string]map[string]bool{}    // type -> set
	observedContentBlocks := map[string]int{}
	observedMissingRequired := map[string][]string{}          // type -> missing fields seen
	parseFailLines := 0

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || len(line) > 512*1024 {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			parseFailLines++
			continue
		}
		typ, _ := obj["type"].(string)
		if typ == "" {
			typ = "<missing>"
		}
		observedTypes[typ]++

		// Track envelope field set per type
		if observedEnvelopeFields[typ] == nil {
			observedEnvelopeFields[typ] = map[string]bool{}
		}
		for k := range obj {
			observedEnvelopeFields[typ][k] = true
		}

		// Required-field check for recognized types
		if req, ok := requiredFieldsByType[typ]; ok {
			for _, f := range req {
				if _, present := obj[f]; !present {
					observedMissingRequired[typ] = append(observedMissingRequired[typ], f)
				}
			}
		}

		// Content block type enumeration
		if msg, ok := obj["message"].(map[string]any); ok {
			if content, ok := msg["content"].([]any); ok {
				for _, blk := range content {
					if m, ok := blk.(map[string]any); ok {
						if bt, _ := m["type"].(string); bt != "" {
							observedContentBlocks[bt]++
						}
					}
				}
			}
		}
	}

	// 3a. Check for new (drifted) top-level types
	for typ, count := range observedTypes {
		if recognizedTypes[typ] || knownUnhandledTypes[typ] {
			continue
		}
		t.Errorf("DRIFT (new type): %q (%d events) is neither recognized nor known-unhandled — investigate and add to one of the maps in this file", typ, count)
	}
	for typ, count := range observedTypes {
		if recognizedTypes[typ] {
			t.Logf("OK: type=%s count=%d (translated)", typ, count)
		} else if knownUnhandledTypes[typ] {
			t.Logf("INFO: type=%s count=%d (known-unhandled — silently dropped)", typ, count)
		}
	}

	// 3b. Required-field absences = FAIL
	for typ, missing := range observedMissingRequired {
		uniq := uniqueStrings(missing)
		t.Errorf("FAIL: type=%s missing required field(s): %v", typ, uniq)
	}

	// 3c. New envelope fields on recognized types = INFO (drift signal)
	for typ, fields := range observedEnvelopeFields {
		if !recognizedTypes[typ] {
			continue
		}
		var unconsumed, drift []string
		for f := range fields {
			if consumedEnvelopeFields[f] {
				continue
			}
			if knownUnconsumedEnvelopeFields[f] {
				unconsumed = append(unconsumed, f)
			} else {
				drift = append(drift, f)
			}
		}
		sort.Strings(unconsumed)
		sort.Strings(drift)
		if len(drift) > 0 {
			t.Errorf("DRIFT (new envelope field on type=%s): %v — investigate, then add to consumedEnvelopeFields or knownUnconsumedEnvelopeFields", typ, drift)
		}
		if len(unconsumed) > 0 {
			t.Logf("INFO: type=%s carries unconsumed fields %v (signal we are dropping)", typ, unconsumed)
		}
	}

	// 3d. Content block drift
	for bt, count := range observedContentBlocks {
		switch {
		case recognizedContentBlockTypes[bt]:
			t.Logf("OK: content_block=%s count=%d", bt, count)
		case knownUnhandledContentBlockTypes[bt]:
			t.Logf("INFO: content_block=%s count=%d (known-unhandled)", bt, count)
		default:
			t.Errorf("DRIFT (new content block type): %q (%d occurrences) — investigate and add to one of the maps in this file", bt, count)
		}
	}

	if parseFailLines > 0 {
		t.Logf("INFO: %d lines failed to parse during schema enumeration", parseFailLines)
	}

	// ---- Synthesized drift summary ----
	baseline := loadBaseline(t)
	live := observedVersionsInSession(t, latest)
	liveVerStr := "<unknown>"
	if len(live) == 1 {
		liveVerStr = "v" + live[0]
	} else if len(live) > 1 {
		liveVerStr = fmt.Sprintf("%v (mixed)", live)
	}

	var newTypes, knownUnhandled []string
	for typ := range observedTypes {
		if recognizedTypes[typ] {
			continue
		}
		if knownUnhandledTypes[typ] {
			knownUnhandled = append(knownUnhandled, typ)
		} else {
			newTypes = append(newTypes, typ)
		}
	}
	sort.Strings(newTypes)
	sort.Strings(knownUnhandled)

	var newFields, unconsumedFields []string
	for typ, fields := range observedEnvelopeFields {
		if !recognizedTypes[typ] {
			continue
		}
		for f := range fields {
			if consumedEnvelopeFields[f] {
				continue
			}
			if knownUnconsumedEnvelopeFields[f] {
				unconsumedFields = append(unconsumedFields, typ+"."+f)
			} else {
				newFields = append(newFields, typ+"."+f)
			}
		}
	}
	sort.Strings(newFields)
	sort.Strings(unconsumedFields)

	missingRequiredCount := 0
	for _, missing := range observedMissingRequired {
		missingRequiredCount += len(uniqueStrings(missing))
	}

	t.Logf("─── Drift Summary (live %s vs baseline v%s) ─────────────", liveVerStr, baseline.Version)
	t.Logf("  Required fields missing:        %d   (FAIL if >0)", missingRequiredCount)
	t.Logf("  Unrecognized event types:       %d   (FAIL if >0)  %v", len(newTypes), newTypes)
	t.Logf("  Unrecognized envelope fields:   %d   (FAIL if >0)  %v", len(newFields), newFields)
	t.Logf("  Known-unhandled event types:    %d   (silently dropped) %v", len(knownUnhandled), knownUnhandled)
	t.Logf("  Known-unconsumed env fields:    %d   (signal we drop)   %v", len(unconsumedFields), unconsumedFields)
	switch {
	case missingRequiredCount > 0 || len(newTypes) > 0 || len(newFields) > 0:
		t.Logf("  Status:                         BREAKING DRIFT — contract needs update")
	case len(live) == 1 && live[0] != baseline.Version:
		t.Logf("  Status:                         VERSION DRIFT — additive only, recapture baseline")
	case len(knownUnhandled) > 0 || len(unconsumedFields) > 0:
		t.Logf("  Status:                         ALIGNED with baseline (additive backlog exists — see observations.md)")
	default:
		t.Logf("  Status:                         FULLY ALIGNED")
	}
	t.Logf("─────────────────────────────────────────────────────────────────")
}

// ---------- Pass 5: Non-Compliance Coverage Report ----------

// TestPass5_NonComplianceReport quantifies the gap between what Claude Code
// actually emits at the baseline version and what aOa's parser consumes.
// Unlike Pass 3 (which only flags drift since last validation), this pass
// answers: "where are we non-compliant *right now*?"
//
// All output is via t.Logf — this test never fails. Its job is to inform.
func TestPass5_NonComplianceReport(t *testing.T) {
	baseline := loadBaseline(t)
	m := baseline.Manifest

	// Helpers to extract observed sets from the manifest JSON.
	getStringSet := func(path ...string) map[string]bool {
		var cur any = m
		for _, k := range path {
			obj, ok := cur.(map[string]any)
			if !ok {
				return nil
			}
			cur = obj[k]
		}
		out := map[string]bool{}
		switch v := cur.(type) {
		case []any:
			for _, x := range v {
				if s, ok := x.(string); ok {
					out[s] = true
				}
			}
		case map[string]any:
			for k := range v {
				out[k] = true
			}
		}
		return out
	}

	// --- Observed surface at baseline ---
	observedTypes := getStringSet("sample", "event_counts_by_type")
	observedUsage := getStringSet("usage_fields")
	observedSubtypes := getStringSet("system_subtypes")
	observedToolUseResult := getStringSet("tool_use_result_shapes")

	envByType := map[string]map[string]bool{}
	if envObj, ok := m["top_level_envelope_fields"].(map[string]any); ok {
		for typ, v := range envObj {
			if arr, ok := v.([]any); ok {
				envByType[typ] = map[string]bool{}
				for _, x := range arr {
					if s, ok := x.(string); ok {
						envByType[typ][s] = true
					}
				}
			}
		}
	}

	msgByRole := map[string]map[string]bool{}
	if msgObj, ok := m["message_fields_by_role"].(map[string]any); ok {
		for role, v := range msgObj {
			if arr, ok := v.([]any); ok {
				msgByRole[role] = map[string]bool{}
				for _, x := range arr {
					if s, ok := x.(string); ok {
						msgByRole[role][s] = true
					}
				}
			}
		}
	}

	// --- Coverage computation ---
	type cov struct {
		consumed, total int
		gaps            []string
	}
	pct := func(c cov) string {
		if c.total == 0 {
			return "n/a"
		}
		return fmt.Sprintf("%d%%", c.consumed*100/c.total)
	}

	// Event types
	typeCov := cov{}
	for typ := range observedTypes {
		typeCov.total++
		if recognizedTypes[typ] {
			typeCov.consumed++
		} else {
			typeCov.gaps = append(typeCov.gaps, typ)
		}
	}
	sort.Strings(typeCov.gaps)

	// Envelope fields per type — only count for recognized types
	envCovByType := map[string]cov{}
	for typ, fields := range envByType {
		if !recognizedTypes[typ] {
			continue
		}
		c := cov{}
		for f := range fields {
			c.total++
			if consumedEnvelopeFields[f] {
				c.consumed++
			} else {
				c.gaps = append(c.gaps, f)
			}
		}
		sort.Strings(c.gaps)
		envCovByType[typ] = c
	}

	// Usage fields
	usageCov := cov{}
	for f := range observedUsage {
		usageCov.total++
		if consumedUsageFields[f] {
			usageCov.consumed++
		} else {
			usageCov.gaps = append(usageCov.gaps, f)
		}
	}
	sort.Strings(usageCov.gaps)

	// Message-level fields (assistant — superset of user). Fields explicitly
	// marked SKIP in the integration plan count as "consumed" for compliance
	// purposes (intentional decisions, not drift).
	msgCov := cov{}
	var skippedMsgFields []string
	if assistantFields := msgByRole["assistant"]; assistantFields != nil {
		for f := range assistantFields {
			msgCov.total++
			switch {
			case consumedMessageFields[f]:
				msgCov.consumed++
			case skippedMessageFields[f]:
				msgCov.consumed++
				skippedMsgFields = append(skippedMsgFields, f)
			default:
				msgCov.gaps = append(msgCov.gaps, f)
			}
		}
	}
	sort.Strings(msgCov.gaps)
	sort.Strings(skippedMsgFields)

	// System subtypes
	subCov := cov{}
	for s := range observedSubtypes {
		subCov.total++
		if consumedSystemSubtypes[s] {
			subCov.consumed++
		} else {
			subCov.gaps = append(subCov.gaps, s)
		}
	}
	sort.Strings(subCov.gaps)

	// toolUseResult shapes
	turCov := cov{}
	for k := range observedToolUseResult {
		turCov.total++
		if consumedToolUseResultShapes[k] {
			turCov.consumed++
		} else {
			turCov.gaps = append(turCov.gaps, k)
		}
	}
	sort.Strings(turCov.gaps)

	// --- Build report (single source of truth — logged AND written to file) ---
	var b strings.Builder
	wf := func(format string, args ...any) {
		line := fmt.Sprintf(format, args...)
		t.Log(line)
		b.WriteString(line)
		b.WriteString("\n")
	}

	wf("### Coverage")
	wf("")
	wf("| Surface area                | Consumed | Total | Coverage |")
	wf("|-----------------------------|----------|-------|----------|")
	wf("| Event types                 | %d        | %d     | %s       |", typeCov.consumed, typeCov.total, pct(typeCov))
	envTypes := make([]string, 0, len(envCovByType))
	for k := range envCovByType {
		envTypes = append(envTypes, k)
	}
	sort.Strings(envTypes)
	for _, typ := range envTypes {
		c := envCovByType[typ]
		wf("| Envelope fields (%-9s) | %d        | %d    | %s      |", typ, c.consumed, c.total, pct(c))
	}
	wf("| Usage fields                | %d        | %d    | %s      |", usageCov.consumed, usageCov.total, pct(usageCov))
	wf("| Message fields (assistant)  | %d        | %d     | %s      |", msgCov.consumed, msgCov.total, pct(msgCov))
	wf("| System subtypes             | %d        | %d     | %s      |", subCov.consumed, subCov.total, pct(subCov))
	wf("| toolUseResult shapes        | %d        | %d     | %s       |", turCov.consumed, turCov.total, pct(turCov))
	wf("")
	wf("### Gaps")
	wf("")

	totalGaps := len(turCov.gaps) + len(typeCov.gaps) + len(usageCov.gaps) + len(msgCov.gaps) + len(subCov.gaps)
	for _, c := range envCovByType {
		totalGaps += len(c.gaps)
	}
	if totalGaps == 0 {
		wf("**Fully integrated.** No DROPPED fields remain at this baseline.")
		wf("")
		if len(skippedMsgFields) > 0 {
			wf("Intentionally skipped (documented [SKIP] decisions):")
			wf("- message-level: %v — pure API echo, no value on canonical event", skippedMsgFields)
			wf("")
		}
	} else {
		if len(turCov.gaps) > 0 {
			wf("**[Critical] toolUseResult shapes — %d/%d consumed**", turCov.consumed, turCov.total)
			wf("- tools dropped: %v", turCov.gaps)
			wf("")
		}
		if len(typeCov.gaps) > 0 {
			wf("**[High] Unhandled event types — %d ignored**", len(typeCov.gaps))
			wf("- types: %v", typeCov.gaps)
			wf("")
		}
		envHasGaps := false
		for _, c := range envCovByType {
			if len(c.gaps) > 0 {
				envHasGaps = true
				break
			}
		}
		if envHasGaps {
			wf("**[Medium] Envelope context fields dropped**")
			for _, typ := range envTypes {
				c := envCovByType[typ]
				if len(c.gaps) > 0 {
					wf("- `%s`: %v", typ, c.gaps)
				}
			}
			wf("")
		}
		if len(usageCov.gaps) > 0 {
			wf("**[Medium-Low] Usage metadata dropped — %d/%d**", usageCov.total-usageCov.consumed, usageCov.total)
			wf("- fields: %v", usageCov.gaps)
			wf("")
		}
		if len(msgCov.gaps) > 0 {
			wf("**[Low] Message-level assistant fields dropped — %d/%d**", msgCov.total-msgCov.consumed, msgCov.total)
			wf("- fields: %v", msgCov.gaps)
			wf("")
		}
		if len(subCov.gaps) > 0 {
			wf("**[Low] System subtypes not branched on — %d/%d**", subCov.total-subCov.consumed, subCov.total)
			wf("- subtypes: %v", subCov.gaps)
			wf("")
		}
	}
	wf("See `versions/v%s-observed/observations.md` for the version-specific narrative.", baseline.Version)

	writeReportSection(t, "claude-session", "Claude Code Session JSONL", baseline.Version, b.String())
}

// writeReportSection updates compliance/REPORT.md by replacing the section
// between "<!-- BEGIN {id} -->" and "<!-- END {id} -->" markers. If the file
// or markers don't exist, they are created. Each compliance test package
// owns one section keyed by id (e.g., "claude-session", "claude-statusline").
//
// Run from the test's working directory (compliance/{surface}/), so the
// report file is at "../REPORT.md".
func writeReportSection(t *testing.T, id, title, version, body string) {
	t.Helper()
	const reportPath = "../REPORT.md"
	beginMarker := fmt.Sprintf("<!-- BEGIN %s -->", id)
	endMarker := fmt.Sprintf("<!-- END %s -->", id)

	// Section content: marker, header, body, marker.
	timestamp := time.Now().UTC().Format("2006-01-02 15:04 UTC")
	section := fmt.Sprintf(
		"%s\n## %s — v%s\n\n*Last regenerated: %s*\n\n%s\n%s",
		beginMarker, title, version, timestamp, strings.TrimRight(body, "\n"), endMarker,
	)

	existing, err := os.ReadFile(reportPath)
	if err != nil {
		// Initialize a fresh report with both surface placeholders, then
		// substitute our section. The other surface's section will be
		// updated when its test runs.
		header := "# aOa Compliance Report\n\n" +
			"> Auto-generated by `go test -tags compliance ./compliance/...`. Do not edit by hand — your changes will be overwritten on the next test run.\n\n" +
			"This report quantifies where aOa's parser is non-compliant relative to the contracts in this folder. \"Non-compliant\" means signal Claude Code emits that aOa does not consume — not necessarily a breakage.\n\n" +
			"---\n\n"
		placeholders := map[string]string{
			"claude-session":    "<!-- BEGIN claude-session -->\n## Claude Code Session JSONL\n\n*(not yet generated — run `go test -tags compliance ./compliance/claude-session/...`)*\n<!-- END claude-session -->",
			"claude-statusline": "<!-- BEGIN claude-statusline -->\n## Claude Code Status Line\n\n*(not yet generated — run `go test -tags compliance ./compliance/claude-statusline/...`)*\n<!-- END claude-statusline -->",
		}
		// Put our section in its slot, leave the other as placeholder.
		placeholders[id] = section
		full := header + placeholders["claude-session"] + "\n\n---\n\n" + placeholders["claude-statusline"] + "\n"
		if err := os.WriteFile(reportPath, []byte(full), 0o644); err != nil {
			t.Logf("WARN: could not write %s: %v", reportPath, err)
			return
		}
		t.Logf("Initialized %s with section %q", reportPath, id)
		return
	}

	content := string(existing)
	beginIdx := strings.Index(content, beginMarker)
	endIdx := strings.Index(content, endMarker)
	if beginIdx == -1 || endIdx == -1 || endIdx < beginIdx {
		// Markers missing or malformed — append the section at the end.
		updated := strings.TrimRight(content, "\n") + "\n\n---\n\n" + section + "\n"
		if err := os.WriteFile(reportPath, []byte(updated), 0o644); err != nil {
			t.Logf("WARN: could not write %s: %v", reportPath, err)
		} else {
			t.Logf("Appended section %q to %s (markers were missing)", id, reportPath)
		}
		return
	}

	// Replace from begin marker through end marker (inclusive).
	endLen := beginIdx + len(content[beginIdx:endIdx]) + len(endMarker)
	updated := content[:beginIdx] + section + content[endLen:]
	if err := os.WriteFile(reportPath, []byte(updated), 0o644); err != nil {
		t.Logf("WARN: could not write %s: %v", reportPath, err)
		return
	}
	t.Logf("Updated section %q in %s", id, reportPath)
}

func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}
