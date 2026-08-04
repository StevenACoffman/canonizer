# canonizer — TODO

canonizer produces Claude coding rulesets from source documents. It reproduces the
`ai-skill` pipeline (`~/Documents/agent-orange/go-advice/ai-skill`) as a
climax-structured CLI with every path configurable and the heavy lifting offloaded
to `skillet`. This backlog combines the current repo state with the rigor backlog
from `~/Documents/agent-orange/go-advice/ai_skill_todo.md`.

**Core takeaway (unchanged from ai_skill_todo):** the pipeline has strong
*design-time* rigor, but every quality gate is **self-assessment by the same model
that produced the artifact**. The transferable lesson: *a fresh, independent grader
checking against runnable ground truth beats the producer grading its own work.*
canonizer is now the concrete tool where that gate would live.

______________________________________________________________________

## Done

- **`distill`** — for every Markdown source under `--source`, fill a per-source
  distillation prompt and write `*_prompt.md` to `--out`, via
  `skillet/ruleset/distill.Generate`. Byte-identical to ai-skill's binary for the
  same template.
- **`synthesize`** — read the `*_rules.md` in `--rulesets`, assemble them into the
  synthesis prompt (one `<ruleset>` block per input) via
  `skillet/ruleset/synthesize`, write to `--out` or stdout.
- **All paths configurable** — templates are embedded defaults, overridable with
  `--template`; no binary-relative / cwd lookup, no absolute paths, no `os.Getenv`.
  (Replaces ai-skill's fragile three-tier template lookup and the hardcoded absolute
  paths in `make_distill.sh`.)
- Lint-clean (`golangci-lint`), structurally clean (`climax lint`), tested (pure
  `synth` core + end-to-end command tests through `cmd.Run`).

______________________________________________________________________

## Decided — architecture

- [x] **canonizer is a prompt-filler.** It fills prompts for a model an agent runs;
  it never calls a model itself. Every rigor item that needs judgment (cold critic,
  "the anchor supports the claim") follows the same loop as `distill`/`synthesize`:
  **canonizer emits a prompt → an agent runs it → the agent feeds findings back →
  canonizer/skillet apply the deterministic gate.** This keeps the deterministic gate
  in canonizer/skillet and the judgment in the agent — consistent with skillet's rule
  that *a model handles only what a deterministic check cannot decide.* No
  model-invocation seam is added.

## P0 — centralization

- [x] **Pushed synthesis upstream into skillet.** `internal/synth` is gone; canonizer
  now consumes `skillet/ruleset/synthesize` (`Marker`, `Input`, `FillTemplate`,
  `LoadInputs`), sibling to `ruleset/distill`. canonizer's `cmd/synthesize` is a thin
  shell (load → fill → write); exegesis/skillsaw can reuse the package.
- [x] **Bumped skillet to the released tag.** `canonizer/go.mod` now requires
  `github.com/StevenACoffman/skillet v0.4.0` (was the `ruleset-synthesize` branch
  pseudo-version); the `ruleset/synthesize` package is released.

______________________________________________________________________

## P1 — independent verification (highest leverage)

The `critic_step` bundle: **A + C + D** together move the pipeline from
"self-refined" to "independently verified." This is the suggested first build.

- [ ] **A. Cold critic for rulesets.** Add a critic prompt asset + command that
  emits a prompt giving a fresh grader *only* the source + candidate ruleset (never
  the distillation), asking it to flag rules that are unsupported, redundant, or fail
  the three-test bar. Per the prompt-filler posture, canonizer emits the prompt and
  an agent runs it; the agent returns `skillet/finding.Diagnostic` findings that
  canonizer gates on (D). No in-process agent spawning.
- [ ] **C. Planted-defect negative control.** Inject a known-bad rule (vague /
  unsupported / contradicts source) and confirm the cold critic rejects it; halt the
  run if it passes. Mirrors adh's `oracle selftest`. Nothing today proves the gate
  can fail.
- [ ] **D. Structured findings + machine gate → `skillet/finding`.** Replace prose
  self-report with `finding.Diagnostic{Severity,Category,Path,Message}`; the run does
  not advance while any `unsupported`/`vague`/`duplicate` finding is open
  (`Result.HasBlocking()` over error severity). canonizer already consumes skillet;
  this is a direct offload.

______________________________________________________________________

## P2 — executable rules & provenance

- [ ] **B. "Findings must run" → `skillet/ruleset` + `skillet/judge`.** Attach a
  discriminating ✗/✓ worked-example pair to each `[MUST]`/`[SHOULD]`;
  `ruleset.Rule.Bad`/`Good` already model the pair. A cold agent (or `judge.Check` +
  `Score`) verifies the rule yields opposite verdicts on the pair. Rules with no
  discriminating example fail the two-reviewer test and are cut. Requires emitting
  rulesets in a parseable form — see *Ruleset parsing* below.
- [ ] **E. Proof-of-provenance → `skillet/{identity,proof,markdown}`.** Require each
  rule to cite the **source anchor** (section/quote) it derives from, plus a pass
  that confirms the anchor exists (`markdown` section lookup, deterministic) and —
  via an agent-run prompt — that it supports the claim. Hash-bind rule↔source with
  `proof.Artifact`/`identity.Hash`.
  - [ ] **skillet prerequisite:** add a per-rule `SourceAnchor` field to
    `ruleset.Rule` — today `Source` is ruleset-level, so provenance can't bind per
    rule.

______________________________________________________________________

## P3 — loop governance

- [ ] **F. Rework budget with terminal escalation.** Enforce a hard cap of N
  refine cycles; after N a ruleset either passes the cold critic or is flagged
  `needs-human` — never silently shipped. Today the 2–3 cap is advice, not enforced.
- [ ] **G. Model-gate (convention only).** State as policy: run critic/synthesis on
  a reasoning-class model. No enforcing seam exists, so it is a convention.

______________________________________________________________________

## Ruleset parsing & verification (canonizer-specific)

- [ ] **Decide the ruleset output form.** `skillet/ruleset.Parse` round-trips only the
  *canonical* `§N.M [SEV][LEVEL]` form `ruleset.Render` emits; the distilled
  `*_rules.md` files are free-form (`## N.` + `**Do**`/`**Do not**`). Before any
  verify command (B/E) can load rulesets as structured `ruleset.Rule`s, either the
  synthesis prompt must emit the canonical form, or canonizer needs a free-form
  reader. Resolve this before building B/E.

______________________________________________________________________

## Housekeeping (from ai_skill_todo)

- [ ] **Rubric scores are relative, not absolute.** The self-assessment rubric should
  gate only on *delta between iterations*; the **cold critic (A)** — not the
  self-score — is the real ship gate. This is the core self-assessment weakness
  showing up in the gate's own arithmetic.
- Note: ai_skill_todo's "duplicate `distill_step/`" item is a `go-advice` source-repo
  concern; canonizer sidesteps it by taking `--source`/`--out` as flags rather than
  baking a source tree into a script.

______________________________________________________________________

*Recorded 2026-08-04. Sources: this repository's state, and
`~/Documents/agent-orange/go-advice/ai_skill_todo.md` (rigor backlog + the
"Centralize on skillet" offload analysis). See `PLAN.md` for the implementation
plan and its design review against `summary_rules.md`.*
