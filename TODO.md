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

## P1 — independent verification (highest leverage) — IMPLEMENTED

The `critic_step` bundle **A + C + D** is built: the `critic` command emits a
cold-critic prompt; an agent runs it; the `gate` command self-tests, then blocks on
the returned findings. This moves the pipeline from "self-refined" to "independently
verified."

- [x] **A. Cold critic for rulesets.** `internal/prompt/critic_prompt.md` +
  `internal/critic.FillPrompt` + the `canonizer critic --source --ruleset` command
  emit a prompt giving a fresh grader *only* the source + candidate ruleset (never the
  distillation), asking it to flag `unsupported`/`vague`/`duplicate` rules as strict
  `skillet/finding` JSON. Prompt-filler posture: canonizer emits; an agent runs it.
- [x] **C. Planted-defect negative control.** `internal/gate.SelfTest` feeds a planted
  blocking finding and a clean one through the gate and errors unless it discriminates;
  the `gate` command runs it every invocation (and `gate --selftest` runs it alone) and
  refuses to gate if the control fails. Mirrors adh's `oracle selftest`.
- [x] **D. Structured findings + machine gate → `skillet/finding`.** `canonizer gate`
  parses the agent's findings into `finding.Result` and returns `root.ExitError(1)`
  while `internal/gate.Blocking` finds any error-severity finding — the blocking
  decision offloaded to skillet's severity model.

Follow-ups surfaced while building: wire `critic`→`gate` into a scripted stage (F's
rework loop); once the canonical-form work lands, feed each rule's ✗/✓ pair through the
critic (B).

______________________________________________________________________

## P2 — executable rules & provenance

- [ ] **B. "Findings must run" → `skillet/ruleset` + `skillet/judge`.** Attach a
  discriminating ✗/✓ worked-example pair to each `[MUST]`/`[SHOULD]`;
  `ruleset.Rule.Bad`/`Good` already model the pair. A cold agent (or `judge.Check` +
  `Score`) verifies the rule yields opposite verdicts on the pair. Rules with no
  discriminating example fail the two-reviewer test and are cut. Depends on the
  canonical-form output decided under *Ruleset parsing & verification* below (load via
  `ruleset.Parse`).
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

- [x] **Decided: emit the canonical form.** The distill and synthesize prompts must
  produce the canonical `§N.M [SEV][LEVEL]` form that `skillet/ruleset.Render` emits
  and `ruleset.Parse` round-trips, rather than the free-form `## N.` +
  `**Do**`/`**Do not**` layout. This makes structured loading (`[]ruleset.Rule`)
  deterministic for the B/E verify commands, and honors skillet's locked
  `Render`/`Parse` round-trip contract. Rejected: a free-form reader — parsing
  free-form *model* output is brittle exactly where it must be reliable, and skillet
  deliberately declined to parse hand-authored files.
- [x] **Emit canonical rulesets from the prompt templates.** Both embedded templates
  now instruct **pure canonical** output — a `Source:`/`Scope:` block then a flat
  sequence of `§N.M [SEV][LEVEL]` rule blocks (rationale, ✗, ✓), grouped by the section
  number, with no `##` headings, tables, appendices, or stray lines (any of which would
  corrupt `ruleset.Parse`). The `{{SOURCE_CONTENT}}`/`{{DESTINATION_CONTENT}}` and
  `{{RULESETS}}` placeholders are unchanged. A `prompt_test` token guard catches a
  future edit that drops the format. Chose pure canonical over prose+canonical (no
  extractor, no drift). **Unblocks B and E.**
- [x] **Load rulesets via `ruleset.Parse` in the verify path.** `critic` now parses the
  candidate ruleset through `skillet/ruleset.Parse` before emitting the prompt: a
  non-canonical ruleset fails fast (`critic: candidate ruleset: …`), and it reports the
  rule count. The parsed `[]ruleset.Rule` is the seam B (feed each ✗/✓ pair to `judge`)
  and E (per-rule provenance) will consume.

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
