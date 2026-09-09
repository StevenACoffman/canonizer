// Package signoff decides whether a verify run may record a verification event, and who it
// is attributed to.
//
// A ruleset stands on its own content; an event says a named actor read it and agreed.
// skillet's ruleset format carries the list at version 4 and canonizer is the tool that
// appends to it, so the policy for *when* an event may be appended lives here.
//
// **The policy is adh's, transposed rather than reinvented.** adh's contextstore settled it
// first and its reasoning is the authority: the actor comes from configured identity and
// never from a flag, an unset identity refuses rather than recording an anonymous event, and
// a sign-off is refused on a run that condemned the thing being signed. Only the definition
// of "condemned" is canonizer's -- a blocking diagnostic here, drift there.
package signoff

import (
	errs "errors"
	"io/fs"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/goccy/go-yaml"

	"github.com/StevenACoffman/skillet/finding"
	"github.com/StevenACoffman/skillet/verification"
	errors "github.com/StevenACoffman/toerr/errors"
)

// ConfigName is the file --config defaults to, resolved against the working directory.
const ConfigName = ".canonizer.yaml"

// Actor is an attribution that has been validated.
//
// It is a type rather than a checked string so that an unvalidated actor is unrepresentable
// rather than merely discouraged: every value of this type came through NewActor, so no
// caller has to re-establish that the class is present.
//
// The id is kept whole rather than split into class and name. Event.By is the full string
// and consumers parse it themselves -- gnosis reads the raw form deliberately, because OKF
// makes raw and parsed actors two different populations -- so splitting and rejoining here
// would risk a round trip that changes what was configured.
type Actor struct {
	id string
}

// NewActor validates id as an actor and returns it.
//
// The class is required and never inferred. A bare name would have to be guessed into a
// class, and guessing "human" is the guess that lets an automated runner mint a human's
// sign-off -- the failure the whole config-derived-actor rule exists to prevent.
//
// Ensures: the returned Actor is zero exactly when err is non-nil; it is pure.
func NewActor(id string) (Actor, error) {
	trimmed := strings.TrimSpace(id)
	class, name, found := strings.Cut(trimmed, ":")
	if !found || strings.TrimSpace(class) == "" || strings.TrimSpace(name) == "" {
		return Actor{}, errors.New(
			"signoff: actor must be <class>:<name> (e.g. human:steve, check:nightly); got " +
				id)
	}
	return Actor{id: trimmed}, nil
}

// String is the actor id as configured, which is what Event.By carries.
func (a Actor) String() string { return a.id }

// IsZero reports whether no actor is configured. It is the state LoadActor returns for a
// missing file, and the state Decide refuses on.
func (a Actor) IsZero() bool { return a.id == "" }

// LoadActor reads the configured actor from path.
//
// **The path is a parameter and the actor is not.** Naming a different file still means the
// actor came from a file somebody wrote and reviewed, which is what the rule protects;
// letting the actor itself be passed in is what it forbids.
//
// **A missing file is not an error.** Every canonizer command other than a sign-off works
// without a config, so a repository that never signs anything should not have to carry one.
// The absence surfaces as a zero Actor and becomes an error only in Decide, where it is
// actually load-bearing -- refusing at load time would break commands that need no actor.
//
// A file that exists and is malformed *is* an error: someone wrote it intending it to work.
//
// Ensures: the returned Actor is valid or zero, never invalid; it reads exactly one file.
func LoadActor(path string) (Actor, error) {
	data, err := os.ReadFile(path)
	if errs.Is(err, fs.ErrNotExist) {
		return Actor{}, nil
	}
	if err != nil {
		return Actor{}, errors.WrapWithMessage(err, "signoff: read config",
			slog.String("path", path))
	}
	var file struct {
		Identity struct {
			Actor string `yaml:"actor"`
		} `yaml:"identity"`
	}
	if err := yaml.Unmarshal(data, &file); err != nil {
		return Actor{}, errors.WrapWithMessage(err, "signoff: parse config",
			slog.String("path", path))
	}
	if strings.TrimSpace(file.Identity.Actor) == "" {
		return Actor{}, nil
	}
	return NewActor(file.Identity.Actor)
}

// Decide returns the event to record, or the reason this run may not record one.
//
// **A run that condemned the ruleset cannot also attest to it.** That is adh's rule and its
// wording is the reason: a sign-off on such a run "would attest to a state the same command
// disproved". Here the test is a blocking diagnostic. Advisories do not refuse -- they are
// findings for a person to weigh, and "blocking" is already this repository's word for a
// defect that must be fixed.
//
// **Refusing the whole run rather than signing the good parts** is also adh's, and it has no
// analogue to weaken here: a ruleset is one document, so there is no partial sign-off to be
// tempted by.
//
// **An unset actor refuses rather than recording an anonymous event**, because an event with
// no actor records nothing. It is reported *after* the blocking check, so a caller with both
// problems hears about the ruleset first: the findings are the substantive defect and the
// missing config is one-time setup, and fixing the config would not make the run signable.
//
// **The attribution is attributable, not authenticated.** It says which actor this checkout
// was configured as, not who was at the keyboard. Config-derived is what makes it better
// than a flag -- a flag would let any caller mint a human's sign-off -- and it is not a
// claim about identity that anything here can verify.
//
// Takes the whole Result rather than its diagnostics so that "blocking" is the kernel's
// definition via HasBlocking, which cmd/loop and cmd/budget already gate on. A second count
// here would be a second definition, and the two would drift.
//
// Requires: at is the time to record.
// Ensures: on success the event carries a non-empty By and an RFC3339 UTC At; on refusal the
//
//	returned event is zero; it is pure.
func Decide(result *finding.Result, actor Actor, at time.Time) (verification.Event, error) {
	if result.HasBlocking() {
		return verification.Event{}, errors.New(
			"signoff: refusing to sign a run with blocking findings; " +
				"a sign-off would attest to a state this run disproved")
	}
	if actor.IsZero() {
		return verification.Event{}, errors.New(
			"signoff: no actor configured; add identity.actor to " + ConfigName +
				` (e.g. actor: "human:steve") -- an event with no actor records nothing`)
	}
	return verification.Event{By: actor.String(), At: at.UTC().Format(time.RFC3339)}, nil
}
