package verify_test

import (
	"strings"
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

// TestCategoryValuesAreTheWireContract pins each category's string, which the constants
// deliberately do not.
//
// The behavioural tests above assert literals for the same reason: a test that compares
// the constant against itself passes however the value is edited, so the one thing it
// cannot catch is the change that matters. These strings are read by `gate` and by
// anything consuming the findings JSON, so renaming one is a breaking change and should
// fail here rather than downstream.
func TestCategoryValuesAreTheWireContract(t *testing.T) {
	t.Parallel()
	cases := map[string]struct{ got, want string }{
		"unexecutable":        {verify.CategoryUnexecutable, "unexecutable"},
		"non-discriminating":  {verify.CategoryNonDiscriminating, "non-discriminating"},
		"no-anchor":           {verify.CategoryNoAnchor, "no-anchor"},
		"anchor-absent":       {verify.CategoryAnchorAbsent, "anchor-absent"},
		"unspecific":          {verify.CategoryUnspecific, "unspecific"},
		"nothing-examined":    {verify.CategoryNothingExamined, "nothing-examined"},
		"non-canonical":       {verify.CategoryNonCanonical, "non-canonical"},
		"anchor-drift":        {verify.CategoryAnchorDrift, "anchor-drift"},
		"anchor-stale":        {verify.CategoryAnchorStale, "anchor-stale"},
		"anchor-fabricated":   {verify.CategoryAnchorFabricated, "anchor-fabricated"},
		"unbounded":           {verify.CategoryUnbounded, "unbounded"},
		"warrant-incomplete":  {verify.CategoryWarrantIncomplete, "warrant-incomplete"},
		"anchor-unverifiable": {verify.CategoryAnchorUnverifiable, "anchor-unverifiable"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if tc.got != tc.want {
				t.Errorf(
					"category = %q, want %q; renaming it breaks every consumer",
					tc.got,
					tc.want,
				)
			}
		})
	}
}

func TestRulesCountsWhatTheGatesExamine(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		severities   []ruleset.Severity
		wantEnforced int
		wantTotal    int
		wantAdvisory bool
	}{
		"all enforced": {
			severities:   []ruleset.Severity{ruleset.MUST, ruleset.SHOULD},
			wantEnforced: 2, wantTotal: 2, wantAdvisory: false,
		},
		"mixed": {
			severities:   []ruleset.Severity{ruleset.MUST, ruleset.CONSIDER, ruleset.SHOULD},
			wantEnforced: 2, wantTotal: 3, wantAdvisory: false,
		},
		// The shape the whole item exists for: rules present, none examined, zero
		// diagnostics -- indistinguishable from a clean pass until this reports it.
		"none enforced is advisory": {
			severities:   []ruleset.Severity{ruleset.CONSIDER, ruleset.CONSIDER},
			wantEnforced: 0, wantTotal: 2, wantAdvisory: true,
		},
		// An empty ruleset examines nothing but misleads nobody, so it is not the
		// advisory case.
		"empty ruleset is not advisory": {
			severities:   nil,
			wantEnforced: 0, wantTotal: 0, wantAdvisory: false,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			rs := ruleset.Ruleset{}
			for i, sev := range tc.severities {
				rs.Rules = append(rs.Rules, rule(string(rune('a'+i)), sev, "b", "g", "a"))
			}
			got := verify.Rules(rs)
			if got.Enforced != tc.wantEnforced || got.Total != tc.wantTotal {
				t.Errorf("Rules = %+v, want {Enforced:%d Total:%d}",
					got, tc.wantEnforced, tc.wantTotal)
			}
			if got.Advisory() != tc.wantAdvisory {
				t.Errorf("Advisory() = %t, want %t", got.Advisory(), tc.wantAdvisory)
			}
		})
	}
}

func TestCanonical(t *testing.T) {
	t.Parallel()
	const canonical = "Source: demo\nScope:  Go\n\n" +
		"§1.1  [MUST][CODE]  Always close the connection.\n" +
		"      Leaked connections exhaust the pool.\n" +
		"      ✗  // connection is never closed\n" +
		"      ✓  defer conn.Close()\n" +
		"      ↦  ANCHOR-SENTINEL always release the connection\n"
	cases := map[string]struct {
		raw      string
		wantDiag bool
	}{
		// A format-1 ruleset declares no version and must still round-trip; guarding on
		// the version made this check decline on every file in the corpus.
		"canonical form round-trips": {raw: canonical, wantDiag: false},
		// Two spaces after Scope: is the canonical spelling; one is parseable and not
		// canonical, which is exactly the drift nothing detected before.
		"a reformatted header is not canonical": {
			raw:      strings.Replace(canonical, "Scope:  Go", "Scope: Go", 1),
			wantDiag: true,
		},
		"a reindented rationale is not canonical": {
			raw:      strings.Replace(canonical, "      Leaked", "   Leaked", 1),
			wantDiag: true,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			rs, err := ruleset.Parse(tc.raw)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			assertCanonical(t, verify.Canonical(tc.raw, rs), tc.wantDiag)
		})
	}
}

// assertCanonical checks the diagnostic count and, when one is expected, that it blocks.
// Extracted to keep TestCanonical's table flat: three assertions inside a subtest are what
// push it over the complexity cap.
func assertCanonical(t *testing.T, got []finding.Diagnostic, wantDiag bool) {
	t.Helper()
	if !wantDiag {
		if len(got) != 0 {
			t.Fatalf("Canonical = %+v, want none", got)
		}
		return
	}
	if len(got) != 1 {
		t.Fatalf("Canonical = %+v, want one diagnostic", got)
	}
	if !got[0].Severity.Blocking() {
		t.Errorf("severity = %q, want a blocking one", got[0].Severity)
	}
}
