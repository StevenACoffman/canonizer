package critic_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/canonizer/internal/critic"
	"github.com/StevenACoffman/canonizer/internal/prompt"
)

func TestFillPromptSubstitutesEveryMarker(t *testing.T) {
	t.Parallel()
	got, err := critic.FillPrompt(
		"A {{SOURCE}} B {{RULESET}} C {{DESTINATION_CONTENT}} D", "SRC", "RULES", "DEST")
	if err != nil {
		t.Fatalf("FillPrompt: %v", err)
	}
	if want := "A SRC B RULES C DEST D"; got != want {
		t.Errorf("FillPrompt = %q, want %q", got, want)
	}
}

func TestFillPromptMissingMarkerIsError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		tmpl string
	}{
		{"missing source", "only {{RULESET}} and {{DESTINATION_CONTENT}} here"},
		{"missing ruleset", "only {{SOURCE}} and {{DESTINATION_CONTENT}} here"},
		// A prompt with nowhere to write leaves the grader printing its findings into a
		// transcript, which is how the pipeline's first run lost its artifacts.
		{"missing destination", "only {{SOURCE}} and {{RULESET}} here"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := critic.FillPrompt(tt.tmpl, "s", "r", "d"); err == nil {
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
	filled, err := critic.FillPrompt(prompt.Critic, "SOURCE TEXT", "RULESET TEXT", "DEST")
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

func TestCriticPromptStatesItsCategoryContract(t *testing.T) {
	t.Parallel()
	// The output contract names the categories and the severity rule keys on them, so a
	// category the prompt does not document is one the model can emit and nothing reads --
	// internal/critic validates no reply, so this file is the only place it is pinned.
	//
	// **This assertion used to be a substring of the category line, and that was useless.**
	// It read `Contains("one of `+"`unsupported`, `vague`, `duplicate`"+`")`, which stays
	// true when a fourth category is appended -- so the test that existed to catch exactly
	// that change passed when `coverage` was added on 2026-09-05. Asserting the whole line
	// is what makes it a pin.
	filled, err := critic.FillPrompt(prompt.Critic, "s", "r", "d")
	if err != nil {
		t.Fatal(err)
	}
	const contract = "`\"category\"` is one of `unsupported`, `vague`, `duplicate`, `coverage`"
	if !strings.Contains(filled, contract) {
		t.Errorf("the category contract changed; want the line %q", contract)
	}
	// Every category the contract names must also be defined above it, or the model is
	// told a word the prompt never explains.
	for _, category := range []string{"unsupported", "vague", "duplicate", "coverage"} {
		if !strings.Contains(filled, "- **"+category+"** —") {
			t.Errorf("category %q is named in the output contract but never defined", category)
		}
	}
}

// TestCriticPromptSaysDeclaringAGapIsFree pins the sentence the coverage record depends on.
// The mechanism turns on the critic believing a declared gap is not held against it; stated
// only in code, it would never reach the model that has to act on it.
func TestCriticPromptSaysDeclaringAGapIsFree(t *testing.T) {
	t.Parallel()
	filled, err := critic.FillPrompt(prompt.Critic, "s", "r", "d")
	if err != nil {
		t.Fatal(err)
	}
	// Compared with whitespace collapsed: the prompt is prose and its line wrapping is
	// incidental, so a re-wrap must not fail a test about what it says.
	flat := strings.Join(strings.Fields(filled), " ")
	for _, want := range []string{
		"Declaring a gap costs you nothing",
		"never counted against them",
		"cannot cause the ruleset to be rejected",
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("the prompt no longer tells the critic %q", want)
		}
	}
}
