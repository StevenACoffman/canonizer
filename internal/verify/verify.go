// Package verify runs canonizer's deterministic checks over a parsed ruleset:
// Executable (B) requires each enforced rule to carry a discriminating ✗/✓ pair, and
// Provenance (E) requires each enforced rule to cite a source anchor that appears in
// the source. Both are pure and return skillet/finding diagnostics. The semantic
// judgments — does a pair actually flip a verdict, does an anchor support the claim —
// stay the cold critic's job; this is the deterministic floor beneath them.
package verify

import (
	"strings"

	"github.com/StevenACoffman/skillet/finding"
	"github.com/StevenACoffman/skillet/judge"
	"github.com/StevenACoffman/skillet/ruleset"
	errors "github.com/StevenACoffman/toerr/errors"
)

// Executable returns a diagnostic for every enforced rule that lacks a discriminating
// ✗/✓ pair (B): one with no ✗ or no ✓, or one whose ✓ preferred form appears verbatim
// inside its ✗ counter-example (so the pair demonstrates no change). It scores the
// pair with skillet/judge; whether the pair semantically flips a verdict is the
// critic's judgment, not this gate's.
func Executable(rs ruleset.Ruleset) ([]finding.Diagnostic, error) {
	diags := make([]finding.Diagnostic, 0)
	for i := range rs.Rules {
		r := &rs.Rules[i]
		if !enforced(r.Severity) {
			continue
		}
		if r.Bad == "" || r.Good == "" {
			diags = append(diags, diag(r, "unexecutable", "rule has no discriminating ✗/✓ pair"))
			continue
		}
		score, err := judge.Score(r.Bad, []judge.Check{{Op: judge.OpContains, Arg: r.Good}})
		if err != nil {
			return nil, errors.WrapWithMessage(err, "verify: judge")
		}
		if score.Hard == 1.0 {
			diags = append(diags, diag(r, "non-discriminating",
				"the ✓ form appears inside the ✗ example; the pair shows no change"))
		}
	}
	return diags, nil
}

// Provenance returns a diagnostic for every enforced rule that cites no source anchor,
// or whose anchor is absent from the source (E). The search is whitespace-normalized
// so a quote the model re-wrapped still matches. Whether a present anchor *supports*
// the claim is the critic's `unsupported` judgment.
func Provenance(rs ruleset.Ruleset, source string) []finding.Diagnostic {
	haystack := normalize(source)
	diags := make([]finding.Diagnostic, 0)
	for i := range rs.Rules {
		r := &rs.Rules[i]
		if !enforced(r.Severity) {
			continue
		}
		if r.SourceAnchor == "" {
			diags = append(diags, diag(r, "no-anchor", "rule cites no source anchor"))
			continue
		}
		if !strings.Contains(haystack, normalize(r.SourceAnchor)) {
			diags = append(
				diags,
				diag(r, "anchor-absent", "source anchor is not present in the source"),
			)
		}
	}
	return diags
}

// enforced reports whether a rule's severity is gated. MUST/SHOULD are enforced;
// CONSIDER is advisory and exempt from both checks.
func enforced(sev ruleset.Severity) bool {
	return sev == ruleset.MUST || sev == ruleset.SHOULD
}

// diag builds an error-severity diagnostic located at rule r.
func diag(r *ruleset.Rule, category, message string) finding.Diagnostic {
	return finding.Diagnostic{
		Severity: finding.SeverityError,
		Category: category,
		Path:     "§" + r.Section,
		Message:  message,
	}
}

// normalize collapses every run of whitespace to a single space.
func normalize(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
