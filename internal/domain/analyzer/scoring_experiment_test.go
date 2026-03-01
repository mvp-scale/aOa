package analyzer

import (
	"fmt"
	"math"
	"sort"
	"testing"
)

// ── Synthetic method matrix ─────────────────────────────────────────────
//
// A MethodMatrix is the per-line bitmask representation from the design doc.
// Each entry records which bit fired on which line, with its severity.
// This is the raw signal before any rollup — the full line × bit topology.

type lineHit struct {
	line     int
	tier     Tier
	bit      int
	severity Severity
}

type methodMatrix struct {
	name       string
	totalLines int
	hits       []lineHit
}

// ── Test scenarios ──────────────────────────────────────────────────────
//
// Each scenario encodes a real-world pattern with an expected disposition.
// Scenarios are grouped by the signal characteristic they test.

type scenario struct {
	matrix        methodMatrix
	shouldSurface bool
	reason        string
}

var scenarios = []scenario{

	// ═══════════════════════════════════════════════════════════════════
	// SECURITY dimension
	// Characteristic: critical single findings MUST always surface.
	// Co-occurrence amplifies (SQL concat + raw SQL on same line).
	// ═══════════════════════════════════════════════════════════════════

	{
		matrix: methodMatrix{
			name: "sec_hardcoded_secret", totalLines: 80,
			hits: []lineHit{
				{line: 12, tier: TierSecurity, bit: 17, severity: SevCritical},
			},
		},
		shouldSurface: true,
		reason: "One hardcoded secret in 80 lines — critical overrides low density",
	},
	{
		matrix: methodMatrix{
			name: "sec_aws_key_large_method", totalLines: 200,
			hits: []lineHit{
				{line: 45, tier: TierSecurity, bit: 20, severity: SevCritical},
			},
		},
		shouldSurface: true,
		reason: "Single AWS key in 200-line method — critical always surfaces",
	},
	{
		matrix: methodMatrix{
			name: "sec_sql_cooccurrence", totalLines: 30,
			hits: []lineHit{
				{line: 7, tier: TierSecurity, bit: 0, severity: SevCritical},  // SQL concat
				{line: 8, tier: TierSecurity, bit: 0, severity: SevCritical},  // SQL concat again
				{line: 8, tier: TierSecurity, bit: 2, severity: SevCritical},  // raw SQL — SAME LINE
				{line: 10, tier: TierSecurity, bit: 11, severity: SevWarning}, // log injection
			},
		},
		shouldSurface: true,
		reason: "Bits 0+2 co-occur on line 8 — compound SQL injection signal",
	},
	{
		matrix: methodMatrix{
			name: "sec_path_traversal_cooccur", totalLines: 25,
			hits: []lineHit{
				{line: 6, tier: TierSecurity, bit: 55, severity: SevCritical},
				{line: 6, tier: TierSecurity, bit: 56, severity: SevCritical}, // SAME LINE
			},
		},
		shouldSurface: true,
		reason: "Path from user input + directory traversal on same line — compound",
	},
	{
		matrix: methodMatrix{
			name: "sec_handleTransfer_full", totalLines: 16,
			hits: []lineHit{
				{line: 2, tier: TierSecurity, bit: 35, severity: SevHigh},
				{line: 3, tier: TierSecurity, bit: 35, severity: SevHigh},
				{line: 4, tier: TierSecurity, bit: 35, severity: SevHigh},
				{line: 5, tier: TierSecurity, bit: 17, severity: SevCritical},
				{line: 6, tier: TierSecurity, bit: 55, severity: SevCritical},
				{line: 6, tier: TierSecurity, bit: 56, severity: SevCritical},
				{line: 7, tier: TierSecurity, bit: 0, severity: SevCritical},
				{line: 8, tier: TierSecurity, bit: 0, severity: SevCritical},
				{line: 8, tier: TierSecurity, bit: 2, severity: SevCritical},
				{line: 11, tier: TierSecurity, bit: 43, severity: SevHigh},
				{line: 12, tier: TierSecurity, bit: 11, severity: SevWarning},
				{line: 13, tier: TierSecurity, bit: 0, severity: SevWarning},
				{line: 15, tier: TierSecurity, bit: 42, severity: SevHigh},
			},
		},
		shouldSurface: true,
		reason: "Design doc example — 10 bits, 62% coverage, co-occurrence on lines 6 and 8",
	},

	// ═══════════════════════════════════════════════════════════════════
	// PERFORMANCE dimension
	// Characteristic: "in loop" patterns are critical/warning.
	// Multiple loop issues in same method = real perf problem.
	// Single info-level observation (reflection, sync usage) = noise.
	// ═══════════════════════════════════════════════════════════════════

	{
		matrix: methodMatrix{
			name: "perf_resource_alloc_in_loop", totalLines: 40,
			hits: []lineHit{
				{line: 15, tier: TierPerformance, bit: 4, severity: SevCritical}, // resource_in_loop
			},
		},
		shouldSurface: true,
		reason: "Resource allocation inside loop — critical perf issue, single hit surfaces",
	},
	{
		matrix: methodMatrix{
			name: "perf_n_plus_1_query", totalLines: 30,
			hits: []lineHit{
				{line: 12, tier: TierPerformance, bit: 10, severity: SevCritical}, // query_in_loop
			},
		},
		shouldSurface: true,
		reason: "N+1 query pattern — critical perf, always surfaces",
	},
	{
		matrix: methodMatrix{
			name: "perf_loop_compound", totalLines: 50,
			hits: []lineHit{
				{line: 10, tier: TierPerformance, bit: 0, severity: SevWarning},  // defer_in_loop
				{line: 15, tier: TierPerformance, bit: 15, severity: SevWarning}, // allocation_in_loop
				{line: 15, tier: TierPerformance, bit: 17, severity: SevWarning}, // string_concat_in_loop — SAME LINE
				{line: 20, tier: TierPerformance, bit: 22, severity: SevWarning}, // fmt.Sprint_in_loop
			},
		},
		shouldSurface: true,
		reason: "4 warning-level loop perf issues, co-occurrence on line 15 — compound loop problem",
	},
	{
		matrix: methodMatrix{
			name: "perf_dense_alloc_pattern", totalLines: 80,
			hits: func() []lineHit {
				var h []lineHit
				for i := 0; i < 15; i++ {
					h = append(h, lineHit{
						line: i*5 + 3, tier: TierPerformance, bit: 15, severity: SevWarning, // allocation_in_loop
					})
				}
				return h
			}(),
		},
		shouldSurface: true,
		reason: "Repeated heap allocation in loop — 19% density on single warning bit = clear pattern",
	},
	{
		matrix: methodMatrix{
			name: "perf_single_reflection_info", totalLines: 120,
			hits: []lineHit{
				{line: 60, tier: TierPerformance, bit: 20, severity: SevInfo}, // reflection_call
			},
		},
		shouldSurface: false,
		reason: "One reflection usage in 120 lines — observational info, not a problem",
	},
	{
		matrix: methodMatrix{
			name: "perf_sparse_info_sync", totalLines: 200,
			hits: []lineHit{
				{line: 30, tier: TierPerformance, bit: 9, severity: SevInfo},  // sync_primitive_usage
				{line: 150, tier: TierPerformance, bit: 25, severity: SevInfo}, // map_lookup_in_loop
			},
		},
		shouldSurface: false,
		reason: "2 sparse info hits — sync usage and map lookup are observational, not actionable",
	},
	{
		matrix: methodMatrix{
			name: "perf_lone_warning_huge", totalLines: 300,
			hits: []lineHit{
				{line: 150, tier: TierPerformance, bit: 23, severity: SevWarning}, // sleep_in_handler
			},
		},
		shouldSurface: false,
		reason: "Single time.Sleep in 300-line method — isolated, 0.3% density",
	},

	// ═══════════════════════════════════════════════════════════════════
	// QUALITY dimension
	// Characteristic: density is everything. One ignored error = meh.
	// 40% ignored errors = systematic problem. Breadth also matters:
	// ignored errors + unchecked assertions + panic = quality debt.
	// God function (critical) is structural, always surfaces.
	// ═══════════════════════════════════════════════════════════════════

	{
		matrix: methodMatrix{
			name: "qual_god_function", totalLines: 250,
			hits: []lineHit{
				{line: 1, tier: TierQuality, bit: 21, severity: SevCritical}, // god_function (>200 lines)
			},
		},
		shouldSurface: true,
		reason: "God function — critical quality issue, always surfaces",
	},
	{
		matrix: methodMatrix{
			name: "qual_dense_ignored_errors", totalLines: 50,
			hits: func() []lineHit {
				var h []lineHit
				for i := 0; i < 20; i++ {
					h = append(h, lineHit{
						line: i*2 + 1, tier: TierQuality, bit: 0, severity: SevWarning,
					})
				}
				return h
			}(),
		},
		shouldSurface: true,
		reason: "40% of lines ignore errors — extremely dense warning pattern",
	},
	{
		matrix: methodMatrix{
			name: "qual_breadth_5_bits", totalLines: 40,
			hits: []lineHit{
				{line: 5, tier: TierQuality, bit: 0, severity: SevWarning},     // ignored error
				{line: 10, tier: TierQuality, bit: 1, severity: SevWarning},    // panic_in_lib
				{line: 15, tier: TierQuality, bit: 2, severity: SevWarning},    // unchecked_type_assertion
				{line: 20, tier: TierQuality, bit: 3, severity: SevWarning},    // error_not_checked
				{line: 30, tier: TierObservability, bit: 0, severity: SevInfo}, // print_statement
			},
		},
		shouldSurface: true,
		reason: "5 distinct bits across 2 tiers — systematic quality debt even at low density",
	},
	{
		matrix: methodMatrix{
			name: "qual_error_cluster", totalLines: 60,
			hits: []lineHit{
				{line: 20, tier: TierQuality, bit: 0, severity: SevWarning},  // ignored error
				{line: 21, tier: TierQuality, bit: 3, severity: SevWarning},  // error_not_checked
				{line: 22, tier: TierQuality, bit: 5, severity: SevInfo},     // error_without_context
			},
		},
		shouldSurface: true,
		reason: "3 error-related bits on 3 adjacent lines — clustered error handling problem",
	},
	{
		matrix: methodMatrix{
			name: "qual_warning_pair_same_line", totalLines: 60,
			hits: []lineHit{
				{line: 30, tier: TierQuality, bit: 0, severity: SevWarning}, // ignored error
				{line: 30, tier: TierQuality, bit: 2, severity: SevWarning}, // unchecked assertion — SAME LINE
			},
		},
		shouldSurface: true,
		reason: "2 quality warnings co-occurring on same line — compound issue",
	},
	{
		matrix: methodMatrix{
			name: "qual_single_info_large", totalLines: 150,
			hits: []lineHit{
				{line: 73, tier: TierQuality, bit: 11, severity: SevInfo}, // commented_out_code
			},
		},
		shouldSurface: false,
		reason: "1 info (commented code) in 150 lines — noise",
	},
	{
		matrix: methodMatrix{
			name: "qual_lone_warning_500", totalLines: 500,
			hits: []lineHit{
				{line: 250, tier: TierQuality, bit: 0, severity: SevWarning}, // ignored error
			},
		},
		shouldSurface: false,
		reason: "1 ignored error in 500 lines — isolated, not a pattern",
	},
	{
		matrix: methodMatrix{
			name: "qual_trivial_helper_info", totalLines: 3,
			hits: []lineHit{
				{line: 2, tier: TierQuality, bit: 5, severity: SevInfo}, // error_without_context
			},
		},
		shouldSurface: false,
		reason: "3-line helper with 1 info — trivial",
	},

	// ═══════════════════════════════════════════════════════════════════
	// OBSERVABILITY dimension
	// Characteristic: mostly info-level. print statements and TODOs
	// are noise individually. Dense print (debug left in) or
	// sensitive data in logs (warning) with patterns = signal.
	// ═══════════════════════════════════════════════════════════════════

	{
		matrix: methodMatrix{
			name: "obs_sensitive_log_compound", totalLines: 30,
			hits: []lineHit{
				{line: 10, tier: TierObservability, bit: 11, severity: SevWarning}, // sensitive_log_pattern
				{line: 10, tier: TierObservability, bit: 13, severity: SevWarning}, // pii_in_log — SAME LINE
			},
		},
		shouldSurface: true,
		reason: "Sensitive data + PII in same log line — compound warning, privacy concern",
	},
	{
		matrix: methodMatrix{
			name: "obs_dense_debug_prints", totalLines: 40,
			hits: func() []lineHit {
				var h []lineHit
				for i := 0; i < 12; i++ {
					h = append(h, lineHit{
						line: i*3 + 2, tier: TierObservability, bit: 0, severity: SevInfo, // print_statement
					})
				}
				return h
			}(),
		},
		shouldSurface: true,
		reason: "30% of lines are debug prints — clearly left-in debugging, not production code",
	},
	{
		matrix: methodMatrix{
			name: "obs_resilience_cluster", totalLines: 50,
			hits: []lineHit{
				{line: 10, tier: TierObservability, bit: 15, severity: SevWarning}, // panic_in_handler
				{line: 11, tier: TierObservability, bit: 5, severity: SevWarning},  // recovered_panic_no_log
				{line: 15, tier: TierObservability, bit: 16, severity: SevWarning}, // os_exit_in_lib
			},
		},
		shouldSurface: true,
		reason: "Panic + recover without log + os.Exit — clustered resilience concern, 3 warning bits",
	},
	{
		matrix: methodMatrix{
			name: "obs_single_todo", totalLines: 100,
			hits: []lineHit{
				{line: 42, tier: TierObservability, bit: 1, severity: SevInfo}, // todo_fixme
			},
		},
		shouldSurface: false,
		reason: "One TODO in 100 lines — every codebase has TODOs, not actionable",
	},
	{
		matrix: methodMatrix{
			name: "obs_sparse_info_2_hits", totalLines: 300,
			hits: []lineHit{
				{line: 10, tier: TierObservability, bit: 0, severity: SevInfo},  // print_statement
				{line: 280, tier: TierObservability, bit: 1, severity: SevInfo}, // todo_fixme
			},
		},
		shouldSurface: false,
		reason: "1 print + 1 TODO 270 lines apart — uncorrelated info noise",
	},
	{
		matrix: methodMatrix{
			name: "obs_scattered_info_3", totalLines: 200,
			hits: []lineHit{
				{line: 15, tier: TierObservability, bit: 0, severity: SevInfo},  // print
				{line: 88, tier: TierObservability, bit: 4, severity: SevInfo},  // sleep_in_code
				{line: 190, tier: TierObservability, bit: 14, severity: SevInfo}, // log_without_context
			},
		},
		shouldSurface: false,
		reason: "3 info scattered across 200 lines — no pattern",
	},

	// ═══════════════════════════════════════════════════════════════════
	// ARCHITECTURE dimension
	// Characteristic: structural anti-patterns are often single findings
	// but significant (god object, massive struct = critical).
	// Coupling warnings need density/breadth to matter.
	// Info-level (global var, singleton hint) is observational noise.
	// ═══════════════════════════════════════════════════════════════════

	{
		matrix: methodMatrix{
			name: "arch_massive_struct", totalLines: 100,
			hits: []lineHit{
				{line: 1, tier: TierArchitecture, bit: 4, severity: SevCritical}, // massive_struct (>25 fields)
			},
		},
		shouldSurface: true,
		reason: "Massive struct — critical architecture smell, always surfaces",
	},
	{
		matrix: methodMatrix{
			name: "arch_fat_interface_extreme", totalLines: 80,
			hits: []lineHit{
				{line: 1, tier: TierArchitecture, bit: 8, severity: SevCritical}, // extreme_imports (>30)
				{line: 5, tier: TierArchitecture, bit: 11, severity: SevCritical}, // fat_interface (>20 methods)
			},
		},
		shouldSurface: true,
		reason: "Extreme imports + fat interface — two criticals, compound architecture issue",
	},
	{
		matrix: methodMatrix{
			name: "arch_coupling_breadth", totalLines: 120,
			hits: []lineHit{
				{line: 10, tier: TierArchitecture, bit: 15, severity: SevWarning}, // parameter_explosion
				{line: 20, tier: TierArchitecture, bit: 16, severity: SevWarning}, // function_too_deep
				{line: 40, tier: TierArchitecture, bit: 18, severity: SevWarning}, // handler_too_long
				{line: 60, tier: TierArchitecture, bit: 5, severity: SevWarning},  // excessive_imports
			},
		},
		shouldSurface: true,
		reason: "4 distinct coupling/structure warnings — breadth signals real architecture debt",
	},
	{
		matrix: methodMatrix{
			name: "arch_hexagonal_violation", totalLines: 30,
			hits: []lineHit{
				{line: 5, tier: TierArchitecture, bit: 7, severity: SevWarning}, // domain_imports_adapter
			},
		},
		shouldSurface: true,
		reason: "Domain importing adapter — architectural violation, warning but always meaningful",
	},
	{
		matrix: methodMatrix{
			name: "arch_single_global_var", totalLines: 150,
			hits: []lineHit{
				{line: 3, tier: TierArchitecture, bit: 0, severity: SevInfo}, // global_state
			},
		},
		shouldSurface: false,
		reason: "One global var hint in 150 lines — observational, not actionable",
	},
	{
		matrix: methodMatrix{
			name: "arch_scattered_info", totalLines: 200,
			hits: []lineHit{
				{line: 15, tier: TierArchitecture, bit: 0, severity: SevInfo},  // global_state
				{line: 88, tier: TierArchitecture, bit: 2, severity: SevInfo},  // singleton_pattern
				{line: 190, tier: TierArchitecture, bit: 17, severity: SevInfo}, // long_method_chain
			},
		},
		shouldSurface: false,
		reason: "3 info scattered across 200 lines — observational noise",
	},
	{
		matrix: methodMatrix{
			name: "arch_two_warnings_far", totalLines: 400,
			hits: []lineHit{
				{line: 20, tier: TierArchitecture, bit: 15, severity: SevWarning},  // parameter_explosion
				{line: 380, tier: TierArchitecture, bit: 17, severity: SevInfo}, // long_method_chain
			},
		},
		shouldSurface: false,
		reason: "1 warning + 1 info 360 lines apart — uncorrelated",
	},

	// ═══════════════════════════════════════════════════════════════════
	// CROSS-DIMENSION scenarios
	// Methods with findings spanning multiple tiers.
	// ═══════════════════════════════════════════════════════════════════

	{
		matrix: methodMatrix{
			name: "cross_sec_plus_quality", totalLines: 40,
			hits: []lineHit{
				{line: 10, tier: TierSecurity, bit: 0, severity: SevCritical},  // SQL injection
				{line: 10, tier: TierQuality, bit: 0, severity: SevWarning},    // ignored error — SAME LINE
				{line: 15, tier: TierObservability, bit: 10, severity: SevInfo}, // unstructured log
			},
		},
		shouldSurface: true,
		reason: "SQL injection + ignored error on same line + unstructured log — 3 tiers compound",
	},
	{
		matrix: methodMatrix{
			name: "cross_perf_plus_quality", totalLines: 60,
			hits: []lineHit{
				{line: 10, tier: TierPerformance, bit: 0, severity: SevWarning},  // defer_in_loop
				{line: 15, tier: TierPerformance, bit: 15, severity: SevWarning}, // allocation_in_loop
				{line: 20, tier: TierQuality, bit: 0, severity: SevWarning},      // ignored_error
				{line: 20, tier: TierQuality, bit: 3, severity: SevWarning},      // error_not_checked — SAME LINE
			},
		},
		shouldSurface: true,
		reason: "Perf loop issues + quality error handling issues — cross-cutting debt",
	},
	{
		matrix: methodMatrix{
			name: "cross_sparse_info_multi_tier", totalLines: 250,
			hits: []lineHit{
				{line: 30, tier: TierPerformance, bit: 9, severity: SevInfo},     // sync_primitive_usage
				{line: 100, tier: TierQuality, bit: 11, severity: SevInfo},       // commented_out_code
				{line: 200, tier: TierObservability, bit: 1, severity: SevInfo},  // todo_fixme
				{line: 240, tier: TierArchitecture, bit: 0, severity: SevInfo},   // global_state
			},
		},
		shouldSurface: false,
		reason: "4 info across 4 tiers but all sparse and scattered — noise across dimensions",
	},
}

// ── Matrix analysis helpers ─────────────────────────────────────────────
//
// These extract the structural features a data scientist would compute
// from a binary line × bit matrix.

type matrixFeatures struct {
	// Bit-level
	uniqueBits   int                // how many distinct (tier,bit) pairs fired
	perBitDensity map[tierBit]float64 // per-bit: lines_hit / totalLines
	perBitSeverity map[tierBit]Severity

	// Line-level
	linesWithSignal int     // lines that have at least one bit set
	coverage        float64 // linesWithSignal / totalLines

	// Co-occurrence
	cooccurrences int // count of line-pairs where 2+ distinct bits fire on same line
	hotLines      int // lines with 2+ distinct bits

	// Clustering (proximity)
	maxClusterSize int     // largest group of consecutive signal-bearing lines
	clusterDensity float64 // maxClusterSize / totalLines

	// Severity
	maxSeverity  Severity
	hasCritical  bool
	hasHigh      bool

	// Category (tier-level)
	tiersHit     int // how many distinct tiers have at least one bit
}

func extractFeatures(m methodMatrix) matrixFeatures {
	f := matrixFeatures{
		perBitDensity:  make(map[tierBit]float64),
		perBitSeverity: make(map[tierBit]Severity),
	}

	// Per-bit: count distinct lines
	type bitInfo struct {
		lines    map[int]bool
		severity Severity
	}
	perBit := make(map[tierBit]*bitInfo)
	lineSignals := make(map[int]map[tierBit]bool) // line → set of bits on that line
	tierSet := make(map[Tier]bool)

	for _, h := range m.hits {
		tb := tierBit{h.tier, h.bit}
		bi, ok := perBit[tb]
		if !ok {
			bi = &bitInfo{lines: make(map[int]bool), severity: h.severity}
			perBit[tb] = bi
		}
		bi.lines[h.line] = true

		if lineSignals[h.line] == nil {
			lineSignals[h.line] = make(map[tierBit]bool)
		}
		lineSignals[h.line][tb] = true

		tierSet[h.tier] = true

		if h.severity > f.maxSeverity {
			f.maxSeverity = h.severity
		}
	}

	f.uniqueBits = len(perBit)
	f.tiersHit = len(tierSet)
	f.hasCritical = f.maxSeverity >= SevCritical
	f.hasHigh = f.maxSeverity >= SevHigh
	f.linesWithSignal = len(lineSignals)
	if m.totalLines > 0 {
		f.coverage = float64(f.linesWithSignal) / float64(m.totalLines)
	}

	for tb, bi := range perBit {
		f.perBitDensity[tb] = float64(len(bi.lines)) / float64(m.totalLines)
		f.perBitSeverity[tb] = bi.severity
	}

	// Co-occurrence: lines with 2+ distinct bits
	for _, bits := range lineSignals {
		if len(bits) >= 2 {
			f.hotLines++
			// Count pairs: n*(n-1)/2
			n := len(bits)
			f.cooccurrences += n * (n - 1) / 2
		}
	}

	// Clustering: find longest run of consecutive lines with signal
	if len(lineSignals) > 0 {
		signalLines := make([]int, 0, len(lineSignals))
		for l := range lineSignals {
			signalLines = append(signalLines, l)
		}
		sort.Ints(signalLines)

		maxRun, curRun := 1, 1
		for i := 1; i < len(signalLines); i++ {
			if signalLines[i] <= signalLines[i-1]+2 { // within 2 lines = same cluster
				curRun++
			} else {
				curRun = 1
			}
			if curRun > maxRun {
				maxRun = curRun
			}
		}
		f.maxClusterSize = maxRun
		f.clusterDensity = float64(maxRun) / float64(m.totalLines)
	}

	return f
}

// ── Scoring formulas ────────────────────────────────────────────────────

type scoringFormula struct {
	name  string
	desc  string
	score func(m methodMatrix) float64
	gate  float64
}

// ruleAmplifier simulates the per-rule amplifier field from YAML.
// Rules with amplifier > 1.0 bypass density requirements —
// their presence alone contributes amplifier × weight.
// Default is 0 (no amplifier, density governs).
var ruleAmplifier = map[tierBit]float64{
	// Security: some findings are always meaningful
	// (but critical severity already handles most of these)

	// Architecture: structural violations are categorical
	{TierArchitecture, 7}:  3.0, // domain_imports_adapter — hexagonal violation is always wrong
	{TierArchitecture, 6}:  2.0, // banned_import
	{TierArchitecture, 22}: 2.0, // test_import_in_prod

	// Quality: some warnings are always meaningful
	{TierQuality, 1}: 2.0, // panic_in_lib — panic in library code is always wrong

	// Observability: resilience issues are always meaningful
	{TierObservability, 15}: 2.0, // panic_in_handler
	{TierObservability, 16}: 2.0, // os_exit_in_lib
	{TierObservability, 17}: 2.0, // fatal_log_in_lib

	// Observability: debug artifacts — info severity but density makes them actionable
	{TierObservability, 0}: 1.5, // print_statement — left-in debugging
}

var formulas = func() []scoringFormula {
	return []scoringFormula{

		// ── A: Current implementation (baseline) ────────────────────
		{
			name: "A_or_breadth",
			desc: "Current: OR rollup, weighted sum of unique set bits",
			gate: 3.0,
			score: func(m methodMatrix) float64 {
				seen := make(map[tierBit]bool)
				total := 0.0
				for _, h := range m.hits {
					tb := tierBit{h.tier, h.bit}
					if seen[tb] {
						continue
					}
					seen[tb] = true
					total += float64(severityWeight[h.severity])
				}
				return total
			},
		},

		// ── I: Holistic (previous best — 31/36) ────────────────────
		{
			name: "I_holistc",
			desc: "Severity floor + density modulation + co-occur + cluster (no breadth, no amplifier)",
			gate: 3.0,
			score: func(m methodMatrix) float64 {
				feat := extractFeatures(m)

				base := 0.0
				for tb, density := range feat.perBitDensity {
					sev := feat.perBitSeverity[tb]
					w := float64(severityWeight[sev])
					switch {
					case sev >= SevCritical:
						base += w
					case sev >= SevHigh:
						base += w * math.Max(0.5, math.Min(1.0, density*10))
					case sev >= SevWarning:
						base += w * math.Min(1.0, density*10)
					default:
						base += w * math.Min(1.0, density*5)
					}
				}

				coBonus := float64(feat.cooccurrences) * 2.0
				clusterBonus := 0.0
				if feat.maxClusterSize >= 3 {
					clusterBonus = float64(feat.maxClusterSize) * 0.5
				}

				return base + coBonus + clusterBonus
			},
		},

		// ── L: I + breadth bonus ────────────────────────────────────
		// Adds breadth: 4+ distinct bits is signal regardless of density.
		// This captures "systematic debt" where many different issues
		// each appear once but together paint a picture.
		{
			name: "L_breadth",
			desc: "I + breadth bonus: 3+ distinct warning bits or 4+ any bits adds score",
			gate: 3.0,
			score: func(m methodMatrix) float64 {
				feat := extractFeatures(m)

				base := 0.0
				warnBits := 0
				for tb, density := range feat.perBitDensity {
					sev := feat.perBitSeverity[tb]
					w := float64(severityWeight[sev])
					switch {
					case sev >= SevCritical:
						base += w
					case sev >= SevHigh:
						base += w * math.Max(0.5, math.Min(1.0, density*10))
					case sev >= SevWarning:
						base += w * math.Min(1.0, density*10)
						warnBits++
					default:
						base += w * math.Min(1.0, density*5)
					}
				}

				coBonus := float64(feat.cooccurrences) * 2.0
				clusterBonus := 0.0
				if feat.maxClusterSize >= 3 {
					clusterBonus = float64(feat.maxClusterSize) * 0.5
				}

				// Breadth bonus: many distinct bits = systematic issue
				breadthBonus := 0.0
				if warnBits >= 3 {
					// Each warning bit beyond 2 adds 1.0
					breadthBonus = float64(warnBits-2) * 1.0
				}
				if feat.uniqueBits >= 4 && breadthBonus == 0 {
					breadthBonus = float64(feat.uniqueBits-3) * 0.5
				}

				return base + coBonus + clusterBonus + breadthBonus
			},
		},

		// ── M: I + stronger clustering ──────────────────────────────
		// Treats adjacent lines (±2) as near-co-occurrence.
		// Cluster of 2+ signal lines = bonus scales with unique bits
		// in the cluster, not just cluster size.
		{
			name: "M_clustr",
			desc: "I + stronger cluster: adjacent signals (±2) treated as near-co-occurrence",
			gate: 3.0,
			score: func(m methodMatrix) float64 {
				feat := extractFeatures(m)

				base := 0.0
				for tb, density := range feat.perBitDensity {
					sev := feat.perBitSeverity[tb]
					w := float64(severityWeight[sev])
					switch {
					case sev >= SevCritical:
						base += w
					case sev >= SevHigh:
						base += w * math.Max(0.5, math.Min(1.0, density*10))
					case sev >= SevWarning:
						base += w * math.Min(1.0, density*10)
					default:
						base += w * math.Min(1.0, density*5)
					}
				}

				coBonus := float64(feat.cooccurrences) * 2.0

				// Cluster bonus: scale with both cluster size AND
				// unique bits in the cluster
				clusterBonus := 0.0
				if feat.maxClusterSize >= 2 {
					// Count unique bits within the cluster
					clusterBits := countClusterBits(m)
					clusterBonus = float64(clusterBits) * 1.0
				}

				return base + coBonus + clusterBonus
			},
		},

		// ── N: I + amplifier ────────────────────────────────────────
		// Per-rule amplifier: rules flagged as "always meaningful" bypass
		// density. amplifier × weight is their base contribution.
		{
			name: "N_amplfy",
			desc: "I + per-rule amplifier: priority rules bypass density requirement",
			gate: 3.0,
			score: func(m methodMatrix) float64 {
				feat := extractFeatures(m)

				base := 0.0
				for tb, density := range feat.perBitDensity {
					sev := feat.perBitSeverity[tb]
					w := float64(severityWeight[sev])

					// Check amplifier
					if amp, ok := ruleAmplifier[tb]; ok && amp > 0 {
						base += w * amp // bypass density
						continue
					}

					switch {
					case sev >= SevCritical:
						base += w
					case sev >= SevHigh:
						base += w * math.Max(0.5, math.Min(1.0, density*10))
					case sev >= SevWarning:
						base += w * math.Min(1.0, density*10)
					default:
						base += w * math.Min(1.0, density*5)
					}
				}

				coBonus := float64(feat.cooccurrences) * 2.0
				clusterBonus := 0.0
				if feat.maxClusterSize >= 3 {
					clusterBonus = float64(feat.maxClusterSize) * 0.5
				}

				return base + coBonus + clusterBonus
			},
		},

		// ── P: Full composite — I + breadth + cluster + amplifier ───
		// All three improvements combined.
		{
			name: "P_full",
			desc: "Full: severity floor + density + co-occur + breadth + cluster + amplifier",
			gate: 3.0,
			score: func(m methodMatrix) float64 {
				feat := extractFeatures(m)

				// Component 1: Severity-anchored base with amplifier
				base := 0.0
				warnBits := 0
				for tb, density := range feat.perBitDensity {
					sev := feat.perBitSeverity[tb]
					w := float64(severityWeight[sev])

					// Amplifier: priority rules get amp as floor,
					// density can push higher (amp + density contribution)
					if amp, ok := ruleAmplifier[tb]; ok && amp > 0 {
						// Floor = w * amp. Density bonus on top, uncapped.
						densityContrib := density * w * 10 // scale density generously
						base += w*amp + densityContrib
						if sev >= SevWarning {
							warnBits++
						}
						continue
					}

					switch {
					case sev >= SevCritical:
						base += w
					case sev >= SevHigh:
						base += w * math.Max(0.5, math.Min(1.0, density*10))
					case sev >= SevWarning:
						base += w * math.Min(1.0, density*10)
						warnBits++
					default:
						base += w * math.Min(1.0, density*5)
					}
				}

				// Component 2: Co-occurrence
				coBonus := float64(feat.cooccurrences) * 2.0

				// Component 3: Cluster — scale with unique bits in cluster
				clusterBonus := 0.0
				if feat.maxClusterSize >= 2 {
					clusterBits := countClusterBits(m)
					clusterBonus = float64(clusterBits) * 1.0
				}

				// Component 4: Breadth
				breadthBonus := 0.0
				if warnBits >= 3 {
					breadthBonus = float64(warnBits-2) * 1.0
				}
				if feat.uniqueBits >= 4 && breadthBonus == 0 {
					breadthBonus = float64(feat.uniqueBits-3) * 0.5
				}

				return base + coBonus + clusterBonus + breadthBonus
			},
		},
	}
}()

// countClusterBits counts distinct (tier,bit) pairs within the largest
// cluster of adjacent signal-bearing lines (±2 line gap tolerance).
func countClusterBits(m methodMatrix) int {
	// Build line → set of bits
	lineSignals := make(map[int]map[tierBit]bool)
	for _, h := range m.hits {
		tb := tierBit{h.tier, h.bit}
		if lineSignals[h.line] == nil {
			lineSignals[h.line] = make(map[tierBit]bool)
		}
		lineSignals[h.line][tb] = true
	}

	if len(lineSignals) == 0 {
		return 0
	}

	// Sort signal lines
	signalLines := make([]int, 0, len(lineSignals))
	for l := range lineSignals {
		signalLines = append(signalLines, l)
	}
	sort.Ints(signalLines)

	// Find clusters and track the largest by unique bits
	type cluster struct {
		bits map[tierBit]bool
	}
	best := &cluster{bits: make(map[tierBit]bool)}
	cur := &cluster{bits: make(map[tierBit]bool)}

	// Seed with first line
	for tb := range lineSignals[signalLines[0]] {
		cur.bits[tb] = true
	}

	for i := 1; i < len(signalLines); i++ {
		if signalLines[i] <= signalLines[i-1]+2 {
			// Continue cluster
			for tb := range lineSignals[signalLines[i]] {
				cur.bits[tb] = true
			}
		} else {
			// End cluster, start new
			if len(cur.bits) > len(best.bits) {
				best = cur
			}
			cur = &cluster{bits: make(map[tierBit]bool)}
			for tb := range lineSignals[signalLines[i]] {
				cur.bits[tb] = true
			}
		}
	}
	if len(cur.bits) > len(best.bits) {
		best = cur
	}

	return len(best.bits)
}

// ── The tests ───────────────────────────────────────────────────────────

func TestScoringFormulas(t *testing.T) {
	for _, f := range formulas {
		t.Run(f.name, func(t *testing.T) {
			t.Logf("Formula: %s", f.desc)
			t.Logf("Gate: %.1f", f.gate)
			t.Log("")

			pass, fail := 0, 0
			for _, s := range scenarios {
				score := f.score(s.matrix)
				surfaced := score >= f.gate
				correct := surfaced == s.shouldSurface

				status := "PASS"
				if !correct {
					status = "FAIL"
					fail++
				} else {
					pass++
				}

				wantStr := "SUPPRESS"
				if s.shouldSurface {
					wantStr = "SURFACE"
				}
				gotStr := "suppressed"
				if surfaced {
					gotStr = "surfaced"
				}

				linesHit := make(map[int]bool)
				for _, h := range s.matrix.hits {
					linesHit[h.line] = true
				}
				coverage := float64(len(linesHit)) / float64(s.matrix.totalLines) * 100

				feat := extractFeatures(s.matrix)

				t.Logf("  [%s] %-35s  score=%6.2f  want=%-7s got=%-10s  (%3d lines, %4.1f%% cov, %d bits, %d co-oc, clust=%d)",
					status, s.matrix.name, score, wantStr, gotStr,
					s.matrix.totalLines, coverage, feat.uniqueBits, feat.cooccurrences, feat.maxClusterSize)
				if !correct {
					t.Logf("         → %s", s.reason)
				}
			}
			t.Logf("")
			t.Logf("  Result: %d/%d correct", pass, pass+fail)
			if fail > 0 {
				t.Errorf("  %d scenarios misclassified", fail)
			}
		})
	}
}

func TestScoringFormulaSummary(t *testing.T) {
	t.Log("═══ Scoring Formula Comparison ═══")
	t.Log("")

	// Header
	header := fmt.Sprintf("%-37s %-6s", "Scenario", "Want")
	for _, f := range formulas {
		n := f.name
		if len(n) > 10 {
			n = n[:10]
		}
		header += fmt.Sprintf("  %-8s", n)
	}
	t.Log(header)
	t.Log("─────────────────────────────────────────────────────────────────────────────────────────────────────────")

	formulaCorrect := make([]int, len(formulas))

	for _, s := range scenarios {
		wantStr := "SUPPR"
		if s.shouldSurface {
			wantStr = "SURF"
		}
		row := fmt.Sprintf("%-37s %-6s", s.matrix.name, wantStr)

		for fi, f := range formulas {
			score := f.score(s.matrix)
			surfaced := score >= f.gate
			correct := surfaced == s.shouldSurface

			mark := " ✓"
			if !correct {
				mark = " ✗"
			} else {
				formulaCorrect[fi]++
			}
			row += fmt.Sprintf("  %s%5.1f", mark, score)
		}
		t.Log(row)
	}

	t.Log("")
	total := len(scenarios)
	summary := fmt.Sprintf("%-37s %-6s", "TOTAL CORRECT", "")
	for fi := range formulas {
		summary += fmt.Sprintf("  %d/%2d   ", formulaCorrect[fi], total)
	}
	t.Log(summary)
}
