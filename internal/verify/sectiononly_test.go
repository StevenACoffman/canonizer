package verify_test

import (
	"testing"

	"github.com/StevenACoffman/canonizer/internal/verify"
	"github.com/StevenACoffman/skillet/finding"
	"github.com/StevenACoffman/skillet/ruleset"
)

const anchorSource = "In practice we define our services with an interface in the root package."

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
