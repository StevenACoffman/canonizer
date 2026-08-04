package cmd_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/canonizer/cmd"
)

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
	var stdout, stderr bytes.Buffer
	err := cmd.Run(context.Background(), args, strings.NewReader(""), &stdout, &stderr)
	return stdout.String(), err
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
