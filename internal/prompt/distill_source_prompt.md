# Prompt: Distill a Source Document into a Claude Coding Ruleset

## Purpose

This prompt produces a ruleset that Claude loads as context before reviewing or
generating code. The output is not a summary — it is a specification that
overrides or supplements Claude's defaults. When the source contradicts Claude's
general training, the source wins. Rules must be written so Claude can apply
them mechanically to a code sample, a design decision, or a proposed
architecture without needing to re-read the source.

---

## Task

Distill <source>{{SOURCE_CONTENT}}</source> into a ruleset for Claude.

The source may contain advice at any level: code patterns, API and interface
design, module structure, architectural constraints, platform strategy, or
development methodology. All of it should produce rules. A rule is ready when
Claude can apply it to a concrete artefact — a function, a module boundary, a
deployment plan — and reach the same verdict the source's author would reach.

---

## Output structure

Begin the output with a metadata block:

```
Source: [title and author, or "unknown" if not stated]
Scope:  [language(s), paradigm(s), domain(s), and architectural context — derived from the source, not assumed]
```

Then numbered sections derived from the source's own structure, numbered 1,
2, 3 … in the order they appear in the output, independent of any numbering in
the source. Order sections so that foundational constraints precede derived
ones — a rule constraining how other rules are applied belongs before the rules
it constrains. Each section contains rules followed by an anti-patterns table.
Close with a master anti-patterns table across all sections.

If the source has no clear section structure, derive sections from the dominant
concerns the rules cluster around. Do not produce a flat list.

### Rule format

```
§2.3  [MUST][CODE]   Never discard an error return without an explicit decision.
      Silently dropping errors removes the caller's only signal that an
      operation failed; bugs become invisible until they corrupt state downstream.
      ✗  result, _ = db.Exec(query)
      ✓  result, err = db.Exec(query); if err != nil { return fmt.Errorf(...) }

§5.1  [MUST][ARCH]   Keep business logic out of the persistence layer.
      Embedding domain rules in stored procedures or ORM hooks couples
      correctness to a specific database technology; unit-testing the logic
      or migrating the database then requires the full database stack.
      ✗  Validation trigger in PostgreSQL enforces a domain invariant
      ✓  Domain service validates the invariant before calling the repository

§7.2  [SHOULD][METHOD]  Deploy each change independently rather than batching releases.
      Batched deployments make it impossible to attribute a production incident
      to a specific change and force full rollback when only one change is defective.
```

Within each section, order rules `[MUST]` first, then `[SHOULD]`, then
`[CONSIDER]`. Within each severity tier, place more fundamental rules before
derived ones.

Include an illustration only when the rule is non-obvious or frequently
misapplied. For `[CODE]` rules, show a code contrast. For `[ARCH]` rules, show
a structural contrast. For `[METHOD]` rules, a concrete counter-example is
sufficient when one exists in the source. Use the source's own examples wherever
available — they are the most authoritative test cases.

### Anti-patterns table format

| Anti-pattern | Level | Rule violated | Failure mode | Preferred alternative |
|---|---|---|---|---|

---

## Rule levels and severity

Tag every rule with a severity and a level. Both tags are required.

### Severity

| Tag | In review | In generation or design |
|---|---|---|
| `[MUST]` | Always flag; block approval | Never produce or propose |
| `[SHOULD]` | Flag with justification and alternative | Avoid; note explicitly when a tradeoff forces it |
| `[CONSIDER]` | Raise only when asked to improve | Prefer but do not enforce |

### Mapping source language to severity

| Source says | Assign |
|---|---|
| "never", "always", "must", "do not", "required" | `[MUST]` |
| "should", "prefer", "avoid", "better to", "recommended" | `[SHOULD]` |
| "consider", "may", "can", "sometimes useful", "worth exploring" | `[CONSIDER]` |
| "it depends", conditional phrasing | Express as a conditional rule (see Special cases) |

When the source's language is ambiguous, assign the lower severity and note the
ambiguity inline.

### Level

| Tag | Applies when Claude is |
|---|---|
| `[CODE]` | Reading or writing code at the expression, function, or module level |
| `[ARCH]` | Evaluating or proposing system structure: service boundaries, dependencies, data flow, coupling |
| `[METHOD]` | Advising on process: how work is organised, sequenced, tested, or deployed |

Always include a level tag; `[CODE]` is the default when the rule applies at
the expression, function, or module level. `[ARCH]` and `[METHOD]` rules apply
not only during code review but whenever Claude proposes a design, evaluates a
tradeoff, or structures generated code — they constrain what Claude recommends,
not just what it flags.

When a rule has a code-level symptom but an architectural root cause — "never
open a database connection inside an HTTP handler" — tag it `[ARCH]` and
describe the code-level manifestation in the rationale. The level tag reflects
where the constraint is enforced, not where the violation is observed.

---

## Extracting rules

### From prose

For each claim the source makes, ask: what would a reviewer need to check to
verify this claim holds in a codebase? That check is the rule.

### From examples

The source's code and design examples are as important as its prose. For each
example, ask: what rule does this example demonstrate that the surrounding text
does not state explicitly? The most important rules are sometimes only shown,
never stated. Extract them.

### From structural descriptions

Architecture diagrams, system descriptions, and deployment topologies imply
boundary rules. "Service A calls Service B synchronously" is not a fact to
record — it is a pattern to extract a rule from: under what conditions does the
source endorse or condemn it?

Example: a diagram showing five independently deployed services sharing one
database does not become the rule "use a shared database." It becomes: "Do not
use a shared mutable data store as the primary coordination mechanism between
independently deployed services; it couples their deployment schedules and
prevents independent scaling. `[MUST][ARCH]`"

---

## The quality bar

A rule is ready when it passes all three tests.

### 1. The replacement test

Negate the rule. If the negation is also defensible from the source text, the
rule is too vague — sharpen it or discard it.

### 2. The two-reviewer test

Two people applying the rule independently to the same code sample, design
description, or process artefact should reach the same verdict. Rules that
require unstated judgment fail this test.

### 3. The failure-mode test

The rationale must name what breaks — at runtime, at review time, at
maintenance time, or at organisational scale — when the rule is violated.
Restatements of the principle in abstract terms do not pass.

### Examples

| Fails | Passes |
|---|---|
| "Keep functions small." | "Extract any logic that requires understanding state not visible in the current function's signature, because the reader must hold invisible context to reason about correctness. `[SHOULD][CODE]`" |
| "Handle errors properly." | "Never discard an error return without an explicit decision; silently dropping it removes the caller's only signal of failure. `[MUST][CODE]`" |
| "Separate concerns." | "Do not let a module that owns persistence also own routing or serialisation; when one module changes for two unrelated reasons, all its dependents must be retested for both. `[SHOULD][ARCH]`" |
| "Release often." | "Do not batch unrelated changes into a single deployment; when an incident occurs, the inability to isolate which change caused it forces rollback of all changes, including safe ones. `[SHOULD][METHOD]`" |
| "Design for scalability." | "Do not use a shared mutable data store as the coordination mechanism between independently deployed services; it couples their deployment schedules and makes independent scaling impossible. `[MUST][ARCH]`" |
| "Write testable code." | "Do not instantiate collaborators inside a function; accept them as parameters so tests can substitute them without rewriting the function under test. `[MUST][CODE]`" |

---

## Special cases

### Missing rationale

When the source gives a rule without explanation, construct the rationale from
the failure cases the source describes elsewhere. If no failure case exists,
assign `[CONSIDER]` and append: *(rationale not given in source — applied
conservatively).*

### Conditionality

When the source's advice depends on context, express the condition explicitly
rather than hedging:

> `§4.1  [MUST][ARCH]`  When a service boundary crosses a team boundary, treat
> inter-service calls as untrusted external I/O. When both sides of the boundary
> are owned by the same team, a shared library is acceptable.
> **Criterion:** ownership determines the boundary type, not deployment topology.

### Contradictions

When the source contradicts itself or contradicts widely accepted practice,
state both positions as conditional rules, name the tension, and provide a
resolution criterion. Do not average them into vague advice, and do not silently
prefer the position that matches Claude's training.

### Source versus Claude's defaults

When a rule from the source conflicts with Claude's general training or common
practice, the source takes precedence. Mark such rules with a trailing note:

> *(Overrides common practice — apply as stated.)*

---

## Exclusions and volume

**Do not include:**

- Content that produces no testable rule: motivation, analogy, biography,
  throat-clearing.
- Rules trivially satisfied by any non-pathological code or design.
- Rules requiring knowledge of future requirements to apply.
- Direct quotations — every rule must be rewritten as an imperative.

**On volume:** Calibrate rule density to the source's emphasis. A source that
argues at length about error handling should produce more error-handling rules
than a source that treats it in passing. After drafting, prune: if removing a
rule leaves no case where Claude would generate, approve, or propose something
the source would reject, remove it. Prefer twenty precise rules over fifty
overlapping ones.

---

## Verification pass

Before submitting, confirm each rule satisfies all of the following:

1. **Negation test:** The negation is clearly wrong given the source text.
2. **Severity calibration:** The severity tag reflects the source's position,
   not Claude's inference about software engineering in general.
3. **Failure mode:** Every `[MUST]` rule names a concrete failure in one
   sentence.
4. **Level correctness:** Every `[ARCH]` and `[METHOD]` rule constrains a
   design decision or process artefact, not just a line of code.
5. **Imperative form:** Every rule is written as a direct instruction to Claude,
   not a description of what good developers do.
6. **Source fidelity:** No rule asserts more than the source supports. Where the
   source hedges, the rule hedges or assigns lower severity.
7. **Anti-patterns tables:** Every section has a populated anti-patterns table;
   the master table is present. Each table includes patterns the source warns
   against that do not rise to a dedicated rule, in addition to the ✗ examples
   drawn from the section's rules.

Revise or drop any rule that fails. Do not pad the ruleset to appear
comprehensive.

## Final Output

The final output is a single markdown document that can be loaded into Claude
and should be saved to <destination>{{DESTINATION_CONTENT}}</destination>
