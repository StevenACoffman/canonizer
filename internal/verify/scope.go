package verify

import (
	"github.com/StevenACoffman/skillet/markdown"
	"github.com/StevenACoffman/skillet/ruleset"
)

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

	// Symbolic is how many enforced rules name a symbol a checker can see -- a code span,
	// a link, or one of the identifier shapes concrete recognises in bare prose.
	//
	// **This was a per-rule finding until 2026-09-08 and is a count because the finding's
	// claim was false.** As `unspecific` it fired on 63 of 147 enforced rules asserting each
	// "names no object, tool or API", while one of them named GoMock and another named a
	// filter struct parameter. Zero of the flagged rules carried a code span against 94% of
	// those that passed, so what it measured was backtick-convention adherence: a true
	// property of a document and a false accusation against a rule.
	//
	// As a proportion it answers a question nothing else could: the same prompt distilling
	// eight sources produced rates from 9% to 78%, so a specificity reading is only as
	// comparable as the typography of the run that produced it.
	Symbolic int

	// SectionOnly is how many enforced rules carry an anchor naming a section and nothing
	// else, so no substring search could confirm or refute their provenance.
	//
	// Named for sectionOnly, which decides it, so a reader can trace the count to the rule.
	//
	// Counted because the line above over-reported: such a rule *was* examined by the
	// executability gates and *was not* examined for provenance, and "examined 9 of 11"
	// could not say so. Provenance already reports each one as an advisory; what was missing
	// was the total.
	SectionOnly int
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
// It parses each enforced rule's statement as markdown, which it did not have to do when it
// only counted severities. That is one pass rather than a second function walking the same
// slice, which is the trade §4 prefers -- and it stays pure, since markdown.Parse reads
// nothing outside the string it is given.
//
// Requires: nothing; rs may hold no rules.
// Ensures:  Enforced <= Total; Symbolic <= Enforced; SectionOnly <= Enforced;
//
//	Total == len(rs.Rules); it is pure and reads no I/O.
func Rules(rs *ruleset.Ruleset) Scope {
	s := Scope{Total: len(rs.Rules)}
	for i := range rs.Rules {
		r := &rs.Rules[i]
		if !enforced(r.Severity) {
			continue
		}
		s.Enforced++
		if concrete(r.Statement, markdown.Parse(r.Statement)) {
			s.Symbolic++
		}
		if sectionOnly(r.SourceAnchor) {
			s.SectionOnly++
		}
	}
	return s
}
