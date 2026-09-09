package verify_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/canonizer/internal/verify"
	"github.com/StevenACoffman/skillet/finding"
	"github.com/StevenACoffman/skillet/judge"
	"github.com/StevenACoffman/skillet/ruleset"
)

// ruleWithCheck builds an enforced rule carrying one contains-check and both examples.
func ruleWithCheck(bad, good, arg string) ruleset.Rule {
	return ruleset.Rule{
		Section: "1.1", Severity: ruleset.MUST, Level: ruleset.CODE,
		Statement: "close it", Bad: bad, Good: good,
		Checks: []judge.Check{{Op: judge.OpContains, Arg: arg}},
	}
}

func TestSoundness(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		rule     ruleset.Rule
		wantDiag bool
	}{
		"a discriminating check passes": {
			rule: ruleWithCheck("conn.Close() // never deferred", "defer conn.Close()",
				"// never deferred"),
		},
		// The measured trap: ✓ contains ✗ in the commonest shape a fix takes, so a check
		// written as "contains the bad text" passes on both and discriminates nothing.
		"a check passing on both examples is unsound": {
			rule:     ruleWithCheck("conn.Close()", "defer conn.Close()", "conn.Close()"),
			wantDiag: true,
		},
		"a check missing its own bad example is unsound": {
			rule:     ruleWithCheck("conn.Close()", "defer conn.Close()", "ABSENT"),
			wantDiag: true,
		},
		// Untested is not unsound, and canonizer must not add that opinion: no ruleset in
		// this corpus carries a check, so it would fire on every rule.
		"a rule with no checks is not reported": {
			rule: ruleset.Rule{
				Section: "1.1", Severity: ruleset.MUST, Level: ruleset.CODE,
				Statement: "close it", Bad: "a", Good: "b",
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assertSoundness(t,
				verify.Soundness(&ruleset.Ruleset{Rules: []ruleset.Rule{tc.rule}}),
				tc.wantDiag)
		})
	}
}

// TestSoundnessReadsADocument closes the loop: the checks it runs come off the wire format
// skillet v0.30.0 added, not out of a hand-built struct.
func TestSoundnessReadsADocument(t *testing.T) {
	t.Parallel()
	doc := "---\nformat: 3\n---\nSource: s\nScope:  Go\n\n" +
		"§1.1  [MUST][CODE]  Close it.\n" +
		"      ✗  conn.Close()\n" +
		"      ✓  defer conn.Close()\n" +
		"      ⊨  contains  conn.Close()\n"
	rs, err := ruleset.Parse(doc)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	got := verify.Soundness(&rs)
	if len(got) != 1 {
		t.Fatalf("Soundness = %+v, want the non-discriminating rule flagged", got)
	}
	if !strings.Contains(got[0].Message, "discriminate") {
		t.Errorf("Message = %q, want it to say the checks do not discriminate", got[0].Message)
	}
}

// assertSoundness checks the single finding, or that there is none. Extracted to keep the
// table flat: the sound branch plus three assertions inside a subtest is what pushes it over
// the complexity cap.
func assertSoundness(t *testing.T, got []finding.Diagnostic, wantDiag bool) {
	t.Helper()
	if !wantDiag {
		if len(got) != 0 {
			t.Fatalf("Soundness = %+v, want none", got)
		}
		return
	}
	if len(got) != 1 {
		t.Fatalf("Soundness = %+v, want one finding", got)
	}
	// Blocking is the point: unlike Specificity and Conflicts, this one is settled -- the
	// checks ran against the rule's own examples and the answer is not a judgement a reader
	// supplies.
	if !got[0].Severity.Blocking() {
		t.Errorf("severity = %q, want a blocking one", got[0].Severity)
	}
	if got[0].Path != "§1.1" {
		t.Errorf("Path = %q, want this repo's § convention", got[0].Path)
	}
	if !strings.Contains(got[0].Category, "unsound") {
		t.Errorf("Category = %q, want unsound", got[0].Category)
	}
}
