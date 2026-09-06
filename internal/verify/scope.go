package verify

import "github.com/StevenACoffman/skillet/ruleset"

// Scope is how much of a ruleset the enforced-rule gates actually looked at.
//
// Executable, Provenance and Specificity each open by skipping a rule whose severity is
// not enforced, and until 2026-09-05 nothing recorded how many that was. A ruleset of
// entirely CONSIDER rules therefore produced zero diagnostics -- byte-identical to one
// where every enforced rule passed -- and `gate` shipped on that silence.
//
// The defect was never the skipping. A CONSIDER rule genuinely is not required to carry a
// ✗/✓ pair or a source anchor, so exempting it is correct; what was missing is that the
// exemption was invisible. So this is a count, not a predicate: it changes no verdict and
// only lets a reader see the proportion of a ruleset these gates never look at.
//
// It is not finding.Unexamined, and the difference is the one skillet's own doc draws.
// Unexamined is testimony -- a critic's unverifiable claim about its own behaviour. This is
// mechanical: code applied a documented rule and can say so exactly. skillet has no type
// for the second, so it stays local here until a consumer beyond canonizer wants one.
type Scope struct {
	// Enforced is how many rules the gates examined.
	Enforced int
	// Total is how many rules the ruleset holds.
	Total int
}

// Advisory reports whether the gates examined nothing, which is the case a reader must not
// mistake for a clean pass.
//
// A ruleset with rules but none enforced is the dangerous shape: it emits no diagnostics
// and is indistinguishable, in the findings alone, from one that passed every check. An
// empty ruleset is not this case -- it has nothing to examine and nothing to mislead
// anyone about.
//
// Ensures: false when Total is 0; pure.
func (s Scope) Advisory() bool { return s.Total > 0 && s.Enforced == 0 }

// Rules returns the scope of a verify run over rs.
//
// Requires: nothing; rs may hold no rules.
// Ensures:  Enforced <= Total; Total == len(rs.Rules); it is pure and reads no I/O.
func Rules(rs ruleset.Ruleset) Scope {
	s := Scope{Total: len(rs.Rules)}
	for i := range rs.Rules {
		if enforced(rs.Rules[i].Severity) {
			s.Enforced++
		}
	}
	return s
}
