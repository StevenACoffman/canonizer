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
	"strings"

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
	Source      []string
	Ruleset     string
	FindingsOut string
	Template    string
	Out         string
	Flags       *ff.FlagSet
	Command     *ff.Command
}

// New creates and registers the critic command under parent.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("critic").SetParent(parent.Flags)
	// Repeatable so a synthesized candidate can be graded against every source it was
	// merged from. The grader sees them in the order given, each in its own block.
	cfg.Flags.StringListVar(&cfg.Source, 0, "source",
		"source document the grader reads (repeatable)")
	// Names a file where distill's and synthesize's --rulesout name a directory, and the
	// asymmetry has a reason. Those derive each artifact's name from its source; a critic
	// run's findings belong to one candidate at one *attempt*, and the attempt number is the
	// driver's knowledge -- canonizer holds no counter -- so the caller supplies the name.
	cfg.Flags.StringVar(&cfg.FindingsOut, 0, "findingsout", "",
		"file the prompt tells the grader to write its findings JSON to")
	cfg.Flags.StringVar(&cfg.Ruleset, 0, "ruleset", "",
		"candidate *_rules.md to critique")
	cfg.Flags.StringVar(&cfg.Template, 0, "template", "",
		"path to a critic prompt template (empty uses the built-in default)")
	cfg.Flags.StringVar(&cfg.Out, 0, "out", "",
		"file to write the filled prompt into (empty writes to stdout)")
	cfg.Command = &ff.Command{
		Name: "critic",
		Usage: "canonizer critic --source PATH... --ruleset PATH [--findingsout FILE] " +
			"[--out FILE] [--template PATH]",
		ShortHelp: "emit a cold-critic prompt for a candidate ruleset",
		LongHelp: `Fill the cold-critic prompt with a source document and a candidate
ruleset and write it to --out or stdout. A fresh agent runs the prompt and returns
JSON findings (skillet/finding shape); the "gate" command then blocks the ruleset
while any finding is blocking.

--source is repeatable, so a synthesized ruleset is graded against every document it
was merged from. --findingsout is the file the prompt tells the grader to write its
findings to; without it the prompt names no destination and a grader prints its JSON
into a transcript instead of leaving an artifact.

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
	if len(cfg.Source) == 0 {
		return errors.New("critic: --source is required")
	}
	if cfg.Ruleset == "" {
		return errors.New("critic: --ruleset is required")
	}
	tmpl, err := prompt.Resolve(cfg.Template, prompt.Critic)
	if err != nil {
		return errors.WrapWithMessage(err, "critic")
	}
	source, err := cfg.readSources()
	if err != nil {
		return err
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
	filled, err := crit.FillPrompt(tmpl, source, string(candidate), cfg.destination())
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

// readSources concatenates every --source into the one block the template carries, each
// labelled with the path it came from.
//
// **The grader sees the sources joined where verify iterates them, and the asymmetry is
// deliberate.** verify asks a mechanical question -- is this text present -- where a seam
// between two documents could manufacture a match. A grader is asked whether the ruleset is
// supported by what it reads, and a labelled boundary is something it can see and reason
// about rather than a substring hazard.
//
// Ensures: every path given is named in the result; a missing file is an error rather than
//
//	a silent skip, because a grader given fewer sources than the ruleset was built from
//	would report unsupported rules that are in fact supported.
func (cfg *Config) readSources() (string, error) {
	var b strings.Builder
	for i, path := range cfg.Source {
		body, err := os.ReadFile(path)
		if err != nil {
			return "", errors.WrapWithMessage(err, "critic: read source",
				slog.String("path", path))
		}
		if i > 0 {
			b.WriteString("\n\n")
		}
		if len(cfg.Source) > 1 {
			fmt.Fprintf(
				&b,
				"<document path=%q>\n%s\n</document>",
				path,
				strings.TrimSpace(string(body)),
			)
			continue
		}
		b.WriteString(string(body))
	}
	return b.String(), nil
}

// destination is the instruction the prompt carries about where the grader's findings go.
//
// **An empty --findingsout states today's behaviour rather than leaving a hole**, matching
// what synthesize does for an absent --rulesout: replacing the marker with nothing leaves a
// heading over an empty section, and leaving the marker puts a raw placeholder in front of
// an agent.
//
// Ensures: never empty; it is pure.
func (cfg *Config) destination() string {
	if cfg.FindingsOut == "" {
		return "No destination was given. Print your findings JSON as your reply, and say " +
			"in one line that no output path was supplied."
	}
	return "Write your findings JSON to this exact path:\n\n" +
		"<destination>" + cfg.FindingsOut + "</destination>\n\n" +
		"Write the file. Do not print the JSON as your reply."
}
