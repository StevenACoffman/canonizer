package verify_test

import (
	"testing"

	"github.com/StevenACoffman/canonizer/internal/verify"
)

// elidedSource carries two spans a quoter would plausibly join with an elision, plus the
// two halves of the variadic case below sitting apart from each other, so a test that
// wrongly splits Go syntax finds both fragments and passes.
const elidedSource = "I rarely expose internal details like transactions to the rest of " +
	"my application because it couples them to the storage layer, and honestly " +
	"it's rarely necessary. Elsewhere we call f(a, b) and later close ) done."

func TestAnElidedQuotationMatchesEachFragment(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		anchor string
		want   string // "" means the anchor was found
	}{{
		name:   "an unelided quotation still matches whole",
		anchor: `§Boundaries: "I rarely expose internal details like transactions"`,
		want:   "",
	}, {
		name:   "both fragments present",
		anchor: `§Boundaries: "I rarely expose internal details ... it's rarely necessary"`,
		want:   "",
	}, {
		name:   "an ellipsis character behaves as three dots",
		anchor: "§Boundaries: \"I rarely expose internal details … it's rarely necessary\"",
		want:   "",
	}, {
		name:   "the second fragment is absent",
		anchor: `§Boundaries: "I rarely expose internal details ... it is always necessary"`,
		want:   verify.CategoryAnchorAbsent,
	}, {
		name:   "the first fragment is absent",
		anchor: `§Boundaries: "I frequently expose transactions ... it's rarely necessary"`,
		want:   verify.CategoryAnchorAbsent,
	}, {
		// Fold trims, so a quotation ending in an elision loses the space that would make
		// the mark a separator and the mark stays inside the final fragment. Rejecting is
		// deliberate: the convention the prompt states is an elision marking a gap between
		// two quoted spans, and a trailing mark claims an unbounded remainder instead. The
		// corpus carries zero leading and zero trailing elisions, so this is a pinned
		// decision rather than a measured need -- and it is pinned so that widening it
		// later has to be a choice somebody makes on purpose.
		name:   "a trailing elision claims an unbounded remainder and is refused",
		anchor: `§Boundaries: "I rarely expose internal details ... "`,
		want:   verify.CategoryAnchorAbsent,
	}, {
		name:   "an anchor of nothing but elisions does not pass on an empty conjunction",
		anchor: `§Boundaries: "... ... "`,
		want:   verify.CategoryAnchorAbsent,
	}, {
		name:   "go variadic syntax is not an elision and is not split",
		anchor: `§Boundaries: "call f(a, ...) done"`,
		want:   verify.CategoryAnchorAbsent,
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			rs := anchored(c.anchor)
			got, _ := categoryOf(t, verify.Provenance(&rs, []string{elidedSource}))
			if got != c.want {
				t.Errorf("category = %q, want %q\nanchor: %s", got, c.want, c.anchor)
			}
		})
	}
}
