package cmd_test

import (
	"path/filepath"
	"testing"
)

// TestVerifyDeployedCopies drives --deployed end to end: an identical copy is silent, and
// an edited copy or a missing one is a deployed-drift finding naming its path.
func TestVerifyDeployedCopies(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		copy      string // content written to the deployed path; "" means no file
		wantDrift bool
	}{
		"identical copy": {copy: cleanRuleset},
		"edited copy":    {copy: cleanRuleset + "\nOne more line.\n", wantDrift: true},
		"missing copy":   {wantDrift: true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			rules := filepath.Join(dir, "rules.md")
			deployed := filepath.Join(dir, "deployed-rules.md")
			writeFile(t, rules, cleanRuleset)
			if tc.copy != "" {
				writeFile(t, deployed, tc.copy)
			}

			stdout, _, _ := runIO(t, "verify", "--ruleset", rules, "--deployed", deployed)
			if got := contains(categoriesOf(t, stdout), "deployed-drift"); got != tc.wantDrift {
				t.Errorf("deployed-drift reported = %v, want %v\n%s", got, tc.wantDrift, stdout)
			}
		})
	}
}
