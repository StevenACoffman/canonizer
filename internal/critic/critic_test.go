package critic_test

import (
	"testing"

	"github.com/StevenACoffman/canonizer/internal/critic"
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
