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

// stated builds a rule carrying a specific Statement, which is what Specificity reads.
func stated(section string, sev ruleset.Severity, statement string) ruleset.Rule {
	r := rule(section, sev, "bad", "good", "anchor")
	r.Statement = statement
	return r
}

func TestSpecificityFlagsGeneralAdvice(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		statement string
		wantFlag  bool
		wantCat   string
	}{
		"names a symbol in backticks": {
			statement: "Call `ctx.Done()` before returning from the handler.", wantFlag: false,
		},
		"hedged even though it names a tool": {
			statement: "Use `errgroup` as appropriate for concurrent fetches.",
			wantFlag:  true, wantCat: "softening",
		},
		"pure prose names nothing actionable": {
			statement: "Decompose the work into smaller steps.",
			wantFlag:  true, wantCat: "unspecific",
		},
		"a link counts as concrete": {
			statement: "Follow [the retry policy](docs/retry.md) on every write.", wantFlag: false,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := verify.Specificity(ruleset.Ruleset{
				Rules: []ruleset.Rule{stated("1", ruleset.MUST, tc.statement)},
			})
			if !tc.wantFlag {
				if len(got) != 0 {
					t.Errorf("flagged a specific rule: %+v", got)
				}
				return
			}
			if len(got) != 1 {
				t.Fatalf("want one finding, got %+v", got)
			}
			if got[0].Category != tc.wantCat {
				t.Errorf("category = %q, want %q", got[0].Category, tc.wantCat)
			}
		})
	}
}

func TestSpecificityIsNeverBlocking(t *testing.T) {
	t.Parallel()
	// The constraint lives in the code, not in a convention about how to call it:
	// Executable and Provenance are the only checks that stop a ship, and this must not
	// be able to join them however it is invoked.
	rs := ruleset.Ruleset{Rules: []ruleset.Rule{
		stated("1", ruleset.MUST, "Handle errors carefully."),
		stated("2", ruleset.SHOULD, "Be careful with dangerous operations."),
		stated("3", ruleset.MUST, "Use it as appropriate."),
	}}
	got := verify.Specificity(rs)
	if len(got) == 0 {
		t.Fatal("expected these three to be flagged; the test proves nothing otherwise")
	}
	for _, d := range got {
		if d.Severity != finding.SeverityWarning {
			t.Errorf("finding %+v is not a warning; it could block a ship", d)
		}
		if d.Severity.Blocking() {
			t.Errorf("finding %+v reports as blocking", d)
		}
	}
}

func TestSpecificityIgnoresUnenforcedRules(t *testing.T) {
	t.Parallel()
	// An advisory note on a rule nobody enforces is noise, and the other two checks
	// skip CONSIDER for the same reason.
	got := verify.Specificity(ruleset.Ruleset{
		Rules: []ruleset.Rule{stated("1", ruleset.CONSIDER, "Be careful out there.")},
	})
	if len(got) != 0 {
		t.Errorf("flagged an unenforced rule: %+v", got)
	}
}

func TestSpecificityReportsOneFindingPerRule(t *testing.T) {
	t.Parallel()
	// A statement that both hedges and names nothing gets one note, not two: the
	// reader's action is the same either way, and doubling it inflates rework budget.
	got := verify.Specificity(ruleset.Ruleset{
		Rules: []ruleset.Rule{stated("1", ruleset.MUST, "Handle it as appropriate.")},
	})
	if len(got) != 1 {
		t.Errorf("want a single finding for one rule, got %+v", got)
	}
}
