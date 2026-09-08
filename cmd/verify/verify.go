// Package verify implements the "verify" command: it parses a canonical ruleset and
// runs canonizer's deterministic checks over it — executability (B) and, given a
// source, provenance (E) — emitting skillet/finding JSON for the "gate" command to
// block on. With --proof it also writes a proof packet binding the ruleset to the
// source bytes. Every path is a flag.
package verify

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/canonizer/cmd/root"
	"github.com/StevenACoffman/canonizer/internal/signoff"
	vfy "github.com/StevenACoffman/canonizer/internal/verify"
	"github.com/StevenACoffman/skillet/atomicfile"
	"github.com/StevenACoffman/skillet/finding"
	"github.com/StevenACoffman/skillet/identity"
	"github.com/StevenACoffman/skillet/proof"
	"github.com/StevenACoffman/skillet/ruleset"
	errors "github.com/StevenACoffman/toerr/errors"
)

// Config holds the verify command's flag values and ff wiring. It embeds
// *root.Config for shared I/O.
type Config struct {
	*root.Config
	Ruleset string
	Source  string
	Proof   string
	Against string
	Out     string
	SignOff bool
	Flags   *ff.FlagSet
	Command *ff.Command

	// ConfigPath is the file the attributed actor is read from, bound to --config.
	ConfigPath string

	// Now is the clock a recorded event is stamped from. Injected because a test cannot
	// otherwise assert a timestamp, and canonizer had no clock convention before this.
	Now func() time.Time
}

// New creates and registers the verify command under parent.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("verify").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.Ruleset, 0, "ruleset", "",
		"canonical *_rules.md to verify")
	cfg.Flags.StringVar(&cfg.Source, 0, "source", "",
		"source document; enables the provenance (anchor) checks")
	cfg.Flags.StringVar(&cfg.Proof, 0, "proof", "",
		"write a proof packet binding the ruleset and source bytes to this path")
	cfg.Flags.StringVar(&cfg.Against, 0, "against-proof", "",
		"a proof packet from distill time; splits an absent anchor into fabricated, stale or drift")
	cfg.Flags.StringVar(&cfg.Out, 0, "out", "",
		"findings JSON destination (empty writes to stdout)")
	cfg.Flags.BoolVar(&cfg.SignOff, 0, "sign-off",
		"record a verification event on the ruleset, attributed to the configured actor")
	cfg.Flags.StringVar(&cfg.ConfigPath, 0, "config", signoff.ConfigName,
		"file holding identity.actor, the attribution a --sign-off is recorded under")
	cfg.Now = time.Now
	cfg.Command = &ff.Command{
		Name:      "verify",
		Usage:     "canonizer verify --ruleset PATH [--source PATH] [--proof PATH] [--out FILE]",
		ShortHelp: "check a ruleset's executability and provenance, emitting findings",
		LongHelp: `Parse a canonical ruleset and run canonizer's deterministic checks:
executability (every enforced rule carries a discriminating ✗/✓ pair) and — with
--source — provenance (every enforced rule cites a source anchor present in the
source). The result is a skillet/finding JSON document; pipe it to "gate" to block
on it. With --proof, also write a proof packet binding the ruleset to the source.

With --sign-off, append a verification event to the ruleset's frontmatter recording
who confirmed it. The attribution comes from identity.actor in --config, never from
a flag, because a caller-supplied actor would let anyone mint a human's sign-off. It
is attributable rather than authenticated: it says which actor this checkout was
configured as, not who was at the keyboard.

A sign-off is refused three ways, and each refusal is the point rather than an
obstacle: when the run found blocking findings, because it would attest to a state
the same run disproved; when --source was not given, because nothing then searched
for the anchors and the event would vouch for provenance nobody examined; and when
no actor is configured, because an event with no actor records nothing.

  identity:
    actor: "human:steve"    # <class>:<name>; the class is never inferred`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

// exec parses the ruleset, runs the deterministic checks, emits the findings, and —
// when --proof is set — writes the binding packet. Flags are already parsed.
func (cfg *Config) exec(_ context.Context, _ []string) error {
	if cfg.Ruleset == "" {
		return errors.New("verify: --ruleset is required")
	}
	raw, err := os.ReadFile(cfg.Ruleset)
	if err != nil {
		return errors.WrapWithMessage(err, "verify: read ruleset", slog.String("path", cfg.Ruleset))
	}
	rs, err := ruleset.Parse(string(raw))
	if err != nil {
		return errors.WrapWithMessage(err, "verify: parse ruleset")
	}
	diags, err := vfy.Executable(&rs)
	if err != nil {
		return errors.Wrap(err) // vfy already prefixes "verify:"
	}
	// Both are advisory and independent of --source, so they run before the provenance
	// block rather than inside it: a run without a source must still report them.
	diags = append(diags, vfy.Softening(&rs)...)
	diags = append(diags, vfy.Conflicts(&rs)...)
	diags = append(diags, vfy.Canonical(string(raw), &rs)...)
	diags = append(diags, vfy.Limitations(&rs)...)
	diags = append(diags, vfy.Soundness(&rs)...)
	if cfg.Source != "" {
		source, readErr := os.ReadFile(cfg.Source)
		if readErr != nil {
			return errors.WrapWithMessage(
				readErr,
				"verify: read source",
				slog.String("path", cfg.Source),
			)
		}
		state, stateErr := cfg.sourceState(source)
		if stateErr != nil {
			return stateErr
		}
		diags = append(diags, vfy.Drift(&rs, string(source), state)...)
	} else {
		// No source is not "no provenance problem": it is the third state, where an
		// anchor is neither present nor absent because nothing searched for it. Reported
		// here rather than left silent, because silence is what a later reader -- or the
		// summary line below -- turns into a pass.
		diags = append(diags, vfy.Unverifiable(&rs)...)
	}
	diags = append(diags, cfg.reportScope(&rs)...)
	finding.Sort(diags)
	if err := cfg.emit(finding.Result{Diagnostics: diags}); err != nil {
		return err
	}
	cfg.renderAct()
	// After emit, so the findings a refusal refers to have already been reported: a caller
	// told "there are blocking findings" can see which ones without re-running.
	if err := cfg.signOff(&rs, &finding.Result{Diagnostics: diags}); err != nil {
		return err
	}
	if cfg.Proof != "" {
		return cfg.writeProof()
	}
	return nil
}

// signOff appends a verification event to the ruleset when this run permits one.
//
// The policy is signoff.Decide's; this is the shell around it -- load the actor, ask, and
// on a yes write the ruleset back.
//
// **Writing is atomic, unlike the other three writes in this command.** Those create new
// files, where a partial write costs a re-run. This one replaces a document a human owns
// and cannot regenerate, so a crash mid-write must not be able to truncate it.
//
// **It appends to rs.Verified**, so the ruleset it is given is modified rather than copied.
// Called last and with exec's own local, which is why that is safe here rather than merely
// convenient -- but it is a modified parameter and so worth saying.
//
// **Re-rendering cannot reformat the body, and that follows from the refusal rather than
// from care.** Canonical runs on every verify and reports a non-canonical ruleset as a
// blocking finding, so Decide has already refused every document whose stored form differs
// from its rendering. What Render emits here is therefore the original bytes plus the
// frontmatter block.
func (cfg *Config) signOff(rs *ruleset.Ruleset, result *finding.Result) error {
	if !cfg.SignOff {
		return nil
	}
	if cfg.Source == "" {
		// Measured, and the incentive ran backwards without this: withholding --source
		// drops the blocking count on the stored corpus from 17 to 5, 15 to 3 and 8 to 2,
		// because the anchor checks are replaced by Unverifiable, which is advisory. A
		// caller could therefore get closer to a signable run by supplying less evidence.
		// Refusing is the same fail-closed rule this family keeps arriving at: not checked
		// must not read as clean.
		return errors.New(
			"verify: --sign-off needs --source; without it the anchors are never " +
				"searched for, so the event would attest to provenance nothing examined")
	}
	actor, err := signoff.LoadActor(cfg.ConfigPath)
	if err != nil {
		return errors.Wrap(err) // signoff already prefixes "signoff:"
	}
	event, err := signoff.Decide(result, actor, cfg.Now())
	if err != nil {
		return errors.Wrap(err)
	}
	rs.Verified = append(rs.Verified, event)
	if writeErr := atomicfile.WriteFile(
		cfg.Ruleset, []byte(ruleset.Render(rs)), 0o600,
	); writeErr != nil {
		return errors.WrapWithMessage(writeErr, "verify: write ruleset",
			slog.String("path", cfg.Ruleset))
	}
	_, _ = fmt.Fprintf(cfg.Stderr, "verify: recorded %s at %s\n", event.By, event.At)
	return nil
}

// reportScope states how much of the ruleset the enforced-rule gates examined, and
// returns a diagnostic for the one case a reader could misread.
//
// The line goes to stderr on every run because the proportion is worth seeing even when it
// is 100%: it is the number nobody could see before, and a reader who only ever meets it
// on the bad day has no baseline to compare against.
//
// The diagnostic is emitted only when nothing was examined, and it goes into the findings
// rather than staying on stderr so it reaches `gate` and anything else reading the JSON --
// a fact that lives only in a terminal is a fact somebody did not read. Advisory, never
// blocking: a ruleset of entirely CONSIDER rules is legitimate, and the defect being fixed
// here was the silence, not the ruleset.
func (cfg *Config) reportScope(rs *ruleset.Ruleset) []finding.Diagnostic {
	scope := vfy.Rules(rs)
	_, _ = fmt.Fprintf(cfg.Stderr, "verify: examined %d of %d rule(s); %d exempt as advisory\n",
		scope.Enforced, scope.Total, scope.Total-scope.Enforced)
	// Printed on every run including at 100%, for the reason the line above is: it is a
	// number nobody could see before, and a reader who only ever meets it at 100% never
	// learns what it means. The same prompt over eight sources produced rates from 9% to
	// 78%, so a specificity reading is only as comparable as the run's typography.
	_, _ = fmt.Fprintf(cfg.Stderr,
		"verify: %d of %d enforced rule(s) name a symbol a checker can see\n",
		scope.Symbolic, scope.Enforced)
	// Printed only when non-zero, and the asymmetry is deliberate: zero is the case in all
	// 162 anchors of the corpus, so a line reporting none of them every run costs attention
	// and teaches nothing. The line above is a proportion informative anywhere in its range;
	// this one is an exception report.
	if scope.SectionOnly > 0 {
		_, _ = fmt.Fprintf(cfg.Stderr,
			"verify: %d of %d anchor(s) name a section only; their provenance was not searched\n",
			scope.SectionOnly, scope.Enforced)
	}
	if !scope.Advisory() {
		return nil
	}
	return []finding.Diagnostic{{
		Severity: finding.SeverityWarning,
		Action:   finding.ActionHuman,
		Category: vfy.CategoryNothingExamined,
		Path:     "ruleset",
		Message: fmt.Sprintf(
			"no rule is enforced, so these gates examined none of %d; "+
				"an empty result here means unchecked, not clean", scope.Total),
	}}
}

// sourceState reports whether the source still hashes to the digest recorded at distill
// time, which is the second signal Drift crosses with each rule's anchor.
//
// Without --against-proof there is no second signal and the answer is SourceUnknown, under
// which Drift falls back to Provenance exactly. That is the honest default: with one signal
// an absent anchor cannot be told from a moved source, and guessing would be worse than
// saying so.
//
// A packet that does not name the source is SourceUnknown rather than an error. It means
// this proof was taken over a different set of files, which is a mismatch to report by
// declining to answer -- not a reason to fail a verify run that is otherwise fine.
func (cfg *Config) sourceState(source []byte) (vfy.SourceState, error) {
	if cfg.Against == "" {
		return vfy.SourceUnknown, nil
	}
	packet, err := proof.Load(cfg.Against)
	if err != nil {
		return vfy.SourceUnknown, errors.WrapWithMessage(
			err, "verify: load proof", slog.String("against-proof", cfg.Against))
	}
	digest, ok := digestOf(&packet, cfg.Source)
	if !ok {
		_, _ = fmt.Fprintf(cfg.Stderr,
			"verify: %s records no artifact for %s; anchor drift not separated\n",
			cfg.Against, cfg.Source)
		return vfy.SourceUnknown, nil
	}
	if identity.Hash(string(source)) == digest {
		return vfy.SourceUnchanged, nil
	}
	return vfy.SourceChanged, nil
}

// digestOf returns the digest the packet records for path, matching on the file name so a
// packet written from another working directory still resolves.
func digestOf(packet *proof.Packet, path string) (string, bool) {
	want := filepath.Base(path)
	for _, a := range packet.Artifacts {
		if filepath.Base(a.Path) == want {
			return a.Digest, true
		}
	}
	return "", false
}

// renderAct names the act this run performed and the act it deliberately did not.
//
// verify is the *structural* half of a two-act split -- schema, anchors, canonical form,
// all local and deterministic -- and `critic` is the *semantic* half, a fresh grader
// re-earning the verdicts. VAC's rule for the same pair is that the structural verifier
// never performs semantic replay **and says so in its output**, because a clean structural
// pass is otherwise trivially read as a full one.
//
// It states only the negative. The checks that ran printed their own findings above, so
// naming them again would repeat what the output already shows; what no other line says is
// that a green run is silent about support *by design*.
//
// **This is deliberately not in the findings JSON, and the reason is worth recording.**
// finding.Unexamined is the obvious home and is the wrong one: skillet documents it as
// testimony -- an agent's unverifiable claim about its own behaviour -- and explicitly
// separates it from the case "where code decided a check did not apply and can say so
// mechanically", which is this. A diagnostic is worse: it would fire on every run, so every
// clean verify would report one finding and the count would stop meaning anything. The
// machine-readable half needs a producer field on finding.Result, which is skillet's.
func (cfg *Config) renderAct() {
	// The clause about anchors is conditional because it was **false** on a run without
	// --source: nothing searched for them, and claiming they are present is exactly the
	// laundering of "not checked" into "checked and clean" that the unverifiable state
	// exists to prevent. Corrected when that state landed.
	claim := "this ruleset is well-formed and its anchors are present in the source, " +
		"not that the source supports what any rule claims"
	if cfg.Source == "" {
		claim = "this ruleset is well-formed -- no source was supplied, so nothing " +
			"searched for its anchors at all"
	}
	_, _ = fmt.Fprintln(cfg.Stderr,
		"verify: structural checks only; semantic replay not performed (a pass means "+
			claim+")")
}

// emit writes the findings as JSON to --out, or stdout when it is empty.
func (cfg *Config) emit(result finding.Result) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return errors.WrapWithMessage(err, "verify: marshal findings")
	}
	data = append(data, '\n')
	if cfg.Out == "" {
		_, _ = cfg.Stdout.Write(data)
		return nil
	}
	if writeErr := os.WriteFile(cfg.Out, data, 0o600); writeErr != nil {
		return errors.WrapWithMessage(
			writeErr,
			"verify: write findings",
			slog.String("out", cfg.Out),
		)
	}
	return nil
}

// writeProof binds the ruleset (and source, when given) to their exact bytes in a
// proof packet. root "" uses the paths as given, so absolute or cwd-relative both work.
func (cfg *Config) writeProof() error {
	paths := []string{cfg.Ruleset}
	if cfg.Source != "" {
		paths = append(paths, cfg.Source)
	}
	packet, err := proof.Create("", "ruleset-provenance", "", paths)
	if err != nil {
		return errors.WrapWithMessage(err, "verify: create proof")
	}
	if err := proof.Save(cfg.Proof, &packet); err != nil {
		return errors.WrapWithMessage(err, "verify: save proof", slog.String("proof", cfg.Proof))
	}
	_, _ = fmt.Fprintf(cfg.Stderr, "verify: wrote proof %s\n", cfg.Proof)
	return nil
}
