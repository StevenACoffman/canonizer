// Package rework implements the "rework" command: it fills the revision template with a
// candidate ruleset and the findings raised against it, and writes the prompt an agent runs
// to revise the ruleset in place.
//
// **It is the only prompt canonizer emits that asks an agent to edit an artifact rather
// than produce one**, which is why it exists as a command at all. The text lived in
// pipeline.sh's heredoc, where it was unversioned, untested, and invisible to the template
// tests that guard every other prompt against silently losing the canonical form — and it
// is the prompt with the most leverage in the pipeline, since a bad revision instruction
// damages a ruleset that already passed synthesis.
package rework

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/canonizer/cmd/root"
	"github.com/StevenACoffman/canonizer/internal/prompt"
	errors "github.com/StevenACoffman/toerr/errors"
)

const (
	rulesetMarker  = "{{RULESET}}"
	findingsMarker = "{{FINDINGS}}"
)

// Config holds the rework command's flag values and ff wiring. It embeds *root.Config for
// shared I/O.
type Config struct {
	*root.Config
	Ruleset  string
	Findings []string
	Out      string
	Template string
	Flags    *ff.FlagSet
	Command  *ff.Command
}

// New creates and registers the rework command under parent.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("rework").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.Ruleset, 0, "ruleset", "",
		"candidate ruleset the agent should revise in place")
	// Repeatable because a round produces two findings documents -- the deterministic checks
	// and the independent grader -- and an agent reworking against one of them would resolve
	// half the round. Ordering is the caller's; each is embedded under its own path.
	cfg.Flags.StringListVar(&cfg.Findings, 0, "findings",
		"findings JSON raised against the ruleset (repeatable)")
	cfg.Flags.StringVar(&cfg.Out, 0, "out", "",
		"prompt destination (empty writes to stdout)")
	cfg.Flags.StringVar(&cfg.Template, 0, "template", "",
		"override the built-in rework template")
	cfg.Command = &ff.Command{
		Name:      "rework",
		Usage:     "canonizer rework --ruleset PATH --findings FILE [--findings FILE] [--out FILE]",
		ShortHelp: "emit the prompt an agent runs to revise a ruleset against its findings",
		LongHelp: `Fill the revision template with a candidate ruleset and every findings
document raised against it, and write the prompt to --out or stdout. An agent runs the
prompt and writes the revised ruleset back over the candidate.

--findings is repeatable: a refine round produces deterministic findings from verify and
graded findings from a cold critic, and an agent reworking against only one of them
resolves only half the round.

canonizer calls no model, so this emits the prompt and stops. The driver holding the
attempt counter runs it -- see pipeline.sh.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

// exec reads the candidate and every findings document, fills the template, and writes the
// prompt. Flags are already parsed.
func (cfg *Config) exec(_ context.Context, _ []string) error {
	if cfg.Ruleset == "" {
		return errors.New("rework: --ruleset is required")
	}
	if len(cfg.Findings) == 0 {
		return errors.New("rework: at least one --findings is required")
	}
	tmpl, err := prompt.Resolve(cfg.Template, prompt.Rework)
	if err != nil {
		return errors.WrapWithMessage(err, "rework")
	}
	candidate, err := os.ReadFile(cfg.Ruleset)
	if err != nil {
		return errors.WrapWithMessage(err, "rework: read ruleset",
			slog.String("path", cfg.Ruleset))
	}
	findings, err := cfg.readFindings()
	if err != nil {
		return err
	}
	filled := strings.Replace(tmpl, rulesetMarker, string(candidate), 1)
	filled = strings.Replace(filled, findingsMarker, findings, 1)
	return cfg.emit(filled)
}

// readFindings concatenates every --findings document, each labelled with the path it came
// from.
//
// Labelled because the two documents in a round are not interchangeable: one is a
// deterministic check and the other a grader's judgement, and an agent deciding what to
// change benefits from knowing which said what. A bare concatenation of JSON would also be
// invalid JSON, so the labels double as the reason it is legible at all.
//
// Ensures: the result names every path given; a missing file is an error rather than a
//
//	silent skip, because a round that reworked against half its findings would look
//	like one that reworked against all of them.
func (cfg *Config) readFindings() (string, error) {
	var b strings.Builder
	for i, path := range cfg.Findings {
		body, err := os.ReadFile(path)
		if err != nil {
			return "", errors.WrapWithMessage(err, "rework: read findings",
				slog.String("path", path))
		}
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(
			&b,
			"<findings from=%q>\n%s\n</findings>",
			path,
			strings.TrimSpace(string(body)),
		)
	}
	return b.String(), nil
}

// emit writes the filled prompt to --out, or stdout when it is empty.
func (cfg *Config) emit(filled string) error {
	if cfg.Out == "" {
		_, _ = fmt.Fprint(cfg.Stdout, filled)
		return nil
	}
	if err := os.WriteFile(cfg.Out, []byte(filled), 0o600); err != nil {
		return errors.WrapWithMessage(err, "rework: write prompt", slog.String("out", cfg.Out))
	}
	_, _ = fmt.Fprintf(cfg.Stderr, "wrote %s\n", cfg.Out)
	return nil
}
