package ports

import "testing"

// L20 contract drift (v2.1.181 → v2.1.193): Claude Code now stamps
// `origin: {kind: "human"}` on genuinely TYPED user prompts. The old rule
// "real user input has Origin == nil" silently filtered every real prompt —
// promptN, bigrams, the conversation feed, and Debrief all went dark while
// token/usage attribution kept working (assistant events were unaffected).
// Census across all real session logs: kinds are exactly nil (old format +
// tool results), "human" (typed), "task-notification" (injected).
func TestIsRealUser_HumanOriginIsReal(t *testing.T) {
	ev := &SessionEvent{
		Kind:   EventUserInput,
		Text:   "Bye.",
		Origin: map[string]any{"kind": "human"},
	}
	if !ev.IsRealUser() {
		t.Fatal("origin kind=human is a typed prompt (v2.1.193+) — must count as real user input")
	}
}

func TestIsRealUser_NilOriginStaysReal(t *testing.T) {
	ev := &SessionEvent{Kind: EventUserInput, Text: "hello"}
	if !ev.IsRealUser() {
		t.Fatal("pre-2.1.19x logs carry no origin on typed prompts — nil must stay real (backfill compatibility)")
	}
}

func TestIsRealUser_InjectedKindsFiltered(t *testing.T) {
	for _, kind := range []string{"task-notification", "compact", "anything-else"} {
		ev := &SessionEvent{
			Kind:   EventUserInput,
			Text:   "injected",
			Origin: map[string]any{"kind": kind},
		}
		if ev.IsRealUser() {
			t.Fatalf("origin kind=%q is injected — must never count as a user prompt", kind)
		}
	}
}

func TestIsRealUser_NonUserKindNeverReal(t *testing.T) {
	ev := &SessionEvent{Kind: EventAIResponse, Origin: map[string]any{"kind": "human"}}
	if ev.IsRealUser() {
		t.Fatal("only EventUserInput can be real user input")
	}
}
