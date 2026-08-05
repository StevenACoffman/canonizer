// Package budget implements the "budget" command: it reads a findings result and,
// given where the refine loop stands, decides whether to ship, rework, or escalate to
// a human — the terminal-escalation half of the rework budget. It prints the verdict
// and exits 0 (ship), 2 (rework), or 1 (needs-human) so a driver can branch.
package budget

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/canonizer/cmd/root"
	bdg "github.com/StevenACoffman/canonizer/internal/budget"
	"github.com/StevenACoffman/skillet/finding"
	errors "github.com/StevenACoffman/toerr/errors"
)

// Config holds the budget command's flag values and ff wiring. It embeds
// *root.Config for shared I/O.
type Config struct {
	*root.Config
	Findings string
	Attempt  int
	Max      int
	Flags    *ff.FlagSet
	Command  *ff.Command
}

// New creates and registers the budget command under parent.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("budget").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.Findings, 0, "findings", "",
		"JSON findings to judge (empty reads stdin)")
	cfg.Flags.IntVar(&cfg.Attempt, 0, "attempt", 1,
		"the refine attempt just completed (1-based)")
	cfg.Flags.IntVar(&cfg.Max, 0, "max", 3,
		"the rework budget: total attempts allowed")
	cfg.Command = &ff.Command{
		Name:      "budget",
		Usage:     "canonizer budget [--findings FILE] --attempt K --max N",
		ShortHelp: "decide ship / rework / needs-human under the rework budget",
		LongHelp: `Read the findings from a verify or critic run and decide the next step
in the refine loop: ship (no blocking findings), rework (blocked but attempts remain),
or needs-human (blocked and the budget is spent). A ruleset with blocking findings is
never shipped.

The verdict is printed to stdout and signalled by exit code so a driver can branch:
0 = ship, 2 = rework, 1 = needs-human. A loop runs verify → budget each round,
incrementing --attempt, and reworks while budget exits 2.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

// exec reads the findings, decides the verdict, prints it, and translates it to an
// exit code. Flags are already parsed.
func (cfg *Config) exec(_ context.Context, _ []string) error {
	result, err := cfg.readFindings()
	if err != nil {
		return err
	}
	verdict := bdg.Decide(result.HasBlocking(), cfg.Attempt, cfg.Max)
	_, _ = fmt.Fprintln(cfg.Stdout, verdict)
	switch verdict {
	case bdg.Ship:
		_, _ = fmt.Fprintln(cfg.Stderr, "budget: clean — adopt the ruleset")
	case bdg.Rework:
		_, _ = fmt.Fprintf(
			cfg.Stderr,
			"budget: blocked at attempt %d/%d — rework\n",
			cfg.Attempt,
			cfg.Max,
		)
		return root.ExitError(2)
	case bdg.NeedsHuman:
		_, _ = fmt.Fprintf(
			cfg.Stderr,
			"budget: blocked and budget spent (%d/%d) — needs human\n",
			cfg.Attempt,
			cfg.Max,
		)
		return root.ExitError(1)
	}
	return nil
}

// readFindings reads the findings JSON from --findings, or stdin when it is empty,
// and parses it into a finding.Result.
func (cfg *Config) readFindings() (finding.Result, error) {
	var (
		data []byte
		err  error
	)
	if cfg.Findings == "" {
		if data, err = io.ReadAll(cfg.Stdin); err != nil {
			return finding.Result{}, errors.WrapWithMessage(err, "budget: read stdin")
		}
	} else if data, err = os.ReadFile(cfg.Findings); err != nil {
		return finding.Result{}, errors.WrapWithMessage(err, "budget: read findings",
			slog.String("path", cfg.Findings))
	}
	var result finding.Result
	if err := json.Unmarshal(data, &result); err != nil {
		return finding.Result{}, errors.WrapWithMessage(err, "budget: parse findings")
	}
	return result, nil
}
