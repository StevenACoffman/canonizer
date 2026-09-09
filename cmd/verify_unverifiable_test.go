package cmd_test

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestVerifyReportsUnverifiableAnchorsWithNoSource is the measured defect: before this,
// a ruleset whose rules cite anchors got no provenance diagnostic at all when no source was
// supplied, which is byte-identical to a run where every anchor was found.
func TestVerifyReportsUnverifiableAnchorsWithNoSource(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	rules := filepath.Join(dir, "rules.md")
	writeFile(t, rules, cleanRuleset)

	stdout, _, err := runIO(t, "verify", "--ruleset", rules)
	if err != nil {
		t.Fatalf("an unverifiable anchor must not fail the run: %v\n%s", err, stdout)
	}
	if got := categoriesOf(t, stdout); !contains(got, "anchor-unverifiable") {
		t.Errorf("categories = %v, want anchor-unverifiable", got)
	}
}

// TestVerifyDoesNotReportUnverifiableWhenASourceWasGiven keeps the state rare enough to
// mean something: with a source, the anchor is genuinely checked and the answer is
// present-or-absent, never unverifiable.
func TestVerifyDoesNotReportUnverifiableWhenASourceWasGiven(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	rules := filepath.Join(dir, "rules.md")
	source := filepath.Join(dir, "source.txt")
	writeFile(t, rules, cleanRuleset)
	writeFile(t, source, cleanSource)

	stdout, _, err := runIO(t, "verify", "--ruleset", rules, "--source", source)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if got := categoriesOf(t, stdout); contains(got, "anchor-unverifiable") {
		t.Errorf("categories = %v; a checked anchor is not unverifiable", got)
	}
}

// TestVerifyStillBlocksAnAbsentAnchorWithASource pins that the new state did not soften the
// old verdict: a search that ran and found nothing is a different claim from no search, and
// it still blocks.
func TestVerifyStillBlocksAnAbsentAnchorWithASource(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	rules := filepath.Join(dir, "rules.md")
	source := filepath.Join(dir, "source.txt")
	writeFile(t, rules, cleanRuleset)
	writeFile(t, source, "nothing relevant here\n")

	stdout, _, err := runIO(t, "verify", "--ruleset", rules, "--source", source)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	got := categoriesOf(t, stdout)
	if !contains(got, "anchor-absent") {
		t.Errorf("categories = %v, want anchor-absent still reported", got)
	}
	if contains(got, "anchor-unverifiable") {
		t.Errorf("categories = %v; a source was supplied, so nothing is unverifiable", got)
	}
}

// TestVerifyDoesNotClaimAnchorsWereCheckedWhenTheyWereNot covers the summary line, which
// asserted "its anchors are present" on a run that searched for none.
func TestVerifyDoesNotClaimAnchorsWereCheckedWhenTheyWereNot(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	rules := filepath.Join(dir, "rules.md")
	writeFile(t, rules, cleanRuleset)

	_, stderr, err := runIO(t, "verify", "--ruleset", rules)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if strings.Contains(stderr, "its anchors are present") {
		t.Errorf("the summary claims anchors were checked on a run with no source:\n%s", stderr)
	}
	if !strings.Contains(stderr, "no source was supplied") {
		t.Errorf("want the summary to say no source was supplied, got:\n%s", stderr)
	}
}

// TestVerifyStillClaimsAnchorsWhenASourceWasGiven is the other half: the clause must return
// when it is true, or the correction would have replaced an overclaim with an underclaim.
func TestVerifyStillClaimsAnchorsWhenASourceWasGiven(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	rules := filepath.Join(dir, "rules.md")
	source := filepath.Join(dir, "source.txt")
	writeFile(t, rules, cleanRuleset)
	writeFile(t, source, cleanSource)

	_, stderr, err := runIO(t, "verify", "--ruleset", rules, "--source", source)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !strings.Contains(stderr, "its anchors are present") {
		t.Errorf("want the anchor clause when a source was checked, got:\n%s", stderr)
	}
}
