# Canonizer — TODO

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

## Decided — Architecture

- [x] **canonizer is a prompt-filler.** It fills prompts for a model an agent runs;
  it never calls a model itself. Every rigor item that needs judgment (cold critic,
  "the anchor supports the claim") follows the same loop as `distill`/`synthesize`:
  **canonizer emits a prompt → an agent runs it → the agent feeds findings back →
  canonizer/skillet apply the deterministic gate.** This keeps the deterministic gate
  in canonizer/skillet and the judgment in the agent — consistent with skillet's rule
  that *a model handles only what a deterministic check cannot decide.* No
  model-invocation seam is added.

## P0 — Centralization

- [x] **Pushed synthesis upstream into skillet.** `internal/synth` is gone; canonizer
  now consumes `skillet/ruleset/synthesize` (`Marker`, `Input`, `FillTemplate`,
  `LoadInputs`), sibling to `ruleset/distill`. canonizer's `cmd/synthesize` is a thin
  shell (load → fill → write); exegesis/skillsaw can reuse the package.
- [x] **Bumped skillet to the released tag.** `canonizer/go.mod` now requires
  `github.com/StevenACoffman/skillet v0.4.0` (was the `ruleset-synthesize` branch
  pseudo-version); the `ruleset/synthesize` package is released.

______________________________________________________________________

## P1 — Independent Verification (Highest Leverage) — IMPLEMENTED

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

## P2 — Executable Rules & Provenance — IMPLEMENTED

The `verify` command runs both deterministic gates over a parsed ruleset and emits
`skillet/finding` JSON for `gate` to block on:
`canonizer verify --ruleset R --source S | canonizer gate`.

- [x] **B. "Findings must run" → `skillet/ruleset` + `skillet/judge`.**
  `internal/verify.Executable` flags every enforced (`[MUST]`/`[SHOULD]`) rule that
  lacks a discriminating ✗/✓ pair — one with no ✗/✓, or whose ✓ appears verbatim inside
  its ✗ (scored with `judge.OpContains`). The semantic verdict-flip stays the cold
  critic's (A) job.
- [x] **E. Proof-of-provenance → `skillet/{proof,identity}`.**
  `internal/verify.Provenance` flags every enforced rule with no source anchor or an
  anchor absent from the source (whitespace-normalized). `verify --proof P` writes a
  `proof` packet binding the ruleset and source bytes (`identity.Hash`). Whether an
  anchor *supports* the claim is the critic's `unsupported` category, so E adds no new
  prompt.
  - [x] **skillet prerequisite (done):** `ruleset.Rule` gained a `SourceAnchor` field,
    emitted/parsed as an indented `↦` line and round-trip-preserved. Both templates now
    instruct a `↦ <anchor>` line per enforced rule.
- [x] **Bumped skillet to the released tag.** `canonizer/go.mod` now requires
  `github.com/StevenACoffman/skillet v0.5.0` (was the `ruleset-source-anchor` branch
  pseudo-version); `SourceAnchor` is released.

______________________________________________________________________

## P3 — Loop Governance — IMPLEMENTED

- [x] **F. Rework budget with terminal escalation.** `internal/budget.Decide` returns
  ship / rework / needs-human from the blocking state and the attempt counter; the
  `budget` command reads a findings result and exits 0 / 2 / 1 so a driver loops while
  budget remains and escalates to `needs-human` once it is spent — a blocked ruleset is
  never shipped. The driver keeps the counter, so `Decide` stays pure and stateless.
- [x] **G. Model-gate (convention only).** Stated in the root command's LongHelp: run
  the emitted distill/synthesize/critic prompts on a reasoning-class model; canonizer
  does not enforce it. No mechanism — a convention, as scoped.

______________________________________________________________________

## Ruleset Parsing & Verification (Canonizer-Specific)

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

## Housekeeping (From Ai_skill_todo)

- [x] **Rubric scores are relative, not absolute.** canonizer's ship gate is already
  findings-based by design — the cold critic (A) plus the deterministic `verify` → `gate`
  → `budget` chain, never a model self-score (grep confirms no self-score/absolute
  threshold anywhere). The policy is now recorded so it stays that way: a "Refinement
  policy" note in the root help (beside the model policy) and a rationale at the
  `internal/budget` seam — a self-score, if ever added, is advisory (iteration-delta
  only) and never gates adoption.
- Note: ai_skill_todo's "duplicate `distill_step/`" item is a `go-advice` source-repo
  concern; canonizer sidesteps it by taking `--source`/`--out` as flags rather than
  baking a source tree into a script.

______________________________________________________________________

## Cross-Repo Alignment & Follow-Ups (2026-08-05 Survey)

canonizer is the **reference dependency posture** for the skillet family: on skillet
**v0.5.0** and **toerr v0.1.0** directly, with no `replace` directive — the state the
other consumers (skillsaw v0.1.0, adh v0.3.0, exegesis v0.4.0) are being brought toward.
No bump is owed. The remaining work is canonizer's own governance follow-ups:

- [x] **Wire `critic` → `gate` into a scripted stage (F's rework loop).** Done: the
  `loop` command runs one deterministic round — verify the candidate ruleset
  (executability + provenance), merge the agent's cold-critic findings, and decide
  ship / rework / needs-human under the budget — exiting 0 / 2 / 1. It stays a
  prompt-filler (no model call) and stateless (`--attempt`/`--max` passed in, per P3);
  its `LongHelp` documents the wrapping driver that holds the counter. So the
  independent-verification loop is now runnable end-to-end, not just assemblable.
- [x] **Enforce the convention-only policies, or accept them explicitly.** Resolved by
  splitting the two along their nature:
  - **Self-scores never gate adoption → enforced.** The invariant "a blocked ruleset
    never ships" is pinned by `TestDecideBlockingNeverShips` (sweeping the attempt/limit
    grid), and `budget.Decide`'s doc names it as the guard: its only inputs are
    `blocking, attempt, limit` — a self-score must never become a fourth input or a Ship
    path. A future self-score bypass fails the test loudly.
  - **Reasoning-class model gate (G) → accepted, recorded for audit.** canonizer is a
    prompt-filler and cannot observe the grader model, so a hard gate would be theater.
    `loop --model` records the operator's attestation on stderr, explicitly labeled
    "operator-attested, unverified", and never touches the verdict; the root `LongHelp`
    states the gate is the operator's responsibility. Rejected recording it in the
    skillet `proof.Packet`: that needs a cross-repo change to carry an unverifiable claim
    and would read as a digest-bound fact like the real artifacts.
- [x] **Release tags — DONE, and the "tag once a consumer pins canonizer" rule is retired.**
  `v0.3.0` was cut on 2026-08-14 and `HEAD` sits on it. The rule this entry set for itself
  never fired and never will as stated: **nothing pins canonizer by version** — rechecked
  2026-08-14 across skillet, adh, exegesis, skillsaw and unified-thinking — yet three
  releases have been cut anyway. The rule described a consumer-driven cadence this repo
  does not have, so it is replaced by what actually happens: **tag when meaningful work
  lands.** Recorded rather than silently dropped, so the absent trigger is not read as an
  oversight at the next survey.
  The verification below is kept because it is what makes tagging routine — it proves the
  machinery, and none of it needs redoing:
  - `v0.1.0` and `v0.2.0` exist locally **and** on the remote, are published as module
    versions, and both have GitHub releases with artifacts — `v0.2.0` was cut on
    2026-08-09.
  - `.goreleaser.yaml` passes `goreleaser check`, and `release.yml` fires on `v*`.
  - The ldflags seam **injects**: building with goreleaser's exact
    `-X …/cmd/version.Version=` flag reports that version and a resolved `GitCommit`.
    Worth having checked, because a stale module path there fails silently — the binary
    still builds and still reports `dev`.

  Standing note for the next release: a tag publishes a module version, fires CI and
  creates a public GitHub release, so it stays a deliberate act. Nothing here needs
  re-verifying first — `goreleaser check` and the ldflags probe above already passed, and
  they only need repeating if `.goreleaser.yaml` or the `cmd/version` symbol path changes.

______________________________________________________________________

## SkillLens Quality Dimensions — Rule Specificity (2026-08-08)

Source: `~/Documents/agent-orange/skillopt_changes_findings.md`. The sibling tools score
three dimensions taken from `microsoft/SkillLens` (arXiv:2605.23899): failure-mechanism
encoding, actionable specificity, and a high-risk action blacklist — each validated at
65–66% predictive accuracy against downstream utility.

**Why this lands here despite canonizer grading rulesets rather than skills.** The
deterministic gates are `Executable` (does each enforced rule carry a discriminating ✗/✓
pair?) and `Provenance` (does its anchor appear in the source?) — structural and
citational. Neither can tell a rule that encodes a domain failure mechanism from one that
says "handle errors carefully" with a valid anchor and a well-formed ✗/✓ pair. Generic
advice is the characteristic failure of model-distilled rules, and it is precisely what
SkillLens measures — so the gap sits exactly on canonizer's stated reason to exist.

- [x] **Add `verify.Specificity` over `skillet/skilllens` — advisory, never blocking.** DONE (2026-08-09).
      Flag any enforced rule whose text is softening-only or names no domain object, tool
      or API. `finding.SeverityWarning`, so `gate.Blocking` ignores it and
      `TestDecideBlockingNeverShips` stays exactly as true as it is now: `Executable` and
      `Provenance` remain the only things that stop a ship. A general rule is sometimes
      correct, and a deterministic check cannot tell which — so this reports and does not
      decide. Blocked on the skillet promotion (see that TODO).
- [x] **Put the three dimensions in the cold-critic prompt (`internal/prompt/critic_prompt.md`).** DONE (2026-08-09).
      The better fit of the two, and it needs no new machinery. The dimensions are a
      *judgment* rubric, and canonizer's whole architecture routes judgment to a fresh
      grader while keeping deterministic decisions in code — so give the critic the three
      questions and the anti-example for each ("handle errors carefully", "decompose into
      smaller steps", "be careful with dangerous operations"). This extends the existing
      `vague` category from a bare label into a stated test, at zero carrying cost. Feed
      the result through `internal/budget` like any other finding so a rule failing on
      specificity consumes rework budget.
      Fits the P1 posture unchanged: canonizer emits, an agent runs it, the deterministic
      gate decides.
      Landed as three numbered questions under `vague`, each with its anti-example, plus
      the note that a rule can be well written, well sourced and still fail all three.
      **No fourth category was added** — the output contract lists three and the severity
      rule keys on them by name, and `internal/critic` validates nothing, so a new category
      would fail silently downstream.
      Tested on the **filled** prompt rather than the file, which caught a real defect: one
      anti-example had been wrapped across a line break, so the phrase never reached the
      grader intact. Reflowed.
- Note: canonizer needs no `Config`/weights work. The other tools turn these dimensions
      into weighted 1-10 scores; canonizer's gate is findings-based by design (see
      "Rubric scores are relative, not absolute" above), so the dimensions arrive as
      diagnostics and a prompt, never as a number that could become a ship threshold.
- Note (2026-08-09): a cross-repo survey is adding a **derived applicability predicate** to
  `skilllens` so adh and skillsaw can tighten their failure-mechanism dimensions without
  docking documents that legitimately encode no failure mechanism (see
  `../skillet/TODO.md`). **`verify.Specificity` is deliberately not a consumer of it, and
  should stay advisory.** Its false positives are irreducible rather than categorical:
  "prefer composition over inheritance" names nothing concrete and is a perfectly good
  rule, and no derived predicate separates that from a vague one — which is the reason the
  entry above chose advisory severity in the first place. Adding a gate here would be
  machinery that cannot be right. Recorded so the resemblance to the skillsaw/adh work does
  not get mistaken for a shared fix at the next survey.

______________________________________________________________________

## Reasoning-Toolkit Survey (Unified-Thinking, 2026-08-05)

Source: a survey of `~/Documents/git/unified-thinking` (a deterministic Go reasoning
toolkit). Modest relevance — canonizer's ship gate is findings-based, not score-based —
so this is mostly inspiration.

- [x] Track the **cold critic's confidence vs. actual rule quality** — done: `skillet`
  bumped to v0.7.0 (which ships `calibration`) and the `calibrate` command reports
  ECE/MCE/Brier + a per-bin breakdown from a review log
  (`{"samples":[{"confidence":..,"correct":..}]}`) via `calibration.Compute`. It is a
  **report, never a gate**: the ship gate stays findings-based, so calibration never
  blocks adoption (consistent with the self-score invariant `budget` enforces). Sourcing
  the log is the operator's process — no confidence field was added to
  `finding.Diagnostic` and the ship path is untouched. An empty/all-out-of-range log
  says so rather than printing a misleading `ECE 0.000`.
- Inspiration (not a lift): unified-thinking's deterministic **hypothesis-ranking** formula
  (`explanatory·0.4 + parsimony·0.3 + prior·0.3`, with Occam parsimony ≈ `1/(1+#assumptions)`)
  is an apt shape *if* canonizer ever scores/ranks candidate rules rather than only gating
  them; and its fallacy / argument-structure taxonomy could sharpen the cold-critic *prompt*
  (never the deterministic gate).

______________________________________________________________________

*Recorded 2026-08-04. Sources: this repository's state, and
`~/Documents/agent-orange/go-advice/ai_skill_todo.md` (rigor backlog + the
"Centralize on skillet" offload analysis). See `PLAN.md` for the implementation
plan and its design review against `summary_rules.md`.*
