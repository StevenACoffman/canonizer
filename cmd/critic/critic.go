// Package critic implements the "critic" command: it fills the cold-critic prompt
// with a source document and a candidate ruleset and writes it, for a fresh agent to
// run. The agent's JSON findings are then gated by the "gate" command. Every path is
// a flag.
package critic

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/canonizer/cmd/root"
	crit "github.com/StevenACoffman/canonizer/internal/critic"
	"github.com/StevenACoffman/canonizer/internal/prompt"
	"github.com/StevenACoffman/skillet/ruleset"
	errors "github.com/StevenACoffman/toerr/errors"
)

// Config holds the critic command's flag values and ff wiring. It embeds
// *root.Config for shared I/O.
type Config struct {
	*root.Config
	Source   string
	Ruleset  string
	Template string
	Out      string
	Flags    *ff.FlagSet
	Command  *ff.Command
}

// New creates and registers the critic command under parent.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("critic").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.Source, 0, "source", "",
		"source document the ruleset was distilled from")
	cfg.Flags.StringVar(&cfg.Ruleset, 0, "ruleset", "",
		"candidate *_rules.md to critique")
	cfg.Flags.StringVar(&cfg.Template, 0, "template", "",
		"path to a critic prompt template (empty uses the built-in default)")
	cfg.Flags.StringVar(&cfg.Out, 0, "out", "",
		"file to write the filled prompt into (empty writes to stdout)")
	cfg.Command = &ff.Command{
		Name:      "critic",
		Usage:     "canonizer critic --source PATH --ruleset PATH [--out FILE] [--template PATH]",
		ShortHelp: "emit a cold-critic prompt for a candidate ruleset",
		LongHelp: `Fill the cold-critic prompt with a source document and a candidate
ruleset and write it to --out or stdout. A fresh agent runs the prompt and returns
JSON findings (skillet/finding shape); the "gate" command then blocks the ruleset
while any finding is blocking.

The grader sees only the source and the ruleset — never how the ruleset was
produced — so its judgement is independent of the distillation.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

// exec resolves the template, reads the source and ruleset, fills the prompt, and
// writes it to --out or stdout. Flags are already parsed; it reads them from cfg.
func (cfg *Config) exec(_ context.Context, _ []string) error {
	if cfg.Source == "" {
		return errors.New("critic: --source is required")
	}
	if cfg.Ruleset == "" {
		return errors.New("critic: --ruleset is required")
	}
	tmpl, err := prompt.Resolve(cfg.Template, prompt.Critic)
	if err != nil {
		return errors.WrapWithMessage(err, "critic")
	}
	source, err := os.ReadFile(cfg.Source)
	if err != nil {
		return errors.WrapWithMessage(err, "critic: read source", slog.String("path", cfg.Source))
	}
	candidate, err := os.ReadFile(cfg.Ruleset)
	if err != nil {
		return errors.WrapWithMessage(err, "critic: read ruleset", slog.String("path", cfg.Ruleset))
	}
	rs, err := ruleset.Parse(string(candidate))
	if err != nil {
		return errors.WrapWithMessage(err, "critic: candidate ruleset")
	}
	_, _ = fmt.Fprintf(cfg.Stderr, "critic: critiquing %d rule(s)\n", len(rs.Rules))
	filled, err := crit.FillPrompt(tmpl, string(source), string(candidate))
	if err != nil {
		return errors.Wrap(err) // crit already prefixes "critic:"
	}
	if cfg.Out == "" {
		_, _ = fmt.Fprint(cfg.Stdout, filled)
		return nil
	}
	if writeErr := os.WriteFile(cfg.Out, []byte(filled), 0o600); writeErr != nil {
		return errors.WrapWithMessage(writeErr, "critic: write", slog.String("out", cfg.Out))
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "wrote %s\n", cfg.Out)
	return nil
}
