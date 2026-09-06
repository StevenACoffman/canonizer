package verify

import (
	"strings"

	"github.com/StevenACoffman/skillet/finding"
	"github.com/StevenACoffman/skillet/ruleset"
	"github.com/StevenACoffman/skillet/textnorm"
)

// Limitations reports whether rs says what it does not cover.
//
// VAC makes a bundle without explicit non-claims invalid, on the grounds that "a capability
// statement that will not say what it does not cover is an advertisement". A distilled
// ruleset has exactly that failure mode: rules drawn from one book, presented without the
// bounds of that book, read as rules for the whole subject.
//
// **This checks the Scope line rather than a Limitations header, and that is a limitation
// of its own.** skillet's parser reads only Source: and Scope: and Render writes only those
// two -- measured: a Limitations: header parses without error, is dropped on render, and the
// file then fails the blocking canonical-form check. So the header VAC actually wants needs
// a skillet marker and a FormatVersion bump; until then the question "does this artifact
// state its non-claims" is asked of the field that exists.
//
// Advisory, never blocking, for two independent reasons: the detection is a word list, and
// every ruleset in the corpus would fail on the day this shipped. A gate that turns a whole
// corpus red teaches people to bypass the gate.
//
// Ensures: at most one diagnostic, always finding.SeverityWarning; it is pure.
func Limitations(rs ruleset.Ruleset) []finding.Diagnostic {
	scope := strings.TrimSpace(rs.Scope)
	if scope == "" {
		return []finding.Diagnostic{scopeDiag(
			"ruleset states no scope, so nothing bounds what these rules claim to cover")}
	}
	if statesAnExclusion(scope) {
		return nil
	}
	return []finding.Diagnostic{scopeDiag(
		"scope says what is covered but not what is excluded; " +
			"a reader cannot tell where these rules stop applying")}
}

// exclusionWords are the ways a Scope line says what it leaves out.
//
// A function rather than a package-level map, which is the shape this family already uses
// for a vocabulary (related.Kinds, mergestatus.States): one definition and no mutable
// global.
//
// Crude on purpose, and the crudeness is why this check can only ever be advisory. A scope
// can bound itself without any of these words -- "Go HTTP handlers, 2024 stdlib" is a
// limitation stated as a boundary rather than as a negation -- so a ruleset flagged here is
// often fine. Blocking on a word list would be enforcing a writing style and calling it
// rigor.
func exclusionWords() map[string]bool {
	return map[string]bool{
		"not": true, "no": true, "never": true, "excludes": true, "excluding": true,
		"outside": true, "beyond": true, "only": true, "except": true, "without": true,
		"limited": true, "unlike": true,
	}
}

// scopeDiag builds the advisory located at the ruleset rather than at a rule, because this
// is a property of the artifact and no single rule is at fault.
func scopeDiag(message string) finding.Diagnostic {
	return finding.Diagnostic{
		Severity: finding.SeverityWarning,
		Action:   finding.ActionHuman,
		Category: CategoryUnbounded,
		Path:     "ruleset",
		Message:  message,
	}
}

// statesAnExclusion reports whether scope names something it leaves out.
//
// Matching whole words after folding, so "notation" does not read as "not" and a scope
// written with different casing or punctuation still matches.
func statesAnExclusion(scope string) bool {
	words := exclusionWords()
	for _, word := range strings.FieldsFunc(textnorm.Fold(scope), notLetter) {
		if words[strings.ToLower(word)] {
			return true
		}
	}
	return false
}

// notLetter reports whether r separates words for exclusion matching.
func notLetter(r rune) bool {
	return ('a' > r || r > 'z') && ('A' > r || r > 'Z')
}
