// Package verify runs canonizer's deterministic checks over a parsed ruleset:
// Executable (B) requires each enforced rule to carry a discriminating ✗/✓ pair, and
// Provenance (E) requires each enforced rule to cite a source anchor that appears in
// the source. Both are pure and return skillet/finding diagnostics. The semantic
// judgments — does a pair actually flip a verdict, does an anchor support the claim —
// stay the cold critic's job; this is the deterministic floor beneath them.
package verify

import (
	"strings"

	"github.com/StevenACoffman/skillet/finding"
	"github.com/StevenACoffman/skillet/judge"
	"github.com/StevenACoffman/skillet/markdown"
	"github.com/StevenACoffman/skillet/ruleset"
	"github.com/StevenACoffman/skillet/ruleset/conflict"
	"github.com/StevenACoffman/skillet/skilllens"
	"github.com/StevenACoffman/skillet/textnorm"
	errors "github.com/StevenACoffman/toerr/errors"
)

// The categories this package reports, as one enumerable set.
//
// They were string literals at each emit site until 2026-09-05. Naming them is not
// tidying: finding.Category is a bare string, so a typo in a literal produced a category no
// consumer matched and nothing failed -- the diagnostic simply stopped being recognised. As
// constants a typo is a compile error, and the vocabulary can be read in one place instead
// of grepped out of five.
//
// Deliberately constants and not a named type. Whether finding.Category should become
// registrable is a question recorded against skillet, and typing it here would be canonizer
// answering a kernel question for the family on its own.
//
// skilllens.CategorySoftening is referenced where it is emitted rather than restated here,
// for the same reason one spelling matters: skillsaw and adh score that category too, and a
// copy in canonizer would be a second definition of a shared word.
const (
	// CategoryUnexecutable is a rule carrying no discriminating ✗/✓ pair at all.
	CategoryUnexecutable = "unexecutable"
	// CategoryNonDiscriminating is a pair whose ✓ appears inside its ✗, showing no change.
	CategoryNonDiscriminating = "non-discriminating"
	// CategoryNoAnchor is an enforced rule citing no source anchor.
	CategoryNoAnchor = "no-anchor"
	// CategoryAnchorAbsent is an anchor that does not appear in the source.
	CategoryAnchorAbsent = "anchor-absent"
	// CategoryUnspecific is a statement naming no object a reader could act on.
	CategoryUnspecific = "unspecific"
	// CategoryNothingExamined is a ruleset in which no rule is enforced, so the gates
	// examined nothing. It reports on the run rather than on any rule, which is why its
	// Path is the ruleset and not a section.
	CategoryNothingExamined = "nothing-examined"
	// CategoryNonCanonical is a stored ruleset that does not round-trip through Render.
	CategoryNonCanonical = "non-canonical"
	// CategoryAnchorDrift is an anchor still present in a source that has since changed.
	CategoryAnchorDrift = "anchor-drift"
	// CategoryAnchorStale is an anchor absent from a source that has since changed, so
	// whether it was ever there cannot be decided from these bytes.
	CategoryAnchorStale = "anchor-stale"
	// CategoryAnchorFabricated is an anchor absent from the unchanged source it was
	// distilled from -- the one case a missing anchor is certainly a defect.
	CategoryAnchorFabricated = "anchor-fabricated"
	// CategoryUnbounded is a ruleset whose scope states no exclusion, so nothing says
	// where its rules stop applying.
	CategoryUnbounded = "unbounded"
	// CategoryWarrantIncomplete is an adjudicated rule whose warrant does not record all
	// of who, when and why.
	CategoryWarrantIncomplete = "warrant-incomplete"
	// CategoryAnchorUnverifiable is an anchor nothing searched for, because no source was
	// supplied. Distinct from anchor-absent, which means a search ran and found nothing.
	CategoryAnchorUnverifiable = "anchor-unverifiable"
)

// Executable returns a diagnostic for every enforced rule that lacks a discriminating
// ✗/✓ pair (B): one with no ✗ or no ✓, or one whose ✓ preferred form appears verbatim
// inside its ✗ counter-example (so the pair demonstrates no change). It scores the
// pair with skillet/judge; whether the pair semantically flips a verdict is the
// critic's judgment, not this gate's.
func Executable(rs ruleset.Ruleset) ([]finding.Diagnostic, error) {
	diags := make([]finding.Diagnostic, 0)
	for i := range rs.Rules {
		r := &rs.Rules[i]
		if !enforced(r.Severity) {
			continue
		}
		if r.Bad == "" || r.Good == "" {
			diags = append(
				diags,
				diag(r, CategoryUnexecutable, "rule has no discriminating ✗/✓ pair"),
			)
			continue
		}
		score, err := judge.Score(r.Bad, []judge.Check{{Op: judge.OpContains, Arg: r.Good}})
		if err != nil {
			return nil, errors.WrapWithMessage(err, "verify: judge")
		}
		if score.Hard == 1.0 {
			diags = append(diags, diag(r, CategoryNonDiscriminating,
				"the ✓ form appears inside the ✗ example; the pair shows no change"))
		}
	}
	return diags, nil
}

// Provenance returns a diagnostic for every enforced rule that cites no source anchor,
// or whose anchor is absent from the source (E). The search is whitespace-normalized
// so a quote the model re-wrapped still matches. Whether a present anchor *supports*
// the claim is the critic's `unsupported` judgment.
func Provenance(rs ruleset.Ruleset, source string) []finding.Diagnostic {
	diags := make([]finding.Diagnostic, 0)
	for i := range rs.Rules {
		r := &rs.Rules[i]
		if !enforced(r.Severity) {
			continue
		}
		if r.SourceAnchor == "" {
			if d, ok := unanchored(r); ok {
				diags = append(diags, d)
			}
			continue
		}
		if !anchorPresent(source, r.SourceAnchor) {
			diags = append(
				diags,
				diag(r, CategoryAnchorAbsent, "source anchor is not present in the source"),
			)
		}
	}
	return diags
}

// Specificity returns an advisory diagnostic for every enforced rule whose statement
// reads as general advice rather than something a reader could act on: one that hedges
// with softening language, or that names no concrete object at all.
//
// It is **always advisory**. The severity is fixed here rather than taken as an argument
// so no caller can make it blocking: a general rule is sometimes exactly right, and no
// deterministic check can tell which, so this reports and does not decide. Executable and
// Provenance remain the only checks that stop a ship.
//
// False positives are expected and are not a defect to fix. "Prefer composition over
// inheritance" names nothing concrete and is a good rule; it will be flagged, and a reader
// will dismiss it in a second. Making the check quieter by blocking on it instead would
// trade a cheap false alarm for an expensive false stop.
//
// Both signals come from skillet rather than being detected here. The softening
// vocabulary is skilllens's -- the same one skillsaw and adh score -- and the concreteness
// signal is markdown's Links, which carries code-span contents as well as link targets, so
// a rule naming a tool or symbol in backticks has one. A local heuristic would make
// canonizer the third independent implementation of a rubric that was just unified.
//
// Ensures: every returned diagnostic has finding.SeverityWarning; it is pure.
func Specificity(rs ruleset.Ruleset) []finding.Diagnostic {
	diags := make([]finding.Diagnostic, 0)
	for i := range rs.Rules {
		r := &rs.Rules[i]
		if !enforced(r.Severity) {
			continue
		}
		doc := markdown.Parse(r.Statement)
		if hedges := skilllens.SofteningPhrases(doc); len(hedges) > 0 {
			diags = append(diags, advisory(r, skilllens.CategorySoftening,
				"statement hedges ("+hedges[0].Text+"); a reader cannot tell when it applies"))
			continue
		}
		if len(doc.Links) == 0 {
			diags = append(diags, advisory(r, CategoryUnspecific,
				"statement names no object, tool or API a reader could act on"))
		}
	}
	return diags
}

// advisory builds a warning-severity diagnostic located at rule r. Separate from diag
// because the severity is the point: these must never reach gate.Blocking.
//
// Action is guided rather than automatic: a tool can propose concrete wording for a softened
// or unspecific rule, but only a person can confirm the rewrite is still true of the source.
func advisory(r *ruleset.Rule, category, message string) finding.Diagnostic {
	return finding.Diagnostic{
		Severity: finding.SeverityWarning,
		Action:   finding.ActionGuided,
		Category: category,
		Path:     "§" + r.Section,
		Message:  message,
	}
}

// unanchored returns the diagnostic for an enforced rule citing no source anchor, and
// whether there is one.
//
// **A rule with a warrant is sourced differently, not unsourced.** When two rules conflict
// and a person decides, the decision is knowledge present in neither source, so it can carry
// no anchor -- and gating on the anchor alone would reject the highest-value artifact the
// team produces with the check that exists to protect quality. The gate is therefore the
// warrant's presence where the anchor is absent.
//
// An invalid warrant is reported rather than accepted. skillet requires who, when and why,
// and a warrant missing any of them records no decision anyone can review -- which is the
// same failure as no warrant at all, dressed to pass. Present and Valid are separate
// questions in skillet for exactly this reason, and both are asked here.
//
// canonizer's warrant is deliberately smaller than gnosis's: no tiers, no co-signers, no
// reversal links. Those are an authority model this repo bet against, holding that a
// required rationale filters more bad adjudications than a permission check does. Presence
// plus validity is the whole policy.
//
// Ensures: ok is false only for a rule carrying a valid warrant; pure.
func unanchored(r *ruleset.Rule) (finding.Diagnostic, bool) {
	switch {
	case !r.Warrant.Present():
		return diag(r, CategoryNoAnchor, "rule cites no source anchor"), true
	case !r.Warrant.Valid():
		return diag(r, CategoryWarrantIncomplete,
			"rule is adjudicated but its warrant does not record who, when and why; "+
				"an unreviewable decision is not a source"), true
	default:
		return finding.Diagnostic{}, false
	}
}

// anchorPresent reports whether anchor appears in source.
//
// Whitespace-normalized on both sides so a quote the model re-wrapped still matches. It is
// one function rather than a repeated strings.Contains because Provenance and Drift must
// agree on what "present" means -- two spellings of the anchor test is how one check starts
// blocking what the other passes.
func anchorPresent(source, anchor string) bool {
	return strings.Contains(textnorm.Fold(source), textnorm.Fold(anchor))
}

// enforced reports whether a rule's severity is gated. MUST/SHOULD are enforced;
// CONSIDER is advisory and exempt from both checks.
func enforced(sev ruleset.Severity) bool {
	return sev == ruleset.MUST || sev == ruleset.SHOULD
}

// diag builds an error-severity diagnostic located at rule r.
//
// Action is human for all of these, and nothing canonizer emits is automatic. Every blocking
// category needs someone who knows what the source says: an unexecutable rule needs
// rewriting, a missing anchor needs the passage found, and an absent one needs deciding
// whether the source moved or the rule was fabricated. Claiming a tool could close them
// unattended is the misclassification this axis exists to prevent.
func diag(r *ruleset.Rule, category, message string) finding.Diagnostic {
	return finding.Diagnostic{
		Severity: finding.SeverityError,
		Action:   finding.ActionHuman,
		Category: category,
		Path:     "§" + r.Section,
		Message:  message,
	}
}

// Conflicts reports decidable inconsistencies between rules -- the same statement asserted
// at two severities or two levels, or one section claimed twice -- as warnings.
//
// Advisory for the same reason Specificity is: a severity divergence may be a deliberate
// refinement of a general rule, and a deterministic check cannot tell that from a genuine
// contradiction. It reports; the cold critic and a person decide.
//
// The detection itself is skillet's ruleset/conflict, which returns diagnostics with no
// severity precisely so this decision is made here. Action is guided: a tool can propose
// which of two divergent rules to keep, but only a person can say which is right.
func Conflicts(rs ruleset.Ruleset) []finding.Diagnostic {
	found := conflict.Find(rs)
	out := make([]finding.Diagnostic, 0, len(found))
	for i := range found {
		d := found[i]
		d.Severity = finding.SeverityWarning
		d.Action = finding.ActionGuided
		// skillet emits the bare section; the "§" is this repo's presentation convention,
		// applied here so one verify run does not mix "2.4" and "§2.4" in its output.
		d.Path = "§" + d.Path
		out = append(out, d)
	}
	return out
}
