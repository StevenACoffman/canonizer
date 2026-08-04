# Prompt: Cold Critique of a Distilled Ruleset

You are a fresh reviewer. You have **not** seen how this ruleset was produced, and you
must not assume it is correct. You are given only a source document and a candidate
ruleset distilled from it. Your job is to find the rules that should not ship.

## Source

<source>{{SOURCE}}</source>

## Candidate ruleset

<ruleset>{{RULESET}}</ruleset>

## Task

Judge each rule against the source alone. Flag a rule when it fails any of these tests:

- **unsupported** — the rule asserts something the source does not support, or states
  it more strongly than the source does.
- **vague** — the rule cannot be applied to a concrete code sample to reach a
  consistent pass/fail verdict; two reviewers would disagree on what it requires.
- **duplicate** — the rule restates another rule without adding a distinct constraint.

Do not reward coverage: a rule that merely paraphrases the source without changing what
a reader would flag, generate, or avoid is not worth keeping. Judge only against the
source, never against your own prior knowledge.

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
  ]
}
```

- Use `"severity": "error"` for every `unsupported`, `vague`, or `duplicate` finding —
  these block the ruleset from shipping.
- Use `"severity": "warning"` for a softer observation that should be recorded but must
  not block.
- `"category"` is one of `unsupported`, `vague`, `duplicate` (or a short kind for a
  warning). `"path"` locates the rule (its `§` number or heading). `"message"` says why
  it fails, in one sentence.
- If every rule holds, output `{"diagnostics": []}`.
