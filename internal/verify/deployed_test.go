package verify_test

import (
	"slices"
	"testing"

	"github.com/StevenACoffman/canonizer/internal/verify"
	"github.com/StevenACoffman/skillet/finding"
)

func TestDeployed(t *testing.T) {
	t.Parallel()
	verified := []byte("# Rules\n\n## 1. Wrap errors\n")
	cases := map[string]struct {
		copies    []verify.DeployedCopy
		wantPaths []string
	}{
		"no copies named": {},
		"an identical copy passes": {
			copies: []verify.DeployedCopy{{Path: "a/CLAUDE.md", Content: verified}},
		},
		"a one-word edit is drift, not similarity": {
			copies: []verify.DeployedCopy{
				{Path: "a/rules.md", Content: []byte("# Rules\n\n## 1. Wrap all errors\n")},
			},
			wantPaths: []string{"a/rules.md"},
		},
		"a trailing newline is drift": {
			copies: []verify.DeployedCopy{
				{Path: "b/rules.md", Content: append(append([]byte{}, verified...), '\n')},
			},
			wantPaths: []string{"b/rules.md"},
		},
		"a missing copy is reported": {
			copies:    []verify.DeployedCopy{{Path: "gone.md", Missing: true}},
			wantPaths: []string{"gone.md"},
		},
		"only the differing copies, in order": {
			copies: []verify.DeployedCopy{
				{Path: "x.md", Content: []byte("other")},
				{Path: "ok.md", Content: verified},
				{Path: "y.md", Missing: true},
			},
			wantPaths: []string{"x.md", "y.md"},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := verify.Deployed(verified, tc.copies)
			paths := make([]string, 0, len(got))
			for _, d := range got {
				paths = append(paths, d.Path)
				if d.Category != verify.CategoryDeployedDrift ||
					d.Severity != finding.SeverityError {
					t.Errorf("finding %+v, want a blocking deployed-drift", d)
				}
			}
			if !slices.Equal(paths, tc.wantPaths) {
				t.Errorf("findings for %q, want %q", paths, tc.wantPaths)
			}
		})
	}
}
