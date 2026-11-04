package verify_test

import (
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"testing/quick"

	"github.com/StevenACoffman/canonizer/internal/verify"
)

// markupSource heads its sections the ways learn-go-with-tests and measureanything do.
// markdown.Section.Title already drops a heading's inline markup, so the cases below are
// about an anchor that keeps it, or that drops a stop the heading has.
const markupSource = "# Heading shapes\n\n" +
	"## Why `math` matters\n\n" +
	"## Do _not_ mock what you do not own\n\n" +
	"## Prefer **small** interfaces\n\n" +
	"## Write the test first.\n\n" +
	"## Is it worth it?\n\n" +
	"## Name it snake_case_name\n\n" +
	"## 5.2.5 Choosing a sample size\n\n" +
	"## 5.3 Sampling\n\n" +
	"### Human-first design {#human-first-design}\n\n" +
	"## B.6 Steps for modifying a policy set\n\n" +
	"## A note on style\n"

func TestHeadingsAreComparedWithoutInlineMarkup(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		anchor string
		want   string
	}{
		"italics kept by the anchor": {
			anchor: "§Do _not_ mock what you do not own",
			want:   verify.CategoryAnchorSectionOnly,
		},
		"bold kept by the anchor": {
			anchor: "§Prefer **small** interfaces",
			want:   verify.CategoryAnchorSectionOnly,
		},
		"a trailing period the anchor drops": {
			anchor: "§Write the test first",
			want:   verify.CategoryAnchorSectionOnly,
		},
		"a trailing question mark the anchor drops": {
			anchor: "§Is it worth it",
			want:   verify.CategoryAnchorSectionOnly,
		},
		// An identifier is not emphasis, so dropping its underscores names something else.
		"an identifier keeps its underscores": {
			anchor: "§Name it snakecasename",
			want:   verify.CategoryAnchorSectionUnknown,
		},
		"a bare section number": {
			anchor: "§5.2.5",
			want:   verify.CategoryAnchorSectionOnly,
		},
		"a bare section number with a stop": {
			anchor: "§5.2.5.",
			want:   verify.CategoryAnchorSectionOnly,
		},
		// The number must be the heading's whole leading number, in both directions.
		"a number that only starts the heading's": {
			anchor: "§5.2",
			want:   verify.CategoryAnchorSectionUnknown,
		},
		"a number the heading's only starts": {
			anchor: "§5.2.50",
			want:   verify.CategoryAnchorSectionUnknown,
		},
		"a heading's attribute block, which an anchor never writes": {
			anchor: "§Human-first design",
			want:   verify.CategoryAnchorSectionOnly,
		},
		"an appendix section number": {
			anchor: "§B.6",
			want:   verify.CategoryAnchorSectionOnly,
		},
		// A letter alone is not a section number, or the article would be one.
		"a bare letter is not an appendix number": {
			anchor: "§A",
			want:   verify.CategoryAnchorSectionUnknown,
		},
		"only stops names nothing": {
			anchor: "§...",
			want:   verify.CategoryAnchorSectionUnknown,
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			rs := anchored(c.anchor)
			got, blocking := categoryOf(t, verify.Provenance(&rs, []string{markupSource}))
			if got != c.want || blocking {
				t.Errorf("got (%q, blocking=%t), want the advisory %q", got, blocking, c.want)
			}
		})
	}
}

// TestHeadingKeyIsIdempotent is the property that makes keying safe to apply to both sides
// of a comparison: a key is already in the form it would be reduced to.
//
// The strings are drawn from markup, stops and a few letters rather than arbitrary Unicode,
// which almost never holds a `*` pair and so would pass without exercising anything.
// Backslashes are excluded because escapes are resolved once by design, so `\\a` keys as
// `\a`.
func TestHeadingKeyIsIdempotent(t *testing.T) {
	t.Parallel()
	const alphabet = "ab _*`.:?… Z5{#}"
	runes := []rune(alphabet)
	generate := func(values []reflect.Value, r *rand.Rand) {
		var b strings.Builder
		for range r.Intn(12) {
			b.WriteRune(runes[r.Intn(len(runes))])
		}
		values[0] = reflect.ValueOf(b.String())
	}
	property := func(s string) bool {
		once := verify.HeadingKey(s)
		return verify.HeadingKey(once) == once
	}
	if err := quick.Check(property, &quick.Config{MaxCount: 2000, Values: generate}); err != nil {
		t.Error(err)
	}
}
