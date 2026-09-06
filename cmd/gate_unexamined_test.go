package cmd_test

import (
	"path/filepath"
	"strings"
	"testing"
)

// writeFindings writes a findings document and returns its path.
func writeFindings(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "findings.json")
	writeFile(t, path, body)
	return path
}

// TestGateRendersTheCoverageRecord checks the record reaches a reader at all: it is
// advisory, so nothing else in the run would reveal that it was dropped.
func TestGateRendersTheCoverageRecord(t *testing.T) {
	t.Parallel()
	path := writeFindings(t, `{"diagnostics":[],"unexamined":[
		{"aspect":"the source's worked examples","reason":"judged the prose sections only"}]}`)

	stdout, _, err := runIO(t, "gate", "--findings", path)
	if err != nil {
		t.Fatalf("gate: %v\n%s", err, stdout)
	}
	if !strings.Contains(stdout, "did not examine") {
		t.Errorf("want the coverage record rendered, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "the source's worked examples — judged the prose sections only") {
		t.Errorf("want the aspect and its reason, got:\n%s", stdout)
	}
}

// TestGateDoesNotBlockOnADeclaredGap is the guarantee the whole mechanism rests on. A
// critic that can be penalised for naming a gap learns to name none, which costs the gap
// and the finding it would have arrived with.
func TestGateDoesNotBlockOnADeclaredGap(t *testing.T) {
	t.Parallel()
	path := writeFindings(t, `{"diagnostics":[],"unexamined":[
		{"aspect":"§7","reason":"ran the rules against §3 only"},
		{"aspect":"§9","reason":"no worked example to compare against"}]}`)

	stdout, _, err := runIO(t, "gate", "--findings", path)
	if err != nil {
		t.Fatalf("declaring gaps must not block: %v\n%s", err, stdout)
	}
	if !strings.Contains(stdout, "0 blocking") {
		t.Errorf("want a clean verdict alongside the record, got:\n%s", stdout)
	}
}

// TestGateCountsOnlyDiagnosticsAsFindings keeps the two lists apart in the arithmetic as
// well as in the output: a coverage entry is not a finding.
func TestGateCountsOnlyDiagnosticsAsFindings(t *testing.T) {
	t.Parallel()
	path := writeFindings(t, `{"diagnostics":[],"unexamined":[
		{"aspect":"§7","reason":"ran the rules against §3 only"}]}`)

	stdout, _, err := runIO(t, "gate", "--findings", path)
	if err != nil {
		t.Fatalf("gate: %v", err)
	}
	if !strings.Contains(stdout, "clean (0 finding(s), 0 blocking)") {
		t.Errorf("a coverage entry must not be counted as a finding, got:\n%s", stdout)
	}
}

// TestGateStillBlocksWhenAGapIsDeclared pins the other direction: naming a gap must not
// buy absolution for a real defect reported in the same reply.
func TestGateStillBlocksWhenAGapIsDeclared(t *testing.T) {
	t.Parallel()
	path := writeFindings(t, `{"diagnostics":[{"severity":"error","category":"unsupported",
		"path":"§1.1","message":"the source never claims this"}],
		"unexamined":[{"aspect":"§7","reason":"ran the rules against §3 only"}]}`)

	stdout, _, err := runIO(t, "gate", "--findings", path)
	if err == nil {
		t.Fatalf("a blocking finding must still block, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "did not examine") {
		t.Errorf("the record should still be shown on a blocked run, got:\n%s", stdout)
	}
}

// TestGateRejectsAReplyWithAMalformedGap covers the reject-the-whole-reply rule at the
// command boundary: silently dropping the entry is how a reply that says nothing passes
// for one that found nothing.
func TestGateRejectsAReplyWithAMalformedGap(t *testing.T) {
	t.Parallel()
	path := writeFindings(t, `{"diagnostics":[],"unexamined":[{"aspect":"§7"}]}`)

	if _, err := run(t, "gate", "--findings", path); err == nil {
		t.Error("gate accepted a reply whose coverage record states no reason")
	}
}

// TestGateSaysNothingWhenNoGapWasDeclared keeps silence meaning silence: announcing "no
// gaps declared" would read as a positive fact about coverage, and it is the absence of one.
func TestGateSaysNothingWhenNoGapWasDeclared(t *testing.T) {
	t.Parallel()
	path := writeFindings(t, `{"diagnostics":[]}`)

	stdout, _, err := runIO(t, "gate", "--findings", path)
	if err != nil {
		t.Fatalf("gate: %v", err)
	}
	if strings.Contains(stdout, "did not examine") {
		t.Errorf("want no coverage section when none was declared, got:\n%s", stdout)
	}
}
