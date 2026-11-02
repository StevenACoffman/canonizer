package verify

import (
	"fmt"

	"github.com/StevenACoffman/skillet/finding"
	"github.com/StevenACoffman/skillet/identity"
)

// DeployedCopy is one copy of a ruleset as it was found where it is deployed: a project's
// CLAUDE.md include, an agent's rules directory, a vendored file.
type DeployedCopy struct {
	Path    string // where the copy was looked for, as the caller named it
	Content []byte // the copy's bytes; ignored when Missing
	Missing bool   // true when nothing exists at Path
}

// Deployed reports every deployed copy that is not byte-identical to the verified
// ruleset, as a blocking deployed-drift finding.
//
// Requires: verified is the ruleset this run verified, as read.
// Ensures:  one error-severity diagnostic per copy that is missing or whose identity.Hash
//
//	differs from verified's, in the given order; nothing for an identical copy; pure.
//
// **Exact identity, never similarity.** A copy edited by one word is a different
// ruleset that no one verified, and a near-duplicate threshold would pass it — the
// reason ruleset/conflict refuses one too. It is the check skillsaw's `portable` makes
// for skills, applied to a ruleset: a sign-off attests to these bytes, so a copy is
// covered by it only if it is these bytes.
func Deployed(verified []byte, copies []DeployedCopy) []finding.Diagnostic {
	want := identity.Hash(string(verified))
	var diags []finding.Diagnostic
	for _, c := range copies {
		switch got := identity.Hash(string(c.Content)); {
		case c.Missing:
			diags = append(diags, deployedDiag(c.Path,
				"deployed copy is missing; nothing exists at this path"))
		case got != want:
			diags = append(diags, deployedDiag(c.Path, fmt.Sprintf(
				"deployed copy differs from the verified ruleset (hash %s, verified %s); "+
					"recopy it, or verify it as a ruleset of its own", got, want)))
		}
	}
	return diags
}

// deployedDiag builds one deployed-drift finding. ActionGuided rather than automatic:
// the fix is a copy, but a copy edited on purpose means the deployed side may be the one
// to keep, and only a person knows which.
func deployedDiag(path, message string) finding.Diagnostic {
	return finding.Diagnostic{
		Severity: finding.SeverityError,
		Action:   finding.ActionGuided,
		Category: CategoryDeployedDrift,
		Path:     path,
		Message:  message,
	}
}
