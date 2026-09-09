// Package gate implements the "gate" command: it self-tests, then reads the JSON
// findings from a cold-critic run and blocks (non-zero exit) while any finding is
// blocking. The findings source is a flag; the self-test runs every invocation.
package gate

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/canonizer/cmd/root"
	"github.com/StevenACoffman/canonizer/internal/critic"
	gatelib "github.com/StevenACoffman/canonizer/internal/gate"
	"github.com/StevenACoffman/skillet/finding"
	errors "github.com/StevenACoffman/toerr/errors"
)

// Config holds the gate command's flag values and ff wiring. It embeds *root.Config
// for shared I/O.
type Config struct {
	*root.Config
	Findings string
	SelfTest bool
	Flags    *ff.FlagSet
	Command  *ff.Command
}

// New creates and registers the gate command under parent.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("gate").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.Findings, 0, "findings", "",
		"JSON findings file to gate (empty reads stdin)")
	cfg.Flags.BoolVar(&cfg.SelfTest, 0, "selftest",
		"run only the negative-control self-test and exit")
	cfg.Command = &ff.Command{
		Name:      "gate",
		Usage:     "canonizer gate [--findings FILE] [--selftest]",
		ShortHelp: "block a ruleset while any cold-critic finding is blocking",
		LongHelp: `Read the JSON findings a cold-critic run produced (skillet/finding
shape) from --findings or stdin, print them, and exit non-zero while any finding is
blocking (error severity). Before gating, gate runs a planted-defect self-test and
refuses to run if it cannot tell a blocking finding from a clean one.

Pass --selftest to run only that self-test and exit.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

// exec runs the self-test guard, then (unless --selftest) reads the findings, prints
// them, and returns ExitError(1) while any finding blocks. Flags are already parsed.
func (cfg *Config) exec(_ context.Context, _ []string) error {
	if err := gatelib.SelfTest(); err != nil {
		return errors.Wrap(err) // gatelib already prefixes "gate:"
	}
	if cfg.SelfTest {
		_, _ = fmt.Fprintln(cfg.Stdout, "gate: self-test passed")
		return nil
	}
	result, err := cfg.readFindings()
	if err != nil {
		return err
	}
	finding.Sort(result.Diagnostics)
	for i := range result.Diagnostics {
		d := result.Diagnostics[i]
		_, _ = fmt.Fprintf(
			cfg.Stdout,
			"%s [%s] %s: %s\n",
			d.Severity,
			d.Category,
			d.Path,
			d.Message,
		)
	}
	blocking := gatelib.Blocking(result)
	cfg.renderUnexamined(result.Unexamined)
	if len(blocking) > 0 {
		_, _ = fmt.Fprintf(cfg.Stderr, "gate: blocked by %d finding(s)\n", len(blocking))
		return root.ExitError(1)
	}
	_, _ = fmt.Fprintf(
		cfg.Stdout,
		"gate: clean (%d finding(s), 0 blocking)\n",
		len(result.Diagnostics),
	)
	// Stated on the clean path and only there. A blocked run is in no danger of being
	// overclaimed; the sentence someone quotes into a commit message is this one.
	_, _ = fmt.Fprintln(cfg.Stderr,
		"gate: clean means no deterministic check objected and one cold critic did not "+
			"either -- not that the rules are correct or that the set covers its scope")
	return nil
}

// renderUnexamined prints what the critic said it did not look at, below the findings and
// clearly separated from them.
//
// It is printed after the blocking decision has already been made, and nothing here can
// change it. That ordering is the guarantee, not a formality: a critic able to affect the
// outcome by declaring a gap learns to declare none, which costs the gap and the finding it
// would have come with. Separated in the output for the same reason -- a reader scanning
// for what is wrong must not meet these among the defects, because they are not defects.
//
// A reply that named no gaps is left silent rather than announced. Saying "no gaps
// declared" would read as a positive fact about coverage, and it is the absence of one.
func (cfg *Config) renderUnexamined(entries []finding.Unexamined) {
	if len(entries) == 0 {
		return
	}
	_, _ = fmt.Fprintf(cfg.Stdout,
		"\ngate: the critic reported %d angle(s) it did not examine "+
			"(recorded, never blocking):\n", len(entries))
	for _, u := range entries {
		_, _ = fmt.Fprintf(cfg.Stdout, "  - %s — %s\n", u.Aspect, u.Reason)
	}
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
			return finding.Result{}, errors.WrapWithMessage(err, "gate: read stdin")
		}
	} else if data, err = os.ReadFile(cfg.Findings); err != nil {
		return finding.Result{}, errors.WrapWithMessage(err, "gate: read findings",
			slog.String("path", cfg.Findings))
	}
	result, err := critic.ParseReply(data)
	if err != nil {
		// critic already prefixes; a reply rejected for a malformed coverage record is
		// reported as such rather than reduced to "bad JSON", which it is not.
		return finding.Result{}, errors.Wrap(err)
	}
	return result, nil
}
