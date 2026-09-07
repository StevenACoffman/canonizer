package verify

import (
	"regexp"

	"github.com/StevenACoffman/skillet/markdown"
)

// identifierPatterns are the shapes a Go identifier takes in prose that markdown alone
// cannot see: a qualified name, an import path, a call, a pointer type.
//
// A function rather than package-level vars, the shape this family uses for a vocabulary
// (related.Kinds, mergestatus.States) and the one gochecknoglobals leaves alone. The
// patterns are compiled per call, which is affordable because they run only on statements
// that already failed the cheaper code-span test.
//
// **Measured against the 48 statements of the first real distillation**, with code spans
// stripped so each pattern is judged on bare prose: `pkg.Name` matched 7, `Foo()` 5, `*T` 4,
// `pkg/name` 2. A `snake_case` pattern was written and dropped — it matched **0**, and a
// pattern that fires on nothing is a claim nobody has checked.
func identifierPatterns() []*regexp.Regexp {
	return []*regexp.Regexp{
		regexp.MustCompile(`\b[a-z][a-z0-9_]*\.[A-Z][A-Za-z0-9_]*`), // sql.DB
		regexp.MustCompile(`\b[a-z][a-z0-9_]*/[a-z][a-z0-9_/]*`),    // database/sql
		regexp.MustCompile(`\b[A-Za-z_][A-Za-z0-9_]*\(\)`),          // Open()
		regexp.MustCompile(`\*[A-Za-z_][A-Za-z0-9_.]*`),             // *sql.DB
	}
}

// concrete reports whether a statement names something a reader could act on.
//
// The code-span test comes first and answers most statements: a rule that backticks its
// symbols is concrete by the cheapest check there is, and the patterns below never run.
//
// **The second test exists because the first measures typography.** markdown's Links
// carries code-span contents, so a statement naming `database/sql`, `*sql.DB` and `Open()`
// in plain prose has no Links at all and read as naming nothing. Measured on the first real
// distillation: one ruleset backticked nothing and every one of its 26 enforced rules was
// flagged; another backticked most of its symbols and 4 were. The check was tracking the
// writer's punctuation, not the rule's content.
//
// **Widened here rather than in skillet.** markdown.Links is the kernel's, and its doc
// records that mixing link targets with code spans is deliberate *because four consumers
// read it*; widening it there changes a signal three other tools depend on. What is widened
// here is canonizer's question, not skillet's datum.
//
// **A known residual, recorded because it bounds what this check can mean.** Widening moves
// the noisy ruleset from 29 flagged to 17 and leaves the well-formatted one at 4 exactly —
// so it buys accuracy without going blind. The 17 that remain name no symbol and are still
// perfectly actionable: *"Open a transaction at the top of every service method"* is a
// definite instruction with a definite target. **Naming an identifier is not the same as
// being actionable**, and no widening of these patterns closes that. This is why the check
// is advisory and must stay so.
//
// Ensures: true when doc holds any link or code span, or statement carries an identifier
//
//	shape; it is pure.
func concrete(statement string, doc *markdown.Doc) bool {
	if len(doc.Links) > 0 {
		return true
	}
	for _, p := range identifierPatterns() {
		if p.MatchString(statement) {
			return true
		}
	}
	return false
}
