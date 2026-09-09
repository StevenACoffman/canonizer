package verify

import (
	"github.com/StevenACoffman/skillet/finding"
	"github.com/StevenACoffman/skillet/ruleset"
)

// Canonical reports whether raw is byte-identical to the canonical rendering of rs.
//
// Nothing asserted this before 2026-09-05: no non-test call to ruleset.Render existed in
// this repo, so a ruleset could be *parseable* while Render(Parse(x)) != x -- reordered,
// reformatted, or quietly dropping a field Parse tolerates and Render does not reproduce.
//
// It is load-bearing rather than cosmetic. ruleset/conflict compares normalized rule text,
// so a non-canonical ruleset can make two identical rules look different, or two different
// rules look identical, before any comparison runs. The contradiction check is only as
// trustworthy as the form it reads.
//
// **There is deliberately no format-version guard, and one was written and removed.** The
// backlog entry that asked for this check sequenced it behind a version reader, reasoning
// that re-rendering an older file would report drift where the honest answer is "this is
// format N and I write M". skillet already answers that inside Render: formatOf returns the
// ruleset's own format and renderFormat emits no header below 2, so a format-1 file renders
// as format 1. Measured before removing the guard -- a header-less ruleset parses as
// format 1 and round-trips byte-identically. Guarding on rs.Format != FormatVersion made
// the check decline on every file in the corpus, since none declare a version; it was a
// vacuous check dressed as a careful one.
//
// The one case where formatOf disagrees with the file is a format-1 ruleset carrying a
// warrant, which renders as format 2. Reporting that as drift is correct: the file needs
// migrating, and saying so is the point.
//
// Blocking, like the other two structural checks: a stored ruleset that does not round-trip
// is a defect in the artifact, and the repair is to write the canonical form.
//
// Requires: rs is the result of ruleset.Parse(raw).
// Ensures:  at most one diagnostic, and none when raw round-trips; it is pure.
func Canonical(raw string, rs *ruleset.Ruleset) []finding.Diagnostic {
	if ruleset.Render(rs) == raw {
		return nil
	}
	return []finding.Diagnostic{{
		Severity: finding.SeverityError,
		Action:   finding.ActionAutomatic,
		Category: CategoryNonCanonical,
		Path:     "ruleset",
		Message: "stored form differs from the canonical rendering; " +
			"rule comparison reads normalized text and cannot be trusted on it",
	}}
}
