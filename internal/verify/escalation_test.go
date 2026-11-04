package verify_test

import (
	"testing"

	"github.com/StevenACoffman/canonizer/internal/verify"
	"github.com/StevenACoffman/skillet/ruleset"
)

// noted returns a one-rule ruleset whose statement and rationale are the given text.
func noted(sev ruleset.Severity, statement, rationale string) ruleset.Ruleset {
	return ruleset.Ruleset{Rules: []ruleset.Rule{{
		Section: "4.1", Severity: sev, Level: ruleset.ARCH,
		Statement: statement, Rationale: rationale, Bad: "b", Good: "g",
		SourceAnchor: `§Abstraction: "enough semantics"`,
	}}}
}

func TestAnEscalationNoteIsReportedOnlyAgainstOneSource(t *testing.T) {
	t.Parallel()

	const book = "# Abstraction\n\nYour abstractions must have enough semantics.\n"
	const other = "# Elsewhere\n\nSomething else entirely.\n"

	cases := map[string]struct {
		rs      ruleset.Ruleset
		sources []string
		want    string
	}{
		// The shape the domain_analysis run shipped: the note in the rationale, its first
		// half reworded from the prompt's.
		"a reworded severity note in the rationale": {
			rs: noted(ruleset.MUST, "Give the abstractions semantics.",
				"Structure alone cannot be executed (the source treats the runnability "+
					"half as [SHOULD]; stricter applied)."),
			sources: []string{book},
			want:    verify.CategoryEscalationSingleSource,
		},
		"the prompt's conflict note in the statement": {
			rs: noted(ruleset.SHOULD, "Prefer A (no distinguishing criterion found — "+
				"stricter position applied).", "r"),
			sources: []string{book},
			want:    verify.CategoryEscalationSingleSource,
		},
		"the MUST-against-CONSIDER note, recapitalised and wrapped": {
			rs: noted(ruleset.MUST, "Do A.",
				"Sources Disagree\nsignificantly on severity."),
			sources: []string{book},
			want:    verify.CategoryEscalationSingleSource,
		},
		// Advisory rules are read too: the note misstates the source whatever the level.
		"a note on a CONSIDER rule": {
			rs:      noted(ruleset.CONSIDER, "Consider A (stricter applied).", "r"),
			sources: []string{book},
			want:    verify.CategoryEscalationSingleSource,
		},
		// The case the note was written for.
		"two sources, where the note can be true": {
			rs:      noted(ruleset.MUST, "Do A (stricter applied).", "r"),
			sources: []string{book, other},
			want:    "",
		},
		// No source means nothing was checked against, which Unverifiable reports.
		"no source": {
			rs:      noted(ruleset.MUST, "Do A (stricter applied).", "r"),
			sources: nil,
			want:    "",
		},
		"no note": {
			rs:      noted(ruleset.MUST, "Apply the stricter of the two limits.", "r"),
			sources: []string{book},
			want:    "",
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, blocking := categoryOf(t, verify.Escalation(&c.rs, c.sources))
			if got != c.want {
				t.Errorf("category = %q, want %q", got, c.want)
			}
			if blocking {
				t.Errorf("category %q blocks; it must stay advisory", got)
			}
		})
	}
}

// TestVerifyAndLoopBothReportAnEscalationNote pins the routing: loop calls Provenance and
// verify calls Drift, so the check must reach both or the two disagree about a ruleset.
func TestVerifyAndLoopBothReportAnEscalationNote(t *testing.T) {
	t.Parallel()

	const book = "# Abstraction\n\nYour abstractions must have enough semantics.\n"
	rs := noted(ruleset.MUST, "Do A (stricter applied).", "r")

	cases := map[string]func(*testing.T) string{
		"Provenance, as loop calls it": func(t *testing.T) string {
			t.Helper()
			got, _ := categoryOf(t, verify.Provenance(&rs, []string{book}))
			return got
		},
		"Drift with an unchanged source, as verify calls it": func(t *testing.T) string {
			t.Helper()
			got, _ := categoryOf(t, verify.Drift(&rs, []string{book}, verify.SourceUnchanged))
			return got
		},
	}
	for name, run := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := run(t); got != verify.CategoryEscalationSingleSource {
				t.Errorf("category = %q, want %q", got, verify.CategoryEscalationSingleSource)
			}
		})
	}
}
