package verify_test

import (
	"testing"

	"github.com/StevenACoffman/canonizer/internal/verify"
	"github.com/StevenACoffman/skillet/finding"
	"github.com/StevenACoffman/skillet/ruleset"
)

// TestDriftCrossesTheTwoSignals walks the whole matrix. The point of the check is that
// these four cells were one category before: every one of them reported anchor-absent and
// blocked, so a routine source edit and a hallucinated citation got the same response.
func TestDriftCrossesTheTwoSignals(t *testing.T) {
	t.Parallel()
	const anchor = "ANCHOR-SENTINEL always release the connection"
	const withAnchor = "The docs say: " + anchor + ".\n"
	const withoutAnchor = "The docs say something else entirely.\n"

	cases := map[string]struct {
		source       string
		state        verify.SourceState
		wantCategory string
		wantBlocking bool
	}{
		"anchor present, source unchanged": {
			source: withAnchor, state: verify.SourceUnchanged,
			wantCategory: "", wantBlocking: false,
		},
		"anchor present, source changed is drift and does not block": {
			source: withAnchor, state: verify.SourceChanged,
			wantCategory: verify.CategoryAnchorDrift, wantBlocking: false,
		},
		"anchor absent, source unchanged is the one certain defect": {
			source: withoutAnchor, state: verify.SourceUnchanged,
			wantCategory: verify.CategoryAnchorFabricated, wantBlocking: true,
		},
		"anchor absent, source changed is stale and does not block": {
			source: withoutAnchor, state: verify.SourceChanged,
			wantCategory: verify.CategoryAnchorStale, wantBlocking: false,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			rs := ruleset.Ruleset{Rules: []ruleset.Rule{
				rule("1.1", ruleset.MUST, "bad", "good", anchor),
			}}
			assertDrift(t, verify.Drift(&rs, tc.source, tc.state), tc.wantCategory, tc.wantBlocking)
		})
	}
}

// assertDrift checks the single diagnostic's category and whether it blocks.
func assertDrift(t *testing.T, got []finding.Diagnostic, wantCategory string, wantBlocking bool) {
	t.Helper()
	if wantCategory == "" {
		if len(got) != 0 {
			t.Fatalf("Drift = %+v, want none", got)
		}
		return
	}
	if len(got) != 1 {
		t.Fatalf("Drift = %+v, want one diagnostic", got)
	}
	if got[0].Category != wantCategory {
		t.Errorf("category = %q, want %q", got[0].Category, wantCategory)
	}
	if got[0].Severity.Blocking() != wantBlocking {
		t.Errorf("blocking = %t, want %t", got[0].Severity.Blocking(), wantBlocking)
	}
}

// TestDriftWithoutASecondSignalIsProvenance pins the fallback: with one signal, an absent
// anchor blocking remains the honest verdict, so behaviour is unchanged for callers that
// pass no proof packet.
func TestDriftWithoutASecondSignalIsProvenance(t *testing.T) {
	t.Parallel()
	rs := ruleset.Ruleset{Rules: []ruleset.Rule{
		rule("1.1", ruleset.MUST, "bad", "good", "MISSING-ANCHOR"),
	}}
	const source = "nothing relevant here\n"

	got := verify.Drift(&rs, source, verify.SourceUnknown)
	want := verify.Provenance(&rs, source)
	if len(got) != len(want) || len(got) != 1 {
		t.Fatalf("Drift = %+v, Provenance = %+v; want the same single diagnostic", got, want)
	}
	if got[0].Category != want[0].Category {
		t.Errorf("category = %q, want Provenance's %q", got[0].Category, want[0].Category)
	}
	if !got[0].Severity.Blocking() {
		t.Error("the one-signal fallback must still block an absent anchor")
	}
}

// TestDriftReportsAMissingAnchorRegardlessOfSource keeps the two questions separate: a rule
// that cited nothing cannot have been invalidated by a source edit.
func TestDriftReportsAMissingAnchorRegardlessOfSource(t *testing.T) {
	t.Parallel()
	rs := ruleset.Ruleset{Rules: []ruleset.Rule{
		rule("1.1", ruleset.MUST, "bad", "good", ""),
	}}
	for _, state := range []verify.SourceState{verify.SourceUnchanged, verify.SourceChanged} {
		got := verify.Drift(&rs, "anything", state)
		if len(got) != 1 || got[0].Category != verify.CategoryNoAnchor {
			t.Errorf("Drift = %+v, want no-anchor regardless of source state", got)
		}
	}
}
