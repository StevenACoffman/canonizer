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

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/canonizer/cmd/root"
	vfy "github.com/StevenACoffman/canonizer/internal/verify"
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
	Flags   *ff.FlagSet
	Command *ff.Command
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
	cfg.Command = &ff.Command{
		Name:      "verify",
		Usage:     "canonizer verify --ruleset PATH [--source PATH] [--proof PATH] [--out FILE]",
		ShortHelp: "check a ruleset's executability and provenance, emitting findings",
		LongHelp: `Parse a canonical ruleset and run canonizer's deterministic checks:
executability (every enforced rule carries a discriminating ✗/✓ pair) and — with
--source — provenance (every enforced rule cites a source anchor present in the
source). The result is a skillet/finding JSON document; pipe it to "gate" to block
on it. With --proof, also write a proof packet binding the ruleset to the source.`,
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
	diags, err := vfy.Executable(rs)
	if err != nil {
		return errors.Wrap(err) // vfy already prefixes "verify:"
	}
	// Both are advisory and independent of --source, so they run before the provenance
	// block rather than inside it: a run without a source must still report them.
	diags = append(diags, vfy.Specificity(rs)...)
	diags = append(diags, vfy.Conflicts(rs)...)
	diags = append(diags, vfy.Canonical(string(raw), rs)...)
	diags = append(diags, vfy.Limitations(rs)...)
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
		diags = append(diags, vfy.Drift(rs, string(source), state)...)
	} else {
		// No source is not "no provenance problem": it is the third state, where an
		// anchor is neither present nor absent because nothing searched for it. Reported
		// here rather than left silent, because silence is what a later reader -- or the
		// summary line below -- turns into a pass.
		diags = append(diags, vfy.Unverifiable(rs)...)
	}
	diags = append(diags, cfg.reportScope(rs)...)
	finding.Sort(diags)
	if err := cfg.emit(finding.Result{Diagnostics: diags}); err != nil {
		return err
	}
	cfg.renderAct()
	if cfg.Proof != "" {
		return cfg.writeProof()
	}
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
func (cfg *Config) reportScope(rs ruleset.Ruleset) []finding.Diagnostic {
	scope := vfy.Rules(rs)
	_, _ = fmt.Fprintf(cfg.Stderr, "verify: examined %d of %d rule(s); %d exempt as advisory\n",
		scope.Enforced, scope.Total, scope.Total-scope.Enforced)
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
