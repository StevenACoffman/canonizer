package verify

import (
	"github.com/StevenACoffman/skillet/finding"
	"github.com/StevenACoffman/skillet/ruleset"
)

// Soundness reports the rules whose checks do not discriminate between their own examples.
//
// A rule ships both answers already -- Bad is the case it must flag, Good the case it must
// not -- and until skillet v0.30.0 carried no predicate to run against them, so nothing
// could tell a rule that discriminates from one that would fire on ordinary work.
// gate.SelfTest proves the *gate* discriminates and says nothing about any individual rule;
// this is that control moved to where the rules are.
//
// **Blocking, unlike the other two advisory checks, and the difference is decidability.**
// Specificity and Conflicts report things a deterministic check cannot settle -- a general
// rule is sometimes right, a severity divergence is sometimes a deliberate refinement. This
// one is settled: the checks were run against the rule's own examples and either
// discriminated or did not. There is no judgement left for a reader to supply, which is the
// line Executable and Provenance already sit on.
//
// **Soundness before completeness.** A rule that fires on ordinary work gets the tool
// switched off, so the negative case matters more than the missed one -- and it is the case
// an author will not write unprompted.
//
// A rule carrying no checks is **not** reported. ruleset.Sound already declines to, and
// canonizer must not add a "rules should carry checks" opinion on top: that is a separate
// policy, and no ruleset in this corpus carries a check yet, so it would fire on every rule.
//
// Ensures: one diagnostic per unsound rule, in rule order, each finding.SeverityError; it
//
//	is pure.
func Soundness(rs *ruleset.Ruleset) []finding.Diagnostic {
	unsound := ruleset.Sound(rs)
	diags := make([]finding.Diagnostic, 0, len(unsound))
	for _, u := range unsound {
		diags = append(diags, finding.Diagnostic{
			Severity: finding.SeverityError,
			// Human, as every blocking category here is: repairing an unsound rule means
			// rewriting a predicate or an example, and only someone who knows what the
			// rule means can choose which.
			Action:   finding.ActionHuman,
			Category: CategoryUnsound,
			// skillet emits the bare section; the "§" is this repo's presentation
			// convention, applied here so one verify run does not mix "2.4" and "§2.4".
			Path:    "§" + u.Section,
			Message: u.Reason,
		})
	}
	return diags
}
