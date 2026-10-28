package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runMain calls run as main does, with the program name first, and returns the exit
// code and what was written to stderr.
func runMain(t *testing.T, args ...string) (code int, stderr string) {
	t.Helper()
	var errOut bytes.Buffer
	code = run(context.Background(), append([]string{"canonizer"}, args...),
		strings.NewReader(""), io.Discard, &errOut)
	return code, errOut.String()
}

func TestRunShowsTheAttributesAnErrorCarries(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "nope_rules.md")

	code, stderr := runMain(t, "fmt", "--ruleset", path)
	if code != exitFail {
		t.Errorf("exit code = %d, want %d", code, exitFail)
	}
	if !strings.Contains(stderr, "error: fmt: read ruleset:") {
		t.Errorf("stderr does not report the failure:\n%s", stderr)
	}
	if !strings.Contains(stderr, "path="+path) {
		t.Errorf("stderr does not show the path attribute:\n%s", stderr)
	}
}

func TestRunKeepsItsExitCodes(t *testing.T) {
	t.Parallel()
	// Three spaces after the level tag, where Render emits two: parseable, not canonical.
	nonCanonical := filepath.Join(t.TempDir(), "r_rules.md")
	if err := os.WriteFile(nonCanonical, []byte("Source: s\nScope:  x\n\n"+
		"§1.1  [MUST][CODE]   Close it.\n"+
		"      because a leaked handle outlives the request.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// silent marks an outcome the command has already reported, which run must not
	// repeat as an error line. It is what tells a command's ExitError(1) apart from a
	// failure, since both exit 1.
	cases := map[string]struct {
		args   []string
		want   int
		silent bool
	}{
		"help succeeds":          {args: []string{"-h"}, want: exitSuccess},
		"no subcommand succeeds": {args: nil, want: exitSuccess},
		"a command's ExitError is kept": {
			args:   []string{"fmt", "--check", "--ruleset", nonCanonical},
			want:   1,
			silent: true,
		},
		"an unknown subcommand fails": {args: []string{"bogus"}, want: exitFail},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			code, stderr := runMain(t, tc.args...)
			if code != tc.want {
				t.Errorf("exit code = %d, want %d\nstderr:\n%s", code, tc.want, stderr)
			}
			if tc.silent && strings.Contains(stderr, "error:") {
				t.Errorf("run reported an outcome the command already reported:\n%s", stderr)
			}
		})
	}
}
