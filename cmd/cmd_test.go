package cmd_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/canonizer/cmd"
	"github.com/StevenACoffman/canonizer/cmd/root"
)

// cleanRuleset is an executable, provenanced canonical ruleset: its MUST rule has a
// discriminating ✗/✓ pair and a ↦ anchor that appears in cleanSource, so it clears both
// verify checks.
const cleanRuleset = "Source: demo\nScope:  Go\n\n" +
	"§1.1  [MUST][CODE]  Always close the connection.\n" +
	"      Leaked connections exhaust the pool.\n" +
	"      ✗  // connection is never closed\n" +
	"      ✓  defer conn.Close()\n" +
	"      ↦  ANCHOR-SENTINEL always release the connection\n"

// cleanSource contains cleanRuleset's anchor verbatim, so Provenance passes.
const cleanSource = "The pool docs say: ANCHOR-SENTINEL always release the connection.\n"

// unexecutableRuleset is cleanRuleset with its ✗/✓ pair stripped, so its MUST rule is
// unexecutable — a blocking verify finding — while still citing a present anchor.
const unexecutableRuleset = "Source: demo\nScope:  Go\n\n" +
	"§1.1  [MUST][CODE]  Always close the connection.\n" +
	"      Leaked connections exhaust the pool.\n" +
	"      ↦  ANCHOR-SENTINEL always release the connection\n"

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// run drives the dispatcher with injected I/O, the way main() does, and returns
// the error plus captured stdout for assertions.
func run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	stdout, _, err := runIO(t, args...)
	return stdout, err
}

// runIO is run for the cases that also need stderr — where a command writes its
// machine result to stdout and human-facing rationale to stderr.
func runIO(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	var out, errOut bytes.Buffer
	err = cmd.Run(context.Background(), args, strings.NewReader(""), &out, &errOut)
	return out.String(), errOut.String(), err
}

func TestDistillWritesPromptPerSource(t *testing.T) {
	t.Parallel()
	src, out := t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(src, "foo.md"), "# Foo\n\nadvice about foo\n")
	writeFile(t, filepath.Join(src, "bar-baz.md"), "# Bar\n\nadvice about bar\n")

	stdout, err := run(t, "distill", "--source", src, "--out", out)
	if err != nil {
		t.Fatalf("distill: %v", err)
	}
	for _, name := range []string{"foo_prompt.md", "bar-baz_prompt.md"} {
		if _, statErr := os.Stat(filepath.Join(out, name)); statErr != nil {
			t.Errorf("expected %s to be written: %v", name, statErr)
		}
		if !strings.Contains(stdout, name) {
			t.Errorf("stdout did not report %s: %q", name, stdout)
		}
	}
}

func TestDistillRequiredFlags(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"missing source", []string{"distill", "--out", t.TempDir()}, "--source is required"},
		{"missing out", []string{"distill", "--source", t.TempDir()}, "--out is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := run(t, tt.args...)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("got %v, want an error containing %q", err, tt.want)
			}
		})
	}
}

func TestDistillUnreadableTemplate(t *testing.T) {
	t.Parallel()
	_, err := run(t, "distill",
		"--source", t.TempDir(),
		"--out", t.TempDir(),
		"--template", filepath.Join(t.TempDir(), "missing.md"))
	if err == nil || !strings.Contains(err.Error(), "distill:") {
		t.Errorf("got %v, want a distill-prefixed template read error", err)
	}
}

func TestSynthesizeAssemblesRulesets(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "b_rules.md"), "# Beta Rules\n\nbeta body\n")
	writeFile(t, filepath.Join(dir, "a_rules.md"), "# Alpha Rules\n\nalpha body\n")
	writeFile(t, filepath.Join(dir, "notes.md"), "ignored: not a *_rules.md file\n")

	stdout, err := run(t, "synthesize", "--rulesets", dir)
	if err != nil {
		t.Fatalf("synthesize: %v", err)
	}
	// os.ReadDir sorts by name, so Alpha (a_) precedes Beta (b_), and the
	// non-_rules.md file is skipped.
	alpha := strings.Index(stdout, `source="Alpha Rules"`)
	beta := strings.Index(stdout, `source="Beta Rules"`)
	if alpha < 0 || beta < 0 || alpha > beta {
		t.Errorf("blocks missing or out of order (alpha=%d beta=%d)", alpha, beta)
	}
	if strings.Contains(stdout, "ignored: not a") {
		t.Error("synthesize included a non-_rules.md file")
	}
}

func TestSynthesizeRequiresRulesets(t *testing.T) {
	t.Parallel()
	if _, err := run(t, "synthesize"); err == nil ||
		!strings.Contains(err.Error(), "--rulesets is required") {
		t.Errorf("got %v, want a missing --rulesets error", err)
	}
}

func TestSynthesizeEmptyDirIsError(t *testing.T) {
	t.Parallel()
	if _, err := run(t, "synthesize", "--rulesets", t.TempDir()); err == nil ||
		!strings.Contains(err.Error(), "_rules.md") {
		t.Errorf("got %v, want an error about no _rules.md files", err)
	}
}

func TestCriticEmitsPromptWithSourceAndRuleset(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "s.md")
	rules := filepath.Join(dir, "s_rules.md")
	writeFile(t, src, "SOURCE-SENTINEL body\n")
	writeFile(t, rules, "Source: demo\nScope:  Go\n\n"+
		"§1.1  [MUST][CODE]  RULESET-SENTINEL: always close connections.\n"+
		"      Leaked connections exhaust the pool.\n"+
		"      ✗  defer nothing\n"+
		"      ✓  defer conn.Close()\n")

	stdout, err := run(t, "critic", "--source", src, "--ruleset", rules)
	if err != nil {
		t.Fatalf("critic: %v", err)
	}
	for _, want := range []string{"SOURCE-SENTINEL", "RULESET-SENTINEL", "Cold Critique", "diagnostics"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("emitted prompt missing %q", want)
		}
	}
}

func TestCriticRejectsNonCanonicalRuleset(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "s.md")
	rules := filepath.Join(dir, "s_rules.md")
	writeFile(t, src, "a source\n")
	writeFile(t, rules, "§1  no severity or level tags here\n") // malformed header
	if _, err := run(t, "critic", "--source", src, "--ruleset", rules); err == nil ||
		!strings.Contains(err.Error(), "critic:") {
		t.Errorf("got %v, want a critic parse error for a non-canonical ruleset", err)
	}
}

func TestCriticRequiresSourceAndRuleset(t *testing.T) {
	t.Parallel()
	if _, err := run(t, "critic", "--ruleset", "x"); err == nil ||
		!strings.Contains(err.Error(), "--source is required") {
		t.Errorf("got %v, want a missing --source error", err)
	}
}

func TestGateCleanFindingsPass(t *testing.T) {
	t.Parallel()
	f := filepath.Join(t.TempDir(), "findings.json")
	writeFile(t, f, `{"diagnostics":[{"severity":"warning","category":"note","message":"ok"}]}`)
	stdout, err := run(t, "gate", "--findings", f)
	if err != nil {
		t.Fatalf("gate: %v", err)
	}
	if !strings.Contains(stdout, "clean") {
		t.Errorf("expected a clean report, got %q", stdout)
	}
}

func TestGateBlockingFindingsExitNonzero(t *testing.T) {
	t.Parallel()
	f := filepath.Join(t.TempDir(), "findings.json")
	writeFile(
		t,
		f,
		`{"diagnostics":[{"severity":"error","category":"unsupported","path":"§1","message":"bad"}]}`,
	)
	_, err := run(t, "gate", "--findings", f)
	var exit root.ExitError
	if !errors.As(err, &exit) || int(exit) != 1 {
		t.Errorf("got %v, want root.ExitError(1)", err)
	}
}

func TestGateSelfTestPasses(t *testing.T) {
	t.Parallel()
	stdout, err := run(t, "gate", "--selftest")
	if err != nil {
		t.Fatalf("gate --selftest: %v", err)
	}
	if !strings.Contains(stdout, "self-test passed") {
		t.Errorf("expected self-test pass report, got %q", stdout)
	}
}

func TestVerifyEmitsFindingsForUnexecutableRule(t *testing.T) {
	t.Parallel()
	rules := filepath.Join(t.TempDir(), "r_rules.md")
	writeFile(t, rules, "Source: d\nScope:  Go\n\n"+
		"§1.1  [MUST][CODE]  A rule with no discriminating examples.\n"+
		"      Some rationale.\n")
	stdout, err := run(t, "verify", "--ruleset", rules)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !strings.Contains(stdout, "unexecutable") ||
		!strings.Contains(stdout, `"severity": "error"`) {
		t.Errorf("expected an unexecutable error finding, got %q", stdout)
	}
}

func TestVerifyRequiresRuleset(t *testing.T) {
	t.Parallel()
	if _, err := run(t, "verify"); err == nil ||
		!strings.Contains(err.Error(), "--ruleset is required") {
		t.Errorf("got %v, want a missing --ruleset error", err)
	}
}

func TestBudgetShipsCleanFindings(t *testing.T) {
	t.Parallel()
	f := filepath.Join(t.TempDir(), "f.json")
	writeFile(t, f, `{"diagnostics":[]}`)
	stdout, err := run(t, "budget", "--findings", f, "--attempt", "1", "--max", "3")
	if err != nil {
		t.Fatalf("budget: %v", err)
	}
	if !strings.Contains(stdout, "ship") {
		t.Errorf("expected ship verdict, got %q", stdout)
	}
}

func TestBudgetReworksWithBudgetRemaining(t *testing.T) {
	t.Parallel()
	f := filepath.Join(t.TempDir(), "f.json")
	writeFile(t, f, `{"diagnostics":[{"severity":"error","message":"x"}]}`)
	_, err := run(t, "budget", "--findings", f, "--attempt", "1", "--max", "3")
	var exit root.ExitError
	if !errors.As(err, &exit) || int(exit) != 2 {
		t.Errorf("got %v, want root.ExitError(2) (rework)", err)
	}
}

func TestBudgetEscalatesWhenBudgetSpent(t *testing.T) {
	t.Parallel()
	f := filepath.Join(t.TempDir(), "f.json")
	writeFile(t, f, `{"diagnostics":[{"severity":"error","message":"x"}]}`)
	_, err := run(t, "budget", "--findings", f, "--attempt", "3", "--max", "3")
	var exit root.ExitError
	if !errors.As(err, &exit) || int(exit) != 1 {
		t.Errorf("got %v, want root.ExitError(1) (needs-human)", err)
	}
}

// writeLoopFixtures writes cleanRuleset and cleanSource into dir and returns their paths.
func writeLoopFixtures(t *testing.T, dir string) (source, rules string) {
	t.Helper()
	source = filepath.Join(dir, "s.md")
	rules = filepath.Join(dir, "s_rules.md")
	writeFile(t, source, cleanSource)
	writeFile(t, rules, cleanRuleset)
	return source, rules
}

func TestLoopShipsCleanRuleset(t *testing.T) {
	t.Parallel()
	source, rules := writeLoopFixtures(t, t.TempDir())
	stdout, err := run(
		t,
		"loop",
		"--source",
		source,
		"--ruleset",
		rules,
		"--attempt",
		"1",
		"--max",
		"3",
	)
	if err != nil {
		t.Fatalf("loop: %v", err)
	}
	if strings.TrimSpace(stdout) != "ship" {
		t.Errorf("expected a ship verdict on stdout, got %q", stdout)
	}
}

// TestLoopModelAttestationRecordedNotGated shows --model records the operator's grader
// attestation on stderr, labeled unverified, without changing the verdict on stdout.
func TestLoopModelAttestationRecordedNotGated(t *testing.T) {
	t.Parallel()
	source, rules := writeLoopFixtures(t, t.TempDir())
	stdout, stderr, err := runIO(t, "loop",
		"--source", source, "--ruleset", rules, "--model", "some-reasoning-model",
		"--attempt", "1", "--max", "3")
	if err != nil {
		t.Fatalf("loop: %v", err)
	}
	if strings.TrimSpace(stdout) != "ship" {
		t.Errorf("--model must not change the verdict; stdout = %q", stdout)
	}
	if !strings.Contains(stderr, "some-reasoning-model") ||
		!strings.Contains(stderr, "operator-attested, unverified") {
		t.Errorf("expected an unverified grader-model attestation on stderr, got %q", stderr)
	}
}

// TestLoopNoAttestationWithoutModel confirms the attestation line is absent when --model
// is unset — it is opt-in, not a default claim.
func TestLoopNoAttestationWithoutModel(t *testing.T) {
	t.Parallel()
	source, rules := writeLoopFixtures(t, t.TempDir())
	_, stderr, err := runIO(t, "loop",
		"--source", source, "--ruleset", rules, "--attempt", "1", "--max", "3")
	if err != nil {
		t.Fatalf("loop: %v", err)
	}
	if strings.Contains(stderr, "grader model") {
		t.Errorf("no attestation should print without --model, got %q", stderr)
	}
}

func TestLoopReworksWhenBudgetRemaining(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	source := filepath.Join(dir, "s.md")
	rules := filepath.Join(dir, "s_rules.md")
	writeFile(t, source, cleanSource)
	writeFile(t, rules, unexecutableRuleset)
	_, err := run(t, "loop", "--source", source, "--ruleset", rules, "--attempt", "1", "--max", "3")
	var exit root.ExitError
	if !errors.As(err, &exit) || int(exit) != 2 {
		t.Errorf("got %v, want root.ExitError(2) (rework)", err)
	}
}

func TestLoopEscalatesWhenBudgetSpent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	source := filepath.Join(dir, "s.md")
	rules := filepath.Join(dir, "s_rules.md")
	writeFile(t, source, cleanSource)
	writeFile(t, rules, unexecutableRuleset)
	_, err := run(t, "loop", "--source", source, "--ruleset", rules, "--attempt", "3", "--max", "3")
	var exit root.ExitError
	if !errors.As(err, &exit) || int(exit) != 1 {
		t.Errorf("got %v, want root.ExitError(1) (needs-human)", err)
	}
}

// TestLoopMergesCriticFindings is the point of the command: a ruleset that clears verify
// on its own is still reworked when the agent's cold-critic findings carry a blocking
// diagnostic — the critic → gate wire.
func TestLoopMergesCriticFindings(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	source, rules := writeLoopFixtures(t, dir)
	f := filepath.Join(dir, "findings.json")
	writeFile(
		t,
		f,
		`{"diagnostics":[{"severity":"error","category":"unsupported","path":"§1.1","message":"anchor does not support the claim"}]}`,
	)
	_, err := run(t, "loop",
		"--source", source, "--ruleset", rules, "--findings", f, "--attempt", "1", "--max", "3")
	var exit root.ExitError
	if !errors.As(err, &exit) || int(exit) != 2 {
		t.Errorf("got %v, want root.ExitError(2) (rework from a merged critic finding)", err)
	}
}

func TestLoopRejectsNonCanonicalRuleset(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	source := filepath.Join(dir, "s.md")
	rules := filepath.Join(dir, "s_rules.md")
	writeFile(t, source, "a source\n")
	writeFile(t, rules, "§1  no severity or level tags here\n") // malformed header
	if _, err := run(t, "loop", "--source", source, "--ruleset", rules); err == nil ||
		!strings.Contains(err.Error(), "loop:") {
		t.Errorf("got %v, want a loop parse error for a non-canonical ruleset", err)
	}
}

func TestLoopRequiresSourceAndRuleset(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"missing source", []string{"loop", "--ruleset", "x"}, "--source is required"},
		{"missing ruleset", []string{"loop", "--source", "x"}, "--ruleset is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := run(t, tt.args...)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("got %v, want an error containing %q", err, tt.want)
			}
		})
	}
}
