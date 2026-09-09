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
	"path/filepath"
	"strings"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/canonizer/cmd/root"
	"github.com/StevenACoffman/canonizer/internal/prompt"
	"github.com/StevenACoffman/skillet/naming"
	sksynth "github.com/StevenACoffman/skillet/ruleset/synthesize"
	errors "github.com/StevenACoffman/toerr/errors"
)

// destinationMarker is the placeholder the synthesis template carries for where the merged
// ruleset should be written.
//
// Filled here rather than by skillet's FillTemplate, which knows only {{RULESETS}} and
// replaces that one marker. Verified before relying on it: that function requires
// {{RULESETS}} to be present and ignores every other placeholder, so adding this one needs
// no kernel change and no release.
const destinationMarker = "{{DESTINATION_CONTENT}}"

// Config holds the synthesize command's flag values and ff wiring. It embeds
// *root.Config for shared I/O.
type Config struct {
	*root.Config
	Template string
	Rulesets string
	RulesOut string
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
	cfg.Flags.StringVar(&cfg.RulesOut, 0, "rulesout", "",
		"directory the prompt tells the agent to write the merged ruleset into")
	cfg.Flags.StringVar(&cfg.Out, 0, "out", "",
		"file to write the assembled prompt into (empty writes to stdout)")
	cfg.Command = &ff.Command{
		Name: "synthesize",
		Usage: "canonizer synthesize --rulesets DIR [--rulesout DIR] [--out FILE] " +
			"[--template PATH]",
		ShortHelp: "assemble one synthesis prompt from distilled rulesets",
		LongHelp: `Read every *_rules.md in --rulesets and assemble them into a single
prompt that asks a model to merge them into one unified ruleset.

--rulesout is where the prompt tells the agent to write the merged ruleset, exactly as
distill's --rulesout does for each per-source ruleset. Without it the prompt names no
destination and an agent prints the ruleset instead of writing one.

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
	filled = strings.Replace(filled, destinationMarker, cfg.destination(), 1)
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

// destination is the instruction the prompt carries about where its output goes.
//
// **The synthesis prompt named no destination until now, and distill's always has.**
// `distill --rulesout` writes the target into every prompt it generates, which is how each
// distill agent knows the file to create. Without the same thing here an agent given the
// synthesis prompt has nowhere to put the merged ruleset and prints it instead -- the
// failure that produced eight files of plan prose the first time this pipeline ran.
//
// The filename comes from naming.RulesFilename rather than a literal, because distillgen
// already derives ruleset filenames that way and a second spelling of "what a ruleset file
// is called" is how the two paths start disagreeing.
//
// **An empty --rulesout states today's behaviour rather than leaving a hole.** Replacing the
// marker with nothing would leave a heading over an empty section, and leaving the marker
// would put a raw placeholder in front of an agent.
//
// Ensures: never empty; it is pure.
func (cfg *Config) destination() string {
	if cfg.RulesOut == "" {
		return "No destination was given. Print the merged ruleset as your reply, and say " +
			"in one line that no output path was supplied."
	}
	// The ".md" is supplied because RulesFilename preserves an extension rather than adding
	// one: distillgen feeds it a source filename, and a bare directory name would come back
	// as "benbjohnson_rules" with no suffix at all.
	base := filepath.Base(cfg.Rulesets) + ".md"
	path := filepath.Join(cfg.RulesOut, naming.RulesFilename(base))
	return "Write the merged ruleset to this exact path, creating it if it does not exist:\n\n" +
		"<destination>" + path + "</destination>\n\n" +
		"Write the file. Do not print the ruleset as your reply -- the reply is a report " +
		"about the work and the file is the artifact."
}
