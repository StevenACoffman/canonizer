package cmd_test

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestVerifyStatesWhatItDidNotDo pins VAC's rule for a structural verifier: it never
// performs semantic replay and says so, because a clean structural pass is otherwise read
// as a full one.
func TestVerifyStatesWhatItDidNotDo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	rules := filepath.Join(dir, "rules.md")
	writeFile(t, rules, cleanRuleset)

	_, stderr, err := runIO(t, "verify", "--ruleset", rules)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !strings.Contains(stderr, "semantic replay not performed") {
		t.Errorf("want the unperformed act named, got:\n%s", stderr)
	}
}

// TestVerifyStatesItOnAFailingRunToo keeps the statement unconditional: it describes the
// act, not the verdict, so suppressing it when findings exist would make its presence read
// as a signal about the ruleset.
func TestVerifyStatesItOnAFailingRunToo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	rules := filepath.Join(dir, "rules.md")
	writeFile(t, rules, unexecutableRuleset)

	_, stderr, err := runIO(t, "verify", "--ruleset", rules)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !strings.Contains(stderr, "semantic replay not performed") {
		t.Errorf("want the statement on a run with findings too, got:\n%s", stderr)
	}
}

// TestGateStatesWhatCleanMeans covers the moment the overclaim is made: a zero exit is what
// gets quoted, so the qualification belongs beside it.
func TestGateStatesWhatCleanMeans(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "findings.json")
	writeFile(t, path, `{"diagnostics":[]}`)

	_, stderr, err := runIO(t, "gate", "--findings", path)
	if err != nil {
		t.Fatalf("gate: %v", err)
	}
	if !strings.Contains(stderr, "not that the rules are correct") {
		t.Errorf("want the clean verdict qualified, got:\n%s", stderr)
	}
}

// TestGateDoesNotQualifyABlockedRun keeps the statement where it earns its place: a blocked
// run is in no danger of being read as a pass.
func TestGateDoesNotQualifyABlockedRun(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "findings.json")
	writeFile(t, path, `{"diagnostics":[{"severity":"error","category":"unsupported",
		"path":"§1.1","message":"invented"}]}`)

	_, stderr, err := runIO(t, "gate", "--findings", path)
	if err == nil {
		t.Fatal("a blocking finding should exit non-zero")
	}
	if strings.Contains(stderr, "not that the rules are correct") {
		t.Errorf("the qualification belongs on the clean path only, got:\n%s", stderr)
	}
}
