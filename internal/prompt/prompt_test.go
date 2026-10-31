package prompt_test

import (
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/StevenACoffman/canonizer/internal/prompt"
	errors "github.com/StevenACoffman/toerr/errors"
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
	path := filepath.Join(t.TempDir(), "nope.md")
	_, err := prompt.Resolve(path, "FALLBACK")
	if err == nil {
		t.Fatal("Resolve of an unreadable path returned nil; want an error, not a silent fallback")
	}
	// The read error already names the file; a message that names it again reads as two
	// files to an operator.
	if n := strings.Count(err.Error(), path); n != 1 {
		t.Errorf("error names the path %d times, want once: %v", n, err)
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("error no longer matches fs.ErrNotExist: %v", err)
	}
	if !slices.ContainsFunc(errors.Attrs(err), func(a slog.Attr) bool {
		return a.Key == "path" && a.Value.String() == path
	}) {
		t.Errorf("error does not carry the path as an attribute: %v", errors.Attrs(err))
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
	// Synthesize gained a destination in 2026-09: without it the prompt names no output
	// path and an agent prints the merged ruleset instead of writing one.
	if !strings.Contains(prompt.Synthesize, "{{DESTINATION_CONTENT}}") {
		t.Error("Synthesize default is missing the {{DESTINATION_CONTENT}} marker")
	}
	for _, want := range []string{"{{RULESET}}", "{{FINDINGS}}"} {
		if !strings.Contains(prompt.Rework, want) {
			t.Errorf("Rework default is missing %s", want)
		}
	}
}

// TestTemplatesSpecifyCanonicalForm guards against a template edit that silently
// drops the canonical ruleset format the distill/synthesize prompts must instruct —
// the format skillet/ruleset.Parse reads. It is a token check, not a Parse round-trip
// (skillet tests that); it only catches gross drift.
func TestTemplatesSpecifyCanonicalForm(t *testing.T) {
	t.Parallel()
	// Rework is included because it is the template that writes a ruleset *back*: an edit
	// dropping the form from it would produce a revision that fails Canonical on every
	// round, which is the defect this table exists to catch one layer earlier.
	templates := map[string]string{
		"Distill": prompt.Distill, "Synthesize": prompt.Synthesize, "Rework": prompt.Rework,
	}
	for name, tmpl := range templates {
		for _, tok := range []string{"Source:", "Scope:", "§", "[MUST]", "✗", "✓", "↦"} {
			if !strings.Contains(tmpl, tok) {
				t.Errorf("%s template no longer specifies the canonical token %q", name, tok)
			}
		}
	}
}

// TestTemplatesAskForOnePairPerRule pins the instruction that stopped a ruleset losing text.
// The synthesize template once said to "combine them into two contrast pairs within the
// same rule", which the format cannot hold: skillet's parser kept the second pair and fmt
// deleted the first. The parser now refuses a second ✗, ✓ or ↦ line, so every template
// that writes rules must say so, or an agent following it writes a ruleset nothing parses.
func TestTemplatesAskForOnePairPerRule(t *testing.T) {
	t.Parallel()
	templates := map[string]string{
		"Distill": prompt.Distill, "Synthesize": prompt.Synthesize, "Rework": prompt.Rework,
	}
	const want = "Each rule has **exactly one** `✗` line, one `✓` line and one `↦` line."
	for name, tmpl := range templates {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			flat := strings.Join(strings.Fields(tmpl), " ")
			if !strings.Contains(flat, want) {
				t.Errorf("%s template no longer says %q", name, want)
			}
			if strings.Contains(flat, "two contrast pairs") {
				t.Errorf("%s template still asks for two contrast pairs", name)
			}
		})
	}
}
