package verify

import (
	"regexp"
	"strings"

	"github.com/StevenACoffman/skillet/finding"
	"github.com/StevenACoffman/skillet/markdown"
	"github.com/StevenACoffman/skillet/ruleset"
	"github.com/StevenACoffman/skillet/textnorm"
)

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
// Extracted because sectionOnly and sectionNamed must agree on where a section name ends,
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

// sectionNamed reports whether the section an anchor names is a heading in source.
//
// A section-only anchor is not searched for its text -- there is none -- so until this it
// was reported advisory and believed. But the section it names is a checkable claim: a
// source's headings are enumerable, and *does this heading exist* is answerable. Batch 4
// showed the answer already drifting, with one anchor of nine naming `§3. Use a shared mock
// subpackage` where its source has `## #3. ...`.
//
// The headings come from markdown.Sections rather than a scan for `#` lines here. A second
// parser would disagree with the kernel's on setext headings and on `#` inside fenced code,
// and canonizer would then report a fabrication that skillet's own reader does not see.
//
// Requires: anchor is section-only, which is the only state where it is asked.
// Ensures:  false when anchor names no section; it is pure.
func sectionNamed(anchor, source string) bool {
	name := sectionName(anchor)
	if name == "" {
		return false
	}
	want := headingKey(name)
	for _, s := range markdown.Parse(source).Sections {
		if headingKey(s.Title) == want {
			return true
		}
	}
	return false
}

// sectionDiagnostic reports on a section-only anchor: that the section it names is not in
// the source, or failing that, that its provenance went unsearched.
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
// Ensures: exactly one diagnostic, always finding.SeverityWarning; it is pure.
func sectionDiagnostic(r *ruleset.Rule, source string) finding.Diagnostic {
	if !sectionNamed(r.SourceAnchor, source) {
		return advisory(r, CategoryAnchorSectionUnknown,
			"anchor names a section the source has no heading for; the rule may still be "+
				"sound, but nothing here points at where it came from")
	}
	return advisory(r, CategoryAnchorSectionOnly,
		"anchor names a section and quotes nothing, so provenance cannot be "+
			"confirmed or refuted from the source text")
}
