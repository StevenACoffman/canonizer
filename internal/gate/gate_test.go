package gate_test

import (
	"testing"

	"github.com/StevenACoffman/canonizer/internal/gate"
	"github.com/StevenACoffman/skillet/finding"
)

func TestBlockingReturnsSortedErrorsOnly(t *testing.T) {
	t.Parallel()
	r := finding.Result{Diagnostics: []finding.Diagnostic{
		{Severity: finding.SeverityWarning, Category: "note", Message: "w"},
		{Severity: finding.SeverityError, Category: "unsupported", Message: "e1"},
		{Severity: finding.SeverityError, Category: "duplicate", Message: "e2"},
	}}
	blk := gate.Blocking(r)
	if len(blk) != 2 {
		t.Fatalf("got %d blocking findings, want 2 (warnings excluded)", len(blk))
	}
	// finding.Sort orders by category, so "duplicate" precedes "unsupported".
	if blk[0].Category != "duplicate" || blk[1].Category != "unsupported" {
		t.Errorf("blocking not sorted by category: %+v", blk)
	}
}

func TestBlockingEmptyWhenNoErrors(t *testing.T) {
	t.Parallel()
	r := finding.Result{Diagnostics: []finding.Diagnostic{
		{Severity: finding.SeverityWarning, Category: "note", Message: "w"},
	}}
	if blk := gate.Blocking(r); len(blk) != 0 {
		t.Errorf("clean result has %d blocking findings, want 0", len(blk))
	}
}

func TestSelfTestPassesWithAWorkingGate(t *testing.T) {
	t.Parallel()
	if err := gate.SelfTest(); err != nil {
		t.Errorf("SelfTest failed on a working gate: %v", err)
	}
}
