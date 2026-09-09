# Rework

You are revising one candidate ruleset against findings raised about it. The ruleset is
below, and after it every finding a checker or a grader reported.

Revise the ruleset to resolve every **blocking** finding, and nothing beyond that.

<ruleset>{{RULESET}}</ruleset>

<findings>{{FINDINGS}}</findings>

______________________________________________________________________

## What to Change

A finding of severity `error` **blocks** and must be resolved. A `warning` is for you to
weigh: leaving one is a decision you may make, and it is not an omission.

**Do not delete a rule to silence a finding unless the source genuinely does not support
it.** Removing the rule and fixing the rule are different answers, and only one of them
keeps the ruleset faithful to what it was distilled from. A ruleset that passes because it
says less is not a better ruleset — it is a shorter one, and the finding it silenced is
still true of the source it came from.

Where a finding says a rule is not executable, add the `✗`/`✓` pair it lacks. The pair does
not have to be code: for an `[ARCH]` or `[METHOD]` rule it is a contrast between two designs
or two ways of working, written as prose. Do not manufacture a code snippet for a rule that
is not about code.

Where a finding says an anchor is not present in the source, the quotation is wrong and the
rule may still be right. Correct the quotation against the source rather than deleting the
rule, and if no passage supports the rule, then delete it and say so.

## What Not to Change

Leave every rule the findings do not mention exactly as it is, including its section number.
Renumbering to close a gap makes every finding's `path` refer to a different rule than the
one it was raised against, so the next round cannot be compared with this one.

## The Form to Write Back

Preserve the canonical form exactly, because a ruleset is compared byte for byte against its
canonical rendering:

- the `format: 3` block as the first three lines, then `Source:`, `Scope:`, `Limitations:`
- each rule opening `§N.M  [SEVERITY][LEVEL]  statement`, with the section number unchanged
- **two** spaces after each `[LEVEL]` tag, never three
- six spaces of indent on every rationale, `✗`, `✓` and `↦` line
- each rationale on **one line however long** — do not wrap it
- `✗` before `✓`, always
- every `[MUST]` and `[SHOULD]` rule keeps both its `✗`/`✓` pair and its `↦` anchor

Report what you changed and why, naming each finding you resolved and each warning you chose
to leave.
