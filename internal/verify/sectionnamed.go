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
// in: markdown escapes resolved, typography folded, inline markup and trailing stops
// dropped, case ignored.
//
// **This normalisation is the whole check, and its specification is a bug I made.** A first
// version of this comparison, written as a one-off script, reported two anchors as naming
// sections that did not exist. They existed: the source writes `## 2\. Use the "underscore
// test" package`, and markdown.Section keeps that backslash because Title is the raw source
// text. A checker that reports fabrication needs to be right about what the document says,
// so the escape strip is first and the case that caught it is a test.
//
// textnorm.Fold does the typography -- curly quotes, dashes, whitespace runs -- for the same
// reason anchorPresent uses it: a heading and a quotation of it disagree about typography
// more often than about words. withoutMarkup does the rest, and runs after Fold so that a
// stop Fold produces (an ellipsis becomes `...`) is trimmed too.
//
// **Numbering is deliberately kept.** Stripping a leading `#3.` would let the one real
// mismatch in the corpus pass, and would also let an anchor citing section 2 while meaning
// section 3 match a heading it does not name -- a worse error than the slip it excuses.
//
// Ensures: it is pure, and idempotent on text holding no backslash; escapes are resolved
// once, so `\\\\a` keys as `\\a`, which keys as `a`.
func headingKey(s string) string {
	return strings.ToLower(withoutMarkup(textnorm.Fold(escapePattern().ReplaceAllString(s, "$1"))))
}

// withoutMarkup removes the inline markup an anchor's section name may keep -- code spans,
// bold and italic runs -- a heading's trailing attribute block, and trailing sentence stops
// on either side.
//
// **The markup half is for the anchor, not the heading.** markdown.Section.Title is already
// the heading's text without inline markup (`## Do _not_ mock` has the title "Do not mock";
// only backslash escapes survive, which is escapePattern's job). An anchor is written by a
// model, which may copy the markup: `§Do _not_ mock` has no quotation to search, so it reaches
// this comparison as written. A code span in a section-only anchor does not, because a
// backtick span is read as a quotation; it does in the section name before a quotation, once
// that name is compared. The stops are for both sides: `## Is it worth it?` cited as
// `§Is it worth it` is the same heading.
//
// **Single `_` and `*` are stripped only as a pair bounded by non-word characters**, so an
// identifier such as `snake_case_name` survives -- a heading naming that identifier must not
// match one naming `snakecasename`.
//
// **It repeats until nothing changes.** Markup nests (`*_a_*`), and one pass of each pattern
// can leave a pair the other pattern would have taken; a stop can sit inside markup
// (`*Why.*`). Repeating, and collapsing the whitespace a removal leaves, is what makes
// headingKey idempotent, which its property test checks.
//
// Ensures: the result is never longer than s; it is pure.
func withoutMarkup(s string) string {
	patterns, attribute := markupPatterns(), attributePattern()
	for {
		next := attribute.ReplaceAllString(unemphasize(s), "")
		next = strings.TrimRight(strings.TrimSpace(next), ".:?")
		for _, pattern := range patterns {
			next = pattern.ReplaceAllString(next, "$1$2$3")
		}
		// Removing a run can leave two spaces where Fold left one, and a second keying
		// would collapse them; collapsing here keeps the result a fixed point.
		next = strings.Join(strings.Fields(next), " ")
		if next == s {
			return s
		}
		s = next
	}
}

// markupPatterns matches a code span and a single-`*` or single-`_` emphasis run, each with
// the characters bounding it captured so the replacement can put them back. RE2 has no
// lookaround, so the boundaries are matched rather than asserted.
//
// Built per call rather than kept in a package variable, the shape escapePattern uses.
func markupPatterns() []*regexp.Regexp {
	return []*regexp.Regexp{
		regexp.MustCompile("()`([^`]*)`()"),
		regexp.MustCompile(`(^|[^\pL\pN_])\*(\S(?:[^*]*\S)?)\*($|[^\pL\pN_])`),
		regexp.MustCompile(`(^|[^\pL\pN_])_(\S(?:[^_]*\S)?)_($|[^\pL\pN_])`),
	}
}

// attributePattern matches a trailing heading attribute block, `{#human-first-design}`.
//
// It is the heading side's markup rather than the anchor's: clig.dev heads every section
// `### Human-first design {#human-first-design}`, markdown.Section keeps the block in the
// title because goldmark parses attributes only when asked to, and an anchor citing the
// section never writes it. Measured: 53 of cli's anchors were reported against headings they
// named exactly. Only a block opening with `#` is taken, so a heading that ends in prose
// braces keeps them.
func attributePattern() *regexp.Regexp { return regexp.MustCompile(`\s*\{#[^}]*\}\s*$`) }

// headingMatches reports whether a section name, as keyed, names the heading keyed as
// heading.
//
// A name that is only a section number names the heading that number leads: `§5.2.5` cites
// `## 5.2.5 Choosing a sample size`. The number must be the heading's *whole* leading
// number, so `5.2` does not name `5.2.5 ...` -- the same reason headingKey keeps numbering.
//
// Ensures: true whenever name == heading; it is pure.
func headingMatches(name, heading string) bool {
	if name == heading {
		return true
	}
	number := leadingNumber(name)
	return number != "" && number == name && number == leadingNumber(heading)
}

// leadingNumber returns the dotted section number a key starts with, without a trailing
// period, or "" if it starts with none.
//
// Ensures: the result is a prefix of key; it is pure.
func leadingNumber(key string) string {
	return numberPattern().FindString(key)
}

// numberPattern matches the dotted section number at the start of a key. The greedy digit
// runs mean a match never ends inside a number: `5.2.50` matches whole, never as `5.2.5`.
func numberPattern() *regexp.Regexp {
	return regexp.MustCompile(`^\d+(?:\.\d+)*`)
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
	if want == "" {
		// A name that was only markup or stops, such as `§...`, names nothing; without
		// this it would match every heading that also keys to nothing.
		return false
	}
	for i := range sources {
		for _, s := range markdown.Parse(sources[i]).Sections {
			if headingMatches(want, headingKey(s.Title)) {
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
