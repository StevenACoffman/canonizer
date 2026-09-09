package cmd_test

import (
	"path/filepath"
	"testing"
)

// categoriesOf returns the categories a verify run reported.
func categoriesOf(t *testing.T, stdout string) []string {
	t.Helper()
	var out []string
	for _, d := range decodeFindings(t, stdout).Diagnostics {
		out = append(out, d.Category)
	}
	return out
}

// contains reports whether categories holds want.
func contains(categories []string, want string) bool {
	for _, c := range categories {
		if c == want {
			return true
		}
	}
	return false
}

// driftFixture writes a ruleset, a source, and a proof packet binding the two, returning
// the ruleset, source and proof paths.
func driftFixture(t *testing.T, source string) (rulesPath, sourcePath, proofPath string) {
	t.Helper()
	dir := t.TempDir()
	rulesPath = filepath.Join(dir, "rules.md")
	sourcePath = filepath.Join(dir, "source.txt")
	proofPath = filepath.Join(dir, "proof.json")
	writeFile(t, rulesPath, cleanRuleset)
	writeFile(t, sourcePath, source)
	if _, err := run(t, "verify",
		"--ruleset", rulesPath, "--source", sourcePath, "--proof", proofPath); err != nil {
		t.Fatalf("write proof: %v", err)
	}
	return rulesPath, sourcePath, proofPath
}

// TestVerifyAgainstProofSeparatesFabricationFromDrift is the whole point of the second
// signal: before it, both of these reported anchor-absent and blocked identically.
func TestVerifyAgainstProofSeparatesFabricationFromDrift(t *testing.T) {
	t.Parallel()
	t.Run("anchor gone from an unchanged source is fabricated and blocks", func(t *testing.T) {
		t.Parallel()
		// The proof is taken over a source that never held the anchor, so the bytes the
		// rule was distilled from demonstrably never contained it.
		rules, source, proofPath := driftFixture(t, "nothing relevant here\n")
		stdout, _, err := runIO(t, "verify",
			"--ruleset", rules, "--source", source, "--against-proof", proofPath)
		if err != nil {
			t.Fatalf("verify: %v", err)
		}
		if got := categoriesOf(t, stdout); !contains(got, "anchor-fabricated") {
			t.Errorf("categories = %v, want anchor-fabricated", got)
		}
	})

	t.Run("anchor gone after the source changed is stale, not fabricated", func(t *testing.T) {
		t.Parallel()
		// Proof taken over the original source, which is then rewritten without the
		// anchor -- a source edit, not a hallucination.
		rules, source, proofPath := driftFixture(t, cleanSource)
		writeFile(t, source, "the document was rewritten and no longer says that\n")
		stdout, _, err := runIO(t, "verify",
			"--ruleset", rules, "--source", source, "--against-proof", proofPath)
		if err != nil {
			t.Fatalf("verify: %v", err)
		}
		got := categoriesOf(t, stdout)
		if !contains(got, "anchor-stale") {
			t.Errorf("categories = %v, want anchor-stale", got)
		}
		if contains(got, "anchor-fabricated") {
			t.Errorf("categories = %v, must not accuse a changed source of fabrication", got)
		}
	})

	t.Run("anchor surviving a changed source is drift and does not block", func(t *testing.T) {
		t.Parallel()
		rules, source, proofPath := driftFixture(t, cleanSource)
		// Reformatted around the anchor: the source moved, the citation survived.
		writeFile(t, source, "PREAMBLE\n\n"+cleanSource+"\nAPPENDIX\n")
		stdout, _, err := runIO(t, "verify",
			"--ruleset", rules, "--source", source, "--against-proof", proofPath)
		if err != nil {
			t.Fatalf("a surviving anchor must not fail the run: %v", err)
		}
		if got := categoriesOf(t, stdout); !contains(got, "anchor-drift") {
			t.Errorf("categories = %v, want anchor-drift", got)
		}
	})
}

// TestVerifyWithoutProofKeepsTheOldVerdict pins that the new flag is additive: callers not
// passing it see exactly the previous behaviour.
func TestVerifyWithoutProofKeepsTheOldVerdict(t *testing.T) {
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
		t.Errorf("categories = %v, want the unchanged anchor-absent verdict", got)
	}
}
