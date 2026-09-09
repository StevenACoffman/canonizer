package cmd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReworkFillsTheCandidateAndEveryFindingsDocument(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	rules := filepath.Join(dir, "r_rules.md")
	verifyFindings := filepath.Join(dir, "verify.json")
	criticFindings := filepath.Join(dir, "critic.json")
	out := filepath.Join(dir, "rework_prompt.md")
	writeFile(t, rules, "Source: s\nScope:  x\n\n§1.1  [MUST][CODE]  Close it.\n      r\n")
	writeFile(t, verifyFindings, `{"diagnostics":[{"category":"unexecutable"}]}`)
	writeFile(t, criticFindings, `{"diagnostics":[{"category":"vague"}]}`)

	if _, err := run(t, "rework", "--ruleset", rules,
		"--findings", verifyFindings, "--findings", criticFindings, "--out", out); err != nil {
		t.Fatalf("rework: %v", err)
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	got := string(body)

	if !strings.Contains(got, "Close it.") {
		t.Error("the prompt does not carry the candidate ruleset")
	}
	// Both documents, because a round produces two and reworking against one resolves half
	// the round.
	for _, want := range []string{"unexecutable", "vague"} {
		if !strings.Contains(got, want) {
			t.Errorf("the prompt does not carry the %q finding", want)
		}
	}
	// Labelled by path: the deterministic check and the grader are not interchangeable.
	for _, want := range []string{verifyFindings, criticFindings} {
		if !strings.Contains(got, want) {
			t.Errorf("the prompt does not say which document %q findings came from", want)
		}
	}
	if strings.Contains(got, "{{") {
		t.Errorf("an unfilled placeholder survived:\n%s", got)
	}
}

func TestReworkRequiresItsInputs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	rules := filepath.Join(dir, "r_rules.md")
	writeFile(t, rules, "Source: s\nScope:  x\n")

	if _, err := run(t, "rework", "--findings", "x.json"); err == nil {
		t.Error("rework accepted a missing --ruleset")
	}
	// A rework with no findings has nothing to resolve, so it is a mistake rather than a
	// no-op: an agent given it would revise on its own judgement.
	if _, err := run(t, "rework", "--ruleset", rules); err == nil {
		t.Error("rework accepted an empty --findings")
	}
	if _, err := run(
		t,
		"rework",
		"--ruleset",
		rules,
		"--findings",
		filepath.Join(dir, "nope.json"),
	); err == nil {
		t.Error("rework accepted a findings path that does not exist")
	}
}
