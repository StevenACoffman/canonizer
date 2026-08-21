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

## Agent-Red Survey (2026-08-15)

Source: a survey of `~/Documents/agent-red` (26 agent-tooling projects) driven by the
knowledge-base ingestion work. Claims below were checked against the code in both
repositories.

- [x] **`normalize` disagrees with exegesis about what "present in the source" means.** DONE
  2026-08-15 on skillet v0.16.0. `verify` calls `textnorm.Fold` and the local `normalize` is
  deleted. The accepted set grew, as predicted: curly apostrophes, curly doubles and em
  dashes all emitted `anchor-absent` before and do not now — verified by reverting, not
  assumed. `TestFoldingOnlyWidensAcceptance` pins the direction, because folding strictly
  widens what matches and a rule that *stopped* being accepted would mean the normalization
  changed meaning rather than reach.
  Original entry:
  `internal/verify/verify.go:144` is `strings.Join(strings.Fields(s), " ")` — whitespace
  folding only. `exegesis/internal/textnorm.Fold` folds whitespace runs **and** typographic
  characters (curly quotes, en/em dashes, non-breaking and zero-width spaces) before the
  same comparison, precisely because a book, a plain-text extraction, and a Markdown file
  each spell those differently — as that package puts it, a guard that fired on every curly
  apostrophe would not get run. **Consequence today:** a rule whose `↦` anchor was copied
  from a source containing a curly apostrophe emits `anchor-absent` here and blocks, while
  the identical passage passes `exegesis quotecheck`. Two tools in one family answering one
  question differently is the drift skillet exists to prevent. `textnorm` has two callers in
  exegesis (`quotecheck`, `a2check`) and canonizer is the third — recorded as a promotion
  candidate in skillet's TODO under *Contradiction Detection*. Adopt it here when it lands;
  expect the set of accepted anchors to grow, which is the point.
- [ ] **`anchor-absent` conflates a fabrication with a drift, and they warrant opposite
  responses.** `Provenance` emits one category when `r.SourceAnchor` is not found in the
  haystack, whether the anchor was **invented by the model** (a real defect — block, always)
  or the **source moved under it** (a new edition, a reformat, a re-exported PDF — where the
  rule may be entirely sound and only its anchor needs refreshing). Both block identically,
  so the response to a routine source update is indistinguishable from the response to a
  hallucination. `llmwiki` makes the same conflation from the other end (`evidence_invalid`
  on `promote`), and for the same underlying reason: neither tool retains the source bytes
  the anchor was validated against, so "the quote is wrong" and "the source changed" are not
  separable facts. **Separating them needs the immutable evidence archive** described in
  `agent-red/manifesto.md` — with it, `quote ≠ archived bytes` is corruption (fail hard) and
  `archived bytes ≠ current source` is staleness (flag for re-review, do not block). Until
  then, the cheap half is available now: record the source's content hash beside the ruleset
  at distill time, so a later `anchor-absent` can at least *report* whether the source has
  changed since — a different message, not yet a different verdict.
  - [ ] **It is a three-way split, not two.** `gnosis`
    (`~/Documents/git/gnosis/SPEC.md` §4.2–§4.3) has now settled the archive design, and it
    is text-only with **deliberately no PDF extractor** — so a source that cannot be archived
    is admitted as `referenced`: hash and URI recorded, no local text retained. That is a
    third state and it is the one canonizer will hit first, because a rule distilled from a
    book or a PDF standard is the normal case here:

    | State            | Condition                               | Verdict                  |
    | ---------------- | --------------------------------------- | ------------------------ |
    | fabricated       | anchor ∉ archived text                  | block, always            |
    | drifted          | archived text ≠ current source          | flag stale, do not block |
    | **unverifiable** | no archived text exists for this source | **report, never block**  |

    Collapsing `unverifiable` into `fabricated` would block every rule drawn from a PDF —
    which is most of `go-advice`. Collapsing it into a pass would let a fabricated anchor
    through whenever the source happens to be unarchivable, which is the more dangerous
    error. It has to be its own state, carried on the finding, and paired with the
    already-landed `finding.Action` (`human`) so a reader knows the next move is to find a
    quotable source rather than to rewrite the rule.
    Note the shared prerequisite: this is the same not-applicable outcome recorded against
    the `quotecheck` promotion in skillet's TODO. One state, two consumers — which is what
    makes it skillet's to define rather than either tool's to invent.
- [x] **Findings say what is wrong, not who acts.** DONE 2026-08-15: `finding.Action` landed
  in skillet v0.16.0 and every diagnostic here carries one. `diag` is `human` and `advisory`
  is `guided`; **nothing canonizer emits is `automatic`**, because every category needs
  someone who knows what the source says — an unexecutable rule needs rewriting, an absent
  anchor needs deciding whether the source moved or the rule was fabricated.
  **No severity changed**, which this entry required: `gate` and `budget` are untouched and
  `TestDecideBlockingNeverShips` is the same test, verified by diff rather than by it still
  passing. `budget.Decide` deliberately still takes a bool — making rework budget depend on
  `Action` is a policy change with its own before/after.
  Original entry: A `finding.Diagnostic` is
  `{severity, category, path, message}`; severity says whether it blocks. `AgentLint` carries
  `fix_type` per check (`guided` — the tool proposes and a human confirms; `assisted` — the
  tool can generate the fix), stored as data in `standards/evidence.json` alongside the
  evidence for the check itself. Relevant here because `loop` and `budget` govern rework
  rounds: a rework budget spent on findings a human must adjudicate is not the same
  expenditure as one spent on findings the agent can close, and today the two are
  indistinguishable to the loop. A fixed classification per check, no new measurement, and
  **not a severity change** — `Specificity` stays advisory by construction.
- [x] **Contradiction detection lands here first — canonizer is consumer #1.** DONE
  2026-08-15. `verify.Conflicts` wraps `skillet/ruleset/conflict`, which returns diagnostics
  with **no severity** precisely so the policy is made here: warning, for the same reason
  `Specificity` is advisory — a severity divergence may be a deliberate refinement and a
  deterministic check cannot tell. Proven non-blocking by running `gate` over the result
  rather than by reading the constant.
  The sequencing this entry called for held: `textnorm` first, so the conflict checker and
  `Provenance` fold identically inside one binary.
  **Found while wiring: `verify.Specificity` was never called.** Built 2026-08-09, tested,
  and absent from `cmd/verify`, which ran `Executable` and `Provenance` only — canonizer had
  been shipping a check nobody ran. Wired in the same pass.
  Original entry: A ruleset's
  entire claim is that its rules are *internally consistent*, and nothing checks it.
  `verify` establishes that each rule is executable and anchored; two rules can both pass
  and still contradict each other. The shared half is recorded in skillet's TODO as
  `ruleset/conflict`: three predicates exactly decidable over the canonical form today —
  severity divergence, level divergence, and `§`-identity collision after a merge — emitting
  `finding.Diagnostic`, **never a score**, since a "contradiction score" is exactly the ship
  threshold this repo refuses to have. The residue (genuine semantic conflict between two
  prose rules) routes to the existing cold critic, which already sees the source and the
  ruleset but not the reasoning that produced them — no new machinery needed for it.
  Note the sequencing: `conflict` compares normalized rule text, so it depends on the
  `textnorm` item above being settled first, or it will inherit the same disagreement.
- [ ] **Adjudication records have no home and fail `Provenance` by construction.** When two
  rules conflict and a person decides, the decision is knowledge present in neither source,
  so it can carry no `↦` anchor — and `Provenance` will block it as `no-anchor`. That is
  the highest-value artifact the team produces, rejected by the check that exists to
  protect quality. Shape when it is time: a supersession edge plus a human warrant (who,
  when, which review) beside `SourceAnchor`, so an adjudicated rule is *sourced differently*
  rather than *unsourced*, and `enforced(r.Severity)` gates on the warrant's presence
  instead of the anchor's. Held in skillet's TODO until a second consumer wants it; recorded
  here because canonizer is where the false rejection will actually fire.

## Agent-Blue Survey (2026-08-15)

Source: a survey of `~/Documents/agent-blue` (22 projects — the sources the practice came
from). Checked against the code in both repositories.

- [ ] **Nothing asserts a stored ruleset is in canonical form.** `verify` parses
  (`ruleset.Parse`) and never renders: a grep for `Render` across this repo returns no
  non-test hit, and no command carries a `--check` flag. So a ruleset an agent wrote can be
  *parseable* while `Render(Parse(x)) != x` — reordered, reformatted, or quietly dropping a
  field `Parse` tolerates and `Render` does not reproduce. The locked design decision
  (*Ruleset Parsing & Verification*, above) is that we emit and consume the canonical form
  precisely so structured loading is deterministic; nothing enforces that today.
  **This is load-bearing for contradiction detection**, not cosmetic: `ruleset/conflict`
  compares normalized rule text, so a non-canonical ruleset makes two identical rules look
  different, or two different rules look identical, before any comparison runs.
  The pattern to copy is one flag: `agent-blue/modelith`'s
  `render --check` — "verify the committed output is up to date; non-zero exit on drift"
  (`cmd/modelith/main.go:461`), mutually exclusive with `--stdout`, with CI regenerating and
  failing on drift like a generated-code check. **exegesis already has this shape** —
  `cmd/index/index.go:23` and `cmd/normalize/normalize.go:22` both carry `Check bool` — so
  this is family convergence, not a new idea. Emit as a `finding.Diagnostic` so `gate` can
  block on it like every other check.
  **Sequenced behind the format version decided in skillet 2026-08-15.** A round-trip check
  is only meaningful *within* a known grammar: `Render(Parse(x)) != x` on a file written by a
  newer version would report drift where the real answer is "this is v2 and I read v1". So
  the check must refuse an unknown major before it compares, not after. Ship the version
  reader first, then this.
  **canonizer is the whole migration.** The `ruleset` canonical form has exactly two
  consumers — this repo and skillet itself; exegesis, skillsaw and adh have zero references —
  and roughly ten stored files exist, most of them 1-4 rule prompt examples. So a breaking
  format change is a bump here rather than a family-wide event, which is why skillet chose to
  do it now rather than defer it again.
- [ ] **The cold critic reports what it found, never what it did not look at.**
  `critic_prompt.md` asks for `unsupported` / `vague` / `duplicate` findings; an empty
  category is therefore indistinguishable from an uninspected one, and `gate` ships on that
  silence. `agent-blue/super-hermes` closes exactly this hole:
  `skills/prism-scan/SKILL.md:57` appends a **constraint footer** — "This analysis
  maximized X. It did not examine: [1-2 specific alternative angles]" — and `prism-reflect`
  persists a fuller constraint report that **later runs read to steer their lens away from
  angles already exhausted**.
  **It does not compromise the cold split**, which is the reason it is admissible here: a
  coverage record says *what was looked at*, never *what was concluded* or how the ruleset
  was produced. It is categorically unlike the distillation, which `critic` withholds by
  design. Shape: an additive `examined` / `not_examined` block beside the findings array,
  advisory only — a critic that declares a gap must not thereby block, or it will learn to
  declare none.
- [ ] **One cold critic is one opinion.** `agent-blue/evals-differential-oracle` is the
  source of adh's `oracle` self-test, and its thesis applies verbatim to grading:
  *agreement between two independently-built systems is a far stronger signal than either
  passing its own tests*. canonizer runs a single critic prompt; a second grader given the
  same source and ruleset but a different prompt (or a different model, per the model-gate
  convention) would make disagreement visible instead of invisible. Its second net is worth
  noting separately: `src/invariants.py` — "the rules any correct implementation must obey,
  **checked against a board independently of how the result was produced**" — is the
  deterministic complement, and `verify.Executable` / `verify.Provenance` already are that
  net for rulesets. So the gap is only the *second opinion*, not the invariant tier.
  Already absorbed and worth not re-deriving: `gate.SelfTest`'s planted-defect negative
  control is that repo's `impl_buggy.py` idea, and mirrors adh's `oracle selftest`.
- [ ] **OKF states our thesis as two data fields.**
  `agent-blue/knowledge-catalog/okf/SPEC.md` §5.2 keeps `generated` and `verified` separate
  "because who *wrote* a concept need not be who *confirmed* it" — which is this repo's
  entire reason to exist, expressed as frontmatter rather than as a pipeline stage. Two
  details are directly usable if rulesets ever carry provenance metadata: `verified` is a
  **list** of independent verification events, each `{by, at}`, so a human sign-off and an
  automated pass are recorded as distinct checks rather than collapsed; and `verified` is
  **independent of `generated.at`**, so "content changed without re-confirmation" and
  "re-confirmed without regeneration" are separately representable. That is the vocabulary
  the `anchor-absent` split above needs, and adopting it would mean not inventing a third.
  **Evaluated in skillet 2026-08-17 and deliberately not promoted there.** All three repos
  referencing OKF want it conditionally — this entry says *"if rulesets ever carry provenance
  metadata"* — and skillet already carries one speculative extraction with zero importers
  (`provenance`) as the precedent for what that produces. **So when this lands it lands here,
  local to canonizer, until a second consumer appears** — the same call that kept `quotecheck`
  in exegesis.
  This is the nearest of the three to real, because the `anchor-absent` split above is a
  *present* defect rather than a conditional want, and §5.2's independence of `verified` from
  `generated.at` is exactly the distinction it needs: *content changed without
  re-confirmation* versus *re-confirmed without regeneration*.
- [ ] **Say what a score is not allowed to claim.**
  `agent-blue/cc-thinking-skills/analysis/AUDIT.md` pins its evidence file *and* the
  registry it references by SHA-256, then states "If this narrative disagrees with the JSON,
  **the JSON wins**", carries a global disposition of `no_automatic_elevation`, and
  enumerates the claims not authorized: "No public 'proven / validated / improves /
  eval-informed / auto-invoke' claim is authorized. Inferences are labeled." Its published
  headline is that **zero of 28 skills hold a replicated ELEVATE verdict**. This repo already
  refuses to emit a weighted score for exactly this reason; the remaining exposure is prose —
  a `verify` pass with no blocking findings is easy to describe as "verified" when it means
  "no deterministic check objected and one critic agreed". Cheap and entirely documentation:
  state in the README what a clean `gate` does and does not license anyone to say.
- Deliberately NOT adopted: `PolyBrain`'s multi-model orchestration (its "verified claims"
  output contract is the right instinct, but canonizer never calls a model — a second
  grader is a second *prompt*, which is the item above, not an orchestration layer);
  `NFRLocator`'s classifier (statistical, no per-decision provenance);
  `hermes-skill-factory` and `hermes-dojo` (producer- and optimizer-side, no ruleset
  surface).

## Agent-Fuschia Survey (2026-08-18)

Source: a survey of `~/Documents/agent-fuschia` (26 repositories). canonizer takes the most
from it, because `vac-protocol` independently arrived at this repo's central distinction.

- [ ] **`verify` and `critic` are VAC's structural/semantic split, and the output should
  say so.** `agent-fuschia/vac-protocol` §4 calls them "two distinct acts, **never to be
  conflated**": *structural verification* is zero-network, zero-issuer-code — schema valid,
  artifacts hash-identical, closure, limitations stated, every declared number recomputed
  from the artifacts; *semantic replay* clones the issuer at a pinned commit and re-earns
  the verdicts. That is exactly `verify` (Executable, Provenance, deterministic, local) and
  `critic` (a fresh grader's judgment) — arrived at independently, which is the strongest
  evidence the split is right.
  The part we do not do: **"the structural verifier never performs it and says so in its
  output."** A clean `canonizer verify` says nothing about whether the rules are supported;
  a reader or a driver script can easily take it for one. The line to internalize is
  **"a structural PASS means the bundle is *internally honest*, not that the issuer's grader
  agrees."** Emit the fact — `semantic_verification: "not-performed"` in the findings
  result, and a sentence in the human output — pairing with `vac-gate`'s rule that "every
  PASS states what ran and what deliberately did not."
  And the case worth designing for, which the taxonomy currently has no name for: a ruleset
  that passes `verify` and fails `critic` "is a precise, reproducible accusation" — not a
  malformed artifact but a well-formed wrong one.
- [ ] **A ruleset that will not say what it does not cover is an advertisement.** VAC makes
  `claim.limitations` **REQUIRED and non-empty**; a bundle without explicit non-claims is
  invalid (`empty-limitations`), because "a capability statement that will not say what it
  does not cover is an advertisement, and VAC does not carry advertisements". A distilled
  ruleset has exactly this failure mode — `go-advice` rules drawn from one book, presented
  without the scope of that book. This is the same mechanism as the constraint-footer item
  already filed above, but stronger: rather than asking the critic to declare coverage,
  make the *artifact* unable to be well-formed without a stated scope and stated limits.
  Candidate: `ruleset.Ruleset` already carries `Source:` and `Scope:` header lines; a
  `Limitations:` sibling, checked non-empty by `verify`, is a small change with a large
  honesty return.
- [ ] **The critic's categories are missing coverage.** `critic_prompt.md` asks for
  `unsupported` / `vague` / `duplicate` — all properties of an individual rule. VAC §6's
  challenge protocol has three classes and the middle one has no analogue here:
  **coverage** — "the evidence does not support the stated `capability`/`scope`", e.g. "the
  task set is narrower than the scope sentence implies". Applied to a ruleset: the rules may
  each be supported and the set may still not cover what the `Scope:` line claims. That is a
  property of the *set*, invisible to any per-rule category, and it is the failure a reader
  is most likely to be harmed by. Pairs with the limitations item — a stated scope is what
  makes a coverage challenge decidable at all.
- [ ] **Named reasons, from a closed vocabulary.** VAC enumerates nineteen structural
  failure reasons and emits exactly one per failure. `internal/verify` currently produces
  `no-anchor`, `anchor-absent`, and the Executable categories as string literals into
  `finding.Category`, which `skillet` types as a bare `string`. Define canonizer's set as
  constants in one place so the vocabulary is enumerable and a typo is a compile error;
  recorded against skillet as the shared question of whether `Category` should be
  registrable rather than free.
- Note on the three-way `anchor-absent` split filed above: `vac-gate`'s **"'cannot regrade'
  is not 'regraded'"** is the same rule from another direction — an honest "I could not
  check this" must fail closed as its own state, never pass as a check that ran. Three
  consumers now want that state (here, `quotecheck`, adh's `eval`), which is what makes it
  skillet's to define.
- Deliberately NOT adopted: `vac-protocol`'s bundle format itself (its claims are about
  system capability, ours about rules drawn from a source — the discipline transfers, the
  schema does not); `evalmut` (mutation testing for evals is skillsaw's axis — canonizer's
  `gate.SelfTest` planted-defect control is the piece of that idea this repo needs, and it
  already exists); `claim-segmenter-kit` (a `§` rule statement can carry two assertions just
  as a wiki sentence can, so this may become canonizer's problem too — but it is gnosis's
  first, and a second consumer is what would move it into skillet).

______________________________________________________________________

*Recorded 2026-08-04. Sources: this repository's state, and
`~/Documents/agent-orange/go-advice/ai_skill_todo.md` (rigor backlog + the
"Centralize on skillet" offload analysis). See `PLAN.md` for the implementation
plan and its design review against `summary_rules.md`.*
