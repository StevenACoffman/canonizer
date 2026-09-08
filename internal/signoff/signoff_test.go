package signoff_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/StevenACoffman/canonizer/internal/signoff"
	"github.com/StevenACoffman/skillet/finding"
)

// at is the instant every event in this file is stamped from, chosen so the RFC3339 form is
// unambiguous about the UTC conversion below.
func at(t *testing.T) time.Time {
	t.Helper()
	when, err := time.Parse(time.RFC3339, "2026-09-08T14:30:00Z")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return when
}

func TestAnActorMustDeclareItsClass(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		id   string
		ok   bool
	}{
		{"a human", "human:steve", true},
		{"a check", "check:nightly", true},
		{"a name with a colon in it keeps the whole remainder", "human:a:b", true},
		{"surrounding space is trimmed", "  human:steve  ", true},
		{"a bare name is refused rather than guessed into a class", "steve", false},
		{"an empty class", ":steve", false},
		{"an empty name", "human:", false},
		{"nothing at all", "", false},
		{"only a colon", ":", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			actor, err := signoff.NewActor(c.id)
			if (err == nil) != c.ok {
				t.Fatalf("NewActor(%q) err = %v, want ok=%v", c.id, err, c.ok)
			}
			if c.ok && actor.String() != strings.TrimSpace(c.id) {
				t.Errorf("String() = %q, want %q", actor.String(), strings.TrimSpace(c.id))
			}
			if !c.ok && !actor.IsZero() {
				t.Errorf("a refused actor is not zero: %q", actor.String())
			}
		})
	}
}

func TestDecideRefusesARunThatCondemnedTheRuleset(t *testing.T) {
	t.Parallel()

	actor, err := signoff.NewActor("human:steve")
	if err != nil {
		t.Fatalf("NewActor: %v", err)
	}
	blocking := &finding.Result{Diagnostics: []finding.Diagnostic{{
		Severity: finding.SeverityError, Category: "unexecutable", Path: "§1.1",
		Message: "no discriminating pair",
	}}}

	if _, err := signoff.Decide(blocking, actor, at(t)); err == nil {
		t.Error("Decide accepted a run with a blocking finding")
	} else if !strings.Contains(err.Error(), "disproved") {
		t.Errorf("the refusal does not say why: %v", err)
	}
}

func TestDecideAcceptsAdvisoryFindings(t *testing.T) {
	t.Parallel()

	actor, err := signoff.NewActor("human:steve")
	if err != nil {
		t.Fatalf("NewActor: %v", err)
	}
	// Advisories are findings for a person to weigh, not defects that must be fixed, so a
	// ruleset carrying only advisories is signable. If this ever fails, "blocking" and
	// "any finding at all" have been confused.
	advisory := &finding.Result{Diagnostics: []finding.Diagnostic{{
		Severity: finding.SeverityWarning, Category: "softening", Path: "§1.1",
		Message: "names nothing",
	}}}

	event, err := signoff.Decide(advisory, actor, at(t))
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if event.By != "human:steve" {
		t.Errorf("By = %q, want human:steve", event.By)
	}
	if event.At != "2026-09-08T14:30:00Z" {
		t.Errorf("At = %q, want an RFC3339 UTC stamp", event.At)
	}
}

func TestDecideRefusesAnUnconfiguredActor(t *testing.T) {
	t.Parallel()

	clean := &finding.Result{}
	_, err := signoff.Decide(clean, signoff.Actor{}, at(t))
	if err == nil {
		t.Fatal("Decide recorded an anonymous event")
	}
	if !strings.Contains(err.Error(), signoff.ConfigName) {
		t.Errorf("the refusal does not name the file to edit: %v", err)
	}
}

func TestDecideStampsInUTCWhateverZoneItIsGiven(t *testing.T) {
	t.Parallel()

	actor, err := signoff.NewActor("check:nightly")
	if err != nil {
		t.Fatalf("NewActor: %v", err)
	}
	zone := time.FixedZone("UTC+9", 9*60*60)
	event, err := signoff.Decide(&finding.Result{}, actor, at(t).In(zone))
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if event.At != "2026-09-08T14:30:00Z" {
		t.Errorf("At = %q, want the same instant in UTC", event.At)
	}
}

func TestLoadActor(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		write   string // "" writes no file at all
		want    string
		wantErr bool
	}{
		{
			name:  "a configured actor",
			write: "identity:\n  actor: \"human:steve\"\n",
			want:  "human:steve",
		},
		{
			name:  "no file is not an error, because most commands need no actor",
			write: "", want: "",
		},
		{
			name: "a file with no identity block", write: "other: 1\n", want: "",
		},
		{
			name:  "an empty actor reads as unconfigured",
			write: "identity:\n  actor: \"\"\n",
			want:  "",
		},
		{
			name:  "a malformed actor is an error, because someone meant it to work",
			write: "identity:\n  actor: \"steve\"\n", wantErr: true,
		},
		{
			name: "unparseable yaml", write: "identity:\n\tactor: [\n", wantErr: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			actor, err := signoff.LoadActor(configNaming(t, c.write))
			if (err != nil) != c.wantErr {
				t.Fatalf("LoadActor err = %v, wantErr = %v", err, c.wantErr)
			}
			if !c.wantErr && actor.String() != c.want {
				t.Errorf("actor = %q, want %q", actor.String(), c.want)
			}
		})
	}
}

// configNaming returns the path of a config file holding body, or of a file that was never
// written when body is empty -- which is how the no-file case is expressed.
func configNaming(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), signoff.ConfigName)
	if body != "" {
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}
	return path
}
