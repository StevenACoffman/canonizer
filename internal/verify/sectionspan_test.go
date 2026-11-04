package verify_test

import (
	"testing"

	"github.com/StevenACoffman/canonizer/internal/verify"
)

// bookSource has the shapes the anchor section-name convention covers: a preamble, a title
// over everything, colon-bearing chapter headings, a sub-heading repeated under two
// chapters, a heading containing the path separator, and a heading carrying a code span.
const bookSource = "Opening remarks come first.\n\n" +
	"# Book Title\n\n" +
	"## Chapter 4: Risk\n\nRisk is measured in dollars. Every estimate has a range.\n\n" +
	"### Example\n\nThe first example quantifies risk.\n\n" +
	"## Chapter 5: Value\n\nThe value of information is computable. Every estimate has a range.\n\n" +
	"### Example\n\nThe second example computes value.\n\n" +
	"## Input / Output\n\nEvery program reads before it writes.\n\n" +
	"## Why `math` matters\n\nMath is cheap.\n"

// transcriptSource is a talk transcript: prose with no headings to cite.
const transcriptSource = "So, simple made easy. Simplicity is a prerequisite for reliability."

// oneHeadingSource is a short article whose only heading is its title.
const oneHeadingSource = "# Simple Made Easy\n\nSimplicity is a prerequisite for reliability."

func TestAQuotationIsPlacedUnderTheSectionItsAnchorNames(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		sources []string
		anchor  string
		want    string // "" means the anchor is placed
	}{
		"the full heading": {
			sources: []string{bookSource},
			anchor:  `§Chapter 4: Risk: "Risk is measured in dollars"`,
		},
		"a prefix ending at the heading's colon": {
			sources: []string{bookSource},
			anchor:  `§Chapter 4: "Risk is measured in dollars"`,
		},
		"a sibling section": {
			sources: []string{bookSource},
			anchor:  `§Chapter 5: "Risk is measured in dollars"`,
			want:    verify.CategoryAnchorSectionMismatch,
		},
		"a parent section contains its sub-sections": {
			sources: []string{bookSource},
			anchor:  `§Chapter 4: "The first example quantifies risk"`,
		},
		"a repeated heading is resolved by the quotation": {
			sources: []string{bookSource},
			anchor:  `§Example: "The second example computes value"`,
		},
		"a repeated quotation is placed by any occurrence": {
			sources: []string{bookSource},
			anchor:  `§Chapter 5: "Every estimate has a range"`,
		},
		"a path": {
			sources: []string{bookSource},
			anchor:  `§Chapter 5 / Example: "The second example computes value"`,
		},
		"a path through the wrong parent": {
			sources: []string{bookSource},
			anchor:  `§Chapter 4 / Example: "The second example computes value"`,
			want:    verify.CategoryAnchorSectionMismatch,
		},
		"a path that skips a level": {
			sources: []string{bookSource},
			anchor:  `§Book Title / Example: "The first example quantifies risk"`,
		},
		"a heading that contains the path separator": {
			sources: []string{bookSource},
			anchor:  `§Input / Output: "Every program reads before it writes"`,
		},
		"a code span the anchor keeps in the section name": {
			sources: []string{bookSource},
			anchor:  "§Why `math` matters: \"Math is cheap\"",
		},
		"a section the source has no heading for": {
			sources: []string{bookSource},
			anchor:  `§Chapter 9: "Risk is measured in dollars"`,
			want:    verify.CategoryAnchorSectionUnknown,
		},
		// Each fragment is present, but in different sections, so the quotation is under
		// neither: a reader following the citation finds half of it.
		"an elision stitched across two sections": {
			sources: []string{bookSource},
			anchor: `§Chapter 4: "Risk is measured in dollars. ... ` +
				`The value of information is computable"`,
			want: verify.CategoryAnchorSectionMismatch,
		},
		"a quotation from the preamble, before any heading": {
			sources: []string{bookSource},
			anchor:  `§Book Title: "Opening remarks come first"`,
			want:    verify.CategoryAnchorSectionMismatch,
		},
		"a section name against a source with no headings": {
			sources: []string{transcriptSource},
			anchor:  `§Slide 3: "Simplicity is a prerequisite for reliability"`,
			want:    verify.CategoryAnchorSectionHeadingless,
		},
		// A true citation is not reported, even of a source too short of headings for
		// the convention to ask for one.
		"the only heading of a one-heading source": {
			sources: []string{oneHeadingSource},
			anchor:  `§Simple Made Easy: "Simplicity is a prerequisite for reliability"`,
		},
		"a quotation-only anchor is not asked about sections": {
			sources: []string{transcriptSource},
			anchor:  `"Simplicity is a prerequisite for reliability"`,
		},
		"placed by any source": {
			sources: []string{transcriptSource, oneHeadingSource},
			anchor:  `§Simple Made Easy: "Simplicity is a prerequisite for reliability"`,
		},
		// Placed by none: the more specific complaint is the one worth reading.
		"misplaced in one source and headingless in another": {
			sources: []string{transcriptSource, bookSource},
			anchor:  `§Chapter 5: "Risk is measured in dollars"`,
			want:    verify.CategoryAnchorSectionMismatch,
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			rs := anchored(c.anchor)
			got, blocking := categoryOf(t, verify.Provenance(&rs, c.sources))
			if got != c.want || blocking {
				t.Errorf("got (%q, blocking=%t), want (%q, advisory)", got, blocking, c.want)
			}
		})
	}
}

// TestDriftPlacesAQuotationAsProvenanceDoes keeps the two paths agreeing. A changed source
// is the exception, and deliberately: its drift advisory already says the citation may
// need refreshing, and one finding per rule is the convention.
func TestDriftPlacesAQuotationAsProvenanceDoes(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		state verify.SourceState
		want  string
	}{
		"source unknown": {
			state: verify.SourceUnknown,
			want:  verify.CategoryAnchorSectionMismatch,
		},
		"source unchanged": {
			state: verify.SourceUnchanged,
			want:  verify.CategoryAnchorSectionMismatch,
		},
		"source changed": {state: verify.SourceChanged, want: verify.CategoryAnchorDrift},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			rs := anchored(`§Chapter 5: "Risk is measured in dollars"`)
			got, blocking := categoryOf(t, verify.Drift(&rs, []string{bookSource}, c.state))
			if got != c.want || blocking {
				t.Errorf("got (%q, blocking=%t), want (%q, advisory)", got, blocking, c.want)
			}
		})
	}
}
