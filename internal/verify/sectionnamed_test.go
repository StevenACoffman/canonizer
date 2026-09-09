package verify_test

import (
	"testing"

	"github.com/StevenACoffman/canonizer/internal/verify"
)

// headingSource carries the heading shapes a real article uses, each of which broke a
// version of this comparison or plausibly could.
const headingSource = "# Standard Package Layout\n\n" +
	"## #3. Use a shared mock subpackage\n\n" +
	"## 2\\. Use the “underscore test” package\n\n" +
	"## Don’t limit this to third party dependencies\n\n" +
	"### Injecting dependencies at compile time\n\n" +
	"Body text mentioning A better approach without heading it.\n\n" +
	"```go\n// # Not a heading, it is inside a fence\n```\n"

func TestASectionAnchorIsCheckedAgainstTheSourcesHeadings(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		anchor string
		want   string
	}{{
		// The bug this check exists to not repeat. A one-off version of this comparison
		// reported anchors like this as fabricated, because markdown.Section keeps the raw
		// backslash in `2\.` and the normaliser did not strip it.
		name:   "a heading whose number is backslash-escaped in the source",
		anchor: "§2. Use the “underscore test” package",
		want:   verify.CategoryAnchorSectionOnly,
	}, {
		// Not section-only at all, and the interaction is worth pinning. An anchor holding
		// ASCII quotes has a quotable span, so the conjunct in sectionOnly routes it to the
		// text search before this check is reached -- and `underscore test` is in the
		// source, so it passes silently. Agents write the source's curly quotes, which do
		// not trigger that, so the corpus meets the case above rather than this one.
		name:   "a section name in ASCII quotes is read as a quotation, not a section",
		anchor: `§2. Use the "underscore test" package`,
		want:   "",
	}, {
		name:   "an apostrophe folds the same way",
		anchor: "§Don't limit this to third party dependencies",
		want:   verify.CategoryAnchorSectionOnly,
	}, {
		name:   "a deeper heading level still counts",
		anchor: "§Injecting dependencies at compile time",
		want:   verify.CategoryAnchorSectionOnly,
	}, {
		name:   "a trailing colon does not become part of the name",
		anchor: "§Standard Package Layout:",
		want:   verify.CategoryAnchorSectionOnly,
	}, {
		// The real slip from batch 4: the source heads `## #3. ...` and the anchor dropped
		// the `#`. Flagged rather than excused, because normalising leading decoration away
		// would also let an anchor citing section 2 match a heading numbered 3.
		name:   "a heading transcribed without its leading marker",
		anchor: "§3. Use a shared mock subpackage",
		want:   verify.CategoryAnchorSectionUnknown,
	}, {
		name:   "a section the source simply does not have",
		anchor: "§Error taxonomy",
		want:   verify.CategoryAnchorSectionUnknown,
	}, {
		// Body prose is not a heading, so naming it is still a miss.
		name:   "text that appears in the body but heads nothing",
		anchor: "§A better approach",
		want:   verify.CategoryAnchorSectionUnknown,
	}, {
		// The reason this uses markdown.Sections instead of scanning for `#` lines.
		name:   "a hash inside a fenced code block is not a heading",
		anchor: "§Not a heading, it is inside a fence",
		want:   verify.CategoryAnchorSectionUnknown,
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			rs := anchored(c.anchor)
			got, blocking := categoryOf(t, verify.Provenance(&rs, []string{headingSource}))
			if got != c.want {
				t.Errorf("category = %q, want %q\nanchor: %s", got, c.want, c.anchor)
			}
			// Advisory until the normalisation has met a second corpus without a false
			// positive; a checker bug must not be able to fail a ruleset.
			if blocking {
				t.Errorf("category %q blocks; it must stay advisory", got)
			}
		})
	}
}

// TestAnUnknownSectionReplacesTheSectionOnlyNote keeps one finding per rule: a reader told
// the section name is wrong does not also need telling that nothing was searched for it.
func TestAnUnknownSectionReplacesTheSectionOnlyNote(t *testing.T) {
	t.Parallel()
	rs := anchored("§Error taxonomy")
	got := verify.Provenance(&rs, []string{headingSource})
	if len(got) != 1 {
		t.Fatalf("want a single finding, got %+v", got)
	}
	if got[0].Category != verify.CategoryAnchorSectionUnknown {
		t.Errorf("category = %q, want the unknown-section report", got[0].Category)
	}
}

// TestAnAnchorIsSoughtInEverySource is the multi-source contract: a synthesized ruleset
// derives from every source it was merged from, so an anchor drawn from any of them is
// present.
func TestAnAnchorIsSoughtInEverySource(t *testing.T) {
	t.Parallel()

	first := "# Alpha\n\nAlways close what you opened.\n"
	second := "# Beta\n\nNever ignore a returned error.\n"

	cases := []struct {
		name    string
		sources []string
		anchor  string
		want    string
	}{{
		name:    "found in the first",
		sources: []string{first, second},
		anchor:  `§Alpha: "Always close what you opened"`,
		want:    "",
	}, {
		name:    "found in the second",
		sources: []string{first, second},
		anchor:  `§Beta: "Never ignore a returned error"`,
		want:    "",
	}, {
		name:    "found in neither",
		sources: []string{first, second},
		anchor:  `§Alpha: "Always reboot the server"`,
		want:    verify.CategoryAnchorAbsent,
	}, {
		name:    "a section heading from either source counts",
		sources: []string{first, second},
		anchor:  "§Beta",
		want:    verify.CategoryAnchorSectionOnly,
	}, {
		// The reason sources are iterated rather than joined: folding collapses the seam,
		// so a concatenation would let this match text no document contains.
		name:    "a quotation spanning two sources is not present in either",
		sources: []string{first, second},
		anchor:  `§X: "Always close what you opened. Never ignore a returned error."`,
		want:    verify.CategoryAnchorAbsent,
	}, {
		name:    "one source behaves exactly as before",
		sources: []string{first},
		anchor:  `§Alpha: "Always close what you opened"`,
		want:    "",
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			rs := anchored(c.anchor)
			got, _ := categoryOf(t, verify.Provenance(&rs, c.sources))
			if got != c.want {
				t.Errorf("category = %q, want %q\nanchor: %s", got, c.want, c.anchor)
			}
		})
	}
}
