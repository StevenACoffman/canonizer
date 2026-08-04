// Package distill implements the "distill" command: for every Markdown source
// under a tree it fills a per-source distillation prompt and writes a *_prompt.md
// beside the chosen output directory. It reproduces ai-skill's binary over
// skillet/ruleset/distill, taking every path as a flag rather than assuming a
// location relative to the binary or the working directory.
package distill

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/canonizer/cmd/root"
	"github.com/StevenACoffman/canonizer/internal/prompt"
	skdistill "github.com/StevenACoffman/skillet/ruleset/distill"
	errors "github.com/StevenACoffman/toerr/errors"
)

// Config holds the distill command's flag values and ff wiring. It embeds
// *root.Config for shared I/O.
type Config struct {
	*root.Config
	Template string
	Source   string
	Out      string
	Flags    *ff.FlagSet
	Command  *ff.Command
}

// New creates and registers the distill command under parent.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("distill").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.Template, 0, "template", "",
		"path to a distill prompt template (empty uses the built-in default)")
	cfg.Flags.StringVar(&cfg.Source, 0, "source", "",
		"directory tree of source .md files to distill")
	cfg.Flags.StringVar(&cfg.Out, 0, "out", "",
		"directory to write the *_prompt.md files into")
	cfg.Command = &ff.Command{
		Name:      "distill",
		Usage:     "canonizer distill --source DIR --out DIR [--template PATH]",
		ShortHelp: "fill a distillation prompt for every source in a tree",
		LongHelp: `Walk --source for Markdown files and, for each one, write a
*_prompt.md into --out that asks a model to distill that source into a ruleset.

The template defaults to a built-in prompt; pass --template to use your own. The
template must contain the {{SOURCE_CONTENT}} and {{DESTINATION_CONTENT}}
placeholders, which are validated before any file is written. Files named
*_rules.md, *_prompt.md, and hidden directories are skipped.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

// exec resolves the template, generates one prompt per source, and reports each
// written path. Flags are already parsed; it reads them from cfg. skillet already
// prefixes its errors with "distill:", so the boundary wrap adds a trace frame and
// structured attrs rather than a second prefix.
func (cfg *Config) exec(_ context.Context, _ []string) error {
	if cfg.Source == "" {
		return errors.New("distill: --source is required")
	}
	if cfg.Out == "" {
		return errors.New("distill: --out is required")
	}
	tmpl, err := prompt.Resolve(cfg.Template, prompt.Distill)
	if err != nil {
		return errors.WrapWithMessage(err, "distill")
	}
	written, err := skdistill.Generate(tmpl, cfg.Source, cfg.Out)
	if err != nil {
		return errors.Wrap(err, slog.String("source", cfg.Source), slog.String("out", cfg.Out))
	}
	for _, path := range written {
		_, _ = fmt.Fprintf(cfg.Stdout, "wrote %s\n", path)
	}
	return nil
}
