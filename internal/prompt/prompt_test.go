package prompt_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/canonizer/internal/prompt"
)

func TestResolveDefault(t *testing.T) {
	t.Parallel()
	got, err := prompt.Resolve("", "FALLBACK")
	if err != nil || got != "FALLBACK" {
		t.Fatalf(`Resolve("", "FALLBACK") = %q, %v; want "FALLBACK", nil`, got, err)
	}
}

func TestResolveReadsFile(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "override.md")
	if err := os.WriteFile(path, []byte("FROM FILE"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	got, err := prompt.Resolve(path, "FALLBACK")
	if err != nil || got != "FROM FILE" {
		t.Fatalf("Resolve(path, ...) = %q, %v; want %q, nil", got, err, "FROM FILE")
	}
}

func TestResolveMissingPathIsError(t *testing.T) {
	t.Parallel()
	if _, err := prompt.Resolve(filepath.Join(t.TempDir(), "nope.md"), "FALLBACK"); err == nil {
		t.Fatal("Resolve of an unreadable path returned nil; want an error, not a silent fallback")
	}
}

func TestEmbeddedDefaultsCarryPlaceholders(t *testing.T) {
	t.Parallel()
	for _, want := range []string{"{{SOURCE_CONTENT}}", "{{DESTINATION_CONTENT}}"} {
		if !strings.Contains(prompt.Distill, want) {
			t.Errorf("Distill default is missing %s", want)
		}
	}
	if !strings.Contains(prompt.Synthesize, "{{RULESETS}}") {
		t.Error("Synthesize default is missing the {{RULESETS}} marker")
	}
}

// TestTemplatesSpecifyCanonicalForm guards against a template edit that silently
// drops the canonical ruleset format the distill/synthesize prompts must instruct —
// the format skillet/ruleset.Parse reads. It is a token check, not a Parse round-trip
// (skillet tests that); it only catches gross drift.
func TestTemplatesSpecifyCanonicalForm(t *testing.T) {
	t.Parallel()
	templates := map[string]string{"Distill": prompt.Distill, "Synthesize": prompt.Synthesize}
	for name, tmpl := range templates {
		for _, tok := range []string{"Source:", "Scope:", "§", "[MUST]", "✗", "✓", "↦"} {
			if !strings.Contains(tmpl, tok) {
				t.Errorf("%s template no longer specifies the canonical token %q", name, tok)
			}
		}
	}
}
