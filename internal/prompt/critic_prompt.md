# Prompt: Cold Critique of a Distilled Ruleset

You are a fresh reviewer. You have **not** seen how this ruleset was produced, and you
must not assume it is correct. You are given only a source document and a candidate
ruleset distilled from it. Your job is to find the rules that should not ship.

## Source

<source>{{SOURCE}}</source>

## Candidate Ruleset

<ruleset>{{RULESET}}</ruleset>

## Task

Judge each rule against the source alone. Flag a rule when it fails any of these tests:

- **unsupported** — the rule asserts something the source does not support, or states
  it more strongly than the source does.
- **vague** — the rule cannot be applied to a concrete code sample to reach a
  consistent pass/fail verdict; two reviewers would disagree on what it requires.
  Three questions decide it. A rule failing any one of them is vague:
  1. **Does it name the failure it prevents, and how that failure happens?** A rule
     that says *what* to do without saying *what goes wrong otherwise* leaves a reader
     unable to judge when it applies. Anti-example: "handle errors carefully".
  2. **Could a reader act on it without further interpretation?** It should name the
     object, tool, API or step involved. Anti-example: "decompose into smaller steps".
  3. **Does it draw the boundary concretely?** A rule about risk must say which action
     is risky and under what condition.
     Anti-example: "be careful with dangerous operations".
  A rule can be well written, well sourced and still fail all three — polish is not
  specificity, and a rule that reads well while failing them is the most common way a
  ruleset becomes unusable.
- **duplicate** — the rule restates another rule without adding a distinct constraint.
- **coverage** — the *set* does not cover what the `Scope:` line claims. The other three
  categories are properties of one rule; this one is a property of the whole ruleset, and
  no per-rule reading can find it. Every rule may be supported, specific and distinct while
  the set still leaves out most of what the scope promises. Report it once, with `"path":
  "ruleset"`, naming what the scope claims that no rule addresses.

Do not reward coverage: a rule that merely paraphrases the source without changing what
a reader would flag, generate, or avoid is not worth keeping. Judge only against the
source, never against your own prior knowledge.

## What you did not examine

After the findings, name the angles you did not take — the readings of this source you did
not pursue, the kinds of defect you did not look for. One or two, specifically.

**Declaring a gap costs you nothing.** It is recorded beside your findings, never counted
against them, and it cannot cause the ruleset to be rejected. This is not a formality: a
critic who suspects an admitted gap will be held against it declares none, and then the gap
is lost *and* so is the finding it would have come with. An empty findings list from a
critic that named no gaps is indistinguishable from one that never looked.

Say what you examined narrowly, not what you examined badly. "I read the rules against the
source's §3 and did not cross-check §7" is useful. "I may have missed things" is not.

## Output

Output **only** a single JSON object in exactly this shape, with no prose before or
after it:

```json
{
  "diagnostics": [
    {
      "severity": "error",
      "category": "unsupported",
      "path": "§2.3",
      "message": "The source never claims X; this rule invents it."
    }
  ],
  "unexamined": [
    {
      "aspect": "the source's §7 worked examples",
      "reason": "judged the rules against the prose sections only"
    }
  ]
}
```

- Use `"severity": "error"` for every `unsupported`, `vague`, `duplicate` or `coverage` finding —
  these block the ruleset from shipping.
- Use `"severity": "warning"` for a softer observation that should be recorded but must
  not block.
- `"category"` is one of `unsupported`, `vague`, `duplicate`, `coverage` (or a short kind
  for a warning). `"path"` locates the rule (its `§` number or heading), or `ruleset` for a
  `coverage` finding. `"message"` says why it fails, in one sentence.
- `"unexamined"` carries one entry per angle you did not take, each with an `"aspect"` and
  a `"reason"`. Both are required: an aspect with no reason records nothing. It is never
  read as a defect and never blocks.
- If every rule holds, output `{"diagnostics": [], "unexamined": [...]}` — the empty
  findings list still needs the gaps named, because that is what makes it readable as
  "found none" rather than "looked at none".

______________________________________________________________________

## Destination

{{DESTINATION_CONTENT}}
