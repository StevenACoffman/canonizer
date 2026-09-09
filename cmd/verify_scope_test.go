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

// TestScopeLineReportsSymbolAdherence is the measurement the unstated-convention item
// wanted and could not see: the same prompt over eight sources produced rates from 9% to
// 78%, so a specificity reading is only as comparable as the run's typography.
func TestScopeLineReportsSymbolAdherence(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	rules := filepath.Join(dir, "r_rules.md")
	source := filepath.Join(dir, "s.md")
	writeFile(t, source, "# S\n\nAlways close what you opened.\n")
	writeFile(t, rules, "Source: s\nScope:  x\n\n"+
		"§1.1  [MUST][CODE]  Call `ctx.Done()` before returning.\n"+
		"      because reasons\n      ✗  b\n      ✓  g\n"+
		"      ↦  §S: \"Always close what you opened\"\n\n"+
		"§1.2  [MUST][CODE]  Decompose the work into smaller steps.\n"+
		"      because reasons\n      ✗  b\n      ✓  g\n"+
		"      ↦  §S: \"Always close what you opened\"\n")

	_, stderr, err := runIO(t, "verify", "--ruleset", rules, "--source", source)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !strings.Contains(stderr, "1 of 2 enforced rule(s) name a symbol a checker can see") {
		t.Errorf("stderr does not report the symbol rate:\n%s", stderr)
	}
	// The rate prints even at 100%, for the reason the examined line does: a reader who
	// only ever meets a number at its ceiling never learns what it means.
	if !strings.Contains(stderr, "name a symbol") {
		t.Error("the symbol line is conditional; it must print on every run")
	}
}

// TestScopeLineReportsUnsearchableAnchorsOnlyWhenPresent pins the asymmetry: the symbol
// rate always prints, this one only when non-zero, because zero is the case in all 162
// anchors of the corpus and a line reporting none of them every run costs attention.
func TestScopeLineReportsUnsearchableAnchorsOnlyWhenPresent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	source := filepath.Join(dir, "s.md")
	writeFile(t, source, "# S\n\nAlways close what you opened.\n")

	quoted := filepath.Join(dir, "q_rules.md")
	writeFile(t, quoted, "Source: s\nScope:  x\n\n"+
		"§1.1  [MUST][CODE]  Call `ctx.Done()` first.\n"+
		"      because reasons\n      ✗  b\n      ✓  g\n"+
		"      ↦  §S: \"Always close what you opened\"\n")
	_, stderr, err := runIO(t, "verify", "--ruleset", quoted, "--source", source)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if strings.Contains(stderr, "name a section only") {
		t.Errorf("the line printed with nothing to report:\n%s", stderr)
	}

	bare := filepath.Join(dir, "b_rules.md")
	writeFile(t, bare, "Source: s\nScope:  x\n\n"+
		"§1.1  [MUST][CODE]  Call `ctx.Done()` first.\n"+
		"      because reasons\n      ✗  b\n      ✓  g\n"+
		"      ↦  §Transactional boundaries\n")
	_, stderr, err = runIO(t, "verify", "--ruleset", bare, "--source", source)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !strings.Contains(stderr,
		"1 of 1 anchor(s) name a section only; their provenance was not searched") {
		t.Errorf("stderr does not report the unsearched anchor:\n%s", stderr)
	}
}
