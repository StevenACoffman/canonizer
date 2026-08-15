package verify_test

import (
	"testing"

	"github.com/StevenACoffman/canonizer/internal/gate"
	"github.com/StevenACoffman/canonizer/internal/verify"
	"github.com/StevenACoffman/skillet/finding"
	"github.com/StevenACoffman/skillet/ruleset"
)

// TestFoldingOnlyWidensAcceptance pins what adopting skillet/textnorm bought. Each pair is
// a (source spelling, anchor spelling) a real distillation produces: the book uses
// typographic characters and the ruleset was retyped or copied through a tool.
//
// Every one of these emitted anchor-absent under the old whitespace-only normalize, so a
// routine source-typography difference blocked a sound rule while the identical passage
// passed exegesis quotecheck. Verified by reverting: curly apostrophes, curly doubles and
// em dashes all failed before.
//
// The direction is the assertion. Folding strictly widens what matches, so a rule that
// stops being accepted would mean the normalization changed meaning rather than reach.
func TestFoldingOnlyWidensAcceptance(t *testing.T) {
	t.Parallel()
	pairs := []struct{ name, source, anchor string }{
		{"curly apostrophe", "the agent’s own words", "the agent's own words"},
		{"curly doubles", "he said “stop” now", `he said "stop" now`},
		{"em dash", "close it—always", "close it-always"},
		{"nbsp", "close the handle", "close the handle"},
		{"wrapped line", "close   the\nhandle", "close the handle"},
		{"identical", "close the handle", "close the handle"},
	}
	for _, p := range pairs {
		rs := ruleset.Ruleset{Rules: []ruleset.Rule{{
			Section: "1.1", Severity: ruleset.MUST, Level: ruleset.CODE,
			Statement: "Close it.", SourceAnchor: p.anchor,
		}}}
		got := verify.Provenance(rs, p.source)
		absent := 0
		for _, d := range got {
			if d.Category == "anchor-absent" {
				absent++
			}
		}
		if absent != 0 {
			t.Errorf("%s: still reports anchor-absent — folding must accept it", p.name)
		}
	}
}

// TestAdvisoryChecksNeverBlock runs the gate over what Specificity and Conflicts emit,
// rather than reading their severity constant. Both were added to cmd/verify's output, and
// the invariant that matters is not "the field says warning" but "gate lets it ship".
func TestAdvisoryChecksNeverBlock(t *testing.T) {
	t.Parallel()
	// A ruleset that trips both: two rules stating the same thing at different severities,
	// and statements that name nothing concrete and hedge.
	rs := ruleset.Ruleset{Rules: []ruleset.Rule{
		{
			Section: "1.1", Severity: ruleset.MUST, Level: ruleset.CODE,
			Statement: "Handle errors as appropriate.", SourceAnchor: "a",
			Bad: "x", Good: "y",
		},
		{
			Section: "2.4", Severity: ruleset.SHOULD, Level: ruleset.CODE,
			Statement: "Handle errors as appropriate.", SourceAnchor: "a",
			Bad: "x", Good: "y",
		},
	}}
	diags := append(verify.Specificity(rs), verify.Conflicts(rs)...)
	if len(diags) == 0 {
		t.Fatal("fixture tripped neither check; it cannot prove they are non-blocking")
	}
	if blocking := gate.Blocking(finding.Result{Diagnostics: diags}); len(blocking) != 0 {
		t.Errorf("advisory checks reached the gate: %+v", blocking)
	}
	// Every one must also say who acts, or the axis is decoration.
	for _, d := range diags {
		if !d.Action.Valid() {
			t.Errorf("%s carries no Action", d.Category)
		}
	}
}
