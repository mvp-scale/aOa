package main

import "testing"

func TestCLI_UnknownVerbExit2(t *testing.T) {
	if code := run([]string{"frobnicate"}); code != 2 {
		t.Fatalf("unknown verb should exit 2, got %d", code)
	}
}

func TestCLI_NoArgsExit2(t *testing.T) {
	if code := run(nil); code != 2 {
		t.Fatalf("no args should exit 2, got %d", code)
	}
}

func TestCLI_VerbsOK(t *testing.T) {
	for _, v := range []string{"graph", "target", "drift", "ledger"} {
		if code := run([]string{v}); code != 0 {
			t.Fatalf("verb %q should exit 0, got %d", v, code)
		}
		if code := run([]string{v, "--json"}); code != 0 {
			t.Fatalf("verb %q --json should exit 0, got %d", v, code)
		}
	}
}
