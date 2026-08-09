package critic_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/canonizer/internal/critic"
	"github.com/StevenACoffman/canonizer/internal/prompt"
)

func TestFillPromptSubstitutesBothMarkers(t *testing.T) {
	t.Parallel()
	got, err := critic.FillPrompt("A {{SOURCE}} B {{RULESET}} C", "SRC", "RULES")
	if err != nil {
		t.Fatalf("FillPrompt: %v", err)
	}
	if want := "A SRC B RULES C"; got != want {
		t.Errorf("FillPrompt = %q, want %q", got, want)
	}
}

func TestFillPromptMissingMarkerIsError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		tmpl string
	}{
		{"missing source", "only {{RULESET}} here"},
		{"missing ruleset", "only {{SOURCE}} here"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := critic.FillPrompt(tt.tmpl, "s", "r"); err == nil {
				t.Error("FillPrompt with a missing marker returned nil; want an error")
			}
		})
	}
}

func TestCriticPromptStatesTheSpecificityTests(t *testing.T) {
	t.Parallel()
	// The three dimensions are a judgment rubric, and canonizer routes judgment to the
	// critic rather than to code. Asserting them on the *filled* prompt, not the file,
	// is the point: this is what the grader is actually handed.
	filled, err := critic.FillPrompt(prompt.Critic, "SOURCE TEXT", "RULESET TEXT")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"handle errors carefully",              // failure mechanism
		"decompose into smaller steps",         // actionable specificity
		"be careful with dangerous operations", // high-risk boundary
	} {
		if !strings.Contains(filled, want) {
			t.Errorf("the critic is not given the anti-example %q", want)
		}
	}
	// And the substitution still works, so the assertions above are on a real prompt.
	if !strings.Contains(filled, "SOURCE TEXT") || !strings.Contains(filled, "RULESET TEXT") {
		t.Error("FillPrompt did not substitute; the checks above proved nothing")
	}
}

func TestCriticPromptKeepsThreeCategories(t *testing.T) {
	t.Parallel()
	// Deepening `vague` must not add a fourth category: the output contract lists three
	// and the severity rule keys on them by name, so a new one would have the model emit
	// something no rule documents — and internal/critic validates nothing to catch it.
	filled, err := critic.FillPrompt(prompt.Critic, "s", "r")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(filled, "one of `unsupported`, `vague`, `duplicate`") {
		t.Error("the category contract changed; the critic's output shape is not validated in code")
	}
}
