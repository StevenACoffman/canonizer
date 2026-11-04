package verify

import (
	"strings"

	"github.com/StevenACoffman/skillet/finding"
	"github.com/StevenACoffman/skillet/markdown"
	"github.com/StevenACoffman/skillet/ruleset"
)

// pathSeparator splits a section name into the headings of a path, `§Part / Chapter`.
// The spaces are part of it, because headings such as `Input/Output` hold a bare slash.
const pathSeparator = " / "

// The placements, ordered so that a larger value is a more specific complaint. Across
// sources the anchor gets the best placement any source gives it, and a complaint only
// when none places it; among complaints, the most specific is the one worth reading.
const (
	// unsearched: the quotation is not in this source, so it says nothing either way.
	unsearched placement = iota
	// headingless: the source has too few headings to cite by section, and the name is
	// none of them.
	headingless
	// unnamed: the source has no heading of that name.
	unnamed
	// misplaced: the source has a heading of that name, but the quotation is not in it.
	misplaced
	// placed: the quotation is in a section of that name.
	placed
)

// placement is what one source says about where an anchor's quotation sits relative to
// the section the anchor names.
type placement int

// indexedSource is a source with its sections, parsed once.
//
// **Parsed once per check, not once per anchor, because measurement said so.** The first
// version parsed each source for every anchor it placed, and the 14-ruleset corpus run went
// from 22 to 103 seconds: markdown.Parse over a book-length source, once per anchor, is the
// whole difference. The folded text is kept for the same reason: anchorPresent folded every
// source once per anchor before this existed, which was most of the 22, and placing the
// quotation would have done it a second time.
type indexedSource struct {
	// folded is foldSource of the source, so presence is asked without refolding a book.
	folded   string
	sections []section
}

// section is one heading of a source with the bytes it governs.
type section struct {
	// key is the heading's headingKey, the form section names are compared in.
	key string
	// raw is source[start:end]: the heading line to the next heading of the same or higher
	// level, sub-sections included, as markdown.Section measures it.
	raw string
	// start and end are raw's half-open byte range in the source, kept so one section can
	// be asked whether it lies inside another.
	start, end int
}

// sectionPlacementDiagnostic reports an anchor whose quotation is in a source but not under
// the section the anchor names, and whether there is anything to report.
//
// Asked only of an anchor whose quotation Provenance already found, so a fabricated
// quotation keeps its one, blocking, finding and does not also get a placement complaint.
//
// **Advisory, with anchor-section-unknown's expiry.** The comparison rests on headingKey
// and on markdown.Parse's idea of a heading, both of which have been silently wrong on
// unusual markdown before. Blocking on that would fail a ruleset for a defect in the checker.
//
// Ensures: when true, the diagnostic is finding.SeverityWarning; it is pure.
func sectionPlacementDiagnostic(
	r *ruleset.Rule, sources []indexedSource,
) (finding.Diagnostic, bool) {
	name := quotedSectionName(r.SourceAnchor)
	if name == "" {
		return finding.Diagnostic{}, false
	}
	mostSpecific := unsearched
	for i := range sources {
		p := placeQuotation(name, r.SourceAnchor, &sources[i])
		if p == placed {
			return finding.Diagnostic{}, false
		}
		mostSpecific = max(mostSpecific, p)
	}
	return placementDiagnostic(r, mostSpecific)
}

// placementDiagnostic turns the placement the sources agreed on into its finding.
//
// Ensures: false for unsearched and placed, which are not complaints; it is pure.
func placementDiagnostic(r *ruleset.Rule, p placement) (finding.Diagnostic, bool) {
	switch p {
	case misplaced:
		return advisory(r, CategoryAnchorSectionMismatch,
			"the quotation is in the source, but not under the section the anchor names; "+
				"a reader following the citation lands in the wrong place"), true
	case unnamed:
		return advisory(r, CategoryAnchorSectionUnknown,
			"the quotation is in the source, but the source has no heading the anchor's "+
				"section name names"), true
	case headingless:
		return advisory(r, CategoryAnchorSectionHeadingless,
			"the source has too few headings to cite by section; quote it and drop the "+
				"§ section name"), true
	case unsearched, placed:
		return finding.Diagnostic{}, false
	default:
		return finding.Diagnostic{}, false
	}
}

// placeQuotation says where one source puts an anchor's quotation relative to the section
// named name.
//
// Every elision fragment must be in the *same* section, because presentIn is asked of one
// section's text at a time; a quotation stitched from two sections is not under either.
//
// Requires: name is the anchor's section name, non-empty.
// Ensures:  unsearched exactly when the quotation is not in source; it is pure.
func placeQuotation(name, anchor string, source *indexedSource) placement {
	if !presentInFolded(source.folded, anchor) {
		return unsearched
	}
	sections := source.sections
	named := namedSections(name, sections)
	for _, s := range named {
		if presentIn(s.raw, anchor) {
			return placed
		}
	}
	switch {
	case len(named) > 0:
		return misplaced
	case len(sections) < 2:
		return headingless
	default:
		return unnamed
	}
}

// indexSources parses every source once, for the checks that ask about sections.
//
// Ensures: len(result) == len(sources), in the same order; it is pure.
func indexSources(sources []string) []indexedSource {
	indexed := make([]indexedSource, 0, len(sources))
	for _, source := range sources {
		indexed = append(indexed, indexedSource{
			folded: foldSource(source), sections: sectionsOf(source),
		})
	}
	return indexed
}

// sectionsOf returns every heading of source with the bytes it governs.
//
// Ensures: len(result) == len(markdown.Parse(source).Sections); it is pure.
func sectionsOf(source string) []section {
	parsed := markdown.Parse(source).Sections
	sections := make([]section, 0, len(parsed))
	for _, s := range parsed {
		sections = append(sections, section{
			key: headingKey(s.Title), raw: source[s.Start:s.End], start: s.Start, end: s.End,
		})
	}
	return sections
}

// namedSections returns the sections a section name names: those whose heading it names,
// or, failing that and when the name is a path, the sections its last component names
// beneath the ones its earlier components name.
//
// The whole name is tried as one heading first, because a heading can itself contain the
// path separator.
//
// Ensures: every result is an element of sections; it is pure.
func namedSections(name string, sections []section) []section {
	named := headingsNamed(name, sections)
	if len(named) > 0 || !strings.Contains(name, pathSeparator) {
		return named
	}
	components := strings.Split(name, pathSeparator)
	named = headingsNamed(components[0], sections)
	for _, component := range components[1:] {
		named = within(headingsNamed(component, sections), named)
	}
	return named
}

// headingsNamed returns the sections whose heading name names, by headingMatches on the
// heading's full key or on any prefix of it that ends at one of its colons.
//
// The prefix is what lets `§Chapter 4` cite `## Chapter 4: Risk`: the measured corpus cut
// headings at their own colons more often than it copied them whole.
//
// Ensures: every result is an element of sections; nothing when name keys to ""; it is pure.
func headingsNamed(name string, sections []section) []section {
	want := headingKey(name)
	if want == "" {
		return nil
	}
	var named []section
	for _, s := range sections {
		if namesKey(want, s.key) {
			named = append(named, s)
		}
	}
	return named
}

// namesKey reports whether a keyed section name names the heading keyed as heading, whole
// or by a prefix ending at one of its colons.
//
// Ensures: true whenever headingMatches(want, heading); it is pure.
func namesKey(want, heading string) bool {
	if headingMatches(want, heading) {
		return true
	}
	for i := range len(heading) {
		if heading[i] == ':' && headingMatches(want, headingKey(heading[:i])) {
			return true
		}
	}
	return false
}

// within returns the sections of inner that lie strictly inside one of outer.
//
// Strictly, so a path naming the same heading twice does not resolve to it.
//
// Ensures: every result is an element of inner; it is pure.
func within(inner, outer []section) []section {
	var kept []section
	for _, in := range inner {
		for _, out := range outer {
			if out.start < in.start && in.end <= out.end {
				kept = append(kept, in)
				break
			}
		}
	}
	return kept
}

// quotedSectionName returns the section name of a `§Name: "quotation"` anchor, or "" when
// the anchor has no `§` or no quotation after its name.
//
// Ensures: the result is trimmed; it is pure.
func quotedSectionName(anchor string) string {
	i := quotationSeparator(anchor)
	if i < 0 {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(anchor[:i]), "§"))
}
