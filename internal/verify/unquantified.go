package verify

import (
	"regexp"
	"strings"

	"github.com/StevenACoffman/skillet/finding"
	"github.com/StevenACoffman/skillet/ruleset"
)

// unquantifiedTerms is the vocabulary of comparative and threshold hedges: words that make a
// rule depend on an amount it never states.
//
// **A different axis from the discretion phrases Softening reads, which is why this is a
// second check and not a widening of that one.** A discretion phrase says *you may choose*
// -- "as appropriate", "it depends", "at your discretion". These say *some unstated amount*.
// A rule reading "split when a file gets large" commits to an action and refuses to say
// when, and it contains no discretion phrase at all. Measured on batch two: the two
// vocabularies overlap on **zero** of 137 enforced rules.
//
// **A wider list was written first and rejected on measurement.** It also held `large`,
// `small`, `simple`, `complex`, `appropriate`, `reasonable`, `sufficient` and `adequate`,
// and its two extra hits were both wrong: *"Start from a small set of generic error codes --
// `ECONFLICT`..."* and *"Cut assertion verbosity with a small set of helpers -- `assert(tb
// ...)`"*. Both name concrete symbols and are perfectly actionable; "small set" is ordinary
// English rather than an unstated threshold. Precision is what an advisory is worth, so the
// eight that were right are kept and the eight that were not are gone.
//
// A function rather than package-level vars, the shape identifierPatterns uses here and the
// one gochecknoglobals leaves alone.
func unquantifiedTerms() []string {
	return []string{
		"closely", "roughly", "approximately", "enough",
		"too many", "importance", "significant", "substantial",
	}
}

// unquantifiedPattern matches any term in the vocabulary on a word boundary.
//
// Case-insensitive because a hedge at the start of a statement is capitalised, and bounded
// because `importance` must not fire on a rule about an `important` decision -- that word was
// in the rejected list and its removal is only real if the match cannot reach it.
func unquantifiedPattern() *regexp.Regexp {
	return regexp.MustCompile(`(?i)\b(` + strings.Join(unquantifiedTerms(), "|") + `)\b`)
}

// Unquantified returns an advisory diagnostic for every enforced rule that commits to an
// action while leaving the amount it depends on unstated.
//
// It is the half of the retired `unspecific` check that was true. That check claimed a
// statement "names no object, tool or API", fired on 63 of 147 rules, and was false on
// essentially all of them; what separated the few genuinely soft rules was hedging on a
// threshold, and this reads that. All four rules the retiring entry defended as genuinely
// soft -- "closely related", "by importance", "roughly 10K SLOC", "expensive enough" -- are
// caught here.
//
// **Five of 137 enforced rules on batch two, and that is not the 12 the proposal recorded.**
// The 12 was measured on batch one, which was deleted before this shipped, so the number is
// a record rather than something re-checkable and this is not confirmation of it. What
// justifies building on 5 is precision rather than volume: all five are the defended class,
// and for an advisory a reader pays for a wrong one and gains nothing from a check that
// reports little but is right.
//
// **The category is canonizer's, deliberately.** `skilllens.CategorySoftening` is skillet's
// word for skillet's list; emitting it for a vocabulary skillet does not define would make
// these findings claim a provenance they do not have.
//
// **A rule could draw this and a softening finding both, and nothing prevents it.** The old
// Specificity gave one note per rule because doubling inflates a rework budget, and with
// separate functions that guarantee is gone. Measured at zero overlap, so no precedence is
// built -- if overlap appears the question returns, rather than having been answered by a
// mechanism nobody checked.
//
// Ensures: every returned diagnostic has finding.SeverityWarning; it is pure.
func Unquantified(rs *ruleset.Ruleset) []finding.Diagnostic {
	diags := make([]finding.Diagnostic, 0)
	pattern := unquantifiedPattern()
	for i := range rs.Rules {
		r := &rs.Rules[i]
		if !enforced(r.Severity) {
			continue
		}
		if term := pattern.FindString(r.Statement); term != "" {
			diags = append(diags, advisory(r, CategoryUnquantified,
				"statement turns on an unstated amount ("+term+
					"); a reader cannot tell when it is met"))
		}
	}
	return diags
}
