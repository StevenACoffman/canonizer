// Package budget is canonizer's rework-budget decision: given whether a ruleset
// still has blocking findings and where it sits in the refine loop, it returns
// whether to ship, rework, or escalate to a human. Decide is pure; the command maps
// the verdict to an exit code. The invariant: a ruleset with blocking findings is
// never Ship.
//
// The decision is findings-based, never a model self-score: a self-assessed score is
// inflated, so the ship gate is the cold critic plus the deterministic findings gate,
// not the producer grading itself (the refinement policy in `canonizer --help`). Do
// not add a self-score threshold here.
package budget

// The rework verdicts (also the words the command prints).
const (
	Ship       Verdict = "ship"        // no blocking findings — adopt the ruleset
	Rework     Verdict = "rework"      // blocked, budget remains — refine and retry
	NeedsHuman Verdict = "needs-human" // blocked, budget spent — escalate, never ship
)

// Verdict is the terminal decision of the rework budget.
type Verdict string

// Decide returns the rework verdict. It is Ship only when nothing blocks; a blocked
// ruleset is Rework while attempts remain (attempt < limit) and NeedsHuman once the
// budget is spent (attempt >= limit) — so a blocked ruleset is never shipped. attempt
// is the attempt just completed (1-based); limit is the total attempts allowed.
func Decide(blocking bool, attempt, limit int) Verdict {
	switch {
	case !blocking:
		return Ship
	case attempt < limit:
		return Rework
	default:
		return NeedsHuman
	}
}
