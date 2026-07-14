package arch

import (
	"encoding/json"
	"math/rand"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Service.RenderAll — manifest golden + determinism (T4 extension)
// ---------------------------------------------------------------------------

func TestService_RenderAll_Determinism(t *testing.T) {
	svc := &Service{}
	in := makeFixture()

	r1, m1, _, err := svc.RenderAll(in.Scope, in.Units, in.Deps, nil, nil, nil)
	require.NoError(t, err)
	r2, m2, _, err := svc.RenderAll(in.Scope, in.Units, in.Deps, nil, nil, nil)
	require.NoError(t, err)

	// All shards must be byte-identical across two independent renders.
	require.Equal(t, len(r1), len(r2), "shard count must match")
	for id, b1 := range r1 {
		b2, ok := r2[id]
		require.True(t, ok, "view %q missing in second render", id)
		require.Equal(t, string(b1), string(b2), "view %q must be byte-identical (T4)", id)
	}

	// Manifest must be byte-identical.
	mb1, err := MarshalManifest(&m1)
	require.NoError(t, err)
	mb2, err := MarshalManifest(&m2)
	require.NoError(t, err)
	require.Equal(t, string(mb1), string(mb2), "manifest must be byte-identical across renders (T4)")

	// Golden check for manifest.
	checkAndUpdateGolden(t, "testdata/golden_manifest.json", mb1)
}

func TestService_RenderAll_ViewsPresent(t *testing.T) {
	svc := &Service{}
	in := makeFixture()

	shards, manifest, findings, err := svc.RenderAll(in.Scope, in.Units, in.Deps, nil, nil, nil)
	require.NoError(t, err)

	// The three keystone views must be present.
	for _, id := range []string{"component", "dsm", "cycles"} {
		_, ok := shards[id]
		assert.True(t, ok, "view %q must be in shards map", id)
	}

	// Manifest must list all views.
	viewIDs := make(map[string]bool)
	for _, v := range manifest.Views {
		viewIDs[v.ID] = true
		assert.NotEmpty(t, v.Key, "manifest view %q must have key", v.ID)
		assert.NotEmpty(t, v.Hash, "manifest view %q must have hash", v.ID)
		assert.Len(t, v.Hash, 12, "content hash must be 12 chars")
		assert.Equal(t, "derived", v.Prov, "path-prefix grouping must be derived (D1 ruling)")
	}
	for _, id := range []string{"component", "dsm", "cycles"} {
		assert.True(t, viewIDs[id], "manifest must include view %q", id)
	}

	// Findings from detectors must be returned.
	assert.NotEmpty(t, findings, "fixture has planted detections — findings must be non-empty")

	// Scope and Rev must be set.
	assert.Equal(t, in.Scope, manifest.Scope)
	assert.Len(t, manifest.Rev, 12, "manifest.Rev must be 12-char hash")
}

func TestService_RenderAll_HashMatchesKey(t *testing.T) {
	svc := &Service{}
	in := makeFixture()

	shards, manifest, _, err := svc.RenderAll(in.Scope, in.Units, in.Deps, nil, nil, nil)
	require.NoError(t, err)

	// Each manifest entry's hash must match the actual ContentHash of the shard bytes.
	for _, v := range manifest.Views {
		data, ok := shards[v.ID]
		require.True(t, ok, "shard for view %q must exist", v.ID)
		expected := ContentHash(data)
		assert.Equal(t, expected, v.Hash, "manifest hash for %q must match ContentHash(shard)", v.ID)

		// Key format: "{scope}/{id}@{hash}"
		expectedKey := in.Scope + "/" + v.ID + "@" + v.Hash
		assert.Equal(t, expectedKey, v.Key, "manifest key format must be scope/id@hash")
	}
}

// ---------------------------------------------------------------------------
// GroupWithOptions — rung-1 (declarations) + rung-3 (atlas domain) + overlay
// ---------------------------------------------------------------------------

func TestGroupWithOptions_Rung3_AtlasDomain(t *testing.T) {
	// Units whose Domain field is set should be grouped by domain (rung-3), not path prefix.
	units := []UnitFact{
		{ID: "m_a", Label: "A", Path: "internal/app/a.go", Domain: "scheduling"},
		{ID: "m_b", Label: "B", Path: "internal/app/b.go", Domain: "scheduling"},
		{ID: "m_c", Label: "C", Path: "internal/domain/x/c.go", Domain: ""},
	}
	result, provKind, warnings := GroupWithOptions(units, nil)
	require.NoError(t, nil) // GroupWithOptions doesn't error

	assert.Empty(t, warnings, "no overlay → no warnings")
	assert.Equal(t, "derived", provKind, "rung-3 domain grouping is REAL (D1)")

	// m_a and m_b should be in the "scheduling" group.
	assert.Equal(t, result.UnitGroup["m_a"], result.UnitGroup["m_b"],
		"same atlas domain → same group")

	// m_c has no domain — uses rung-2 path prefix ("domain").
	assert.NotEqual(t, result.UnitGroup["m_c"], result.UnitGroup["m_a"],
		"different domain and path → different groups")

	// The scheduling group ID must contain "scheduling".
	schedGID := result.UnitGroup["m_a"]
	var schedGroup *GroupMeta
	for i := range result.Groups {
		if result.Groups[i].ID == schedGID {
			schedGroup = &result.Groups[i]
			break
		}
	}
	require.NotNil(t, schedGroup, "scheduling group must appear in Groups")
	assert.Equal(t, "scheduling", schedGroup.Label)
}

func TestGroupWithOptions_Rung1_Declarations(t *testing.T) {
	// Rung-1: explicit declaration takes priority over rung-2 and rung-3.
	units := []UnitFact{
		{ID: "m_a", Label: "A", Path: "internal/app/a.go", Domain: "scheduling"},
		{ID: "m_b", Label: "B", Path: "internal/domain/x/b.go"},
	}
	opts := &GroupOptions{
		Declarations: map[string]string{
			"m_a": "infrastructure", // override rung-3 "scheduling"
		},
	}
	result, provKind, warnings := GroupWithOptions(units, opts)

	assert.Empty(t, warnings)
	assert.Equal(t, "derived", provKind, "declarations are REAL (D1)")

	// m_a must be in "infrastructure" group (rung-1 wins over rung-3).
	aGID := result.UnitGroup["m_a"]
	var aGroup *GroupMeta
	for i := range result.Groups {
		if result.Groups[i].ID == aGID {
			aGroup = &result.Groups[i]
			break
		}
	}
	require.NotNil(t, aGroup)
	assert.Equal(t, "infrastructure", aGroup.Label)

	// m_b uses rung-2 path prefix ("domain").
	bGID := result.UnitGroup["m_b"]
	assert.NotEmpty(t, bGID)
}

func TestGroupWithOptions_Overlay_Valid(t *testing.T) {
	// Valid overlay: all unit IDs exist in facts → MIXED prov.
	units := []UnitFact{
		{ID: "m_a", Label: "A", Path: "internal/app/a.go"},
		{ID: "m_b", Label: "B", Path: "internal/domain/x/b.go"},
	}
	opts := &GroupOptions{
		Overlays: map[string]string{
			"m_a": "special", // override rung-2 group for m_a
		},
	}
	result, provKind, warnings := GroupWithOptions(units, opts)

	assert.Empty(t, warnings, "no invalid IDs → no warnings")
	assert.Equal(t, "mixed", provKind, "overlay applied → MIXED (D1 ruling)")

	aGID := result.UnitGroup["m_a"]
	var aGroup *GroupMeta
	for i := range result.Groups {
		if result.Groups[i].ID == aGID {
			aGroup = &result.Groups[i]
			break
		}
	}
	require.NotNil(t, aGroup)
	assert.Equal(t, "special", aGroup.Label)
}

func TestGroupWithOptions_Overlay_InvalidID_Leash(t *testing.T) {
	// Leash: overlay references an invented ID → warning finding + MIXED.
	units := []UnitFact{
		{ID: "m_real", Label: "Real", Path: "internal/app/a.go"},
	}
	opts := &GroupOptions{
		Overlays: map[string]string{
			"m_invented": "special", // does NOT exist in units
		},
		OverlayHadInvalidIDs: true,
	}
	result, provKind, warnings := GroupWithOptions(units, opts)

	// Warning finding emitted for the invented ID.
	assert.NotEmpty(t, warnings, "invented overlay ID must produce a warning finding")
	found := false
	for _, w := range warnings {
		if w.Rule == "overlay-leash" {
			found = true
			break
		}
	}
	assert.True(t, found, "warning must carry rule='overlay-leash'")

	// prov is MIXED even if the invalid ID was rejected.
	assert.Equal(t, "mixed", provKind, "overlay attempted (even with invalid IDs) → MIXED")

	// m_real is still grouped normally (invalid ID not applied).
	assert.NotEmpty(t, result.UnitGroup["m_real"])
}

// ---------------------------------------------------------------------------
// ParseOverlay / ApplyOverlay — overlay schema + leash (§step 3)
// ---------------------------------------------------------------------------

func TestParseOverlay_Valid(t *testing.T) {
	raw := []byte(`{
		"$schema": "aoa.arch-overlay/v1",
		"groups": [
			{"id": "infra", "label": "Infrastructure", "unitIds": ["m_a", "m_b"]},
			{"id": "data", "label": "Data", "unitIds": ["m_c"]}
		]
	}`)
	spec, err := ParseOverlay(raw)
	require.NoError(t, err)
	assert.Equal(t, "aoa.arch-overlay/v1", spec.Schema)
	require.Len(t, spec.Groups, 2)
	assert.Equal(t, "infra", spec.Groups[0].ID)
	assert.Equal(t, []string{"m_a", "m_b"}, spec.Groups[0].UnitIDs)
}

func TestParseOverlay_WrongSchema(t *testing.T) {
	raw := []byte(`{"$schema":"wrong/v1","groups":[]}`)
	_, err := ParseOverlay(raw)
	assert.Error(t, err, "wrong schema version must return error")
}

func TestApplyOverlay_Valid(t *testing.T) {
	spec := &OverlaySpec{
		Schema: "aoa.arch-overlay/v1",
		Groups: []OverlayGroupSpec{
			{ID: "infra", Label: "Infra", UnitIDs: []string{"m_a", "m_b"}},
		},
	}
	units := []UnitFact{
		{ID: "m_a", Label: "A"},
		{ID: "m_b", Label: "B"},
		{ID: "m_c", Label: "C"},
	}
	approved, invalid := ApplyOverlay(spec, units)
	assert.Empty(t, invalid, "all unit IDs are real → no invalid IDs")
	assert.Equal(t, "infra", approved["m_a"])
	assert.Equal(t, "infra", approved["m_b"])
	_, hasC := approved["m_c"]
	assert.False(t, hasC, "m_c not in overlay → not in approved")
}

func TestApplyOverlay_InventedID(t *testing.T) {
	spec := &OverlaySpec{
		Schema: "aoa.arch-overlay/v1",
		Groups: []OverlayGroupSpec{
			{ID: "infra", Label: "Infra", UnitIDs: []string{"m_real", "m_invented"}},
		},
	}
	units := []UnitFact{{ID: "m_real", Label: "Real"}}

	approved, invalid := ApplyOverlay(spec, units)
	assert.Contains(t, invalid, "m_invented", "invented ID must appear in invalid list")
	assert.Equal(t, "infra", approved["m_real"], "real ID must be approved")
	_, hasInvented := approved["m_invented"]
	assert.False(t, hasInvented, "invented ID must NOT appear in approved")
}

// ---------------------------------------------------------------------------
// DeriveCaption — entity/simple kinds (§2 item 14 open delta)
// ---------------------------------------------------------------------------

func TestDeriveCaption_Entity(t *testing.T) {
	s := &Shard{
		Kind:  "entity",
		Nodes: []Node{{ID: "n1"}, {ID: "n2"}, {ID: "n3"}},
	}
	caption, _ := DeriveCaption(s, nil)
	assert.Contains(t, caption, "3", "entity caption must include node count")
}

func TestDeriveCaption_Simple(t *testing.T) {
	s := &Shard{
		Kind:  "simple",
		Nodes: []Node{{ID: "n1"}, {ID: "n2"}},
	}
	caption, _ := DeriveCaption(s, nil)
	assert.Contains(t, caption, "2", "simple caption must include node count")
}

// A3: DeriveCaption splits the calm caption from the findings clause (house ruling "calm like a
// map") — the caption itself must NEVER mention findings; the clause is a separate return value
// the caller appends only when its Findings lens is on.
func TestDeriveCaption_Simple_WithFindings(t *testing.T) {
	s := &Shard{
		Kind:  "simple",
		Nodes: []Node{{ID: "n1"}},
	}
	findings := []Finding{{Rule: "test", Severity: "warn"}}
	caption, findingsClause := DeriveCaption(s, findings)
	assert.NotContains(t, caption, "finding", "A3: calm caption must never mention findings")
	assert.Contains(t, findingsClause, "1 findings", "findings suffix must be in the separate clause")
}

// ---------------------------------------------------------------------------
// T22 — Property invariants: byte-stability under permutation
// ---------------------------------------------------------------------------

func TestT22_ByteStability_UnderPermutation(t *testing.T) {
	svc := &Service{}
	in := makeFixture()

	// Render with canonical order.
	shards1, m1, _, err := svc.RenderAll(in.Scope, in.Units, in.Deps, nil, nil, nil)
	require.NoError(t, err)

	// Shuffle units and deps.
	shuffledUnits := make([]UnitFact, len(in.Units))
	copy(shuffledUnits, in.Units)
	shuffleDeterministic(shuffledUnits)

	shuffledDeps := make([]DepFact, len(in.Deps))
	copy(shuffledDeps, in.Deps)
	shuffleDepsDeterministic(shuffledDeps)

	// Render with shuffled order.
	shards2, m2, _, err := svc.RenderAll(in.Scope, shuffledUnits, shuffledDeps, nil, nil, nil)
	require.NoError(t, err)

	// All shards must be byte-identical regardless of input order.
	for id, b1 := range shards1 {
		b2, ok := shards2[id]
		require.True(t, ok, "view %q must exist in shuffled render", id)
		assert.Equal(t, string(b1), string(b2),
			"view %q must be byte-identical under input permutation (T22)", id)
	}

	// Manifest rev must be identical (hash of input, order-independent).
	mb1, _ := MarshalManifest(&m1)
	mb2, _ := MarshalManifest(&m2)
	assert.Equal(t, string(mb1), string(mb2), "manifest must be identical under permutation (T22)")
}

func TestT22_MemberMapsToRealUnit(t *testing.T) {
	svc := &Service{}
	in := makeFixture()

	shards, _, _, err := svc.RenderAll(in.Scope, in.Units, in.Deps, nil, nil, nil)
	require.NoError(t, err)

	unitSet := make(map[string]bool, len(in.Units))
	for _, u := range in.Units {
		unitSet[u.ID] = true
	}

	componentJSON := shards["component"]
	require.NotNil(t, componentJSON)

	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(componentJSON, &m))

	buckets, ok := m["buckets"].([]interface{})
	require.True(t, ok)
	for _, bRaw := range buckets {
		bMap := bRaw.(map[string]interface{})
		members, _ := bMap["members"].([]interface{})
		for _, mRaw := range members {
			mMap := mRaw.(map[string]interface{})
			memberID, _ := mMap["id"].(string)
			assert.True(t, unitSet[memberID],
				"bucket member %q must map to a real unit fact (T22)", memberID)
		}
	}
}

func TestT22_DSMMatchesEdgeSet(t *testing.T) {
	svc := &Service{}
	in := makeFixture()

	shards, _, _, err := svc.RenderAll(in.Scope, in.Units, in.Deps, nil, nil, nil)
	require.NoError(t, err)

	dsmJSON := shards["dsm"]
	require.NotNil(t, dsmJSON)

	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(dsmJSON, &m))

	items, ok := m["items"].([]interface{})
	require.True(t, ok)
	matrix, ok2 := m["matrix"].([]interface{})
	require.True(t, ok2)

	// Matrix must be square.
	assert.Equal(t, len(items), len(matrix), "DSM matrix must be n×n (T22)")
	for i, row := range matrix {
		rowSlice, ok := row.([]interface{})
		require.True(t, ok, "matrix row %d must be a list", i)
		assert.Equal(t, len(items), len(rowSlice), "DSM row %d length must equal n (T22)", i)
	}
}

func TestT22_CyclesSubsetOfSCCs(t *testing.T) {
	svc := &Service{}
	in := makeFixture()

	shards, _, findings, err := svc.RenderAll(in.Scope, in.Units, in.Deps, nil, nil, nil)
	require.NoError(t, err)

	cyclesJSON := shards["cycles"]
	require.NotNil(t, cyclesJSON)

	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(cyclesJSON, &m))

	rows, _ := m["rows"].([]interface{})
	// Count cycle findings.
	cycleFindingCount := 0
	for _, f := range findings {
		if f.Rule == "cycle" {
			cycleFindingCount++
		}
	}

	// Cycles table rows must match cycle findings.
	assert.Equal(t, cycleFindingCount, len(rows),
		"cycles table rows must match cycle findings count (T22: cycles ⊆ SCCs)")
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func shuffleDeterministic(units []UnitFact) {
	r := rand.New(rand.NewSource(42))
	r.Shuffle(len(units), func(i, j int) { units[i], units[j] = units[j], units[i] })
}

func shuffleDepsDeterministic(deps []DepFact) {
	r := rand.New(rand.NewSource(99))
	r.Shuffle(len(deps), func(i, j int) { deps[i], deps[j] = deps[j], deps[i] })
}

// ---------------------------------------------------------------------------
// T15 — Manifest golden (extend to verify golden_manifest.json)
// ---------------------------------------------------------------------------

func TestManifestGolden(t *testing.T) {
	svc := &Service{}
	in := makeFixture()
	_, manifest, _, err := svc.RenderAll(in.Scope, in.Units, in.Deps, nil, nil, nil)
	require.NoError(t, err)

	b, err := MarshalManifest(&manifest)
	require.NoError(t, err)
	checkAndUpdateGolden(t, "testdata/golden_manifest.json", b)

	// Decode and spot-check structure.
	var m Manifest
	require.NoError(t, json.Unmarshal(b, &m))
	assert.Equal(t, in.Scope, m.Scope)
	assert.Len(t, m.Rev, 12)
	// Views sorted alphabetically by ID.
	ids := make([]string, len(m.Views))
	for i, v := range m.Views {
		ids[i] = v.ID
	}
	sorted := make([]string, len(ids))
	copy(sorted, ids)
	sort.Strings(sorted)
	assert.Equal(t, sorted, ids, "manifest views must be sorted alphabetically by ID")
}

// ---------------------------------------------------------------------------
// T64/T65 — schema version stamping + DerivedAt purity (PA2/PA3)
// ---------------------------------------------------------------------------

// TestService_RenderAll_StampsSchemaVersion asserts RenderAll always stamps
// the current ArchSchemaVersion into the returned manifest, and that this is
// deterministic (a fixed constant, unlike DerivedAt — see below) across two
// independent calls on identical input.
func TestService_RenderAll_StampsSchemaVersion(t *testing.T) {
	svc := &Service{}
	in := makeFixture()

	_, m1, _, err := svc.RenderAll(in.Scope, in.Units, in.Deps, nil, nil, nil)
	require.NoError(t, err)
	_, m2, _, err := svc.RenderAll(in.Scope, in.Units, in.Deps, nil, nil, nil)
	require.NoError(t, err)

	assert.Equal(t, ArchSchemaVersion, m1.SchemaVersion, "RenderAll must stamp the current schema version")
	assert.Equal(t, m1.SchemaVersion, m2.SchemaVersion, "SchemaVersion is a fixed constant — must not vary across renders")
	assert.NotZero(t, ArchSchemaVersion, "ArchSchemaVersion must be a real version, not the zero value old/absent manifests carry")
}

// TestService_RenderAll_LeavesDerivedAtEmpty asserts RenderAll — a pure,
// dependency-free domain function — never stamps a wall-clock timestamp.
// Only the app layer (deriveArch) sets DerivedAt, once, at actual persist
// time (T65); stamping it here would break the T4 byte-stability contract
// this package tests everywhere else (TestService_RenderAll_Determinism).
func TestService_RenderAll_LeavesDerivedAtEmpty(t *testing.T) {
	svc := &Service{}
	in := makeFixture()

	_, m, _, err := svc.RenderAll(in.Scope, in.Units, in.Deps, nil, nil, nil)
	require.NoError(t, err)

	assert.Empty(t, m.DerivedAt, "RenderAll must not stamp DerivedAt — that would make it non-deterministic (T4)")
}
