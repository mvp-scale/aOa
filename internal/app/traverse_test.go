package app

import (
	"reflect"
	"testing"
)

// The traversal primitive behind `arch blast` (reverse edges → dependents) and
// the future `aoa map`. adj here is a REVERSE adjacency: adj[X] = units that import X.
// A→B, C→B, D→C  ⇒  reverse: B imported by {A,C}; C imported by {D}.
func reverseFixture() map[string][]string {
	return map[string][]string{
		"b": {"c", "a"}, // unsorted on purpose — output must still be deterministic
		"c": {"d"},
	}
}

func TestBFSReachable_AllHops(t *testing.T) {
	got := bfsReachable(reverseFixture(), "b", 10)
	want := []string{"a", "c", "d"} // A,C depend directly on B; D depends on C → transitively on B
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("blast(b, k=10) = %v, want %v", got, want)
	}
}

func TestBFSReachable_HopBounded(t *testing.T) {
	got := bfsReachable(reverseFixture(), "b", 1)
	want := []string{"a", "c"} // one hop: direct dependents only, not D
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("blast(b, k=1) = %v, want %v", got, want)
	}
}

func TestBFSReachable_ExcludesSeedAndDedups(t *testing.T) {
	adj := map[string][]string{"x": {"y", "y", "x"}} // self-edge + dup
	got := bfsReachable(adj, "x", 10)
	want := []string{"y"} // seed excluded, duplicates collapsed
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestBFSReachable_Deterministic(t *testing.T) {
	a := bfsReachable(reverseFixture(), "b", 10)
	b := bfsReachable(reverseFixture(), "b", 10)
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("nondeterministic: %v vs %v", a, b)
	}
}
