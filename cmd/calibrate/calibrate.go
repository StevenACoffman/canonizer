// Package calibrate implements the "calibrate" command: an offline audit of the cold
// critic's confidence against how its flags actually held up on review. It reads a
// review log of {confidence, correct} samples and reports skillet/calibration's ECE,
// MCE, and Brier score — surfacing an over- or under-confident critic. It is a report,
// never a gate: the ship gate stays findings-based (see internal/budget), so calibration
// never blocks adoption.
package calibrate

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/canonizer/cmd/root"
	"github.com/StevenACoffman/skillet/calibration"
	errors "github.com/StevenACoffman/toerr/errors"
)

// samplesDoc is the review-log wire format: {"samples":[{"confidence":..,"correct":..}]}.
// calibration.Sample carries no JSON tags, but encoding/json matches "confidence" and
// "correct" case-insensitively, so it decodes directly — no wrapper type.
type samplesDoc struct {
	Samples []calibration.Sample `json:"samples"`
}

// Config holds the calibrate command's flag values and ff wiring. It embeds *root.Config
// for shared I/O.
type Config struct {
	*root.Config
	Samples string
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the calibrate command under parent.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("calibrate").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.Samples, 0, "samples", "",
		`review-log JSON: {"samples":[{"confidence":0.9,"correct":true}, ...]}`)
	cfg.Command = &ff.Command{
		Name:      "calibrate",
		Usage:     "canonizer calibrate --samples PATH",
		ShortHelp: "report the cold critic's calibration (ECE/MCE/Brier) from a review log",
		LongHelp: `Report how well the cold critic's stated confidence matches how its
flags held up on review, via skillet/calibration: Expected and Maximum Calibration Error
and the Brier score, with a per-bin breakdown. A high-confidence "unsupported"/"vague"
flag that keeps getting overturned shows up as a large gap between confidence and
accuracy.

This is an offline audit, never a gate: the ship gate stays findings-based (cold critic
+ verify -> gate -> budget), so calibration never blocks adoption. Producing the log is
the operator's process — have the critic state a confidence per finding, and on review
mark each finding correct or incorrect.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

// exec reads the review log, computes the calibration report, and prints it. Flags are
// already parsed.
func (cfg *Config) exec(_ context.Context, _ []string) error {
	if cfg.Samples == "" {
		return errors.New("calibrate: --samples is required")
	}
	data, err := os.ReadFile(cfg.Samples)
	if err != nil {
		return errors.WrapWithMessage(
			err,
			"calibrate: read samples",
			slog.String("path", cfg.Samples),
		)
	}
	var doc samplesDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return errors.WrapWithMessage(err, "calibrate: parse samples")
	}
	cfg.report(calibration.Compute(doc.Samples))
	return nil
}

// report prints rep to stdout. A zero-sample report is stated plainly rather than as
// zeroed metrics: ECE 0.000 over no samples reads as "perfectly calibrated", the opposite
// of the truth.
func (cfg *Config) report(rep calibration.Report) {
	if rep.Samples == 0 {
		_, _ = fmt.Fprintln(cfg.Stdout, "calibrate: no in-range samples scored (nothing to report)")
		return
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "calibrate: %d samples scored\n", rep.Samples)
	_, _ = fmt.Fprintf(
		cfg.Stdout,
		"  ECE   %.3f   (expected calibration error; lower is better)\n",
		rep.ECE,
	)
	_, _ = fmt.Fprintf(cfg.Stdout, "  MCE   %.3f   (worst bin)\n", rep.MCE)
	_, _ = fmt.Fprintf(cfg.Stdout, "  Brier %.3f\n", rep.Brier)
	_, _ = fmt.Fprintln(cfg.Stdout, "  bins (low -> high confidence):")
	for i := range rep.Buckets {
		b := &rep.Buckets[i]
		_, _ = fmt.Fprintf(
			cfg.Stdout,
			"    conf %.2f  acc %.2f  n %d\n",
			b.Confidence,
			b.Accuracy,
			b.Count,
		)
	}
}
