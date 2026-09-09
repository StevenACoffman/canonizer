package cmd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// criticFixture writes a source and a candidate ruleset and returns their paths.
func criticFixture(t *testing.T) (source, rules string) {
	t.Helper()
	dir := t.TempDir()
	source = filepath.Join(dir, "s.md")
	rules = filepath.Join(dir, "r_rules.md")
	writeFile(t, source, "# S\n\nAlways close what you opened.\n")
	writeFile(t, rules, "Source: s\nScope:  x\n\n§1.1  [MUST][CODE]  Close it.\n      r\n")
	return source, rules
}

// TestCriticNamesWhereTheGraderWritesItsFindings is the third instance of one gap: distill
// and synthesize each tell the agent where its artifact goes, and critic did not, so the
// driver appended a destination with a heredoc.
func TestCriticNamesWhereTheGraderWritesItsFindings(t *testing.T) {
	t.Parallel()
	source, rules := criticFixture(t)
	out := filepath.Join(t.TempDir(), "critic_prompt.md")
	findings := filepath.Join(t.TempDir(), "critic_findings_2.json")

	if _, err := run(t, "critic", "--source", source, "--ruleset", rules,
		"--findingsout", findings, "--out", out); err != nil {
		t.Fatalf("critic: %v", err)
	}
	body, readErr := os.ReadFile(out)
	if readErr != nil {
		t.Fatalf("ReadFile: %v", readErr)
	}
	got := string(body)
	if !strings.Contains(got, "<destination>"+findings+"</destination>") {
		t.Errorf("the prompt does not name the findings destination:\n%s", got)
	}
	if strings.Contains(got, "{{") {
		t.Errorf("an unfilled placeholder survived:\n%s", got)
	}
}

// TestCriticWithoutAFindingsDestinationStillProducesAUsablePrompt keeps the flag optional:
// an empty --findingsout states today's behaviour rather than leaving a raw marker in front
// of a grader.
func TestCriticWithoutAFindingsDestinationStillProducesAUsablePrompt(t *testing.T) {
	t.Parallel()
	source, rules := criticFixture(t)
	out := filepath.Join(t.TempDir(), "critic_prompt.md")

	_, err := run(t, "critic", "--source", source, "--ruleset", rules, "--out", out)
	if err != nil {
		t.Fatalf("critic: %v", err)
	}
	body, readErr := os.ReadFile(out)
	if readErr != nil {
		t.Fatalf("ReadFile: %v", readErr)
	}
	got := string(body)
	if strings.Contains(got, "{{") {
		t.Errorf("an unfilled placeholder survived:\n%s", got)
	}
	if !strings.Contains(got, "No destination was given") {
		t.Errorf("the prompt does not say that no output path was supplied:\n%s", got)
	}
}

// TestCriticGradesAgainstEverySource pins the repeatable flag: a synthesized ruleset derives
// from every document it was merged from, and a grader given fewer would report unsupported
// rules that are in fact supported.
func TestCriticGradesAgainstEverySource(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	first := filepath.Join(dir, "a.md")
	second := filepath.Join(dir, "b.md")
	rules := filepath.Join(dir, "r_rules.md")
	out := filepath.Join(dir, "critic_prompt.md")
	writeFile(t, first, "# Alpha\n\nAlways close what you opened.\n")
	writeFile(t, second, "# Beta\n\nNever ignore a returned error.\n")
	writeFile(t, rules, "Source: s\nScope:  x\n\n§1.1  [MUST][CODE]  Close it.\n      r\n")

	if _, err := run(t, "critic", "--source", first, "--source", second,
		"--ruleset", rules, "--out", out); err != nil {
		t.Fatalf("critic: %v", err)
	}
	body, readErr := os.ReadFile(out)
	if readErr != nil {
		t.Fatalf("ReadFile: %v", readErr)
	}
	got := string(body)
	for _, want := range []string{"Always close what you opened", "Never ignore a returned error"} {
		if !strings.Contains(got, want) {
			t.Errorf("the prompt is missing %q", want)
		}
	}
	// Labelled, because two documents in one block are otherwise indistinguishable.
	if !strings.Contains(got, first) || !strings.Contains(got, second) {
		t.Errorf("the prompt does not name which document is which:\n%s", got)
	}
}
