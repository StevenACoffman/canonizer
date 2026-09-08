package verify_test

import (
	"testing"

	"github.com/StevenACoffman/canonizer/internal/verify"
)

// TestAnAnchorMayQuoteWithBackticks covers the delimiter anchorText did not read, and the
// precedence rule for an anchor carrying both.
func TestAnAnchorMayQuoteWithBackticks(t *testing.T) {
	t.Parallel()

	const source = "We declare `FindDialByID(ctx context.Context, id int) (*Dial, error)` " +
		"on the service, and callers must `defer rows.Close()` after every query."

	cases := []struct {
		name   string
		anchor string
		want   string
	}{{
		name:   "a backtick span present in the source",
		anchor: "§Abstracting: `FindDialByID(ctx context.Context, id int) (*Dial, error)`",
		want:   "",
	}, {
		name:   "a backtick span absent from the source",
		anchor: "§Abstracting: `FindDialByName(ctx context.Context) error`",
		want:   verify.CategoryAnchorAbsent,
	}, {
		// The section prefix is not part of the quotation, and searching for it along with
		// the span is the defect the prefix fix corrected for double quotes.
		name:   "the section prefix is not searched",
		anchor: "§A Section That Appears Nowhere: `defer rows.Close()`",
		want:   "",
	}, {
		// Backticks nest inside the quotation, so the outer delimiter bounds the passage.
		name:   "double quotes win over backticks when an anchor carries both",
		anchor: "§Helper methods: \"callers must `defer rows.Close()` after every query\"",
		want:   "",
	}, {
		name:   "an unterminated backtick falls back to the whole anchor",
		anchor: "callers must `defer rows.Close()` after every query",
		want:   "",
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			rs := anchored(c.anchor)
			got, _ := categoryOf(t, verify.Provenance(&rs, source))
			if got != c.want {
				t.Errorf("category = %q, want %q\nanchor: %s", got, c.want, c.anchor)
			}
		})
	}
}

// TestPairedEmphasisIsFoldedAndSingleMarkersAreNot is the narrow rule, and the snake_case
// case is why it is narrow: a rule folding every marker run scored three anchors worse.
func TestPairedEmphasisIsFoldedAndSingleMarkersAreNot(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		source string
		anchor string
		want   string
	}{{
		name:   "an anchor quoting rendered text meets bold markers in the source",
		source: "Name the package after the database __only__, never the application.",
		anchor: `§Naming: "Name the package after the database only"`,
		want:   "",
	}, {
		name:   "asterisk bold folds the same way",
		source: "Name the package after the database **only**, never the application.",
		anchor: `§Naming: "Name the package after the database only"`,
		want:   "",
	}, {
		name:   "an anchor that quoted the markers verbatim still matches",
		source: "Name the package after the database __only__, never the application.",
		anchor: `§Naming: "the database __only__, never"`,
		want:   "",
	}, {
		// The measured hazard: `_memberships FROM dial_` is a legal single-marker pair, so
		// folding every run would corrupt both sides of this and break the match.
		name:   "a snake_case identifier is left alone",
		source: "Filter with `id IN (SELECT dial_id FROM dial_memberships WHERE user_id = ?)`.",
		anchor: "§Filters: `id IN (SELECT dial_id FROM dial_memberships WHERE user_id = ?)`",
		want:   "",
	}, {
		name:   "a mismatched pair is not emphasis and is not folded",
		source: "The value is **text__ in the source, oddly enough.",
		anchor: `§Odd: "The value is **text__ in the source"`,
		want:   "",
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			rs := anchored(c.anchor)
			got, _ := categoryOf(t, verify.Provenance(&rs, c.source))
			if got != c.want {
				t.Errorf("category = %q, want %q\nanchor: %s", got, c.want, c.anchor)
			}
		})
	}
}
