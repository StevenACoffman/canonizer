package verify_test

import (
	"testing"

	"github.com/StevenACoffman/canonizer/internal/verify"
	"github.com/StevenACoffman/skillet/ruleset"
)

// enforcedRule builds a MUST rule carrying stmt, with a discriminating pair so only
// Specificity has anything to say about it.
func enforcedRule(stmt string) ruleset.Ruleset {
	return ruleset.Ruleset{Rules: []ruleset.Rule{{
		Section: "1.1", Severity: ruleset.MUST, Level: ruleset.CODE,
		Statement: stmt, Bad: "b", Good: "g", SourceAnchor: "a",
	}}}
}

// unspecific reports whether Specificity flagged the statement as naming nothing.
func unspecific(t *testing.T, stmt string) bool {
	t.Helper()
	rs := enforcedRule(stmt)
	for _, d := range verify.Specificity(&rs) {
		if d.Category == verify.CategoryUnspecific {
			return true
		}
	}
	return false
}

func TestConcreteness(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		statement string
		wantFlag  bool
	}{
		// The cheapest test, and the one that answered most statements before widening.
		"a backticked symbol is concrete": {
			statement: "Confine every reference to `database/sql` to one package.",
		},
		// The measured defect: real distillations name symbols in plain prose, and every
		// one of these read as naming nothing.
		"an import path in bare prose is concrete": {
			statement: "Confine every reference to database/sql to one package.",
		},
		"a qualified name in bare prose is concrete": {
			statement: "Define a package-owned DB type that wraps sql.DB.",
		},
		"a call in bare prose is concrete": {
			statement: "Expose Open() and Close() on the wrapper type.",
		},
		"a pointer type in bare prose is concrete": {
			statement: "Keep the wrapped *sql.DB unexported.",
		},
		// Soundness: the check must still catch a rule that names nothing at all. These
		// are the shapes it flagged in the well-formatted ruleset, and widening left them
		// flagged — accuracy bought without going blind.
		"a vague qualifier is still flagged": {
			statement: "Create a subpackage only for a real dependency boundary.", wantFlag: true,
		},
		"an unquantified threshold is still flagged": {
			statement: "Order a file's declarations by importance.", wantFlag: true,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := unspecific(t, tc.statement); got != tc.wantFlag {
				t.Errorf("unspecific = %t, want %t for %q", got, tc.wantFlag, tc.statement)
			}
		})
	}
}

// TestWideningDoesNotSilenceTheSofteningCheck keeps the two signals independent: a hedged
// statement is caught by SofteningPhrases before concreteness is asked, and widening the
// second must not have made the first unreachable.
func TestWideningDoesNotSilenceTheSofteningCheck(t *testing.T) {
	t.Parallel()
	// Names a real symbol *and* hedges: concrete by the widened test, still hedged.
	rs := enforcedRule("Prefer sql.DB over a raw driver where appropriate.")
	got := verify.Specificity(&rs)
	if len(got) != 1 {
		t.Fatalf("Specificity = %+v, want the hedge reported", got)
	}
	if got[0].Category == verify.CategoryUnspecific {
		t.Errorf("category = %q; a hedged statement naming a symbol is softening, "+
			"not unspecific", got[0].Category)
	}
}
