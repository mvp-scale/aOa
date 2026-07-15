package test

// FDN-4 (board #30): the closed-area gate for the facts substrate.
//
// internal/domain/facts (the compactor/detector) and internal/domain/arch
// (the rendition domain) must stay dependency-free per hexagonal law
// (CLAUDE.md) — neither may import internal/app or an internal/adapters/*
// package. internal/app already imports both domain packages (the correct
// direction: app -> domain); a reverse import would create an import cycle
// and break the "domain has no knowledge of its callers" invariant the
// compactor's own doc comment relies on (internal/domain/facts/compactor.go:
// "domain/facts must stay dependency-free ... internal/app already imports
// this package, so the reverse import is not an option").
//
// Reuses buildSelfDepGraph (arch_selftest_test.go, T11) rather than
// reimplementing an import-graph walk — one source of truth for aOa's own
// intra-module dependency graph.

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestDependencyDirection_SubstrateStaysClosed asserts that no dependency
// edge in aOa's own package graph runs FROM internal/domain/facts or
// internal/domain/arch TO internal/app or any internal/adapters/* package —
// the substrate closed-area gate FDN-4 installs. A future change that has
// the facts/arch domain reach up into the app or adapter layer (e.g. for
// convenience) fails this test rather than silently introducing a cycle.
func TestDependencyDirection_SubstrateStaysClosed(t *testing.T) {
	units, deps := buildSelfDepGraph(t)
	assert.NotEmpty(t, units, "buildSelfDepGraph must find at least one package")

	substrateInfixes := []string{"/internal/domain/facts", "/internal/domain/arch"}
	higherInfixes := []string{"/internal/app", "/internal/adapters/bbolt", "/internal/adapters/socket"}

	isSubstrate := func(pkg string) bool {
		for _, infix := range substrateInfixes {
			if strings.Contains(pkg, infix) {
				return true
			}
		}
		return false
	}
	isHigher := func(pkg string) bool {
		for _, infix := range higherInfixes {
			if strings.Contains(pkg, infix) {
				return true
			}
		}
		return false
	}

	var violations int
	for _, d := range deps {
		if isSubstrate(d.FromUnit) && isHigher(d.ToUnit) {
			violations++
			t.Errorf("FDN-4 closed-area violation: substrate package %q imports higher-layer package %q (%s:%d)",
				d.FromUnit, d.ToUnit, d.File, d.Line)
		}
	}
	t.Logf("FDN-4: checked %d dep edges — %d closed-area violations (must be 0)", len(deps), violations)
}
