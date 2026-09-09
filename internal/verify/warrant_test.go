package verify_test

import (
	"testing"

	"github.com/StevenACoffman/canonizer/internal/verify"
	"github.com/StevenACoffman/skillet/finding"
	"github.com/StevenACoffman/skillet/ruleset"
)

// adjudicated returns an enforced rule with no anchor and the given warrant.
func adjudicated(w ruleset.Warrant) *ruleset.Ruleset {
	r := rule("1.1", ruleset.MUST, "bad", "good", "")
	r.Warrant = w
	return &ruleset.Ruleset{Rules: []ruleset.Rule{r}}
}

func TestProvenanceGatesOnTheWarrantWhereTheAnchorIsAbsent(t *testing.T) {
	t.Parallel()
	valid := ruleset.Warrant{By: "steve", At: "2026-09-05", Rationale: "§2.1 and §4.3 conflicted"}
	cases := map[string]struct {
		warrant      ruleset.Warrant
		wantCategory string
	}{
		// The item's whole point: an adjudicated rule is sourced differently, not
		// unsourced, so the check that protects quality must stop rejecting it.
		"a valid warrant replaces the anchor": {
			warrant: valid, wantCategory: "",
		},
		"no warrant is still unsourced": {
			warrant: ruleset.Warrant{}, wantCategory: verify.CategoryNoAnchor,
		},
		// An unreviewable decision is not a source. Reported rather than accepted, or the
		// warrant becomes a way to opt out of provenance entirely.
		"a warrant with no rationale records no decision": {
			warrant:      ruleset.Warrant{By: "steve", At: "2026-09-05"},
			wantCategory: verify.CategoryWarrantIncomplete,
		},
		"a warrant with no author records no decision": {
			warrant:      ruleset.Warrant{At: "2026-09-05", Rationale: "they conflicted"},
			wantCategory: verify.CategoryWarrantIncomplete,
		},
		"a warrant with an unparseable date records no decision": {
			warrant: ruleset.Warrant{
				By:        "steve",
				At:        "last tuesday",
				Rationale: "they conflicted",
			},
			wantCategory: verify.CategoryWarrantIncomplete,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assertCategory(
				t,
				verify.Provenance(adjudicated(tc.warrant), []string{"any source"}),
				tc.wantCategory,
			)
		})
	}
}

// assertCategory checks the single diagnostic's category, or that there is none.
func assertCategory(t *testing.T, got []finding.Diagnostic, want string) {
	t.Helper()
	if want == "" {
		if len(got) != 0 {
			t.Fatalf("got %+v, want no diagnostic", got)
		}
		return
	}
	if len(got) != 1 {
		t.Fatalf("got %+v, want one diagnostic", got)
	}
	if got[0].Category != want {
		t.Errorf("category = %q, want %q", got[0].Category, want)
	}
}

// TestDriftAppliesTheSameWarrantPolicy is the agreement the two checks must keep: routing
// both through unanchored is what stops one blocking what the other passes.
func TestDriftAppliesTheSameWarrantPolicy(t *testing.T) {
	t.Parallel()
	valid := ruleset.Warrant{By: "steve", At: "2026-09-05", Rationale: "§2.1 and §4.3 conflicted"}
	rs := adjudicated(valid)
	for _, state := range []verify.SourceState{
		verify.SourceUnknown, verify.SourceUnchanged, verify.SourceChanged,
	} {
		if got := verify.Drift(rs, []string{"any source"}, state); len(got) != 0 {
			t.Errorf("Drift with state %v = %+v, want an adjudicated rule accepted", state, got)
		}
	}
}
