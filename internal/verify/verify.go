// Package verify runs canonizer's deterministic checks over a parsed ruleset:
// Executable (B) requires each enforced rule to carry a discriminating ✗/✓ pair, and
// Provenance (E) requires each enforced rule to cite a source anchor that appears in
// the source. Both are pure and return skillet/finding diagnostics. The semantic
// judgments — does a pair actually flip a verdict, does an anchor support the claim —
// stay the cold critic's job; this is the deterministic floor beneath them.
package verify

import (
	"regexp"
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
	// CategoryUnsound is a rule whose checks do not discriminate between its own ✗ and ✓.
	CategoryUnsound = "unsound"
	// CategoryUnquantified is a statement that turns on an amount it never states.
	CategoryUnquantified = "unquantified"
	// CategoryAnchorSectionOnly is an anchor that names a section and says nothing else,
	// so there is no text to search the source for.
	CategoryAnchorSectionOnly = "anchor-section-only"
)

// elision is the mark a quoter puts where words were skipped, and the trailing space is
// load-bearing.
//
// The prompt asks for "... " and the corpus obeys: of 31 ellipses in its anchors, the 30
// that mark an elision are followed by whitespace and the one that is not is Go variadic
// syntax inside a code span -- `tx.QueryContext(ctx, ...)`. Splitting on a bare "..." would
// cut that into `tx.QueryContext(ctx,` and `)`, and a lone `)` is in every source, so a code
// anchor could pass on a fragment carrying no evidence at all.
//
// Requiring the space costs nothing measured -- both spellings score 117 of 162 on today's
// corpus -- so this closes a failure channel rather than buying anchors. It is spelled out
// because a reader comparing the two would otherwise see a distinction with no measured
// difference and simplify it away.
//
// Safe as a plain string because textnorm.Fold collapses whitespace runs to a single space
// and rewrites U+2026 to three dots before any of this runs, so both "…" and an ellipsis
// broken across a line arrive here in this one spelling.
//
// The space is written as an escape for the reason skillet writes its own space-like folds
// that way: a literal trailing space in source is invisible, so a later reader could neither
// tell it is part of the separator nor notice if it were silently edited away.
const elision = "...\u0020"

// Executable returns a diagnostic for every enforced rule that lacks a discriminating
// ✗/✓ pair (B): one with no ✗ or no ✓, or one whose ✓ preferred form appears verbatim
// inside its ✗ counter-example (so the pair demonstrates no change). It scores the
// pair with skillet/judge; whether the pair semantically flips a verdict is the
// critic's judgment, not this gate's.
func Executable(rs *ruleset.Ruleset) ([]finding.Diagnostic, error) {
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
func Provenance(rs *ruleset.Ruleset, source string) []finding.Diagnostic {
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
		if sectionOnly(r.SourceAnchor) {
			diags = append(diags, advisory(r, CategoryAnchorSectionOnly,
				"anchor names a section and quotes nothing, so provenance cannot be "+
					"confirmed or refuted from the source text"))
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

// Softening returns an advisory diagnostic for every enforced rule whose statement hedges,
// so that a reader cannot tell when it applies.
//
// It is **always advisory**. The severity is fixed here rather than taken as an argument
// so no caller can make it blocking: a hedged rule is sometimes exactly right, and no
// deterministic check can tell which, so this reports and does not decide. Executable and
// Provenance remain the only checks that stop a ship.
//
// The vocabulary is skilllens's -- the same one skillsaw and adh score -- rather than a word
// list here, which would make canonizer a third independent implementation of a rubric that
// was just unified.
//
// **It was Specificity until 2026-09-08, and it reported a second thing that is now a
// count.** That signal claimed a statement "names no object, tool or API a reader could act
// on", and on the eight-ruleset corpus it fired on 63 of 147 enforced rules while the claim
// was false on essentially all of them: one flagged rule names GoMock, another names a
// filter struct parameter. What separated the few genuinely soft rules was hedging on a
// threshold -- which is this function, and the reason it kept the half that was true.
// The per-rule question belongs to the cold critic, whose `vague` test asks it in the same
// words at blocking severity; what survives here is the proportion, in Scope.Symbolic.
//
// Ensures: every returned diagnostic has finding.SeverityWarning; it is pure.
func Softening(rs *ruleset.Ruleset) []finding.Diagnostic {
	diags := make([]finding.Diagnostic, 0)
	for i := range rs.Rules {
		r := &rs.Rules[i]
		if !enforced(r.Severity) {
			continue
		}
		if hedges := skilllens.SofteningPhrases(markdown.Parse(r.Statement)); len(hedges) > 0 {
			diags = append(diags, advisory(r, skilllens.CategorySoftening,
				"statement hedges ("+hedges[0].Text+"); a reader cannot tell when it applies"))
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
//
// **It matches the quoted span, not the whole anchor, and matching the whole anchor was a
// defect.** The form the distill prompt asks for -- and its own worked example writes -- is
// `§Section: "the quote"`, so a correct anchor carries a section prefix that is *about* the
// source rather than *from* it. Searching for the whole string therefore failed on every
// anchor ever written: measured on the first real ruleset, 0 of 26 whole anchors were
// present while 11 of the same 26 quoted the article verbatim. The check was reporting a
// fabrication rate of 100% against a ruleset whose anchors were largely sound.
//
// **A quotation may be elided, and until this split it the eliding was punished.** A quoter
// who joins two spans of one passage with "... " is being careful, yet the joined string
// matched nothing, so a faithful quotation drew the verdict an invention gets. Measured on
// the corpus: 20 of 65 absent anchors are exactly this shape, the largest single class and
// every one of them sound.
//
// The conjunction is what makes that safe to accept. Each span must be in the source on its
// own, so a fabricator still has to fabricate text the source contains -- twice. It is a
// weaker claim than one long span found whole, and the weakness is named rather than left
// for a reader to notice: fragments may come from anywhere in the source, and their **order
// is not checked**, so "B ... A" passes against a source reading "A ... B". Carrying an
// offset between fragments would close that, and no anchor in the corpus exhibits it; the
// gap is priced and left open.
//
// Ensures: false when no non-empty fragment survives, so an anchor of nothing but elisions
//
//	cannot pass on an empty conjunction -- and neither can an empty quotation, which the
//	single strings.Contains this replaced would have reported as present; it is pure.
func anchorPresent(source, anchor string) bool {
	folded := unemphasize(textnorm.Fold(source))
	found := 0
	quoted := unemphasize(textnorm.Fold(anchorText(anchor)))
	for _, fragment := range strings.Split(quoted, elision) {
		trimmed := strings.TrimSpace(fragment)
		if trimmed == "" {
			// An elision at either end, or two in a row, yields an empty fragment.
			// Skipping rather than failing keeps the conjunction to the spans actually
			// quoted; the counter is what stops every fragment being skipped.
			continue
		}
		if !strings.Contains(folded, trimmed) {
			return false
		}
		found++
	}
	return found > 0
}

// sectionOnly reports whether an anchor names a section and says nothing else.
//
// The distill prompt permits *"a short quote or section reference"*, and a bare section
// reference -- `§Transactional boundaries` -- cannot be found in the source by any substring
// search. Reporting it as anchor-absent gives it the verdict a fabricated quotation gets,
// which is the conflation Drift exists to undo appearing one layer down: *searched and not
// found* and *nothing to search for* are different answers and only one is a defect.
//
// **The question is deliberately "does it say anything beyond the section name", not "does
// it carry a quotation".** The second was written first and is wrong twice over. A bare
// quotation with no quote marks would be classified as unsearchable and stop being checked,
// silently undoing the fallback anchorText preserves. And `§Errors: every method takes ctx
// first` carries no quotation while being a *paraphrase* rather than a section reference --
// eight of the measured ruleset's anchors are that shape, and they are the ruleset's own
// defect. Keying on quote marks would turn every one of them into an advisory.
//
// So a section-only anchor is one whose section token is all there is: `§Errors`, `§4.2`.
// Anything after it -- quoted or not -- is content the gate can and should search for.
//
// **The residual, named rather than closed**: a paraphrase that is colonless *and* quotes
// nothing would still read as a section name. The corpus holds **zero** such anchors -- of
// 157, 155 are `§name: content` and one is the quoted case above -- so no length or
// word-count bound is guessed here. A real section heading is short and a paraphrase is not,
// but that is a threshold, and this package has refused thresholds without a corpus before.
//
// Ensures: false for any anchor carrying a quotation or trailing prose; false for an empty
//
//	anchor, which Provenance reports as no-anchor before reaching here; it is pure.
func sectionOnly(anchor string) bool {
	rest, found := strings.CutPrefix(strings.TrimSpace(anchor), "§")
	if !found {
		return false
	}
	// An anchor holding a quotable span has something to search for, whatever its
	// punctuation, so it is not section-only regardless of the colon test below.
	//
	// This is a *conjunct*, and the distinction from the predicate rejected when this
	// shipped is the whole reason it is safe. That one was "section-only means it carries no
	// quotation", which would have made every colon-bearing paraphrase advisory. This asks
	// both questions, so the rule is strictly narrower than before and nothing reported as a
	// defect today becomes advisory.
	//
	// It exists because the colon test has a hole the corpus found: a paraphrase written
	// without a colon reads as one long section name. The instance was
	// "§3 `NewTestDB` opening a real Bolt database, contrasted with §4 `TestYoClient`",
	// which describes the source rather than pointing at it and carries two code spans to
	// search for.
	if firstDelimited(anchor, '"') != "" || firstDelimited(anchor, '`') != "" {
		return false
	}
	// The colon is the separator the form uses -- `§Errors: "quote"` -- so content after it
	// is what the gate should search, and its absence is what makes an anchor a bare
	// section reference. Splitting on whitespace instead would reject `§Transactional
	// boundaries`, since a section name may be several words.
	name, content, hasColon := strings.Cut(rest, ":")
	if strings.TrimSpace(name) == "" {
		return false
	}
	return !hasColon || strings.TrimSpace(content) == ""
}

// anchorText returns the part of an anchor that should be found in the source: the quoted
// span when there is one, and otherwise the anchor as written.
//
// The fallback keeps a bare quotation working -- an anchor that is just the quote, with no
// section prefix, is still what most of the corpus writes -- so this widens what matches
// without narrowing it.
//
// Only the first quoted span is taken. The convention places the quotation last and uses
// the prefix for the section, so a second pair of quotes inside one anchor is a shape
// nothing in the corpus writes; taking the first keeps the rule stateable in one sentence
// rather than guessing which of several spans was meant.
//
// **A backtick span counts as a quotation too, and double quotes win when both are
// present.** An anchor may quote an identifier rather than prose -- the prompt writes that
// form, as in “§Remove dependencies: `FindDialByID(ctx context.Context) (*Dial, error)``` -- and
// reading only double quotes fell back to the whole anchor and searched the section
// prefix along with it, which is the defect the prefix fix corrected surviving in the
// delimiter it did not consider. Measured: four anchors of that shape, three of them
// present in the source and reported absent.
//
// Double quotes win because “§Helper methods: "`defer rows.Close()`"“ nests the
// backticks *inside* the quotation, so the outer delimiter is the one bounding the passage.
//
// Ensures: the result is non-empty whenever anchor is; it is pure.
func anchorText(anchor string) string {
	if quoted := firstDelimited(anchor, '"'); quoted != "" {
		return quoted
	}
	if quoted := firstDelimited(anchor, '`'); quoted != "" {
		return quoted
	}
	return anchor
}

// firstDelimited returns the first non-blank span of s bounded by a pair of delim, or "" if
// there is no such pair.
//
// Extracted rather than written twice: anchorText needs the same scan for two delimiters,
// and two copies of "find the first delimited span" is how the two spellings start
// disagreeing about what a span is. What stays in anchorText is the knowledge of *which*
// delimiter wins, which is the part that is actually a decision.
//
// Ensures: the result is either empty or a non-blank substring of s; it is pure.
func firstDelimited(s string, delim byte) string {
	start := strings.IndexByte(s, delim)
	if start < 0 {
		return ""
	}
	rest := s[start+1:]
	end := strings.IndexByte(rest, delim)
	if end < 0 {
		return ""
	}
	if span := rest[:end]; strings.TrimSpace(span) != "" {
		return span
	}
	return ""
}

// unemphasize removes paired markdown emphasis markers, keeping the text between them.
//
// The sources are markdown and use __bold__; an anchor quotes what a reader sees, so `only`
// in the anchor met `__only__` in the source and missed. Diffing near-miss anchors against
// their closest source window showed this as the largest single systematic cause.
// Measured: 11 anchors, taking the corpus from 42 absent to 31.
//
// Applied to the anchor as well as the source, so an anchor that did quote the markers
// verbatim still matches. Folding one side only would trade one mismatch for another.
//
// **Only paired double markers are folded, and the narrow rule beats the wide one on
// measurement rather than on caution.** A rule folding every marker run scores *three
// anchors worse*, because single-marker italic matches across snake_case: in
// `id IN (SELECT dial_id FROM dial_memberships`, the span `_memberships FROM dial_` is a
// legal `_..._` run. Doubling the marker removes that entire class of false pair.
//
// **Nothing protects a code span, because nothing needs to.** Of 238 code spans in the
// sources, 8 hold a marker character -- `[]*Dial`, `COUNT(*) OVER()`, `"name_asc"`,
// `*myapp.Error` -- and not one is a paired run, so this cannot reach them. A guard would be
// machinery for a case the corpus does not contain, which is the ground a `snake_case`
// pattern was dropped on at zero matches. The residual, stated rather than guarded: a code
// span holding a genuine `__dunder__` would lose its markers, and none exists here.
//
// Local to canonizer rather than widening textnorm.Fold, on the markdown.Links precedent:
// Fold's doc does describe this class of disagreement, but four consumers read it, and what
// wants widening is this repository's question rather than the kernel's datum.
//
// Ensures: the result is never longer than s; it is pure.
func unemphasize(s string) string {
	for _, pattern := range emphasisPatterns() {
		s = pattern.ReplaceAllString(s, "$1")
	}
	return s
}

// emphasisPatterns matches a __bold__ run and a **bold** run, each capturing what it wraps.
//
// **One pattern per delimiter, because RE2 has no backreference.** A single
// `(\*\*|__)(.+?)(\*\*|__)` cannot require the closing marker to match the opening one, so
// it would fold `**text__` -- a mismatched pair that is not emphasis at all. Two patterns
// state the constraint the regexp language cannot.
//
// Built per call rather than kept in a package variable, which is the shape
// identifierPatterns uses in this package and what gochecknoglobals requires.
func emphasisPatterns() []*regexp.Regexp {
	return []*regexp.Regexp{
		regexp.MustCompile(`\*\*(.+?)\*\*`),
		regexp.MustCompile(`__(.+?)__`),
	}
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
func Conflicts(rs *ruleset.Ruleset) []finding.Diagnostic {
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
