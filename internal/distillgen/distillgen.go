// Package distillgen walks a source tree and writes one distillation prompt per source.
//
// The filling itself is skillet's (`ruleset/distill.FillTemplate`, pure); what lives here
// is the walk, the link arithmetic, and the choice of where the ruleset a prompt asks for
// should be written. That split matches the rest of the family: skillet holds the pure
// core and the consumer does the walking, the way exegesis's indexgen walks a tree over
// skillet's skill.Discover.
//
// It replaced a call to `ruleset/distill.Generate`, whose signature could not express a
// separate rules destination -- it always pointed a prompt at a path beside its source.
// canonizer is that function's only consumer in the family, so this is a relocation rather
// than a second implementation.
package distillgen

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/StevenACoffman/skillet/naming"
	skdistill "github.com/StevenACoffman/skillet/ruleset/distill"
	errors "github.com/StevenACoffman/toerr/errors"
)

// generator accumulates the prompts written while walking a source tree.
type generator struct {
	tmpl      string
	promptDir string
	rulesDir  string // empty means "beside the source"
	written   []string
}

// Generate walks sourceRoot for Markdown sources, fills tmpl for each, and writes a
// *_prompt.md into promptOut. It returns the written prompt paths in walk order.
//
// rulesOut is where each prompt tells the agent to write the ruleset it produces. Empty
// means beside the source, which is what this did before the directory was configurable
// and remains the default so an existing invocation is unchanged.
//
// **Every link in a prompt is relative to that prompt's own directory**, which is only
// correct if the agent runs with that directory as its working directory. Nothing in a
// Markdown link says where it is anchored, so an agent invoked elsewhere resolves the
// `../` from the wrong floor. The prompts say so in their own text and the CLI help says
// so; this comment is the third place because it is the property the arithmetic here
// exists to produce.
//
// Requires: tmpl carries both placeholders; sourceRoot exists.
// Ensures:  one prompt per source file, skipping *_rules.md, *_prompt.md and hidden
//
//	directories; the template is validated once before any file is written.
func Generate(tmpl, sourceRoot, promptOut, rulesOut string) ([]string, error) {
	if _, err := skdistill.FillTemplate(tmpl, "", ""); err != nil {
		return nil, errors.Wrap(err)
	}
	absPrompt, err := filepath.Abs(promptOut)
	if err != nil {
		return nil, errors.WrapWithMessage(err, "distill: resolve out dir")
	}
	absRules := ""
	if rulesOut != "" {
		if absRules, err = filepath.Abs(rulesOut); err != nil {
			return nil, errors.WrapWithMessage(err, "distill: resolve rules dir")
		}
		if err = os.MkdirAll(absRules, 0o755); err != nil {
			return nil, errors.WrapWithMessage(err, "distill: create rules dir")
		}
	}
	if err = os.MkdirAll(absPrompt, 0o755); err != nil {
		return nil, errors.WrapWithMessage(err, "distill: create out dir")
	}
	g := &generator{tmpl: tmpl, promptDir: absPrompt, rulesDir: absRules}
	if walkErr := filepath.WalkDir(sourceRoot, g.walk); walkErr != nil {
		return nil, errors.WrapWithMessage(walkErr, "distill: walk sources")
	}
	return g.written, nil
}

// walk visits one entry, writing a prompt for each source file.
func (g *generator) walk(path string, d fs.DirEntry, err error) error {
	if err != nil {
		return errors.Wrap(err)
	}
	if d.IsDir() {
		if strings.HasPrefix(d.Name(), ".") && d.Name() != "." {
			return fs.SkipDir
		}
		return nil
	}
	if !isSource(path, d) {
		return nil
	}
	written, genErr := g.one(path)
	if genErr != nil {
		return genErr
	}
	g.written = append(g.written, written)
	return nil
}

// one fills the template for a single source and writes its prompt.
func (g *generator) one(source string) (string, error) {
	absSource, err := filepath.Abs(source)
	if err != nil {
		return "", errors.WrapWithMessage(err, "distill: resolve source")
	}
	title, err := naming.TitleFromFile(absSource)
	if err != nil {
		return "", errors.WrapWithMessage(err, "distill")
	}
	base := filepath.Base(absSource)
	rulesDir := g.rulesDir
	if rulesDir == "" {
		rulesDir = filepath.Dir(absSource)
	}
	sourceLink, err := link(g.promptDir, absSource, title)
	if err != nil {
		return "", err
	}
	destLink, err := link(g.promptDir, filepath.Join(rulesDir, naming.RulesFilename(base)),
		title+" Rules")
	if err != nil {
		return "", err
	}
	filled, err := skdistill.FillTemplate(g.tmpl, sourceLink, destLink)
	if err != nil {
		return "", errors.Wrap(err)
	}
	promptPath := filepath.Join(g.promptDir, naming.PromptFilename(base))
	if err = os.WriteFile(promptPath, []byte(filled), 0o644); err != nil {
		return "", errors.WrapWithMessage(err, "distill: write prompt")
	}
	return promptPath, nil
}

// isSource reports whether path is a Markdown source rather than a generated artifact.
func isSource(path string, d fs.DirEntry) bool {
	return !d.IsDir() &&
		strings.HasSuffix(path, ".md") &&
		!strings.HasSuffix(path, "_rules.md") &&
		!strings.HasSuffix(path, "_prompt.md")
}

// link builds a Markdown link to target relative to base, with forward slashes so the
// link reads the same on every platform.
func link(base, target, text string) (string, error) {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return "", errors.WrapWithMessage(err, "distill: relative path")
	}
	return "[" + text + "](" + filepath.ToSlash(rel) + ")", nil
}
