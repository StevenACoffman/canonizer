// Package prompt is the source of canonizer's prompt templates: it embeds the
// default distill and synthesis templates and resolves a caller-supplied path
// against them, so no template lives at a hardcoded filesystem location. A caller
// asks for a template by passing an override path (empty means "use the default")
// and never touches the embed machinery or the file read itself.
package prompt

import (
	_ "embed"
	"fmt"
	"os"
)

// Distill is the default per-source distillation template. Its
// {{SOURCE_CONTENT}} and {{DESTINATION_CONTENT}} placeholders are the ones
// skillet/ruleset/distill validates and fills.
//
//go:embed distill_source_prompt.md
var Distill string

// Synthesize is the default multi-ruleset synthesis template. Its single
// {{RULESETS}} marker is filled by skillet/ruleset/synthesize with one block per
// input.
//
//go:embed synthesize_rulesets_prompt.md
var Synthesize string

// Critic is the default cold-critic template. Its {{SOURCE}} and {{RULESET}}
// markers are filled by internal/critic with the source document and the
// candidate ruleset a fresh grader critiques.
//
//go:embed critic_prompt.md
var Critic string

// Resolve returns the template to use: the contents of the file at path when path
// is non-empty, otherwise the compiled-in fallback. It is the single home for the
// "override by path, else use the default" rule, so no command repeats it. A
// non-empty path that cannot be read is an error rather than a silent fall back to
// the default, which would hide the operator's mistake.
func Resolve(path, fallback string) (string, error) {
	if path == "" {
		return fallback, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read template %s: %w", path, err)
	}
	return string(b), nil
}
