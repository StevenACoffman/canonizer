// Package gate is canonizer's deterministic findings gate: it decides which
// diagnostics from a cold-critic run block a ruleset from shipping, and proves —
// before a real run is trusted — that the gate can actually fail. Both functions are
// pure: they read no I/O and no clock.
package gate

import (
	"github.com/StevenACoffman/skillet/finding"
	errors "github.com/StevenACoffman/toerr/errors"
)

// Blocking returns the diagnostics in r that fail the gate — the error-severity
// findings — sorted deterministically. A ruleset ships only when this is empty. It
// returns the offending diagnostics (not just a bool like finding.HasBlocking) so a
// report can name exactly what blocked.
func Blocking(r finding.Result) []finding.Diagnostic {
	blocking := make([]finding.Diagnostic, 0, len(r.Diagnostics))
	for _, d := range r.Diagnostics {
		if d.Severity.Blocking() {
			blocking = append(blocking, d)
		}
	}
	finding.Sort(blocking)
	return blocking
}

// SelfTest is the planted-defect negative control (adh's oracle self-test analog):
// it feeds a planted blocking finding and a clean one through Blocking and returns an
// error if the gate does not discriminate — proof the gate has teeth before it is
// trusted on a real critic run.
func SelfTest() error {
	planted := finding.Result{Diagnostics: []finding.Diagnostic{
		{Severity: finding.SeverityError, Category: "unsupported", Message: "planted defect"},
	}}
	if len(Blocking(planted)) == 0 {
		return errors.New("gate: self-test failed — a blocking finding did not block")
	}
	clean := finding.Result{Diagnostics: []finding.Diagnostic{
		{Severity: finding.SeverityWarning, Category: "note", Message: "planted note"},
	}}
	if len(Blocking(clean)) != 0 {
		return errors.New("gate: self-test failed — a non-blocking finding blocked")
	}
	return nil
}
