# Prompt: Synthesise Multiple Distilled Rulesets into a Single Unified Ruleset

## Purpose

This prompt takes two or more rulesets produced by the distillation prompt and
produces a single document Claude can load in place of all of them. Two
properties must hold simultaneously in the output:

**Coverage:** Every constraint that would cause Claude to flag, reject, or avoid
something in any individual input must still do so in the combined output.
Nothing is lost in the merge.

**Compression:** The output must have fewer rules than the sum of the inputs.
Synthesis is not concatenation. Rules that address the same concern must be
merged; rules that are special cases of a general rule must be subsumed. If the
output has as many rules as the inputs combined, the synthesis failed.

Where inputs conflict, the conflict must be resolved — not suppressed — so the
output is unambiguous on every point where the inputs were.

---

## Task

Synthesise the following rulesets into a single unified ruleset:

```
{{RULESETS}}
```

---

## Output structure

Begin with a metadata block:

```
Sources:          [title and author for each input ruleset]
Scope:            [union of all input scopes; per-rule annotations where scopes diverge]
Synthesised from: [N rulesets]
```

Then numbered sections in a structure derived from the inputs (see Step 2).
Each section ends with an anti-patterns table that consolidates the ✗
counter-examples from that section's rules plus any anti-patterns the source
documents name that do not rise to a full rule. The document ends with a master
anti-patterns table across all sections; apply the same A–E cases to merge,
subsume, or resolve conflicting anti-pattern entries just as you do for rules —
do not concatenate the section tables.

Rule format is unchanged from the input rulesets:

```
§2.3  [MUST][CODE]   Imperative stated directly to Claude.
      Rationale naming the concrete failure mode on violation.
      ✗  Counter-example
      ✓  Preferred alternative
```

---

## Synthesis pipeline

Apply these steps in order. Within Steps 3–6, different groups can be
processed in parallel — but within each group, Steps 3, 4, and 5 are
sequential: resolve the group first (Step 3), then reconcile its severity
(Step 4), then resolve its scope (Step 5). Step 7 (pruning) requires the
complete draft and cannot begin until all groups are fully resolved.

### Step 1 — Inventory

For each rule in each input, write a one-line summary of its constraint and
assign it a group label. Two rules belong to the same group when they address
the same concern — regardless of wording, section placement, or source. Use
the same-verdict heuristic: if applying both rules independently to the same
artefact always yields the same ✓/✗ verdict, they address the same concern.
This grouping is the working input to all subsequent steps.

Format each group as:

```
Group [G]: [one-line description of the shared concern]
  - Ruleset 1 §N.M: [severity][level] [summary]
  - Ruleset 2 §N.M: [severity][level] [summary]
```

A rule that has no counterpart in any other input forms a group of one.

### Step 2 — Derive the section structure

Do not union the input section structures mechanically.

1. Treat each group label from Step 1 as a candidate theme. Merge candidate
   themes that are facets of the same concept; split any theme that conflates
   independently applicable concerns.
2. A section that would contain only one or two rules should be absorbed into
   the most closely related section unless it represents a categorically
   distinct concern.
3. Order sections so that rules which constrain other rules come first:
   architectural and methodological rules before the code-level rules they
   constrain; foundational patterns before derived ones.
4. Do not name sections after source documents or authors. Name them after the
   concern.
5. The synthesised document should have no more sections than the largest input's
   section count, unless the inputs cover categorically distinct domains with no
   overlap. If the draft exceeds this, return to Step 1 and check whether groups
   were split too finely.

### Step 3 — Resolve each group

Apply the first matching case to each group.

---

**Case A — Identical in substance.**
All rules in the group assert the same constraint, differing only in wording.

→ Merge into one rule. Select wording by this priority:
  1. The wording from the more domain-specific source (Go-specific over
     language-agnostic; domain-specific over general).
  2. If equal specificity, the wording that passes the two-reviewer test with
     less additional context — i.e., is self-contained on its own line.
  3. If still tied, the shorter.

Drop attribution. If any input version of the rule fails the quality bar
(vague, no failure mode, does not pass the replacement test), do not carry
forward the failure — rewrite the rule to pass before including it.

---

**Case B — One rule is a special case of another.**
Rule X says "never do Y." Rule Z says "never do Y in situation S." Z adds no
constraint that X does not already impose.

→ Keep rule X. If the specific case Z adds meaningful diagnostic or illustrative
value, note it parenthetically: *(applies especially in situation S, where the
failure manifests as …).* Do not keep Z as a separate rule.

---

**Case C — Complementary aspects of the same concern.**
The rules address the same underlying principle but each captures an aspect the
other does not, and both aspects are independently applicable.

→ Attempt to merge into one rule. If the merged rule fails the two-reviewer
test because it is too broad to apply consistently, keep two adjacent rules
within the same section. Do not keep them in separate sections.

---

**Case D — Conflicting prescriptions.**
Two or more rules give incompatible advice for the same situation.

Do not suppress the conflict. Resolve it:

1. **Specificity criterion.** If one source is more specific to the context
   (language, domain, scale), its prescription takes precedence within that
   context. State the other as the default outside that context.

2. **Conditional form.** Express the resolution as a conditional rule with an
   explicit criterion:

   > `§4.1  [MUST][ARCH]`  When P, apply A. When Q, apply B.
   > **Criterion:** [what distinguishes P from Q].
   > ⚠ [Source 1] prescribes A unconditionally; [Source 2] prescribes B
   > unconditionally. This conditional form is the synthesis.

3. **No distinguishing criterion.** If no criterion can be found, apply the
   prescription that prohibits more behaviour — the rule whose ✗ counter-example
   is a strict superset of the other's — and append: *(no distinguishing
   criterion found — stricter position applied).* When both prescriptions
   prohibit the same behaviour but disagree only on what to do instead (the ✓
   side), state both alternatives and note that the inputs conflict on the
   preferred remedy.

**Emergent conflicts.** After all groups are resolved, scan every `[MUST]` and
`[SHOULD]` rule against every other for implicit contradictions that did not
exist in either input individually — these arise from combining rules whose
interaction was never tested. Resolve any found using the same criterion above.

---

**Case E — Unique rule.**
No other input addresses the same concern.

→ Apply the quality bar first. If the rule passes, include it unchanged. If it
fails (vague, no failure mode, does not survive the replacement test), rewrite
it to pass or discard it. Do not include a poor-quality rule simply because no
input contradicts it.

---

### Step 4 — Reconcile severity

When the same rule appears with different severity tags across inputs, use this
table:

| Input severities | Output severity | Annotation |
|---|---|---|
| All `[MUST]` | `[MUST]` | None |
| `[MUST]` + `[SHOULD]` | `[MUST]` | *(one source treats as `[SHOULD]`; stricter applied)* |
| `[MUST]` + `[CONSIDER]` | `[MUST]` | *(sources disagree significantly on severity)* |
| All `[SHOULD]` | `[SHOULD]` | None |
| `[SHOULD]` + `[CONSIDER]` | `[SHOULD]` | *(one source treats as `[CONSIDER]`; stricter applied)* |
| All `[CONSIDER]` | `[CONSIDER]` | None |

When input language maps ambiguously to severity, apply the severity–language
mapping table from the distillation prompt before reconciling here.

### Step 5 — Resolve scope

- Rule applies across all input scopes → state without qualifier.
- Rule applies only within a subset of the combined scope → append a scope
  annotation: *(Go only)*, *(distributed systems)*, *(event-driven)*.
- General and specific versions of a rule both exist and do not conflict →
  include both; general rule first, specific rule as a refinement immediately
  after.

### Step 6 — Select illustrations

For each merged or reconciled rule, choose one illustration (code contrast,
structural contrast, or counter-example):

- Prefer the illustration from the more domain-specific source.
- If both illustrations cover different failure modes, combine them into two
  contrast pairs within the same rule.
- If neither input provided an illustration and the rule is non-obvious,
  construct one that represents the most common violation.

### Step 7 — Prune

Re-read the complete draft. For each rule, ask: if this rule were removed, is
there a concrete artefact — a function, a module boundary, a deployment plan —
that Claude would now generate, approve, or propose that the combined inputs
would reject? If not, remove the rule. Repeat until no more rules can be pruned.

---

## Coverage accounting

Before writing the final output, produce a one-line account for every rule in
every input:

```
Ruleset 1 §2.3  → output §3.1  (merged with Ruleset 2 §4.7, Case A)
Ruleset 1 §2.4  → output §3.1  (subsumed, Case B)
Ruleset 2 §6.1  → dropped      (failed replacement test: negation equally defensible)
Ruleset 2 §6.2  → output §5.3  (unique, Case E, rewritten to pass failure-mode test)
```

This accounting appears as an appendix to the output, not inline. Its purpose
is to make the synthesis auditable. Every input rule must have an entry.

---

## The quality bar

Every rule in the output must pass all four tests. Do not exempt rules carried
over unchanged; re-verify them in the context of the combined document.

**1. Replacement test.** Negate the rule. If the negation is defensible from
any input source, the rule is too vague — sharpen or discard it.

**2. Two-reviewer test.** Two people applying the rule independently to the same
artefact reach the same verdict without needing to consult the source.

**3. Failure-mode test.** The rationale names what breaks — at runtime, review
time, maintenance time, or organisational scale — on violation. Restatement of
the principle in abstract terms fails.

**4. Non-contradiction test.** No two rules prescribe opposite actions for the
same situation without a conditional that disambiguates them. This includes
emergent contradictions that arise from combining rules whose interaction was
never tested in either input.

---

## Attribution policy

**Omit attribution** when rules merge without conflict. The synthesis speaks for
itself.

**Include attribution** only when it changes how Claude should interpret or
apply the rule:

1. A conditional exists because sources disagree (Case D) — name which source
   holds which position.
2. A rule overrides Claude's defaults — append *(overrides common practice —
   apply as stated).*
3. A rule's scope is narrower than the combined document's scope — append the
   scope annotation.

---

## Verification pass

Before submitting, confirm:

1. **Coverage:** The coverage accounting appendix has an entry for every rule in
   every input. No rule is silently absent.
2. **Compression:** The output has fewer rules than the sum of all inputs.
   If not, re-examine every group resolved as Case C for whether it should have
   been Case A or B instead; also re-examine Case E groups for whether their
   concern truly has no counterpart in any other input or whether the Step 1
   grouping was too narrow.
3. **Non-contradiction:** Every `[MUST]` and `[SHOULD]` rule has been checked
   against every other for implicit conflict, including emergent conflicts.
4. **Scope annotations:** Every rule whose scope is narrower than the combined
   document's scope is annotated.
5. **Imperative form:** Every rule is a direct instruction to Claude, not a
   description of what good developers do.
6. **Unambiguity:** For every `[MUST]` rule, identify the most plausible
   borderline case. If it is unclear whether the rule applies, add the missing
   condition before submitting.
