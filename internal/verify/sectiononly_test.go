package verify_test

import (
	"testing"

	"github.com/StevenACoffman/canonizer/internal/verify"
	"github.com/StevenACoffman/skillet/finding"
	"github.com/StevenACoffman/skillet/ruleset"
)

// anchorSource carries the headings the section-only anchors below name, because a section
// reference is now checked against the source's headings: a fixture without them makes every
// such anchor report an unknown section, which is the check working rather than a failure.
const anchorSource = "# Common CRUD design in Go\n\n" +
	"## Transactional boundaries\n\n" +
	"In practice we define our services with an interface in the root package.\n\n" +
	"## The interface\n\n" +
	"## Errors\n\n" +
	"## 4.2\n"

// anchored builds an enforced rule carrying the given source anchor.
func anchored(anchor string) ruleset.Ruleset {
	return ruleset.Ruleset{Rules: []ruleset.Rule{{
		Section: "1.1", Severity: ruleset.MUST, Level: ruleset.CODE,
		Statement: "do the thing", Bad: "b", Good: "g", SourceAnchor: anchor,
	}}}
}

// categoryOf returns the single diagnostic's category and whether it blocks.
func categoryOf(t *testing.T, diags []finding.Diagnostic) (string, bool) {
	t.Helper()
	if len(diags) == 0 {
		return "", false
	}
	if len(diags) != 1 {
		t.Fatalf("want at most one diagnostic, got %+v", diags)
	}
	return diags[0].Category, diags[0].Severity.Blocking()
}

func TestSectionOnlyAnchor(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		anchor       string
		wantCategory string
		wantBlocking bool
	}{
		// The new behaviour: nothing to search for, so no verdict about the source.
		"a bare section reference": {
			anchor: "§Transactional boundaries", wantCategory: "anchor-section-only",
		},
		"a numbered section reference": {
			anchor: "§4.2", wantCategory: "anchor-section-only",
		},
		"a section reference with a trailing colon": {
			anchor: "§Errors:", wantCategory: "anchor-section-only",
		},
		// Unchanged: a quoted span is searched and found.
		"section prefix plus a quotation present in the source": {
			anchor: `§The interface: "we define our services with an interface"`,
		},
		// Unchanged: a quoted span that is absent still blocks.
		"section prefix plus a fabricated quotation": {
			anchor:       `§The interface: "we forbid interfaces in the root package"`,
			wantCategory: "anchor-absent", wantBlocking: true,
		},
		// The first thing revision 2's predicate would have broken: a bare quotation with
		// no quote marks must still be searched, not written off as unsearchable.
		"a bare quotation with no marks is still searched and found": {
			anchor: "we define our services with an interface",
		},
		// The second, and the more damaging: a paraphrase under a section prefix is the
		// ruleset's own defect. Keying on quote marks would have made all eight of them
		// advisory and hidden a real finding.
		"a paraphrase under a section prefix still blocks": {
			anchor:       "§The interface: every method takes ctx first",
			wantCategory: "anchor-absent", wantBlocking: true,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			rs := anchored(tc.anchor)
			cat, blocking := categoryOf(t, verify.Provenance(&rs, anchorSource))
			if cat != tc.wantCategory || blocking != tc.wantBlocking {
				t.Errorf("Provenance = (%q, blocking=%t), want (%q, blocking=%t) for %q",
					cat, blocking, tc.wantCategory, tc.wantBlocking, tc.anchor)
			}
		})
	}
}

// TestDriftAgreesWithProvenanceOnSectionOnly is the agreement the two checks must keep:
// routing both through one predicate is what stops one blocking what the other passes.
func TestDriftAgreesWithProvenanceOnSectionOnly(t *testing.T) {
	t.Parallel()
	rs := anchored("§Transactional boundaries")
	for _, state := range []verify.SourceState{
		verify.SourceUnknown, verify.SourceUnchanged, verify.SourceChanged,
	} {
		cat, blocking := categoryOf(t, verify.Drift(&rs, anchorSource, state))
		if cat != "anchor-section-only" || blocking {
			t.Errorf("Drift with state %v = (%q, blocking=%t), want the section-only "+
				"advisory regardless of what the source did", state, cat, blocking)
		}
	}
}

// TestSectionNamesMayContainSpaces pins the fix that the first predicate got wrong: a
// section name is often several words, so splitting on whitespace rejected the commonest
// real form. The colon is the separator the anchor format actually uses.
func TestSectionNamesMayContainSpaces(t *testing.T) {
	t.Parallel()
	for _, anchor := range []string{
		"§Transactional boundaries",
		"§The interface",
		"§Common CRUD design in Go",
	} {
		rs := anchored(anchor)
		cat, _ := categoryOf(t, verify.Provenance(&rs, anchorSource))
		if cat != "anchor-section-only" {
			t.Errorf("anchor %q reported %q, want anchor-section-only", anchor, cat)
		}
	}
}

// TestAnAnchorWithSomethingToSearchIsNotSectionOnly closes the hole the corpus found: a
// paraphrase written without a colon read as one long section name and drew an advisory,
// which is the treatment the colon rule was chosen to keep paraphrases *out* of.
func TestAnAnchorWithSomethingToSearchIsNotSectionOnly(t *testing.T) {
	t.Parallel()

	const source = "## Transactional boundaries\n\n## 4.2\n\n" +
		"We call `NewTestDB` to open a real Bolt database, and `TestYoClient` " +
		"to mock the remote client. Transactions stay inside the service method."

	cases := []struct {
		name   string
		anchor string
		want   string
	}{{
		// The measured instance. Two code spans to search for, so the gate must search.
		name: "a colonless paraphrase carrying code spans is searched, not excused",
		anchor: "§3 `NewTestDB` opening a real Bolt database, contrasted with " +
			"§4 `TestYoClient`",
		want: "",
	}, {
		name:   "a colonless anchor carrying a quotation is searched",
		anchor: `§3 the passage reading "Transactions stay inside the service method"`,
		want:   "",
	}, {
		// Unchanged: nothing to search for, so nothing was searched.
		name:   "a bare section reference is still advisory",
		anchor: "§Transactional boundaries",
		want:   verify.CategoryAnchorSectionOnly,
	}, {
		name:   "a numbered bare section is still advisory",
		anchor: "§4.2",
		want:   verify.CategoryAnchorSectionOnly,
	}, {
		// Unchanged, and the case the conjunct must not disturb: a colon-bearing paraphrase
		// quotes nothing and stays a defect rather than becoming an advisory.
		name:   "a colon-bearing paraphrase still fails as absent",
		anchor: "§Errors: every method takes ctx first",
		want:   verify.CategoryAnchorAbsent,
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
