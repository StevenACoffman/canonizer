package verify_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/canonizer/internal/verify"
	"github.com/StevenACoffman/skillet/finding"
	"github.com/StevenACoffman/skillet/quotecheck"
	"github.com/StevenACoffman/skillet/ruleset"
)

func TestUnverifiable(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		rules    []ruleset.Rule
		wantDiag bool
		wantIn   string
	}{
		"one enforced rule citing an anchor": {
			rules:    []ruleset.Rule{rule("1.1", ruleset.MUST, "b", "g", "ANCHOR")},
			wantDiag: true, wantIn: "1 enforced rule(s)",
		},
		"the count is of anchored enforced rules": {
			rules: []ruleset.Rule{
				rule("1.1", ruleset.MUST, "b", "g", "ANCHOR"),
				rule("1.2", ruleset.SHOULD, "b", "g", "ANCHOR"),
			},
			wantDiag: true, wantIn: "2 enforced rule(s)",
		},
		// Nothing went unverified, so a report would name a risk the ruleset does not
		// carry. no-anchor already covers a rule citing nothing.
		"an enforced rule citing no anchor is not unverifiable": {
			rules:    []ruleset.Rule{rule("1.1", ruleset.MUST, "b", "g", "")},
			wantDiag: false,
		},
		// A CONSIDER rule is exempt from provenance, so its anchor going unsearched is not
		// a gap -- the gates never look at it either way.
		"an advisory rule is not counted": {
			rules:    []ruleset.Rule{rule("1.1", ruleset.CONSIDER, "b", "g", "ANCHOR")},
			wantDiag: false,
		},
		"an empty ruleset reports nothing": {
			rules: nil, wantDiag: false,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assertUnverifiable(t, verify.Unverifiable(ruleset.Ruleset{Rules: tc.rules}),
				tc.wantDiag, tc.wantIn)
		})
	}
}

// TestUnverifiableIsNotAnchorAbsent pins the distinction the whole state exists for: a
// search that found nothing and no search at all call for opposite responses, and one
// category for both is what this item was filed to end.
func TestUnverifiableIsNotAnchorAbsent(t *testing.T) {
	t.Parallel()
	rs := ruleset.Ruleset{Rules: []ruleset.Rule{rule("1.1", ruleset.MUST, "b", "g", "ANCHOR")}}

	unverifiable := verify.Unverifiable(rs)
	absent := verify.Provenance(rs, "a source that does not contain it")
	if len(unverifiable) != 1 || len(absent) != 1 {
		t.Fatalf("want one of each; got %+v and %+v", unverifiable, absent)
	}
	if unverifiable[0].Category == absent[0].Category {
		t.Errorf("both report %q; the states must be distinguishable", absent[0].Category)
	}
	if absent[0].Severity.Blocking() == unverifiable[0].Severity.Blocking() {
		t.Errorf("a search that found nothing and no search at all must not share a verdict")
	}
}

// TestUncheckedIsTheZeroValue pins the direction skillet chose and this entry insisted on:
// a status nobody populated must read as "not checked", never as "checked and clean". The
// comment saying so is what a refactor deletes; this is what survives one.
func TestUncheckedIsTheZeroValue(t *testing.T) {
	t.Parallel()
	var zero quotecheck.Status
	if zero != quotecheck.Unchecked {
		t.Errorf("zero Status = %v, want Unchecked so an unpopulated value fails closed", zero)
	}
	if zero == quotecheck.Found {
		t.Error("the zero value reads as Found; an unrun check would launder into a pass")
	}
}

// assertUnverifiable checks the single ruleset-level advisory, or that there is none.
// Extracted to keep the table flat: the absent branch plus three assertions inside a
// subtest is what pushes it over the complexity cap.
func assertUnverifiable(t *testing.T, got []finding.Diagnostic, wantDiag bool, wantIn string) {
	t.Helper()
	if !wantDiag {
		if len(got) != 0 {
			t.Fatalf("Unverifiable = %+v, want none", got)
		}
		return
	}
	if len(got) != 1 {
		t.Fatalf("Unverifiable = %+v, want one diagnostic for the ruleset", got)
	}
	if got[0].Severity.Blocking() {
		t.Errorf("severity = %q; this state must report, never block", got[0].Severity)
	}
	if !strings.Contains(got[0].Message, wantIn) {
		t.Errorf("message = %q, want it to contain %q", got[0].Message, wantIn)
	}
}
