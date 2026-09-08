package cmd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// wrappedRuleset is the shape five of the eight stored rulesets are in: three spaces after
// the level tag, and a rationale a human wrapped across two lines.
const wrappedRuleset = "Source: s\nScope:  x\n\n" +
	"§1.1  [MUST][CODE]   Close it.\n" +
	"      because a leaked handle outlives the request and the process\n" +
	"      eventually runs out of them.\n"

// canonicalRuleset is what Render emits for the same content: two spaces, one rationale line.
const canonicalRuleset = "Source: s\nScope:  x\n\n" +
	"§1.1  [MUST][CODE]  Close it.\n" +
	"      because a leaked handle outlives the request and the process eventually runs out of them.\n"

func TestFmtRewritesANonCanonicalRulesetWithoutLosingText(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "r_rules.md")
	writeFile(t, path, wrappedRuleset)

	stdout, err := run(t, "fmt", "--ruleset", path)
	if err != nil {
		t.Fatalf("fmt: %v", err)
	}
	if !strings.Contains(stdout, "rewritten") {
		t.Errorf("stdout does not report the rewrite: %q", stdout)
	}
	after, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("ReadFile: %v", readErr)
	}
	if string(after) != canonicalRuleset {
		t.Errorf("rewritten form:\n got=%q\nwant=%q", after, canonicalRuleset)
	}
	// The property the entry rests on: wrapping moves, words do not.
	if before, got := strings.Fields(
		wrappedRuleset,
	), strings.Fields(
		string(after),
	); len(
		before,
	) != len(
		got,
	) {
		t.Errorf("word count changed: %d -> %d", len(before), len(got))
	}
}

func TestFmtLeavesACanonicalRulesetAlone(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "r_rules.md")
	writeFile(t, path, canonicalRuleset)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}

	stdout, runErr := run(t, "fmt", "--ruleset", path)
	if runErr != nil {
		t.Fatalf("fmt: %v", runErr)
	}
	if !strings.Contains(stdout, "already canonical") {
		t.Errorf("stdout = %q, want it to report the file was already canonical", stdout)
	}
	// Not rewritten at all, so running this across a corpus does not touch what it need not.
	after, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if !after.ModTime().Equal(info.ModTime()) {
		t.Error("an already-canonical file was rewritten")
	}
}

func TestFmtCheckReportsWithoutWritingAndExitsOne(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "r_rules.md")
	writeFile(t, path, wrappedRuleset)

	stdout, err := run(t, "fmt", "--ruleset", path, "--check")
	if err == nil {
		t.Fatal("--check exited zero on a non-canonical ruleset")
	}
	if !strings.Contains(stdout, "not canonical") {
		t.Errorf("stdout = %q, want it to name the file as not canonical", stdout)
	}
	after, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("ReadFile: %v", readErr)
	}
	if string(after) != wrappedRuleset {
		t.Error("--check wrote to the ruleset")
	}
}

func TestFmtCheckExitsZeroOnACanonicalRuleset(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "r_rules.md")
	writeFile(t, path, canonicalRuleset)

	if _, err := run(t, "fmt", "--ruleset", path, "--check"); err != nil {
		t.Errorf("--check on a canonical ruleset: %v", err)
	}
}

// TestFmtMakesARulesetSignable is the pairing the entry describes: verify refuses to sign a
// non-canonical ruleset, and this is the command that clears the refusal.
func TestFmtMakesARulesetSignable(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "r_rules.md")
	writeFile(t, path, wrappedRuleset)

	if _, err := run(t, "fmt", "--ruleset", path, "--check"); err == nil {
		t.Fatal("the fixture was already canonical, so this proves nothing")
	}
	if _, err := run(t, "fmt", "--ruleset", path); err != nil {
		t.Fatalf("fmt: %v", err)
	}
	if _, err := run(t, "fmt", "--ruleset", path, "--check"); err != nil {
		t.Errorf("still not canonical after fmt: %v", err)
	}
}

func TestFmtRequiresARuleset(t *testing.T) {
	t.Parallel()
	if _, err := run(t, "fmt"); err == nil {
		t.Error("fmt accepted a missing --ruleset")
	}
}
