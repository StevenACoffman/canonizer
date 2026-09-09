package verify_test

import (
	"testing"

	"github.com/StevenACoffman/canonizer/internal/verify"
	"github.com/StevenACoffman/skillet/finding"
	"github.com/StevenACoffman/skillet/ruleset"
)

func TestUnquantifiedFlagsAnUnstatedAmount(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		statement string
		wantFlag  bool
	}{
		// The five real statements from batch two, verbatim in substance. All are the class
		// the retired unspecific entry defended as genuinely soft.
		"a threshold hedged with roughly": {
			statement: "Keep an application in a single package only while it stays under " +
				"roughly 10K SLOC.",
			wantFlag: true,
		},
		"an unquantified count": {
			statement: "Do not create a subpackage because a package has accumulated too " +
				"many files or types.",
			wantFlag: true,
		},
		"a judgement of relatedness": {
			statement: "Default to a single root package when a project's types are closely " +
				"related.",
			wantFlag: true,
		},
		"an unstated ordering criterion": {
			statement: "Order declarations within a file by importance.",
			wantFlag:  true,
		},
		"an unstated sufficiency": {
			statement: "Add unit tests where the dependency is expensive enough to matter.",
			wantFlag:  true,
		},

		// The two statements that killed the wider vocabulary. Both name concrete symbols
		// and are actionable; "small set" is ordinary English, not an unstated threshold.
		"a small set of named codes is not a hedge": {
			statement: "Start from a small set of generic error codes — `ECONFLICT`, " +
				"`EINTERNAL`, `EINVALID`, `ENOTFOUND`.",
			wantFlag: false,
		},
		"a small set of named helpers is not a hedge": {
			statement: "Cut assertion verbosity with a small set of helpers defined in " +
				"your own repository — `assert(tb testing.TB, condition bool)`.",
			wantFlag: false,
		},

		// `important` was in the rejected list, so the word boundary on `importance` has to
		// be real rather than assumed.
		"important is not importance": {
			statement: "Document the important error codes on every exported method.",
			wantFlag:  false,
		},
		"a definite instruction is silent": {
			statement: "Call `ctx.Done()` before returning from the handler.",
			wantFlag:  false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := verify.Unquantified(&ruleset.Ruleset{
				Rules: []ruleset.Rule{stated("1", ruleset.MUST, tc.statement)},
			})
			if !tc.wantFlag {
				if len(got) != 0 {
					t.Errorf("flagged a statement with no unstated amount: %+v", got)
				}
				return
			}
			if len(got) != 1 {
				t.Fatalf("want one finding, got %+v", got)
			}
			if got[0].Category != "unquantified" {
				t.Errorf("category = %q, want unquantified", got[0].Category)
			}
		})
	}
}

// TestUnquantifiedAndSofteningAreDisjointVocabularies is the claim that justifies two checks
// rather than one widened list: a discretion phrase says *you may choose*, an unquantified
// amount says *some unstated quantity*, and on batch two they overlapped on none of 137.
func TestUnquantifiedAndSofteningAreDisjointVocabularies(t *testing.T) {
	t.Parallel()

	discretion := ruleset.Ruleset{Rules: []ruleset.Rule{
		stated("1", ruleset.MUST, "Keep the root package free of subpackage imports; it depends."),
	}}
	if got := verify.Unquantified(&discretion); len(got) != 0 {
		t.Errorf("Unquantified claimed a discretion phrase: %+v", got)
	}
	if got := verify.Softening(&discretion); len(got) != 1 {
		t.Errorf("Softening = %+v, want the discretion phrase reported", got)
	}

	amount := ruleset.Ruleset{Rules: []ruleset.Rule{
		stated("1", ruleset.MUST, "Split a package once it grows too many files."),
	}}
	if got := verify.Softening(&amount); len(got) != 0 {
		t.Errorf("Softening claimed an unquantified amount: %+v", got)
	}
	if got := verify.Unquantified(&amount); len(got) != 1 {
		t.Errorf("Unquantified = %+v, want the amount reported", got)
	}
}

func TestUnquantifiedIsNeverBlockingAndSkipsUnenforcedRules(t *testing.T) {
	t.Parallel()

	// A rule with an unstated threshold is sometimes exactly right and no deterministic
	// check can tell which, which is why this may never join the checks that stop a ship.
	got := verify.Unquantified(&ruleset.Ruleset{Rules: []ruleset.Rule{
		stated("1", ruleset.MUST, "Split a file once it grows too many types."),
	}})
	if len(got) == 0 {
		t.Fatal("expected a finding; the test proves nothing otherwise")
	}
	for _, d := range got {
		if d.Severity != finding.SeverityWarning || d.Severity.Blocking() {
			t.Errorf("finding %+v could block a ship", d)
		}
	}

	unenforced := verify.Unquantified(&ruleset.Ruleset{Rules: []ruleset.Rule{
		stated("1", ruleset.CONSIDER, "Split a file once it grows too many types."),
	}})
	if len(unenforced) != 0 {
		t.Errorf("flagged an unenforced rule: %+v", unenforced)
	}
}
