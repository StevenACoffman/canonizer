// Package critic fills the cold-critic prompt: given a source document and a
// candidate ruleset, it produces the prompt a fresh grader runs to flag rules that
// should not ship. FillPrompt is pure; the command shell reads the files and writes
// the result.
package critic

import (
	"strings"

	errors "github.com/StevenACoffman/toerr/errors"
)

// Markers the cold-critic template must contain; FillPrompt replaces them with the
// source document and the candidate ruleset.
const (
	SourceMarker  = "{{SOURCE}}"
	RulesetMarker = "{{RULESET}}"
	// DestinationMarker is where the grader is told to write its findings.
	//
	// The third of its kind: distill and synthesize each carry a --rulesout destination so
	// the agent knows the file to create, and critic did not, so the driver appended one
	// with a heredoc. Validated here like the two markers above rather than filled by the
	// command, because this package owns what the critic template must contain and a second
	// place deciding that is how the two start disagreeing.
	DestinationMarker = "{{DESTINATION_CONTENT}}"
)

// FillPrompt returns tmpl with its {{SOURCE}}, {{RULESET}} and {{DESTINATION_CONTENT}}
// markers replaced by the source document, the candidate ruleset, and the instruction
// saying where the grader writes its findings.
//
// It fails loudly when any marker is absent — a critic prompt missing the first two would
// ask the grader to judge against nothing, and one missing the third leaves it nowhere to
// put the answer — mirroring distill's placeholder validation.
//
// **A custom --template predating the destination marker now fails, and that is the
// intended break.** The other two markers already imposed this contract, so a template
// without the third is as incomplete as one without {{RULESET}}; prompt.Resolve made the
// same choice for an unreadable override path.
func FillPrompt(tmpl, source, ruleset, destination string) (string, error) {
	for _, marker := range []string{SourceMarker, RulesetMarker, DestinationMarker} {
		if !strings.Contains(tmpl, marker) {
			return "", errors.New("critic: template missing " + marker)
		}
	}
	out := strings.ReplaceAll(tmpl, SourceMarker, source)
	out = strings.ReplaceAll(out, RulesetMarker, ruleset)
	return strings.ReplaceAll(out, DestinationMarker, destination), nil
}
