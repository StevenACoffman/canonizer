package verify_test

import (
	"testing"

	"github.com/StevenACoffman/canonizer/internal/verify"
	"github.com/StevenACoffman/skillet/finding"
	"github.com/StevenACoffman/skillet/ruleset"
)

func rule(section string, sev ruleset.Severity, bad, good, anchor string) ruleset.Rule {
	return ruleset.Rule{
		Section:      section,
		Severity:     sev,
		Level:        ruleset.CODE,
		Statement:    "do the thing",
		Bad:          bad,
		Good:         good,
		SourceAnchor: anchor,
	}
}

func allError(t *testing.T, diags []finding.Diagnostic) {
	t.Helper()
	for _, d := range diags {
		if d.Severity != finding.SeverityError {
			t.Errorf("finding %+v is not error severity", d)
		}
	}
}

func TestExecutableFlagsMissingAndNonDiscriminating(t *testing.T) {
	t.Parallel()
	rs := ruleset.Ruleset{Rules: []ruleset.Rule{
		rule("1.1", ruleset.MUST, "x := f()", "x, err := f()", ""),        // real change → clean
		rule("1.2", ruleset.MUST, "y := g()", "", ""),                     // no ✓ → flag
		rule("1.3", ruleset.SHOULD, "a := h(); b := k()", "a := h()", ""), // ✓ ⊆ ✗ → flag
		rule("1.4", ruleset.CONSIDER, "", "", ""),                         // advisory → exempt
	}}
	diags, err := verify.Executable(rs)
	if err != nil {
		t.Fatalf("Executable: %v", err)
	}
	if len(diags) != 2 {
		t.Fatalf("got %d findings, want 2 (§1.2 unexecutable, §1.3 non-discriminating)", len(diags))
	}
	allError(t, diags)
}

func TestProvenanceFlagsMissingAndAbsentAnchors(t *testing.T) {
	t.Parallel()
	source := "The manual says: always close the connection you open.\nAnd batch your writes."
	rs := ruleset.Ruleset{Rules: []ruleset.Rule{
		rule("1.1", ruleset.MUST, "b", "g", "always close the connection"), // present → clean
		rule("1.2", ruleset.MUST, "b", "g", "use quantum encryption"),      // absent → flag
		rule("1.3", ruleset.SHOULD, "b", "g", ""),                          // no anchor → flag
		rule("1.4", ruleset.CONSIDER, "b", "g", "irrelevant"),              // advisory → exempt
	}}
	diags := verify.Provenance(rs, source)
	if len(diags) != 2 {
		t.Fatalf("got %d findings, want 2 (§1.2 absent, §1.3 no-anchor)", len(diags))
	}
	allError(t, diags)
}

func TestProvenanceMatchesAcrossRewrappedWhitespace(t *testing.T) {
	t.Parallel()
	source := "close   the\nconnection"
	rs := ruleset.Ruleset{Rules: []ruleset.Rule{
		rule("1.1", ruleset.MUST, "b", "g", "close the connection"),
	}}
	if diags := verify.Provenance(rs, source); len(diags) != 0 {
		t.Errorf("whitespace-normalized anchor should match; got %+v", diags)
	}
}
