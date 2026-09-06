package verify

import (
	"strconv"

	"github.com/StevenACoffman/skillet/finding"
	"github.com/StevenACoffman/skillet/quotecheck"
	"github.com/StevenACoffman/skillet/ruleset"
)

// Unverifiable reports the enforced rules whose anchors nothing searched for.
//
// This is the third provenance state, and the one canonizer hits first: a rule distilled
// from a book or a PDF has a source that cannot be handed to `--source` as text, so the
// normal case is that no search happens at all. It is not `fabricated` -- collapsing it
// there would block every rule drawn from a PDF, which is most of go-advice -- and it is
// not a pass, which would let a fabricated anchor through whenever the source happens to be
// unavailable. It is its own state, and it reports without blocking.
//
// **It is not SourceUnknown, and the two must not be conflated.** SourceUnknown means no
// *proof packet* was supplied while the source itself is readable, so an absent anchor is
// still a real finding and still blocks. This is the case where there is nothing to search:
// the anchor is neither present nor absent, because no one looked.
//
// The vocabulary is skillet's `quotecheck.Status`, whose `Unchecked` is exactly this state
// and is deliberately the zero value -- "nobody looked" is not the claim that this is fine.
// Reusing it rather than declaring a local tri-state is what stops canonizer becoming the
// third implementation of a distinction this family already unified, the same call
// Specificity makes by taking skilllens's softening vocabulary instead of a word list.
//
// One diagnostic for the ruleset rather than one per rule: the repair is a single action --
// supply a source -- so a line per rule would repeat one instruction N times, and the count
// belongs in the message where a reader can weigh it.
//
// Requires: nothing.
// Ensures:  at most one diagnostic, always finding.SeverityWarning and never blocking; nil
//
//	when no enforced rule cites an anchor, since nothing then went unverified;
//	it is pure.
func Unverifiable(rs ruleset.Ruleset) []finding.Diagnostic {
	anchored := 0
	for i := range rs.Rules {
		r := &rs.Rules[i]
		if enforced(r.Severity) && r.SourceAnchor != "" {
			anchored++
		}
	}
	if anchored == 0 {
		return nil
	}
	return []finding.Diagnostic{{
		Severity: finding.SeverityWarning,
		// Human, as the state implies: the next move is to find a quotable source, not to
		// rewrite a rule that may be perfectly sound.
		Action:   finding.ActionHuman,
		Category: CategoryAnchorUnverifiable,
		Path:     "ruleset",
		Message: "no source was supplied, so " + strconv.Itoa(anchored) +
			" enforced rule(s) cite an anchor " + quotecheck.Unchecked.String() +
			" against anything; absent here means nobody looked, not that the rule is sound",
	}}
}
