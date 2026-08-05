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
