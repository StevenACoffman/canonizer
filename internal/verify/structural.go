package verify

import (
	"github.com/StevenACoffman/skillet/finding"
	"github.com/StevenACoffman/skillet/ruleset"
)

// Structural runs every check that reads only the ruleset, never a source: executability,
// softening, unquantified terms, conflicts, canonical form, limitations and soundness.
//
// **It exists so verify and loop cannot disagree about a ruleset.** loop used to call
// Executable and Provenance on its own and nothing else, so it never ran Canonical or
// Soundness. On 2026-10-31 it returned ship for a ruleset whose verify findings, written
// the same round, carried a blocking non-canonical error -- and pipeline.sh's final gate
// then refused what loop had just adopted. One list, called by both, is what keeps a check
// added to verify from being silently missing from loop.
//
// Provenance is left to the caller because the two differ there on purpose: verify can
// split an absent anchor by a proof packet (Drift) or report it unverifiable when no
// source was given, and loop requires a source and has no packet.
func Structural(raw string, rs *ruleset.Ruleset) ([]finding.Diagnostic, error) {
	diags, err := Executable(rs)
	if err != nil {
		return nil, err
	}
	diags = append(diags, Softening(rs)...)
	diags = append(diags, Unquantified(rs)...)
	diags = append(diags, Conflicts(rs)...)
	diags = append(diags, Canonical(raw, rs)...)
	diags = append(diags, Limitations(rs)...)
	diags = append(diags, Soundness(rs)...)
	return diags, nil
}
