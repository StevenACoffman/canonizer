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

// namesNoSymbol reports whether the scope count read the statement as naming no symbol a
// checker can see.
//
// This asked Specificity for an `unspecific` diagnostic until 2026-09-08. The signal is the
// same and its verdict is now a proportion rather than an accusation, so the cases below
// still pin what concrete recognises -- they just read it where it now lives.
func namesNoSymbol(t *testing.T, stmt string) bool {
	t.Helper()
	rs := enforcedRule(stmt)
	return verify.Rules(&rs).Symbolic == 0
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
			if got := namesNoSymbol(t, tc.statement); got != tc.wantFlag {
				t.Errorf("namesNoSymbol = %t, want %t for %q", got, tc.wantFlag, tc.statement)
			}
		})
	}
}

// TestASymbolNamingStatementCanStillHedge keeps the two readings independent. A statement
// may name a symbol -- so the count reads it as symbolic -- and still hedge, which is the
// half that stayed a finding when the other became a proportion.
func TestASymbolNamingStatementCanStillHedge(t *testing.T) {
	t.Parallel()
	rs := enforcedRule("Prefer sql.DB over a raw driver where appropriate.")
	if got := verify.Rules(&rs).Symbolic; got != 1 {
		t.Errorf("Symbolic = %d, want 1: the statement names sql.DB", got)
	}
	got := verify.Softening(&rs)
	if len(got) != 1 {
		t.Fatalf("Softening = %+v, want the hedge reported", got)
	}
}

// TestAnchorMatchesTheQuotedSpan pins the defect measured on the first real ruleset: the
// distill prompt asks for `§Section: "quote"`, so searching the source for the whole anchor
// failed on every anchor ever written -- 0 of 26 matched while 11 quoted the article
// verbatim.
func TestAnchorMatchesTheQuotedSpan(t *testing.T) {
	t.Parallel()
	const source = "In practice we define our services with an interface in the root package."
	cases := map[string]struct {
		anchor    string
		wantFound bool
	}{
		// The documented form, and the one that used to fail.
		"section prefix plus quotation": {
			anchor:    `§The interface: "we define our services with an interface"`,
			wantFound: true,
		},
		// The fallback: a bare quotation with no prefix still works.
		"bare quotation with no prefix": {
			anchor:    "we define our services with an interface",
			wantFound: true,
		},
		// Soundness: a prefix must not launder a quotation the source does not contain.
		"section prefix with a fabricated quotation": {
			anchor:    `§The interface: "we forbid interfaces in the root package"`,
			wantFound: false,
		},
		// A section reference carrying no quotation cannot be verbatim-matched, and the
		// prompt permits one. It reads as absent today; see the TODO entry.
		"section reference with no quotation": {
			anchor:    "§The interface",
			wantFound: false,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			rs := ruleset.Ruleset{Rules: []ruleset.Rule{{
				Section: "1.1", Severity: ruleset.MUST, Level: ruleset.CODE,
				Statement: "do the thing", Bad: "b", Good: "g", SourceAnchor: tc.anchor,
			}}}
			found := len(verify.Provenance(&rs, source)) == 0
			if found != tc.wantFound {
				t.Errorf("anchor found = %t, want %t for %q", found, tc.wantFound, tc.anchor)
			}
		})
	}
}
