package budget_test

import (
	"testing"

	"github.com/StevenACoffman/canonizer/internal/budget"
)

func TestDecide(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		blocking bool
		attempt  int
		limit    int
		want     budget.Verdict
	}{
		{"clean ships", false, 1, 3, budget.Ship},
		{"clean ships even past budget", false, 9, 3, budget.Ship},
		{"blocked with budget reworks", true, 1, 3, budget.Rework},
		{"blocked just under limit reworks", true, 2, 3, budget.Rework},
		{"blocked at limit escalates", true, 3, 3, budget.NeedsHuman},
		{"blocked past limit escalates", true, 5, 3, budget.NeedsHuman},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := budget.Decide(tt.blocking, tt.attempt, tt.limit)
			if got != tt.want {
				t.Errorf("Decide(%v, %d, %d) = %q, want %q",
					tt.blocking, tt.attempt, tt.limit, got, tt.want)
			}
		})
	}
}

// TestDecideBlockingNeverShips pins the ship-gate invariant across the whole
// attempt/limit grid: a blocked ruleset is never Ship, and a clean one always is,
// regardless of where it sits in the loop. This is the enforcement behind the
// refinement policy — a self-score wired into Decide could only break the tool's
// "independent grader, not self-assessment" thesis by shipping a blocked ruleset or
// blocking a clean one, and this sweep fails loudly if a later change ever does.
func TestDecideBlockingNeverShips(t *testing.T) {
	t.Parallel()
	for attempt := -1; attempt <= 5; attempt++ {
		for limit := 0; limit <= 5; limit++ {
			if got := budget.Decide(true, attempt, limit); got == budget.Ship {
				t.Errorf(
					"Decide(true, %d, %d) = Ship; a blocked ruleset must never ship",
					attempt,
					limit,
				)
			}
			if got := budget.Decide(false, attempt, limit); got != budget.Ship {
				t.Errorf("Decide(false, %d, %d) = %q, want Ship; a clean ruleset always ships",
					attempt, limit, got)
			}
		}
	}
}
