// Package loop implements the "loop" command: one deterministic round of the rework
// loop. It verifies the candidate ruleset, merges the agent's cold-critic findings, and
// decides ship / rework / needs-human under the budget — so the independent-verification
// loop runs end-to-end rather than being assembled by hand. canonizer calls no model;
// the critic and the rework are the agent's steps between rounds.
package loop

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/canonizer/cmd/root"
	bdg "github.com/StevenACoffman/canonizer/internal/budget"
	vfy "github.com/StevenACoffman/canonizer/internal/verify"
	"github.com/StevenACoffman/skillet/finding"
	"github.com/StevenACoffman/skillet/ruleset"
	errors "github.com/StevenACoffman/toerr/errors"
)

// Config holds the loop command's flag values and ff wiring. It embeds *root.Config for
// shared I/O.
type Config struct {
	*root.Config
	Source   string
	Ruleset  string
	Findings string
	Model    string
	Attempt  int
	Max      int
	Flags    *ff.FlagSet
	Command  *ff.Command
}

// New creates and registers the loop command under parent.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("loop").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.Source, 0, "source", "", "source document (for provenance)")
	cfg.Flags.StringVar(&cfg.Ruleset, 0, "ruleset", "", "candidate canonical *_rules.md")
	cfg.Flags.StringVar(&cfg.Findings, 0, "findings", "",
		"the agent's cold-critic findings JSON for this round (empty = none yet)")
	cfg.Flags.StringVar(&cfg.Model, 0, "model", "",
		"grader model the operator attests the prompts were run on "+
			"(recorded for audit; unverified, never gates the verdict)")
	cfg.Flags.IntVar(&cfg.Attempt, 0, "attempt", 1, "the refine attempt just completed (1-based)")
	cfg.Flags.IntVar(&cfg.Max, 0, "max", 3, "the rework budget: total attempts allowed")
	cfg.Command = &ff.Command{
		Name:      "loop",
		Usage:     "canonizer loop --source PATH --ruleset PATH [--findings FILE] --attempt K --max N",
		ShortHelp: "run one round of the rework loop: verify, merge critic findings, decide",
		LongHelp: `Run one deterministic round of the independent-verification loop: verify
the candidate ruleset (executability + provenance), merge the agent's cold-critic
findings, and decide ship / rework / needs-human under the rework budget. A ruleset with
blocking findings is never shipped.

canonizer calls no model — the critic and the rework are the agent's steps between
rounds. A driver wraps this command, holding the attempt counter:

  canonizer critic --source S --ruleset R --out critic.md   # emit the cold-critic prompt
  # agent runs critic.md -> findings.json, then:
  canonizer loop --source S --ruleset R --findings findings.json --attempt K --max 3
    -> exit 0 ship | 2 rework (agent reworks R; K++; repeat) | 1 needs-human

--model records, for audit, the grader model the operator attests they ran the prompts
on. canonizer cannot observe the model, so it is an unverified attestation, never a gate:
running the prompts on a reasoning-class model stays the operator's responsibility.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

// exec runs one round: it collects the round's findings, prints them, and decides the
// verdict, translating it to an exit code. Flags are already parsed.
func (cfg *Config) exec(_ context.Context, _ []string) error {
	if cfg.Source == "" {
		return errors.New("loop: --source is required")
	}
	if cfg.Ruleset == "" {
		return errors.New("loop: --ruleset is required")
	}
	if cfg.Model != "" {
		// An operator attestation, not a check: canonizer fills prompts an agent runs,
		// so it cannot observe the grader model. Record it for audit, labeled unverified,
		// and never let it touch the verdict.
		_, _ = fmt.Fprintf(
			cfg.Stderr,
			"loop: grader model (operator-attested, unverified): %s\n",
			cfg.Model,
		)
	}
	diags, err := cfg.roundFindings()
	if err != nil {
		return err
	}
	finding.Sort(diags)
	for i := range diags {
		d := &diags[i]
		_, _ = fmt.Fprintf(
			cfg.Stderr,
			"%s [%s] %s: %s\n",
			d.Severity,
			d.Category,
			d.Path,
			d.Message,
		)
	}
	result := finding.Result{Diagnostics: diags}
	verdict := bdg.Decide(result.HasBlocking(), cfg.Attempt, cfg.Max)
	_, _ = fmt.Fprintln(cfg.Stdout, verdict)
	switch verdict {
	case bdg.Ship:
		_, _ = fmt.Fprintln(cfg.Stderr, "loop: clean — adopt the ruleset")
	case bdg.Rework:
		_, _ = fmt.Fprintf(cfg.Stderr,
			"loop: blocked at attempt %d/%d — rework the ruleset and re-run with --attempt %d\n",
			cfg.Attempt, cfg.Max, cfg.Attempt+1)
		return root.ExitError(2)
	case bdg.NeedsHuman:
		_, _ = fmt.Fprintf(cfg.Stderr,
			"loop: blocked and budget spent (%d/%d) — needs human\n", cfg.Attempt, cfg.Max)
		return root.ExitError(1)
	}
	return nil
}

// roundFindings verifies the candidate ruleset (executability + provenance) and appends
// the agent's cold-critic findings for this round.
func (cfg *Config) roundFindings() ([]finding.Diagnostic, error) {
	raw, err := os.ReadFile(cfg.Ruleset)
	if err != nil {
		return nil, errors.WrapWithMessage(
			err,
			"loop: read ruleset",
			slog.String("path", cfg.Ruleset),
		)
	}
	rs, err := ruleset.Parse(string(raw))
	if err != nil {
		return nil, errors.WrapWithMessage(err, "loop: parse ruleset")
	}
	source, err := os.ReadFile(cfg.Source)
	if err != nil {
		return nil, errors.WrapWithMessage(
			err,
			"loop: read source",
			slog.String("path", cfg.Source),
		)
	}
	diags, err := vfy.Executable(rs)
	if err != nil {
		return nil, errors.Wrap(err) // vfy already prefixes "verify:"
	}
	diags = append(diags, vfy.Provenance(rs, string(source))...)
	critic, err := cfg.criticFindings()
	if err != nil {
		return nil, err
	}
	return append(diags, critic...), nil
}

// criticFindings reads the agent's cold-critic findings for this round, or none when
// --findings is empty (the first round, before any critic run).
func (cfg *Config) criticFindings() ([]finding.Diagnostic, error) {
	if cfg.Findings == "" {
		return nil, nil
	}
	data, err := os.ReadFile(cfg.Findings)
	if err != nil {
		return nil, errors.WrapWithMessage(
			err,
			"loop: read findings",
			slog.String("path", cfg.Findings),
		)
	}
	var result finding.Result
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, errors.WrapWithMessage(err, "loop: parse findings")
	}
	return result.Diagnostics, nil
}
