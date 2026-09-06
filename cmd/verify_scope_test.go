package cmd_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/skillet/finding"
)

// advisoryOnlyRuleset holds a rule the enforced-rule gates exempt, so they examine nothing
// and emit nothing. That silence is what this item exists to break.
const advisoryOnlyRuleset = "Source: demo\nScope:  Go\n\n" +
	"§1.1  [CONSIDER][CODE]  Prefer the shorter form.\n" +
	"      It reads better.\n"

// decodeFindings parses a verify run's stdout as a findings document.
func decodeFindings(t *testing.T, stdout string) finding.Result {
	t.Helper()
	var result finding.Result
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode findings: %v\n%s", err, stdout)
	}
	return result
}

// TestVerifyReportsScopeOnEveryRun pins the count as always-present rather than
// bad-day-only: a reader who first meets the proportion when it is wrong has no baseline.
func TestVerifyReportsScopeOnEveryRun(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	rulesPath := filepath.Join(dir, "rules.md")
	writeFile(t, rulesPath, cleanRuleset)

	_, stderr, err := runIO(t, "verify", "--ruleset", rulesPath)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !strings.Contains(stderr, "examined 1 of 1 rule(s); 0 exempt as advisory") {
		t.Errorf("want the scope line on a fully-enforced ruleset, got:\n%s", stderr)
	}
}

// TestVerifyFlagsARulesetItExaminedNothingOf is the measured defect: rules present, none
// enforced, zero diagnostics -- byte-identical to a clean pass before this landed.
func TestVerifyFlagsARulesetItExaminedNothingOf(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	rulesPath := filepath.Join(dir, "rules.md")
	writeFile(t, rulesPath, advisoryOnlyRuleset)

	stdout, stderr, err := runIO(t, "verify", "--ruleset", rulesPath)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !strings.Contains(stderr, "examined 0 of 1 rule(s); 1 exempt as advisory") {
		t.Errorf("want the scope line, got:\n%s", stderr)
	}
	// Asserted by presence rather than by an exact count: other advisory checks legitimately
	// report on the same ruleset (this fixture's scope states no exclusion, so Limitations
	// flags it too), and a count assertion would make every future advisory break this test
	// for no reason.
	var found bool
	for _, d := range decodeFindings(t, stdout).Diagnostics {
		if d.Category != "nothing-examined" {
			continue
		}
		found = true
		// It must reach the JSON rather than living on stderr, and it must never block: a
		// CONSIDER-only ruleset is legitimate, and the defect was the silence.
		if d.Severity.Blocking() {
			t.Errorf("severity = %q, want a non-blocking one", d.Severity)
		}
	}
	if !found {
		t.Errorf("want a nothing-examined advisory, got %+v",
			decodeFindings(t, stdout).Diagnostics)
	}
}

// TestVerifyEmitsNoScopeAdvisoryWhenRulesWereExamined keeps the advisory rare enough to
// mean something: a ruleset the gates did look at gets no such finding.
func TestVerifyEmitsNoScopeAdvisoryWhenRulesWereExamined(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	rulesPath := filepath.Join(dir, "rules.md")
	sourcePath := filepath.Join(dir, "source.txt")
	writeFile(t, rulesPath, cleanRuleset)
	writeFile(t, sourcePath, cleanSource)

	stdout, _, err := runIO(t, "verify", "--ruleset", rulesPath, "--source", sourcePath)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	for _, d := range decodeFindings(t, stdout).Diagnostics {
		if d.Category == "nothing-examined" {
			t.Errorf("a fully-examined ruleset should not carry the advisory: %+v", d)
		}
	}
}
