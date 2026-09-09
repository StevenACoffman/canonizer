package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/canonizer/internal/signoff"
)

// signableSource is the passage the fixture ruleset's anchor quotes verbatim, so a run with
// --source finds it and reports nothing blocking.
const signableSource = "# Guidance\n\nAlways close what you opened, without exception.\n"

// signableRuleset is a ruleset with no blocking findings: one enforced rule with a
// discriminating pair and an anchor present in signableSource. It is written in canonical
// form, which the sign-off refuses to do without.
const signableRuleset = "Source: Guidance\nScope:  Go\n\n" +
	"§1.1  [MUST][CODE]  Close every `io.Closer` you open.\n" +
	"      A leaked handle outlives the request and the process runs out of them.\n" +
	"      ✗  f, _ := os.Open(p)\n" +
	"      ✓  f, _ := os.Open(p); defer f.Close()\n" +
	"      ↦  §Guidance: \"Always close what you opened\"\n"

// signoffFixture writes the signable pair plus a config naming actor, and returns the
// ruleset, source and config paths. An empty actor writes no config file, so the path
// names one that does not exist -- which is the unconfigured case.
func signoffFixture(t *testing.T, actor string) (rulesPath, sourcePath, configPath string) {
	t.Helper()
	dir := t.TempDir()
	rulesPath = filepath.Join(dir, "guidance_rules.md")
	sourcePath = filepath.Join(dir, "guidance.md")
	configPath = filepath.Join(dir, signoff.ConfigName)
	writeFile(t, rulesPath, signableRuleset)
	writeFile(t, sourcePath, signableSource)
	if actor != "" {
		writeFile(t, configPath, "identity:\n  actor: \""+actor+"\"\n")
	}
	return rulesPath, sourcePath, configPath
}

func TestSignOffRecordsAnEventAndLeavesTheRulesetCanonical(t *testing.T) {
	t.Parallel()

	rules, source, conf := signoffFixture(t, "human:steve")

	_, stderr, err := runIO(
		t,
		"verify",
		"--ruleset",
		rules,
		"--source",
		source,
		"--sign-off",
		"--config",
		conf,
	)
	if err != nil {
		t.Fatalf("verify --sign-off: %v\n%s", err, stderr)
	}
	if !strings.Contains(stderr, "recorded human:steve") {
		t.Errorf("stderr does not report the recorded actor:\n%s", stderr)
	}

	after, readErr := os.ReadFile(rules)
	if readErr != nil {
		t.Fatalf("ReadFile: %v", readErr)
	}
	got := string(after)
	if !strings.HasPrefix(got, "---\nformat: 4\nverified:\n  - by: \"human:steve\"\n") {
		t.Errorf("the event is not in the frontmatter block:\n%s", got)
	}
	if !strings.HasSuffix(got, signableRuleset) {
		t.Errorf("the body changed; sign-off must only prepend the block:\n%s", got)
	}

	// The invariant: signing must not make the next run report the drift this one avoided.
	stdout, _, err := runIO(t, "verify", "--ruleset", rules, "--source", source)
	if err != nil {
		t.Fatalf("re-verify: %v", err)
	}
	if cs := categoriesOf(t, stdout); contains(cs, "non-canonical") {
		t.Errorf("a signed ruleset is no longer canonical: %v", cs)
	}
}

func TestASecondSignOffAppendsRatherThanReplaces(t *testing.T) {
	t.Parallel()

	rules, source, conf := signoffFixture(t, "human:steve")

	for i := range 2 {
		if _, stderr, err := runIO(
			t, "verify", "--ruleset", rules, "--source", source, "--sign-off",
			"--config", conf,
		); err != nil {
			t.Fatalf("sign-off %d: %v\n%s", i, err, stderr)
		}
	}
	after, err := os.ReadFile(rules)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if n := strings.Count(string(after), `- by: "human:steve"`); n != 2 {
		t.Errorf("want 2 recorded events, got %d:\n%s", n, after)
	}
}

func TestSignOffIsRefused(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		actor string
		args  func(rules, source, conf string) []string
		want  string
	}{{
		name:  "when the run found blocking findings",
		actor: "human:steve",
		args: func(rules, source, conf string) []string {
			// A source that does not hold the anchor, so the anchor reports absent and the
			// run condemns the very ruleset it is asked to attest to. Written beside the
			// real source rather than reusing the ruleset as its own source, which quotes
			// the anchor in its own arrow line and so passes.
			wrong := filepath.Join(filepath.Dir(source), "unrelated.md")
			if err := os.WriteFile(
				wrong,
				[]byte("# Other\n\nnothing to do with it\n"),
				0o600,
			); err != nil {
				panic(err)
			}
			return []string{
				"verify",
				"--ruleset",
				rules,
				"--source",
				wrong,
				"--sign-off",
				"--config",
				conf,
			}
		},
		want: "disproved",
	}, {
		name:  "when no source was examined",
		actor: "human:steve",
		args: func(rules, _, conf string) []string {
			return []string{"verify", "--ruleset", rules, "--sign-off", "--config", conf}
		},
		want: "--sign-off needs --source",
	}, {
		name:  "when no actor is configured",
		actor: "",
		args: func(rules, source, conf string) []string {
			return []string{
				"verify",
				"--ruleset",
				rules,
				"--source",
				source,
				"--sign-off",
				"--config",
				conf,
			}
		},
		want: "no actor configured",
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			rules, source, conf := signoffFixture(t, c.actor)
			assertRefused(t, rules, c.want, c.args(rules, source, conf))
		})
	}
}

// assertRefused runs args, requires the error to mention want, and requires the ruleset at
// rules to be byte-identical afterwards -- a refusal that still wrote would be the worst
// outcome of the three, since it would record an attestation the policy just declined.
func assertRefused(t *testing.T, rules, want string, args []string) {
	t.Helper()

	before, err := os.ReadFile(rules)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if _, _, runErr := runIO(t, args...); runErr == nil {
		t.Fatal("the sign-off was accepted")
	} else if !strings.Contains(runErr.Error(), want) {
		t.Errorf("error = %v, want it to mention %q", runErr, want)
	}
	after, err := os.ReadFile(rules)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.Equal(after, before) {
		t.Errorf("a refused sign-off still wrote to the ruleset:\n%s", after)
	}
}
