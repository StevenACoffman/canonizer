package verify_test

import (
	"testing"

	"github.com/StevenACoffman/canonizer/internal/verify"
	"github.com/StevenACoffman/skillet/ruleset"
)

func TestLimitations(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		rs       ruleset.Ruleset
		wantDiag bool
	}{
		"states its limits": {
			rs:       ruleset.Ruleset{Scope: "Go", Limitations: "nothing about concurrency"},
			wantDiag: false,
		},
		"states none": {
			rs:       ruleset.Ruleset{Scope: "Go"},
			wantDiag: true,
		},
		"whitespace is not a statement": {
			rs:       ruleset.Ruleset{Scope: "Go", Limitations: "   "},
			wantDiag: true,
		},
		// The old check read Scope for a negation word. A scope that happens to contain one
		// must no longer excuse an absent Limitations: -- the guess overriding the fact is
		// exactly what deleting the word list was for.
		"a scope mentioning an exclusion does not substitute": {
			rs:       ruleset.Ruleset{Scope: "Go service code, not concurrency primitives"},
			wantDiag: true,
		},
		// Equally, a scope with no negation word must not condemn a ruleset that does state
		// its limits: the scope is no longer consulted at all.
		"a scope with no exclusion is irrelevant when limits are stated": {
			rs:       ruleset.Ruleset{Scope: "Go service code", Limitations: "not concurrency"},
			wantDiag: false,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := verify.Limitations(&tc.rs)
			if tc.wantDiag && len(got) != 1 {
				t.Fatalf("Limitations = %+v, want one advisory", got)
			}
			if !tc.wantDiag && len(got) != 0 {
				t.Fatalf("Limitations = %+v, want none", got)
			}
			if tc.wantDiag && got[0].Severity.Blocking() {
				t.Errorf("severity = %q; this check must not block an unmigrated corpus",
					got[0].Severity)
			}
		})
	}
}

// TestLimitationsRoundTripsFromADocument is the end of the change: the header the check now
// reads is one a real ruleset can carry, which was the whole reason the guess existed.
func TestLimitationsRoundTripsFromADocument(t *testing.T) {
	t.Parallel()
	doc := "---\nformat: 3\n---\nSource: s\nScope:  Go\n" +
		"Limitations: nothing about concurrency\n\n" +
		"§1.1  [MUST][CODE]  Close it.\n      because reasons\n"
	rs, err := ruleset.Parse(doc)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if rs.Limitations != "nothing about concurrency" {
		t.Fatalf("Limitations = %q, want the header read", rs.Limitations)
	}
	if got := verify.Limitations(&rs); len(got) != 0 {
		t.Errorf("a ruleset stating its limits was still flagged: %+v", got)
	}
}
