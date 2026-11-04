package verify

import (
	"regexp"
	"strings"

	"github.com/StevenACoffman/skillet/finding"
	"github.com/StevenACoffman/skillet/ruleset"
)

// escalationNotes are the annotations the synthesis prompt tells an agent to append when it
// settles a disagreement between inputs by taking the stronger position.
//
// Each one claims that two sources disagreed: "one source treats as `[SHOULD]`; stricter
// applied" for a severity, "no distinguishing criterion found -- stricter position applied"
// for a conflict, "sources disagree significantly on severity" for `[MUST]` against
// `[CONSIDER]`. Agents reword the first half freely -- the domain_analysis run wrote "the
// source treats the runnability half as [SHOULD]; stricter applied" -- so only the fixed
// half is matched.
//
// A function rather than package-level vars, the shape unquantifiedTerms uses here and the
// one gochecknoglobals leaves alone.
func escalationNotes() []string {
	return []string{
		"stricter applied",
		"stricter position applied",
		"sources disagree significantly on severity",
	}
}

// escalationPattern matches any of escalationNotes, case-insensitively and across any run
// of whitespace, since a rationale is free to wrap or recapitalise a note it carries.
func escalationPattern() *regexp.Regexp {
	notes := escalationNotes()
	alternatives := make([]string, len(notes))
	for i, note := range notes {
		words := strings.Fields(note)
		for j := range words {
			words[j] = regexp.QuoteMeta(words[j])
		}
		alternatives[i] = strings.Join(words, `\s+`)
	}
	return regexp.MustCompile(`(?i)\b(` + strings.Join(alternatives, "|") + `)\b`)
}

// Escalation returns an advisory diagnostic for every rule carrying a note that two sources
// disagreed, when the ruleset was checked against only one source.
//
// **With one source the note cannot be true, and what it records instead is a defect.** The
// synthesis prompt's severity table maps `[MUST]` + `[SHOULD]` to `[MUST]` for two inputs
// that disagree. Given one distilled input, agents merged pairs of that book's rules and
// applied the table anyway, so the weaker rule took the stronger level. Measured on the 14
// shipped kb rulesets plus the first domain_analysis candidate, this fires on kleppmann (11
// rules), authorization (5) and that candidate (7). All three are single-source, no
// distilled input among them carries the note, and nothing else fires. Critics caught such rules
// one or two at a time
// across rounds, as "stronger than the book".
//
// It belongs with Provenance rather than Structural because it is a claim about sources --
// that there were two -- and only a caller holding the sources can test it. Routed through
// Provenance and Drift rather than called by verify and loop separately, so the two cannot
// disagree about it; Structural's comment records what two call sites cost.
//
// **Advisory, not blocking.** A run may legitimately verify a multi-source synthesis against
// one source; its anchors from the others are then reported absent, which blocks on its
// own. And a level stated more strongly than its source is the critic's `unsupported`
// judgment, which this points at rather than replaces.
//
// Requires: sources holds every source the ruleset was distilled from.
// Ensures:  empty unless len(sources) == 1; at most one diagnostic per rule, each
//
//	finding.SeverityWarning; it is pure.
func Escalation(rs *ruleset.Ruleset, sources []string) []finding.Diagnostic {
	diags := make([]finding.Diagnostic, 0)
	if len(sources) != 1 {
		return diags
	}
	pattern := escalationPattern()
	for i := range rs.Rules {
		r := &rs.Rules[i]
		if pattern.MatchString(r.Statement) || pattern.MatchString(r.Rationale) {
			diags = append(diags, advisory(r, CategoryEscalationSingleSource,
				"rule notes a stricter level applied between sources, but it was checked "+
					"against one source; confirm its level against what that source says"))
		}
	}
	return diags
}
