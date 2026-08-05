// Package verify implements the "verify" command: it parses a canonical ruleset and
// runs canonizer's deterministic checks over it — executability (B) and, given a
// source, provenance (E) — emitting skillet/finding JSON for the "gate" command to
// block on. With --proof it also writes a proof packet binding the ruleset to the
// source bytes. Every path is a flag.
package verify

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/canonizer/cmd/root"
	vfy "github.com/StevenACoffman/canonizer/internal/verify"
	"github.com/StevenACoffman/skillet/finding"
	"github.com/StevenACoffman/skillet/proof"
	"github.com/StevenACoffman/skillet/ruleset"
	errors "github.com/StevenACoffman/toerr/errors"
)

// Config holds the verify command's flag values and ff wiring. It embeds
// *root.Config for shared I/O.
type Config struct {
	*root.Config
	Ruleset string
	Source  string
	Proof   string
	Out     string
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the verify command under parent.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("verify").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.Ruleset, 0, "ruleset", "",
		"canonical *_rules.md to verify")
	cfg.Flags.StringVar(&cfg.Source, 0, "source", "",
		"source document; enables the provenance (anchor) checks")
	cfg.Flags.StringVar(&cfg.Proof, 0, "proof", "",
		"write a proof packet binding the ruleset and source bytes to this path")
	cfg.Flags.StringVar(&cfg.Out, 0, "out", "",
		"findings JSON destination (empty writes to stdout)")
	cfg.Command = &ff.Command{
		Name:      "verify",
		Usage:     "canonizer verify --ruleset PATH [--source PATH] [--proof PATH] [--out FILE]",
		ShortHelp: "check a ruleset's executability and provenance, emitting findings",
		LongHelp: `Parse a canonical ruleset and run canonizer's deterministic checks:
executability (every enforced rule carries a discriminating ✗/✓ pair) and — with
--source — provenance (every enforced rule cites a source anchor present in the
source). The result is a skillet/finding JSON document; pipe it to "gate" to block
on it. With --proof, also write a proof packet binding the ruleset to the source.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

// exec parses the ruleset, runs the deterministic checks, emits the findings, and —
// when --proof is set — writes the binding packet. Flags are already parsed.
func (cfg *Config) exec(_ context.Context, _ []string) error {
	if cfg.Ruleset == "" {
		return errors.New("verify: --ruleset is required")
	}
	raw, err := os.ReadFile(cfg.Ruleset)
	if err != nil {
		return errors.WrapWithMessage(err, "verify: read ruleset", slog.String("path", cfg.Ruleset))
	}
	rs, err := ruleset.Parse(string(raw))
	if err != nil {
		return errors.WrapWithMessage(err, "verify: parse ruleset")
	}
	diags, err := vfy.Executable(rs)
	if err != nil {
		return errors.Wrap(err) // vfy already prefixes "verify:"
	}
	if cfg.Source != "" {
		source, readErr := os.ReadFile(cfg.Source)
		if readErr != nil {
			return errors.WrapWithMessage(
				readErr,
				"verify: read source",
				slog.String("path", cfg.Source),
			)
		}
		diags = append(diags, vfy.Provenance(rs, string(source))...)
	}
	finding.Sort(diags)
	if err := cfg.emit(finding.Result{Diagnostics: diags}); err != nil {
		return err
	}
	if cfg.Proof != "" {
		return cfg.writeProof()
	}
	return nil
}

// emit writes the findings as JSON to --out, or stdout when it is empty.
func (cfg *Config) emit(result finding.Result) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return errors.WrapWithMessage(err, "verify: marshal findings")
	}
	data = append(data, '\n')
	if cfg.Out == "" {
		_, _ = cfg.Stdout.Write(data)
		return nil
	}
	if writeErr := os.WriteFile(cfg.Out, data, 0o600); writeErr != nil {
		return errors.WrapWithMessage(
			writeErr,
			"verify: write findings",
			slog.String("out", cfg.Out),
		)
	}
	return nil
}

// writeProof binds the ruleset (and source, when given) to their exact bytes in a
// proof packet. root "" uses the paths as given, so absolute or cwd-relative both work.
func (cfg *Config) writeProof() error {
	paths := []string{cfg.Ruleset}
	if cfg.Source != "" {
		paths = append(paths, cfg.Source)
	}
	packet, err := proof.Create("", "ruleset-provenance", "", paths)
	if err != nil {
		return errors.WrapWithMessage(err, "verify: create proof")
	}
	if err := proof.Save(cfg.Proof, &packet); err != nil {
		return errors.WrapWithMessage(err, "verify: save proof", slog.String("proof", cfg.Proof))
	}
	_, _ = fmt.Fprintf(cfg.Stderr, "verify: wrote proof %s\n", cfg.Proof)
	return nil
}
