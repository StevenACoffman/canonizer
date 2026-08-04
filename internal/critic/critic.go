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
)

// FillPrompt returns tmpl with its {{SOURCE}} and {{RULESET}} markers replaced by
// the source document and the candidate ruleset. It fails loudly when either marker
// is absent — a critic prompt missing one would ask the grader to judge against
// nothing — mirroring distill's placeholder validation.
func FillPrompt(tmpl, source, ruleset string) (string, error) {
	if !strings.Contains(tmpl, SourceMarker) {
		return "", errors.New("critic: template missing " + SourceMarker)
	}
	if !strings.Contains(tmpl, RulesetMarker) {
		return "", errors.New("critic: template missing " + RulesetMarker)
	}
	out := strings.ReplaceAll(tmpl, SourceMarker, source)
	return strings.ReplaceAll(out, RulesetMarker, ruleset), nil
}
