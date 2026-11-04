package verify

import (
	"github.com/StevenACoffman/skillet/finding"
	"github.com/StevenACoffman/skillet/ruleset"
)

const (
	// SourceUnknown means no second signal was supplied, so only the anchor is known.
	SourceUnknown SourceState = iota
	// SourceUnchanged means the source hashes to the digest recorded at distill time.
	SourceUnchanged
	// SourceChanged means the source no longer hashes to the recorded digest.
	SourceChanged
)

// SourceState is what a second signal says about the source a ruleset was distilled from:
// whether its bytes are the ones the anchors were validated against.
type SourceState int

// Drift returns the provenance diagnostics for rs, crossing each rule's anchor with what
// state says about the source.
//
// One signal cannot tell a fabrication from a drift, and Provenance conflates them: it
// emits anchor-absent whether the model invented the anchor -- a real defect, block always
// -- or the source moved under it, where the rule may be entirely sound and only its anchor
// needs refreshing. The response to a routine source update is then indistinguishable from
// the response to a hallucination.
//
// Splitting them was filed as needing an immutable evidence archive. It does not: crossing
// **two** signals gets the full split with no archive, which is ruflo's witness-manifest
// design. canonizer already has the better marker, because r.SourceAnchor was chosen by
// whoever wrote the rule rather than by whoever wrote the check; the missing half was the
// source's digest, and `verify --proof` has been recording it in a proof packet all along.
//
//	anchor   source    verdict
//	present  unchanged pass
//	present  changed   drift     -- advisory; the anchor survived the edit
//	absent   unchanged fabricated -- blocking; these bytes never contained it
//	absent   changed   stale     -- advisory; the anchor did not survive an edit
//
// The third row is the only certain defect, and it is the one a single signal could not
// isolate. With SourceUnknown this falls back to Provenance's verdict exactly, because with
// one signal anchor-absent remains the honest answer.
//
// Ensures: one provenance diagnostic at most per enforced rule, plus Escalation's, which
//
//	asks about the rule's level rather than its anchor; it is pure.
func Drift(rs *ruleset.Ruleset, sources []string, state SourceState) []finding.Diagnostic {
	if state == SourceUnknown {
		return Provenance(rs, sources)
	}
	diags := make([]finding.Diagnostic, 0)
	indexed := indexSources(sources)
	for i := range rs.Rules {
		r := &rs.Rules[i]
		if !enforced(r.Severity) {
			continue
		}
		if d, ok := driftOne(r, sources, indexed, state); ok {
			diags = append(diags, d)
		}
	}
	// Provenance carries this on the SourceUnknown path above; appended here so a proof
	// packet does not change which checks run.
	return append(diags, Escalation(rs, sources)...)
}

// driftOne returns the diagnostic for one rule, and whether there is one.
func driftOne(
	r *ruleset.Rule, sources []string, indexed []indexedSource, state SourceState,
) (finding.Diagnostic, bool) {
	if r.SourceAnchor == "" {
		// No anchor at all is unaffected by what the source did: nothing was ever cited,
		// so there is nothing a source edit could have invalidated. Routed through the
		// same unanchored() as Provenance so the warrant policy has one definition -- two
		// spellings of it is how one check starts blocking what the other passes, which is
		// the trap anchorPresent was factored out to avoid.
		return unanchored(r)
	}
	if d, ok := sectionOnlyDiagnostic(r, sources); ok {
		// Asked before presence, because "is it in the source" is meaningless when there
		// is no text to look for. Same ordering gate uses for coverage before verdict, and
		// routed through the same function as Provenance so the two cannot disagree about
		// what a section reference is or what is said about it.
		return d, true
	}
	if anchorPresent(indexed, r.SourceAnchor) {
		if state == SourceChanged {
			// One finding per rule, and this one already says the citation may need
			// refreshing, which covers a section name the edit moved the quotation out of.
			return advisory(r, CategoryAnchorDrift,
				"source changed since distillation but the anchor still appears in it; "+
					"the rule is intact and its citation may need refreshing"), true
		}
		return sectionPlacementDiagnostic(r, indexed)
	}
	if state == SourceChanged {
		return advisory(r, CategoryAnchorStale,
			"anchor is absent and the source changed since distillation; "+
				"re-locate the passage rather than assuming the rule was invented"), true
	}
	return diag(r, CategoryAnchorFabricated,
		"anchor is absent from the source bytes it was distilled from; "+
			"nothing here was ever cited, so the rule is unsupported"), true
}
