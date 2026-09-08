// Package fmtcmd implements the "fmt" command: it rewrites a ruleset into canonical form,
// or with --check reports that it is not canonical without writing.
//
// It exists because `verify` reports a non-canonical ruleset as a blocking finding and
// nothing offered the remedy. `ruleset.Render` has always produced the answer; a caller was
// told their document was wrong and left without the one command that would fix it. Five of
// the eight stored rulesets are in that state, so the gap was the common case.
//
// **Re-rendering is this command's purpose, which is what makes it safe here.** `--sign-off`
// refuses a non-canonical ruleset precisely so that attesting to one cannot reformat it as a
// side effect; asking for a reformat is a different request, and the two-step -- fmt, then
// sign -- is the workflow rather than the friction.
package fmtcmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/canonizer/cmd/root"
	"github.com/StevenACoffman/skillet/atomicfile"
	"github.com/StevenACoffman/skillet/ruleset"
	errors "github.com/StevenACoffman/toerr/errors"
)

// Config holds the fmt command's flag values and ff wiring. It embeds *root.Config for
// shared I/O.
type Config struct {
	*root.Config
	Ruleset string
	Check   bool
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the fmt command under parent.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("fmt").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.Ruleset, 0, "ruleset", "",
		"canonical *_rules.md to rewrite")
	cfg.Flags.BoolVar(&cfg.Check, 0, "check",
		"report whether the ruleset is canonical without writing (exit 1 if it is not)")
	cfg.Command = &ff.Command{
		Name:      "fmt",
		Usage:     "canonizer fmt --ruleset PATH [--check]",
		ShortHelp: "rewrite a ruleset into canonical form",
		LongHelp: `Rewrite the ruleset at --ruleset into the exact form ruleset.Render emits,
which is what verify's "non-canonical" finding measures against. An already-canonical
file is left alone, so running this across a corpus does not touch what it need not.

With --check, report whether the file is canonical and write nothing, exiting 1 if it
is not. That is the same split exegesis normalize uses.

What this changes, measured across the eight stored rulesets: no text is lost — the
word sequence is identical before and after — and what moves is line wrapping and
spacing. A rule header written with three spaces after its level tag becomes two, and a
rationale a human wrapped across three lines becomes one long line. Stored rulesets
already carry lines of 265-379 characters, so that is the existing shape of the format
rather than a new one, but a hand-wrapped document will not come back wrapped.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

// exec parses the ruleset, compares its stored bytes with its canonical rendering, and
// either reports the difference (--check) or writes the rendering. Flags are already parsed.
//
// **Writing is atomic**, for the reason --sign-off's write is: this replaces a document a
// human owns and cannot regenerate, so an interrupted write must not be able to truncate it.
func (cfg *Config) exec(_ context.Context, _ []string) error {
	if cfg.Ruleset == "" {
		return errors.New("fmt: --ruleset is required")
	}
	raw, err := os.ReadFile(cfg.Ruleset)
	if err != nil {
		return errors.WrapWithMessage(err, "fmt: read ruleset", slog.String("path", cfg.Ruleset))
	}
	rs, err := ruleset.Parse(string(raw))
	if err != nil {
		return errors.WrapWithMessage(err, "fmt: parse ruleset")
	}
	rendered := ruleset.Render(&rs)
	if rendered == string(raw) {
		_, _ = fmt.Fprintf(cfg.Stdout, "%s: already canonical\n", cfg.Ruleset)
		return nil
	}
	if cfg.Check {
		_, _ = fmt.Fprintf(cfg.Stdout, "%s: not canonical\n", cfg.Ruleset)
		return root.ExitError(1)
	}
	if writeErr := atomicfile.WriteFile(cfg.Ruleset, []byte(rendered), 0o600); writeErr != nil {
		return errors.WrapWithMessage(writeErr, "fmt: write ruleset",
			slog.String("path", cfg.Ruleset))
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "%s: rewritten\n", cfg.Ruleset)
	return nil
}
