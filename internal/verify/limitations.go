package verify

import (
	"strings"

	"github.com/StevenACoffman/skillet/finding"
	"github.com/StevenACoffman/skillet/ruleset"
)

// Limitations reports whether rs says what it does not cover.
//
// VAC makes a bundle without explicit non-claims invalid, on the grounds that "a capability
// statement that will not say what it does not cover is an advertisement". A ruleset
// distilled from one book and presented without that book's bounds reads as rules for the
// whole subject.
//
// **This used to read the Scope: line for a negation word, and that guess is gone.** The
// canonical form had no place to state limits, so the check asked whether the field that did
// exist happened to mention an exclusion -- a word list standing in for a datum. skillet
// v0.30.0 added the Limitations: header, so the question can be asked of the field that
// answers it. Keeping the word list as a fallback would be worse than deleting it: a ruleset
// with an empty Limitations: would still pass because its scope said "only", the guess
// silently overriding the fact.
//
// **Nothing here judges whether the stated limits are any good.** `Limitations: none`
// satisfies the field and states nothing, and detecting that would be the same word list in
// a new place. Presence is what a deterministic check can honestly assert; whether the limits
// are honest is the cold critic's, like every other question about meaning here.
//
// **Advisory, and the reason has changed rather than disappeared.** It used to be advisory
// because the detection was a guess. It is advisory now because no ruleset in the corpus
// carries the header yet, so blocking would fail every one of them on the day it ships --
// and a gate that turns a whole corpus red teaches people to bypass the gate. That reason
// expires once the corpus is migrated, which is what would make this blocking.
//
// Ensures: at most one diagnostic, always finding.SeverityWarning; it is pure.
func Limitations(rs *ruleset.Ruleset) []finding.Diagnostic {
	if strings.TrimSpace(rs.Limitations) != "" {
		return nil
	}
	return []finding.Diagnostic{{
		Severity: finding.SeverityWarning,
		Action:   finding.ActionHuman,
		Category: CategoryUnbounded,
		Path:     "ruleset",
		Message: "ruleset states no Limitations:, so nothing says where these rules stop " +
			"applying; a rule set that will not say what it excludes reads as rules for " +
			"the whole subject",
	}}
}
