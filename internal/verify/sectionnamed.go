package verify

import (
	"regexp"
	"strings"

	"github.com/StevenACoffman/skillet/finding"
	"github.com/StevenACoffman/skillet/markdown"
	"github.com/StevenACoffman/skillet/ruleset"
	"github.com/StevenACoffman/skillet/textnorm"
)

// sectionOnlyMessage is what a section-only anchor naming a real heading is told.
const sectionOnlyMessage = "anchor names a section and quotes nothing, so provenance cannot " +
	"be confirmed or refuted from the source text"

// escapePattern matches a markdown backslash escape, which a heading's raw text keeps.
//
// Built per call rather than kept in a package variable, the shape this package uses for a
// pattern and the one gochecknoglobals leaves alone.
func escapePattern() *regexp.Regexp { return regexp.MustCompile(`\\(.)`) }

// headingKey reduces a heading or an anchor's section name to the form the two are compared
// in: markdown escapes resolved, typography folded, case ignored.
//
// **This normalisation is the whole check, and its specification is a bug I made.** A first
// version of this comparison, written as a one-off script, reported two anchors as naming
// sections that did not exist. They existed: the source writes `## 2\. Use the "underscore
// test" package`, and markdown.Section keeps that backslash because Title is the raw source
// text. A checker that reports fabrication needs to be right about what the document says,
// so the escape strip is first and the case that caught it is a test.
//
// textnorm.Fold does the rest -- curly quotes, dashes, whitespace runs -- for the same
// reason anchorPresent uses it: a heading and a quotation of it disagree about typography
// more often than about words.
//
// **Numbering is deliberately kept.** Stripping a leading `#3.` would let the one real
// mismatch in the corpus pass, and would also let an anchor citing section 2 while meaning
// section 3 match a heading it does not name -- a worse error than the slip it excuses.
//
// Ensures: it is pure.
func headingKey(s string) string {
	return strings.ToLower(textnorm.Fold(escapePattern().ReplaceAllString(s, "$1")))
}

// sectionName returns the section an anchor names, or "" if it names none.
//
// Extracted because sectionOnly and sectionOnlyDiagnostic must agree on where a section name ends,
// and they did not: sectionOnly cuts at the colon, so it reads `§Errors:` as naming
// "Errors", while a second spelling here read the name as "Errors:" and reported a heading
// that exists as missing. Two spellings of one question is how one check starts contradicting
// another, which is why anchorPresent was factored out for the same reason.
//
// Ensures: the result is trimmed and never contains a colon; it is pure.
func sectionName(anchor string) string {
	rest, found := strings.CutPrefix(strings.TrimSpace(anchor), "§")
	if !found {
		return ""
	}
	name, _, _ := strings.Cut(rest, ":")
	return strings.TrimSpace(name)
}

// wholeSectionName returns everything an anchor writes after its `§`, trimmed, or "" if it
// has no `§`.
//
// sectionName's reading is cut at the first colon, which is right for the paraphrase shape
// `§Errors: every method takes ctx first` and wrong for a heading that carries a colon of its
// own: `§Datalog: Recursive Relational Queries` names that whole heading. Only the source can
// tell the two apart, so this is the reading headingOnly tests against it.
//
// Ensures: the result never starts with `§`; it is pure.
func wholeSectionName(anchor string) string {
	rest, found := strings.CutPrefix(strings.TrimSpace(anchor), "§")
	if !found {
		return ""
	}
	return strings.TrimSpace(rest)
}

// namesHeading reports whether name is a heading in any of sources.
//
// The headings come from markdown.Sections rather than a scan for `#` lines here. A second
// parser would disagree with the kernel's on setext headings and on `#` inside fenced code,
// and canonizer would then report a fabrication that skillet's own reader does not see.
//
// Ensures: false when name is ""; it is pure.
func namesHeading(name string, sources []string) bool {
	if name == "" {
		return false
	}
	want := headingKey(name)
	for i := range sources {
		for _, s := range markdown.Parse(sources[i]).Sections {
			if headingKey(s.Title) == want {
				return true
			}
		}
	}
	return false
}

// headingOnly reports whether an anchor is, as written, the name of a heading in a source.
//
// It exists for the case sectionOnly cannot see. sectionOnly reads anything after a colon as
// a paraphrase to search for, so an anchor citing a colon-bearing heading whole was searched
// as text, `§` included, and reported as a blocking anchor-absent against a source that has
// that very heading. The corpus held no such anchor when this was written; the shape is the
// natural citation of `## Datalog: Recursive Relational Queries`, so it was fixed before one
// appeared.
//
// **It asks the source, which sectionOnly deliberately does not.** That keeps sectionOnly a
// question about the anchor's text, as Scope needs it, and confines the source-dependent
// reading here. An anchor carrying a quotation is never heading-only: it has text to search.
//
// Ensures: false for an anchor without `§`; it is pure.
func headingOnly(anchor string, sources []string) bool {
	if firstDelimited(anchor, '"') != "" || firstDelimited(anchor, '`') != "" {
		return false
	}
	return namesHeading(wholeSectionName(anchor), sources)
}

// sectionOnlyDiagnostic returns the finding for an anchor that names a section and quotes
// nothing, and whether the anchor is that shape at all.
//
// Provenance and Drift both call this, so the two cannot disagree about what a section
// reference is or what is said about it. They did: Drift reported every section-only anchor
// as unsearched without checking that the section it names exists, so a ruleset verified
// against a proof packet never heard about a misnamed section that the same ruleset without
// one was told about.
//
// The source state is deliberately not a parameter: a section reference is equally
// unsearchable whether the source moved or not, so reporting drift or staleness on it would
// invent a distinction from a comparison that never ran.
//
// One finding per rule rather than both, the convention the specificity checks set. The
// unknown-section report is strictly the more informative of the two -- a reader told the
// name is wrong does not also need telling that nothing was searched for it.
//
// **Advisory, though parity argues for blocking.** A section that does not exist is the same
// class of defect as a quotation that is not there, and anchor-absent blocks. It ships
// advisory because the comparison's failure mode is being *silently wrong on unusual
// markdown*, which has already happened once: the escape bug headingKey exists to prevent
// was a false fabrication report. Blocking on that would fail a ruleset for a defect in the
// checker. **Make it blocking once this normalisation has met a second corpus without a
// false positive** -- the expiry Limitations shipped with, and for the same reason.
//
// Ensures: when true, the diagnostic is finding.SeverityWarning; it is pure.
func sectionOnlyDiagnostic(r *ruleset.Rule, sources []string) (finding.Diagnostic, bool) {
	if headingOnly(r.SourceAnchor, sources) {
		return advisory(r, CategoryAnchorSectionOnly, sectionOnlyMessage), true
	}
	if !sectionOnly(r.SourceAnchor) {
		return finding.Diagnostic{}, false
	}
	if !namesHeading(sectionName(r.SourceAnchor), sources) {
		return advisory(r, CategoryAnchorSectionUnknown,
			"anchor names a section the source has no heading for; the rule may still be "+
				"sound, but nothing here points at where it came from"), true
	}
	return advisory(r, CategoryAnchorSectionOnly, sectionOnlyMessage), true
}
