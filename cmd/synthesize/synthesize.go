// Package synthesize implements the "synthesize" command: it loads the distilled
// *_rules.md files in a directory, assembles them into the synthesis prompt via
// skillet/ruleset/synthesize, and writes the result to a file or stdout. It
// reproduces ai-skill's second prompt asset as an automated step, with every path
// supplied as a flag.
package synthesize

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/canonizer/cmd/root"
	"github.com/StevenACoffman/canonizer/internal/prompt"
	sksynth "github.com/StevenACoffman/skillet/ruleset/synthesize"
	errors "github.com/StevenACoffman/toerr/errors"
)

// Config holds the synthesize command's flag values and ff wiring. It embeds
// *root.Config for shared I/O.
type Config struct {
	*root.Config
	Template string
	Rulesets string
	Out      string
	Flags    *ff.FlagSet
	Command  *ff.Command
}

// New creates and registers the synthesize command under parent.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("synthesize").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.Template, 0, "template", "",
		"path to a synthesis prompt template (empty uses the built-in default)")
	cfg.Flags.StringVar(&cfg.Rulesets, 0, "rulesets", "",
		"directory of distilled *_rules.md files to merge")
	cfg.Flags.StringVar(&cfg.Out, 0, "out", "",
		"file to write the assembled prompt into (empty writes to stdout)")
	cfg.Command = &ff.Command{
		Name:      "synthesize",
		Usage:     "canonizer synthesize --rulesets DIR [--out FILE] [--template PATH]",
		ShortHelp: "assemble one synthesis prompt from distilled rulesets",
		LongHelp: `Read every *_rules.md in --rulesets and assemble them into a single
prompt that asks a model to merge them into one unified ruleset.

The template defaults to a built-in prompt; pass --template to use your own. The
template must contain the {{RULESETS}} marker, which is replaced with one
<ruleset> block per input. With no --out the assembled prompt is written to
stdout.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

// exec resolves the template, loads the rulesets via skillet, fills the prompt,
// and writes it to --out or stdout. Flags are already parsed; it reads them from
// cfg. skillet already prefixes its errors with "synthesize:", so the boundary
// wraps add a trace frame and structured attrs rather than a second prefix.
func (cfg *Config) exec(_ context.Context, _ []string) error {
	if cfg.Rulesets == "" {
		return errors.New("synthesize: --rulesets is required")
	}
	tmpl, err := prompt.Resolve(cfg.Template, prompt.Synthesize)
	if err != nil {
		return errors.WrapWithMessage(err, "synthesize")
	}
	inputs, err := sksynth.LoadInputs(cfg.Rulesets)
	if err != nil {
		return errors.Wrap(err, slog.String("rulesets", cfg.Rulesets))
	}
	if len(inputs) == 0 {
		return errors.New("synthesize: no _rules.md files in " + cfg.Rulesets)
	}
	filled, err := sksynth.FillTemplate(tmpl, inputs)
	if err != nil {
		return errors.Wrap(err)
	}
	if cfg.Out == "" {
		_, _ = fmt.Fprint(cfg.Stdout, filled)
		return nil
	}
	if writeErr := os.WriteFile(cfg.Out, []byte(filled), 0o600); writeErr != nil {
		return errors.WrapWithMessage(writeErr, "synthesize: write", slog.String("out", cfg.Out))
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "wrote %s\n", cfg.Out)
	return nil
}
