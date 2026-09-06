package verify_test

import (
	"testing"

	"github.com/StevenACoffman/canonizer/internal/verify"
	"github.com/StevenACoffman/skillet/ruleset"
)

func TestLimitations(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		scope    string
		wantDiag bool
	}{
		"no scope at all":              {scope: "", wantDiag: true},
		"whitespace scope is no scope": {scope: "   ", wantDiag: true},
		"scope stating only coverage": {
			scope: "Go service code", wantDiag: true,
		},
		"scope stating an exclusion": {
			scope: "Go service code, not concurrency primitives", wantDiag: false,
		},
		"only is an exclusion": {
			scope: "Go HTTP handlers only", wantDiag: false,
		},
		"excludes is an exclusion": {
			scope: "Go service code; excludes generated files", wantDiag: false,
		},
		// A substring match would read "not" out of "notation" and pass a scope that
		// states no exclusion at all.
		"a word merely containing a marker is not an exclusion": {
			scope: "mathematical notation and nothingness", wantDiag: true,
		},
		"case and punctuation do not hide an exclusion": {
			scope: "Go code -- NOT tests.", wantDiag: false,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := verify.Limitations(ruleset.Ruleset{Scope: tc.scope})
			if tc.wantDiag && len(got) != 1 {
				t.Fatalf("Limitations(%q) = %+v, want one advisory", tc.scope, got)
			}
			if !tc.wantDiag && len(got) != 0 {
				t.Fatalf("Limitations(%q) = %+v, want none", tc.scope, got)
			}
			if tc.wantDiag && got[0].Severity.Blocking() {
				t.Errorf("severity = %q; this check must never block", got[0].Severity)
			}
		})
	}
}
