package verify_test

import (
	"testing"

	"github.com/StevenACoffman/canonizer/internal/verify"
	"github.com/StevenACoffman/skillet/ruleset"
)

// scoped builds a ruleset from statements paired with anchors, all MUST so every rule is
// enforced, with a discriminating pair so nothing else has anything to say.
func scoped(pairs ...[2]string) ruleset.Ruleset {
	rs := ruleset.Ruleset{}
	for i, p := range pairs {
		rs.Rules = append(rs.Rules, ruleset.Rule{
			Section: string(rune('1' + i)), Severity: ruleset.MUST, Level: ruleset.CODE,
			Statement: p[0], Bad: "b", Good: "g", SourceAnchor: p[1],
		})
	}
	return rs
}

func TestScopeCountsSymbolNamingRules(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		rules    ruleset.Ruleset
		symbolic int
		enforced int
	}{{
		name:     "a code span counts",
		rules:    scoped([2]string{"Call `ctx.Done()` first.", `§S: "q"`}),
		symbolic: 1, enforced: 1,
	}, {
		name:     "a bare identifier shape counts",
		rules:    scoped([2]string{"Pass sql.DB to the store.", `§S: "q"`}),
		symbolic: 1, enforced: 1,
	}, {
		name:     "prose naming no symbol does not",
		rules:    scoped([2]string{"Decompose the work into smaller steps.", `§S: "q"`}),
		symbolic: 0, enforced: 1,
	}, {
		name: "the count is a proportion of the enforced rules",
		rules: scoped(
			[2]string{"Call `ctx.Done()` first.", `§S: "q"`},
			[2]string{"Decompose the work into smaller steps.", `§S: "q"`},
			[2]string{"Prefer *sql.Tx over a raw handle.", `§S: "q"`},
		),
		symbolic: 2, enforced: 3,
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := verify.Rules(&c.rules)
			if got.Symbolic != c.symbolic {
				t.Errorf("Symbolic = %d, want %d", got.Symbolic, c.symbolic)
			}
			if got.Enforced != c.enforced {
				t.Errorf("Enforced = %d, want %d", got.Enforced, c.enforced)
			}
		})
	}
}

// TestScopeCountsOnlyEnforcedRules keeps both new counts on the same footing as Enforced:
// the gates never look at a CONSIDER rule, so neither number may claim they did.
func TestScopeCountsOnlyEnforcedRules(t *testing.T) {
	t.Parallel()
	rs := ruleset.Ruleset{Rules: []ruleset.Rule{{
		Section: "1", Severity: ruleset.CONSIDER, Level: ruleset.CODE,
		Statement: "Call `ctx.Done()` first.", SourceAnchor: "§Section",
	}}}
	got := verify.Rules(&rs)
	if got.Enforced != 0 || got.Symbolic != 0 || got.SectionOnly != 0 {
		t.Errorf("an unenforced rule was counted: %+v", got)
	}
	if got.Total != 1 {
		t.Errorf("Total = %d, want 1", got.Total)
	}
}

// TestScopeCountsUnsearchableAnchors is the accounting :1724 corrected. Such a rule *was*
// examined for executability and *was not* examined for provenance, and until this the
// scope line could not say so.
func TestScopeCountsUnsearchableAnchors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		rules       ruleset.Ruleset
		sectionOnly int
	}{{
		name:        "a quoted anchor was searched",
		rules:       scoped([2]string{"Call `ctx.Done()` first.", `§Section: "the quote"`}),
		sectionOnly: 0,
	}, {
		name:        "a section with nothing after it was not",
		rules:       scoped([2]string{"Call `ctx.Done()` first.", "§Transactional boundaries"}),
		sectionOnly: 1,
	}, {
		name: "counted across rules",
		rules: scoped(
			[2]string{"Call `ctx.Done()` first.", "§Errors"},
			[2]string{"Pass sql.DB to the store.", `§S: "q"`},
			[2]string{"Prefer *sql.Tx here.", "§4.2"},
		),
		sectionOnly: 2,
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := verify.Rules(&c.rules).SectionOnly; got != c.sectionOnly {
				t.Errorf("SectionOnly = %d, want %d", got, c.sectionOnly)
			}
		})
	}
}
