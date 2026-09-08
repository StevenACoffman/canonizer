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
- **`verify`** — the deterministic floor over a parsed ruleset. Blocking: `Executable`
  (every enforced rule carries a discriminating ✗/✓ pair), `Provenance` / `Drift` (its
  anchor is in the source), `Canonical` (the stored form round-trips through
  `ruleset.Render`). Advisory: `Specificity`, `Conflicts`, and the scope report. With
  `--against-proof` an absent anchor splits four ways instead of one — see the anchor
  entries below. With `--proof` it writes a packet binding ruleset and source bytes.
- **`gate`** — reads a critic reply via `critic.ParseReply` (which rejects a reply whose
  coverage record is malformed rather than draining it), blocks on error-severity findings,
  and renders the critic's `unexamined` record after the decision so it can never affect it.
  Self-tests with a planted defect on every invocation.
- Lint-clean (`golangci-lint`), structurally clean (`climax lint`), tested (pure
  `synth` core + end-to-end command tests through `cmd.Run`).

**State as of 2026-09-05.** Pinned to `skillet v0.27.0`; `golangci-lint run ./...` reports
0 issues and the suite passes. Entries written before that pin may reason about an older
kernel — three did, and were found stale rather than blocked when checked against the code
(`CategorySoftening`, the adjudication warrant, and the canonical-form version reader). **An
entry citing a skillet version below v0.27.0 is worth re-checking before acting on it.**

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
- [x] **`anchor-absent` conflates a fabrication with a drift, and they warrant opposite
  responses.** RESOLVED 2026-09-05 by the entry below, not by this one's own plan.
  **This entry concluded the split "needs the immutable evidence archive" and offered only a
  cheap half in the meantime. That was wrong, and the `ruflo` entry shows why**: crossing two
  signals — the anchor and the source's digest — gets the *full* split with no archive at
  all. The archive would answer a further question (what did the source say *then*), but
  separating a fabrication from a drift never needed it.
  Kept for the reasoning and for the three-way sub-item, which is about gnosis's archive
  design and is untouched by this. Original entry:
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
  - [x] **It is a three-way split, not two — and `Drift` now implements two of the three.**
    DONE 2026-09-06 as `verify.Unverifiable`, advisory, reported when no `--source` is
    supplied.
    **The 2026-09-05 update above framed this as a decision about whether `SourceUnknown`
    should block. That was the wrong question, and reading this entry to its end answers
    it.** `SourceUnknown` means no *proof packet* while the source is readable, so an absent
    anchor is a real finding and still blocks — unchanged. `unverifiable` is the case where
    there is nothing to search, and it was not covered at all.
    **The gap was live and worse than filed.** With no `--source`, `cmd/verify` ran no
    provenance check and emitted nothing, so a ruleset whose rules all cite anchors produced
    output byte-identical to one where every anchor was found — the entry's own warning that
    *"`unverifiable` must not be spelled as an absent value"*, already true. And the summary
    line added the day before asserted *"a pass means this ruleset is well-formed **and its
    anchors are present**"*, which was false on exactly those runs. Both are fixed; the
    clause is now conditional on a source having been supplied.
    **The vocabulary is skillet's, not a third local tri-state.** `quotecheck.Status` is
    reused, and its zero value is pinned by a test rather than by a comment — a comment
    saying "Unchecked must stay the zero value" is what a refactor deletes.
    One diagnostic for the ruleset rather than one per rule: the repair is a single action,
    so N lines would repeat one instruction, and the count of unverified anchors is in the
    message. Original entry: `gnosis`
    **UPDATED 2026-09-05.** `verify.Drift` ships `SourceUnchanged` and `SourceChanged`,
    which are this table's `fabricated` and `drifted`. The third state is the one still
    open, and building the other two made the disagreement precise rather than resolving it:
    **`SourceUnknown` currently falls back to `Provenance`, which blocks.** This table says
    `unverifiable` must *report, never block*. Both readings are defensible and they are not
    the same claim — `SourceUnknown` means *no proof packet was supplied* while the current
    source is still readable, whereas `unverifiable` means *no archived text exists at all*.
    An anchor absent from a readable current source is a stronger signal than an anchor with
    nothing to check against.
    So the open question is narrower than the entry states: **does an absent anchor with no
    second signal block?** It does today, which preserves the pre-2026-09-05 verdict and is
    the conservative choice; the argument against is this entry's, that it blocks every rule
    drawn from a PDF. Deciding it needs the archive distinction, not just the flag.
    Original entry: `gnosis`
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
    **The prerequisite is met — `quotecheck` shipped it in `skillet` v0.18.0 and this entry
    was not updated.** `quotecheck/status.go` carries `Status` with **`Unchecked` as the
    zero value**, `locate` returns it when there are no haystacks, and `Finding.Missing()`
    is deliberately false for an `Unchecked` finding so a caller gating on it asks *"did the
    check find this absent"* rather than *"did the check pass"*. That is the `unverifiable`
    row above, already defined, already shared, and already carrying the fail-safe default
    this entry argued for. The remaining work is canonizer's alone: map `Provenance`'s
    output onto the three states and stop emitting one category for all of them.
    Worth noting the direction the zero value points, because it is the opposite of what a
    naive port would do. `Unchecked` being the zero value means a `Finding` that nothing
    populated reads as *not checked*, never as *checked and clean* — so a caller that
    forgets to run the guard fails closed. Preserve that when mapping: `unverifiable` must
    not be spelled as an absent value that a later refactor can silently turn into a pass.
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
- [x] **Adjudication records have no home and fail `Provenance` by construction.** DONE
  2026-09-05. The entry's own scoping held exactly: *"the kernel carries the datum because
  `ruleset` is skillet's type; the decision stays here."* skillet v0.27.0 shipped
  `ruleset.Rule.Warrant{By, At, Rationale}` with `Present()` and `Valid()`; what landed here
  is the policy -- **an enforced rule with no anchor but a valid warrant is sourced
  differently, not unsourced**, and `Provenance` gates on the warrant where the anchor is
  absent.
  **An invalid warrant is reported, not accepted**, as `warrant-incomplete`. Without that the
  warrant becomes a way to opt out of provenance entirely: a rule could carry an empty
  marker and pass. skillet keeps `Present` and `Valid` as separate questions precisely so
  both can be asked, and both are.
  **`Provenance` and `Drift` route through one `unanchored()`**, so the two cannot disagree
  about the policy -- the same reason `anchorPresent` was factored out when `Drift` landed.
  The smaller-than-gnosis scoping was kept: presence plus validity, no tiers, no co-signers,
  no reversal links, because importing a permission model would be adopting the position
  this repo declined.
  **The false rejection is still not live**, as the entry says: a ruleset carrying a warrant
  renders as format 2 and none exist yet. So this is enabling work, and the test is the
  artifact. Original entry: When two
  rules conflict and a person decides, the decision is knowledge present in neither source,
  so it can carry no `↦` anchor — and `Provenance` will block it as `no-anchor`. That is
  the highest-value artifact the team produces, rejected by the check that exists to
  protect quality. Shape when it is time: a supersession edge plus a human warrant (who,
  when, which review) beside `SourceAnchor`, so an adjudicated rule is *sourced differently*
  rather than *unsourced*, and `enforced(r.Severity)` gates on the warrant's presence
  instead of the anchor's. Held in skillet's TODO until a second consumer wants it; recorded
  here because canonizer is where the false rejection will actually fire.
  **REVIEWED 2026-08-22. Still held, for a different and better reason, and the shape is
  now settled.**
  The hold was "until a second consumer wants it". There are three specifications — this
  one, skillet's, and gnosis's SPEC, which uses the same sentence — so that reason has
  expired. **The real blocker is that `ruleset.Rule` cannot safely gain an optional field
  yet**: a per-rule warrant is a marker line, not frontmatter, and skillet's canonical-form
  entry records that an unknown marker is folded into `Rationale` rather than rejected until
  a format version ships. This is that entry's *second* asker.
  **UNBLOCKED 2026-08-23 by `skillet` v0.19.0.** `ruleset.Parse` now refuses a body line
  opening with an unrecognised symbol, and the format header was already refusing a newer
  `format:` — so the canonical form can gain an optional marker safely and this entry's
  stated blocker is gone. Two things carry over into the work rather than being resolved by
  it. The rejection is on a leading **Unicode symbol**, so a warrant marker must be one
  (`⊢` or similar) and not an ASCII prefix, which the new rule cannot see. And adding a
  marker means bumping `FormatVersion`: skillet's `TestEveryMarkerIsNonASCII` counts the
  table, so forgetting is loud, but the bump is still the author's to make.
  Also worth stating plainly: **the false rejection is not live and cannot be.** `Provenance`
  does reject `SourceAnchor == ""` as `no-anchor`, but an adjudicated rule cannot be
  expressed today, so there is nothing for it to reject. The entry is correctly written in
  future tense and should stay that way rather than reading as a present defect.
  **When it unblocks, canonizer gets a smaller warrant than gnosis's, deliberately.**
  skillet will carry `{By, At, Rationale}` on `ruleset.Rule` and nothing more — no tiers, no
  co-signers, no reversal links. Those are gnosis's §10.6 authority model, which **this repo
  explicitly bet against**: §10.6.4 holds that a required rationale filters more bad
  adjudications than a permission check, and importing a tier model would be adopting the
  position canonizer declined. Two warrants with different obligations is the right outcome,
  the same way `Unexamined` and `limitations` are two fields that look alike and are
  opposites.
  What canonizer owns is the policy: gate `Provenance` on *warrant present* where the anchor
  is absent, so an adjudicated rule is sourced differently rather than blocked. The kernel
  carries the datum because `ruleset` is skillet's type; the decision stays here.

## Agent-Blue Survey (2026-08-15)

Source: a survey of `~/Documents/agent-blue` (22 projects — the sources the practice came
from). Checked against the code in both repositories.

- [x] **Nothing asserts a stored ruleset is in canonical form.** DONE 2026-09-05 as
  `verify.Canonical`, blocking, wired into `verify` — `Render(Parse(raw)) != raw` is now a
  finding.
  **This entry's stated sequencing was satisfied and its stated hazard was not real.** It
  sequenced the check behind a format-version reader, reasoning that re-rendering an older
  file would report drift where the honest answer is "this is format N and I write M".
  skillet answers that inside `Render`: `formatOf` returns the ruleset's *own* format and
  `renderFormat` emits no header below 2.
  **A version guard was written, measured, and removed — it made the check vacuous.**
  Guarding on `rs.Format != FormatVersion` declined on every file in the corpus, because no
  stored ruleset declares a version and they all parse as format 1. Measured before removing
  it: a header-less ruleset parses as format 1 and round-trips byte-identically. A check that
  can never fire is the trap this file keeps catching, and it was nearly shipped here as
  care. Original entry: `verify` parses
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
- [x] **The cold critic reports what it found, never what it did not look at.** DONE
  2026-09-05: `critic_prompt.md` gained the constraint footer, and the record is carried in
  `finding.Result.Unexamined` — which skillet already defines, with the advisory-only
  guarantee structural rather than remembered. The prompt asks for gaps named *specifically*
  ("I read the rules against §3 and did not cross-check §7"), because "I may have missed
  things" records nothing. Original entry:
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
- [x] **One cold critic is one opinion.** REJECTED 2026-09-05 — recorded rather than built,
  and the reason is architectural rather than effort.
  A second prompt file is cheap. The **independence** is not, and it is the whole of the
  claim: this entry's own thesis is that *agreement between two independently-built systems*
  is the strong signal. canonizer *"is a prompt-filler… it never calls a model itself"*
  (**Decided — Architecture**), so it can emit two prompts and has no way to learn whether
  they were answered by two graders or twice by one session.
  Shipping the cheap half would claim a property the architecture cannot deliver — the same
  ground the reduced-independence entry above was closed on: a field canonizer cannot
  populate honestly *"reads as evidence of an isolation nobody checked."*
  Worth keeping from the entry: it notes the deterministic complement is **already built** —
  *"`verify.Executable` / `verify.Provenance` already are that net"* — so the gap was only
  ever the second opinion, which is exactly the part that needs the seam canonizer declines
  to add. The trigger that could reopen this is identity on the reply, letting a critic
  answer from the session that produced the distillation be refused; that is a relay feature
  no tool in the family has. Original entry: `agent-blue/evals-differential-oracle` is the
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
- [x] **OKF states our thesis as two data fields.** DONE 2026-09-08 — the record arrives
      from skillet, the slot is the ruleset frontmatter at format 4, and `verify --sign-off`
      writes it. Details below.
  **Skillet decided 2026-09-07: the verification *record* is promoted, the *fold* is not**
  — `Verification{By, At}`, list-valued, with each consumer keeping its own tier derivation.
  The trigger fired on two repos having independently hand-written that same type
  (`bundle.Verification` in gnosis, `contextstore.Verification` in adh) while their folds
  differ by design.
  **So this entry's own conclusion is half-superseded.** It said *"when this lands it lands
  here, local to canonizer, until a second consumer appears"* — the record now arrives from
  skillet instead, so the `anchor-absent` split can use the shared element rather than
  inventing a third copy, which is what this entry was trying to avoid. What stays local is
  any tier derivation canonizer wants.
  **MEASURED 2026-09-07: the type is available and there is nowhere to put it, so this is
  adopt-on-arrival rather than adopt-now.** `verification.Event` is a *record*, and canonizer
  persists nothing that could hold one. Every candidate slot was checked rather than assumed:
  rulesets carry no provenance metadata — this entry's own unmet condition, and adding a
  header is a skillet format bump; `proof.Packet` is `{Arc, Provenance{GitSHA}, Artifacts}`
  with no field for events and is skillet's type to change; `finding.Result` is
  `{Diagnostics, Unexamined}`, and an envelope around it changes the format `gate` reads,
  which was rejected on this same ground for `semantic_verification`. A grep for every write
  canonizer performs finds three — the findings JSON, the proof packet, and prompt files.
  There is no ledger and no state file.
  **A canonizer-local events file was considered and rejected** — it would be building a
  feature to consume a type, and the events would be testimony canonizer cannot check, the
  ground on which the critic's independence flag was already refused.
  **Corrected 2026-09-07 on both halves of that reasoning.** It cited skillet's package doc
  saying `verification` *"has no importers yet"*; that is stale — **gnosis and adh both pin
  v0.31.0 and use `verification.Event` today**, so the risk it invoked does not apply and
  the doc needs rewriting. And the objection about testimony was right about a **flag** and
  wrong to stop there: adh's `RecordVerification` shows the buildable form, taking the actor
  from *"the repository's configured identity, never from a flag on the invocation"*, since
  a caller-supplied actor lets anyone mint a `human:` event. Config-derived is still
  self-asserted, and adh states that limit rather than implying it.
  **Where the events go is settled by precedent, not open.** Both consumers store them in the
  artifact they are about, under a `verified` key — gnosis in document frontmatter, adh
  appended to the unit's own file. canonizer's artifact is the ruleset, so the slot is a
  slot on the ruleset at **format 4**, which is also the only candidate that satisfies this
  entry's own trigger, *"if rulesets ever carry provenance metadata"*. Filed in
  `skillet/TODO.md`; a `proof.Packet` field was the alternative until the precedent was
  checked.
  **Narrowed 2026-09-08: the slot is the ruleset's YAML frontmatter block, not a `Verified:`
  body header.** `Verified` is list-valued while every existing header is one string, so a
  body header would need an invented delimiter grammar; the frontmatter block already exists
  and holds a list natively. It is also what gnosis actually reads — frontmatter, not a body
  header — and it keeps the record out of reach of the agent that authors the rule body.
  **The trigger this entry set is now the thing being built**, so the next state for this
  item is *adopt*, not *wait*: once skillet models `Verified` and canonizer pins that
  release, `unanchored`'s record and the `anchor-drift`/`anchor-stale` gap below both have a
  place to read from. Reasoning and the measured costs live in `skillet/TODO.md` under "No
  artifact carries verification events" — **one authoritative location**, referenced rather
  than restated, because this is a kernel decision that canonizer consumes.
  **DONE 2026-09-08 against skillet v0.33.0: `verify --sign-off`.** `internal/signoff` holds
  the policy; `cmd/verify` is the shell that loads the actor, asks, and writes.
  **The actor is config-derived, and two cheaper sources were rejected on the policy's own
  grounds.** An environment variable is per-invocation and caller-supplied — a flag with
  worse discoverability — so it fails for the reason the rule exists. Git's `user.email`
  carries no *class*, so it would have to be guessed into one, and guessing `human` is
  exactly the guess adh names as letting an automated runner mint a person's sign-off; it
  would also add the version-control dependency `proof.Create` avoids by taking the SHA from
  its caller. What ships is `identity.actor` in a YAML file, validated `<class>:<name>` with
  no defaulting, read from `--config` (default `.canonizer.yaml`).
  **`--config` is a flag after all, and the repository's own lint rule decided it.** The plan
  rejected one on §4 — a caller does not know a better default — and reached for an injected
  `getenv` like adh's. `forbidigo` forbids `t.Setenv` here (*"pass environment via a getenv
  parameter instead"*) and `cmd.Run` has no getenv seam, so an env var would have been
  untestable through the dispatcher a user actually uses. A flag names the *file*, not the
  actor, so the trust argument is untouched.
  **Three refusals, and the third was not in the plan.** Blocking findings refuse, because a
  sign-off would *"attest to a state this run disproved"* — adh's words, and only the
  definition of condemned differs (a blocking diagnostic here, drift there). An unconfigured
  actor refuses rather than recording an anonymous event. And **`--sign-off` requires
  `--source`**: measured, withholding it drops the blocking count on the stored corpus from
  17 to 5, 15 to 3 and 8 to 2, because the anchor checks are replaced by advisory
  `Unverifiable` — so a caller could get *closer* to a signable run by supplying less
  evidence. That incentive ran backwards and is now closed, which is the same fail-closed
  rule this family keeps arriving at: not checked must not read as clean.
  **A fourth refusal was planned and turned out to be unnecessary.** "Refuse a non-canonical
  ruleset" was there because writing means `Parse` → append → `Render`, which rebuilds the
  document and would silently reformat a non-canonical body — five of eight stored rulesets
  are non-canonical. But `Canonical` already emits `non-canonical` at `severity=error`, so
  the blocking refusal covers it. What was a rule became an **invariant with a test**:
  `Render` only ever runs on a document that already round-trips, so a sign-off cannot
  reformat anything.
  **Measured on the corpus: all eight refuse and none was written.** Every stored ruleset
  carries 3–17 blocking findings. So this ships a write path with no subject today, and the
  entry says so rather than leaving a reader to conclude it is broken.
  **Atomic, unlike this command's other three writes.** Those create new files, where a
  partial write costs a re-run; this one replaces a document a human owns and cannot
  regenerate, so it uses `atomicfile.WriteFile`.
  **Still attributable rather than authenticated**, and stated in `--help` and the README
  rather than implied: it says which actor the checkout was configured as, not who was at
  the keyboard.
  **What `Event` buys when a slot exists**, recorded so the next reader need not re-derive
  it: `anchor-drift` and `anchor-stale` both say *the source changed* and neither can say
  whether anyone re-confirmed the anchors since. That is §5.2's independence of `verified`
  from `generated.at`, and it is the one distinction the four-verdict split still cannot
  draw.
  **The `Warrant`/`Event` overlap noticed here is answered and closed in skillet**: they stay
  unrelated. A warrant *substitutes* for evidence — `unanchored` reads it exactly where
  `SourceAnchor` is empty, and its `Rationale` is the only reviewable content there is — while
  an event *attests to* evidence that already stands. Adding `Rationale` to `Event` would
  break a list that mixes `human:` and `check:` actors; embedding `Event` in `Warrant` would
  imply there was something to verify, which is the case a warrant exists for the absence of.
  What is shared is the raw-string grammar of `By`, and only that.
  **What is still not promoted is the `Generated`/`Verified` block**, and the reason is
  recorded in skillet: YAML cannot round-trip, gnosis re-emits its frontmatter verbatim, so
  a block struct would be decode-only. This entry cites §5.2's independence of `verified`
  from `generated.at` as the vocabulary it needs; that independence is a property of the
  *document*, and reading it still means reading the block yourself.
  Original entry:
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
  **Re-reviewed in skillet 2026-08-22. The decision holds, the trigger changed, and this
  entry is no longer the nearest of the three.** Three things moved, and the sentence above
  claiming primacy is now wrong.
  - **The trigger was replaced.** It was *the first repo that actually stores trust
    metadata*; it is now **the second repo that classifies an actor or derives a trust
    tier**. The reason is that gnosis shipped `gnosis.Actor` — a closed three-kind enum
    (`human:` / `agent:` / `check:`) whose parser rejects `<producer>/<version>` and
    `process:<id>`, two of OKF §7's three forms — **without touching trust metadata at
    all**. A storage trigger could not have fired on that. Storage is not the event;
    classification is, because a mis-classified tier reads as a stronger claim than it is.
  - **Two repos now implement the tier vocabulary and neither derives it as specified.**
    Besides gnosis, `adh`'s `contextstore.Unit.Verified` is a `TrustTier` *string* holding
    `unverified` / `machine-confirmed` / `human-reviewed` directly, serialised under the key
    `verified`, where §5.2 defines that key as a **list of `{by, at}` events**. Same field
    name, same tier names, holding the conclusion instead of the evidence. canonizer is
    therefore third in line rather than nearest, and the useful consequence is that **the
    tier names matching across two hand-written implementations is luck** — if this repo
    ever spells them, spell them from the spec.
  - **Only the fold is promotable, not the record types.** gnosis's `okf` retains
    frontmatter *verbatim* because re-encoding YAML cannot round-trip, so a
    `Generated`/`Verified` struct in skillet would be decode-only. What will move, when a
    second consumer classifies, is §5.3's **fold** — a pure function over a list of actor
    strings, with the contract §7 states outright: *"Consumers that classify trust key off
    the `human:` prefix."* Only `human:` needs recognising; everything else is non-human by
    definition, and an unrecognised actor is never an error and never promotes a tier.
  What survives unchanged is the reason this entry exists: §5.2's independence of `verified`
  from `generated.at` is still the vocabulary the `anchor-absent` split needs, and adopting
  it still beats inventing a third. When it lands it lands **here, local to canonizer**,
  until a second consumer appears — and if what lands is a fold rather than a stored tier,
  that *is* the trigger, so say so in the commit.
- [x] **Say what a score is not allowed to claim.** DONE 2026-09-05 as a README policy
  section enumerating the four claims a clean gate does *not* license — the rules are
  correct, the anchor supports the claim, the set covers its scope, nothing was flagged —
  each with the reason the pipeline cannot support it. `verify` and `gate` now state the
  same thing at the moment it matters (see the entry below).
  **The reason it is written down is that the failure is silent**: a gate that blocks says
  why, and a gate that passes says nothing — so the word chosen for that silence in a commit
  message is where the overclaim enters. The README asks for "passed canonizer's structural
  gate" over "verified". Original entry:
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

- [x] **`verify` and `critic` are VAC's structural/semantic split, and the output should
  say so.** DONE 2026-09-05. `verify` states on every run — passing or failing — that it ran
  structural checks only and did not perform semantic replay; `gate` qualifies its *clean*
  verdict and only that one, since a blocked run is in no danger of being read as a pass.
  **`semantic_verification: "not-performed"` was not added to the findings JSON, and the
  reasoning is the same one that produced `Scope` last pass.** `finding.Unexamined` is the
  obvious home and is the wrong one: skillet documents it as *testimony* — an agent's
  unverifiable claim about its own behaviour — and separates it explicitly from the case
  "where code decided a check did not apply and can say so mechanically", which is this. A
  diagnostic is worse: it would fire on every run, so every clean `verify` would report one
  finding and the count would stop meaning anything.
  So the human half shipped and **the machine-readable half needs a producer field on
  `finding.Result`**, which is skillet's to add — the third item this pass to end there.
  Original entry:
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
- [x] **A ruleset that will not say what it does not cover is an advertisement.** COMPLETED
  2026-09-06 against `skillet v0.30.0`, which shipped the `Limitations:` header this entry
  was waiting on. `verify.Limitations` now reads `rs.Limitations`.
  **The word list is deleted, not demoted to a fallback.** `exclusionWords`,
  `statesAnExclusion` and `notLetter` are gone. Keeping them would have been worse than the
  original guess: a ruleset with an empty `Limitations:` would still pass because its
  `Scope:` happened to contain "only" -- the guess silently overriding the fact it was
  standing in for.
  **The check stays advisory and its justification changed rather than lapsed.** It was
  advisory because the detection was a word list; it is advisory now because no ruleset in
  the corpus carries the header, so blocking would fail every one on the day it ships. That
  reason expires on a migrated corpus, which is the trigger for making it blocking -- and it
  is recorded in the doc comment, because a rule whose stated reason no longer applies is one
  nobody can evaluate later.
  **Deliberately not checked: whether the stated limits are any good.** `Limitations: none`
  satisfies the field and states nothing, and detecting that is the same word list in a new
  place. Presence is what a deterministic check can honestly assert; the rest is the critic's.
  Original entry: PARTLY
  DONE 2026-09-05 as `verify.Limitations`, advisory — and the half that is missing is the
  half this entry actually asked for.
  **The `Limitations:` header cannot be added from here, and it was measured rather than
  assumed.** skillet's parser reads only `Source:` and `Scope:` and `Render` writes only
  those two, so a `Limitations:` line parses without error, is dropped on render, and the
  file then **fails the blocking canonical-form check** that landed earlier this pass. The
  header needs a skillet marker and a `FormatVersion` bump, exactly like the warrant.
  What shipped instead asks the same question of the field that exists: a `Scope:` stating
  only what is covered, with no exclusion, is the advertisement the entry objects to.
  **Advisory for two independent reasons**, both worth keeping when the header lands: the
  detection is a word list and cannot be more than a prompt to look; and every ruleset in
  the corpus would fail on day one, and a gate that turns a whole corpus red teaches people
  to bypass the gate. VAC's `empty-limitations` *invalid* verdict is reachable only once the
  header exists and the corpus has been migrated. Original entry: VAC makes
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
- [x] **The critic's categories are missing coverage.** DONE 2026-09-05: `coverage` is the
  fourth category, documented as a property of the *set* and reported once with
  `"path": "ruleset"`.
  **The test that existed to prevent exactly this change did not fire, and that was the more
  useful find.** `TestCriticPromptKeepsThreeCategories` asserted the substring
  "one of `unsupported`, `vague`, `duplicate`" — which stays true when a fourth is appended.
  It now asserts the whole contract line and checks every named category is also defined
  above it. Original entry: `critic_prompt.md` asks for
  `unsupported` / `vague` / `duplicate` — all properties of an individual rule. VAC §6's
  challenge protocol has three classes and the middle one has no analogue here:
  **coverage** — "the evidence does not support the stated `capability`/`scope`", e.g. "the
  task set is narrower than the scope sentence implies". Applied to a ruleset: the rules may
  each be supported and the set may still not cover what the `Scope:` line claims. That is a
  property of the *set*, invisible to any per-rule category, and it is the failure a reader
  is most likely to be harmed by. Pairs with the limitations item — a stated scope is what
  makes a coverage challenge decidable at all.
- [x] **Named reasons, from a closed vocabulary.** DONE 2026-09-05. `internal/verify` now
  defines its categories as constants in one block, and a new test pins each one's *string*
  because the behavioural tests must keep asserting literals: a test comparing a constant
  against itself passes however the value is edited, so the one change it cannot catch is
  the one that breaks every consumer.
  **The registrable-`Category` question is left open deliberately**, as this entry filed it
  against skillet. Constants get the whole benefit — one place, compile-checked references —
  without canonizer answering a kernel question for the family on its own. Original entry: VAC enumerates nineteen structural
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

## Adopt `skilllens.CategorySoftening` (2026-08-22)

`skillet/skilllens` now exports the category names for its own three detectors, because
**canonizer emitted `softening` while exegesis emitted `skilllens-softening` for the same
`skilllens.SofteningPhrases` call.** One kernel detector, two names — the drift an untyped
`finding.Category` was always going to allow. The full reasoning, including why a closed
enum and a registration seam were both refused, is in `skillet/TODO.md`.

- [x] **Import `skilllens.CategorySoftening`.** DONE — the constant is referenced at
  `internal/verify/verify.go:150`, and go.mod pins **v0.27.0**, nine releases past the
  v0.18.0 this entry was written against. The checkbox was stale rather than the work
  outstanding; corrected 2026-09-05 while auditing the file against the code.
  The line reference in the original title has moved and is left as written, since the entry
  is a record of a decision rather than a pointer. Original entry: waits on a skillet
  release; canonizer pins v0.18.0 and carries no `replace`.
  **The value does not change** — the constant is `"softening"`, which is what canonizer
  emits today. No output moves, `verify_test.go:96`'s `wantCat` stays as it is, and the
  whole change is a literal becoming an identifier. Worth doing anyway: the point is that
  the next person editing either side cannot drift without deleting a reference.
  Two of exegesis's three *do* change value, since it was the one carrying the prefix.
- Recorded rather than actioned: **canonizer's naming was the correct one and the family
  adopted it.** The unprefixed form won because across thirty category values there is not
  one same-word-different-meaning collision, while the only observed defect is one concept
  spelled two ways — so a prefix defends a hazard that never occurred and manufactures the
  one that did. And **canonizer's `no-anchor` / `anchor-absent` pair became the convention
  for the polarity fix**: `no-X` for never declared, `X-absent` for declared and not found.
  exegesis's `skilllens-failure` fired when *no* failure handling was written and read as
  its opposite; it becomes `no-failure-mode`. Two independent derivations of a naming rule,
  and this repo had it first.

## Deep Reads — `ruflo`, `oh-my-agent`, `superpowers` (2026-08-22)

Three repositories the `agent-green` survey had filed as read-shallowly, opened. Written up
in gnosis's `manifesto.md`; these are canonizer's, and they were recorded against gnosis
first, which was the wrong home for a backlog belonging to this tool.

The first one resolves the oldest open defect in this file, and it does it with a mechanism
rather than with the evidence archive that entry has been waiting on.

- [x] **`verify.Provenance` wants two signals crossed, not one anchor looked up.** DONE
  2026-09-05 as `verify.Drift` plus `verify --against-proof PATH`.
  **The second signal was already on disk and nothing read it back.** `verify --proof` has
  been writing a `proof.Packet` binding the ruleset *and the source bytes* since it shipped,
  and `proof.Artifact` carries `{Path, Digest}`. So the file hash this entry asks for
  required no new record at all — only `proof.Load` and a comparison.
  The four verdicts are as filed: anchor present + source unchanged passes; present +
  changed is `anchor-drift` (advisory); **absent + unchanged is `anchor-fabricated`, the one
  certain defect and the only blocking one**; absent + changed is `anchor-stale` (advisory).
  Without `--against-proof` the behaviour is exactly as before — with one signal,
  `anchor-absent` blocking remains the honest verdict, and that fallback is pinned by a test.
  `anchorPresent` was factored out so `Provenance` and `Drift` cannot disagree about what
  "present" means. Original entry: The
  `anchor-absent` entry above concludes that separating a fabrication from a drift *"needs
  the immutable evidence archive"*, and offers only a cheap half in the meantime — record the
  source hash so a later failure can at least *report* whether the source changed. `ruflo`'s
  witness manifest (`docs/validation/README.md`, Layer 2) gets the full split with no archive
  at all, by keeping **two** signals per entry and crossing them: a whole-file sha256 **and**
  a *marker substring* that must remain present while the fix is.
  Its four verdicts map onto this repo's problem directly — hash matches is `Pass`; hash
  differs but the marker is still present is `Drift`, *"acceptable, the codebase advanced"*,
  recorded and **not** blocking; the marker missing is `Regressed`; the file gone is
  `Missing`. Their reason for the second signal is the one that transfers: *"A SHA-256-only
  check would flag every benign whitespace change as a regression. The marker is the
  semantic invariant."*
  For canonizer the marker already exists and is better chosen than theirs, because it was
  picked by whoever wrote the rule rather than by whoever wrote the check: **`r.SourceAnchor`
  is the marker**, and the missing half is the file hash beside it. Crossing the two gives
  *anchor present, source changed* (drift — refresh, do not block) and *anchor absent,
  source changed* (the anchor did not survive an edition change — a different message and a
  different fix from a fabrication), which is exactly the pair the entry above says it cannot
  distinguish. It does not need tier 0. The cheap half already recorded is the whole
  prerequisite.
  Two meta-signals come free from keeping the history, and both are about the check rather
  than the rule: a check that **flaps** between pass and regressed indicts its own marker,
  and a source that **persistently drifts** is one whose rules want re-anchoring rather than
  re-litigating. gnosis reached the same three-state split independently for archived
  sources (SPEC §14.3.2), which is the second derivation.
  One thing not to take: ruflo signs the manifest with an Ed25519 key whose seed is
  `sha256(gitCommit + ':ruflo-witness/v1')` — the commit is public and the derivation is
  published beside the signature, so anyone can forge it. It detects accidental corruption
  and presents as authentication. This repo already holds the correct position (`gate`
  refuses a weighted score for the same class of reason); the instance is worth knowing
  because it is what `vac-protocol`'s refusal of signatures predicts, observed in the wild.
- [x] **A critic that ran with reduced independence must say so, and there is no state for
  it.** CLOSED 2026-08-22 in this entry's own body; the checkbox was never ticked and is
  corrected 2026-09-05. The conclusion stands and is restated at the end of the entry: the
  state cannot occur, because canonizer spawns nothing. Original entry:
  it.** `critic` withholds the distillation by design and that is the whole basis for calling
  its opinion cold. `oh-my-agent`'s judge protocol reaches the same design from scratch — a
  spawned subagent with fresh context, briefed on the criteria and never on what the
  implementer claims — and states outright that *"independence is structural, not a
  prompt-level role-play."*
  What it has that canonizer does not is the degraded path: when the runtime cannot spawn a
  fresh context it runs the protocol inline **and emits an event recording the downgrade**.
  canonizer is either cold or it does not run, which is stricter and therefore fine — until
  the first environment where the cold path is unavailable, at which point the pressure is to
  relax the contract quietly. **Neither `checked` nor `unchecked` covers *checked under
  reduced independence*.** Add the third disposition before it is needed, so the answer to
  that pressure is a recorded downgrade rather than an edit to the contract. Pairs with the
  coverage record below: both are the critic reporting on its own conditions rather than on
  the ruleset.
  **CLOSED 2026-08-22: the state cannot occur here either, and the entry copied a failure
  mode canonizer does not have.** `oh-my-agent`'s downgrade event exists because its judge
  is a **spawned subagent**, a runtime operation that can be unavailable. canonizer spawns
  nothing — the *Decided — Architecture* entry at the top of this file settles it:
  *"canonizer is a prompt-filler… it never calls a model itself."* The critic prompt either
  gets emitted or it does not; there is no fallback path and no capability to probe, so a
  downgrade field could only ever be empty, which reads as evidence of an isolation nobody
  checked.
  **What is true instead, and belongs beside the cold-critic entry rather than as a new
  state:** the critic is cold **by construction of the prompt, not by isolation of the
  reader.** `critic.FillPrompt` withholds the distillation, and that is a fact about the
  text. Whatever runs that prompt may have produced the distillation moments earlier in the
  same session, and canonizer cannot see it — it emits and an agent replies.
  The guarantee is that the distillation is not **supplied**. It is not that the critic did
  not **have** it. Narrow, true, and better than the broad version, which invites a reader
  to stop checking. adh reached the identical conclusion for the same architectural reason;
  both entries were written from a project whose critic works differently.
  **Rejected — a self-declared "fresh context" flag**, testimony from the party that
  benefits from misreporting it. The only real fix is identity on the reply, refusing a
  critic answer from the session that produced the distillation, and that is a relay feature
  neither tool has. Recorded so the trigger is one that can fire.
- [x] **The coverage record is one design that four entries in this family describe
  separately, and it should be promoted once.** RESOLVED 2026-09-05, and it resolved the way
  the entry asked: **promoted once, into skillet, as `finding.Unexamined` + `Result.Unexamined`**
  — carrying the load-bearing constraint structurally rather than by convention, since a
  consumer walking `Diagnostics` to decide an outcome never sees the coverage list. canonizer
  now consumes that type rather than inventing an `examined`/`not_examined` block of its own.
  **One of the four descriptions does not collapse into it, and should not be made to.** The
  `Limitations:` item below is the *ruleset* declaring what it does not cover; this is the
  *critic* reporting what it did not look at. Different author, different moment, different
  object — and a reader needs both, because a well-scoped ruleset graded narrowly and a
  badly-scoped one graded thoroughly are different failures. Kept open on its own terms.
  Original entry: *This is not new work here — it is a note on
  the `cold critic reports what it found` entry above, so it is not double-counted.* The same
  shape appears as: this repo's `examined` / `not_examined` block; this repo's *"a ruleset
  that will not say what it does not cover is an advertisement"* (VAC's REQUIRED non-empty
  `limitations`); adh's *"name the refusals"*; and `skillet`'s own coverage-record entry,
  which already carries the load-bearing constraint — **advisory only; a critic that declares
  a gap must not thereby block, or it will learn to declare none.**
  Four descriptions of one mechanism is how a family ends up with four implementations.
  Reconcile them into one promotion rather than three, and keep skillet's constraint as the
  thing that survives.

*Recorded 2026-08-04. Sources: this repository's state, and
`~/Documents/agent-orange/go-advice/ai_skill_todo.md` (rigor backlog + the
"Centralize on skillet" offload analysis). See `PLAN.md` for the implementation
plan and its design review against `summary_rules.md`.*

## Applicability Is a Rule, and `verify` Does Not Follow It (2026-08-22)

`skillet` closed its open note about a general `Applicability` type: the answer is **no
type**, because the five real sites across skillsaw, adh and gnosis each suppress a
different thing deliberately. What is shared is a rule, lifted from gnosis's `internal/lint`
package doc:

> Applicability is derived, not declared, and a run states what it skipped. A check that
> silently declines to run is indistinguishable from a check that found nothing.

canonizer was not one of the five sites — it has no derived-applicability mechanism — but
the rule's second half lands on it anyway, and this is the first thing found by applying it.

- [x] **`verify` silently drops every non-enforced rule, so a clean result cannot be told
  from an unexamined one.** DONE 2026-09-05 as `verify.Scope` plus a stderr line on every
  run and one advisory diagnostic when nothing was enforced.
  **The entry was right that this is a count and not a predicate**, and right that skipping a
  CONSIDER rule is correct — the defect was only that the skipping was invisible.
  **It must not be `finding.Unexamined`, and skillet's own doc is why.** That type is
  *testimony*: a critic's unverifiable claim about its own behaviour. This is mechanical —
  code applied a documented rule and can say so exactly — and skillet documents the two as
  "deliberately different types" for that reason. skillet has no type for the mechanical
  half, so `Scope` stays local until a consumer beyond canonizer wants one.
  The advisory goes into the findings JSON rather than only to stderr, so it reaches `gate`;
  a fact that lives in a terminal is a fact somebody did not read. Never blocking: a
  CONSIDER-only ruleset is legitimate. An *empty* ruleset is not the advisory case — it
  examines nothing and misleads nobody. Original entry:
  from an unexamined one.** All three checks open with `if !enforced(r.Severity) { continue }`
  (`internal/verify/verify.go:31`, `:59`, `:101`) and nothing downstream records how many
  rules that skipped. A ruleset of entirely `[MAY]` rules produces **zero diagnostics** —
  byte-identical to a ruleset where every enforced rule passed. `gate` then ships on that
  silence, which is the same shape as the cold critic's empty-category problem already filed
  above.
  **This is scope rather than applicability, and the distinction matters for the fix.** A
  `MAY` rule genuinely is not required to carry a ✗/✓ pair or a source anchor, so skipping
  it is correct — the defect is only that the skipping is invisible. So the repair is not a
  predicate; it is a count. Report the enforced and total rule counts alongside the
  diagnostics, and `verify` over a ruleset with zero enforced rules says so rather than
  saying nothing.
  Cheap, and it closes a gap that widens as rulesets grow: the proportion of `MAY` rules is
  exactly the proportion of a ruleset these gates never look at, and today nobody can see
  that number.

## Adopt `finding.Unexamined` (2026-08-22)

`skillet/finding` gained `Unexamined{Aspect, Reason}` and a `Result.Unexamined` field,
promoted on this repo's *cold critic reports what it found, never what it did not look at*
entry plus adh's matching one — two consumers with present defects. Design record in
`skillet/TODO.md`; the parts that bind a caller:

- **Advisory, always.** `Result.HasBlocking` iterates `Diagnostics` only, so a declared gap
  cannot make a result blocking no matter how the caller is written. That is structural
  rather than a discipline, and `TestUnexaminedCannotBlock` pins it against the refactor
  that would break it.
- **`Reason` is required.** `Valid()` rejects either field empty or whitespace-only.
- **It is testimony, not a derived fact** — a critic's claim about its own behaviour.
- [x] **Parse `unexamined` from the critic reply and render it in `gate`.** DONE 2026-09-05
  as `critic.ParseReply`, which replaced the bare `json.Unmarshal` in `gate.readFindings`.
  **Parsing and validating are one call so a caller cannot do the first and forget the
  second** — `finding.Result` already has the field and a json tag, so the reply was in fact
  already being read; what was missing was that nothing checked it. A malformed entry rejects
  the whole reply, as filed, and the error names which entry.
  `gate` renders the record after the blocking decision is made, so nothing in it can change
  the outcome — the ordering is the guarantee, not a formality. A reply naming no gaps prints
  nothing: "no gaps declared" would read as a positive fact about coverage and is the absence
  of one. Original entry: The prompt in
  `internal/prompt/critic_prompt.md` gains a constraint footer asking the critic to name
  the angles it did not take, in `super-hermes`' form — *"This analysis maximized X. It did
  not examine: …"*. The parse is pure and belongs beside the findings parse.
  **Reject the whole reply on an invalid entry rather than dropping it.** That matches how
  adh already handles a malformed finding, and the reason is the same: silently discarding
  half a reply is how an answer that says nothing passes for an answer that found nothing.
  Render below the findings, clearly separated, and never count it toward the exit code.
- [x] **Say in `critic_prompt.md` that declaring a gap is free.** DONE 2026-09-05, and
  pinned by a test that compares with whitespace collapsed, so re-wrapping the prose cannot
  silently drop the sentence the mechanism depends on. Original entry: The whole mechanism turns
  on the critic believing that, and a critic that suspects a declared gap will be held
  against it declares none — which costs both the gap and the finding it would have come
  with. State it in the prompt, not only in the code.

______________________________________________________________________

**One sentence to keep, because two fields here look alike and are opposites.**
`Unexamined` is **generated, per-run, and advisory** — a critic saying what it did not look
at this time. `limitations` (the *"a ruleset that will not say what it does not cover is an
advertisement"* entry above) is **authored, committed, and required** — VAC makes a bundle
without it invalid. Same words, opposite obligations, and merging them would either make
declared gaps blocking or make stated non-claims optional. Both entries now carry this note
so the next reader does not reconcile them.

## Commissioned Gap Report, Round Two — Nothing Lands (2026-08-22)

Source: `~/Documents/agent-green/FPF/canonizer_todo.md`. Checked; nothing lands. **Full
reasoning is in `skillet/TODO.md` under "Round Two, and What Asking for Code-Reality
Verification Actually Bought"**, recorded once for the family.

Its one finding is that `Provenance` emits a flat `anchor-absent` and should map onto
`quotecheck`'s three states. That is the `anchor-absent` entry above, returned in shorter
form — and the entry is the better statement of it: the split is **three-way by state of the
evidence** (fabricated / drifted / unverifiable), not a relabelling of `quotecheck`'s
`Checked`/`Missing`/`Unchecked`, and the entry already records that the v0.18.0 prerequisite
is met and that the remaining work is canonizer's alone. Nothing to add.

The addendum proposes parameterised rulesets via `ailloy`-style templated blanks, so one
master ruleset can be distributed with local overrides. **Refused, and the reason is the
citation rather than the idea.** Its supporting references do not survive a lookup: the same
five source lines are cited verbatim in `steve-skill-market_todo.md` for an unrelated
recommendation, and the lines named as `cloudstrategy_book.md` and `eip_book.md` are
extracts from Russell's *A History of Western Philosophy*. A second item cites "65 other
articles" for the claim that backend and frontend systems have different performance
profiles.

If parameterised rulesets are wanted, they should be argued from a team that needs one —
and the argument has to clear a bar this repo already set: **a threshold a team may override
locally is a threshold whose loosening nobody reviews**, which is the failure `standards/`
was built to make visible. That is a real design question and it is not what the report
asked.

## Two Items Transferred From gnosis's Backlog (2026-08-23)

A re-read of gnosis's `TODO.md` on 2026-08-23 found fourteen entries filed against
sibling repositories. Nine were already in their real homes — `verify.Provenance`'s
two-signal cross and the reduced-independence critic are both above — and two were
nowhere. These are the two.

Worth recording alongside them: gnosis was still carrying five `skillsaw` items as
open that `skillsaw` had already closed. **A backlog that mirrors another
repository's work goes stale in the direction that flatters.** One home, and a
pointer from everywhere else.

- [x] **A known-answer soundness test per rule.** DONE 2026-09-06 as `verify.Soundness`,
  which maps `ruleset.Sound`'s findings onto blocking diagnostics. `skillet v0.30.0` added
  `Rule.Checks` and the `⊨` marker, closing the block this entry recorded.
  **Blocking, unlike the other two advisory checks, and the difference is decidability.**
  `Specificity` and `Conflicts` report things a deterministic check cannot settle. This one
  is settled: the checks ran against the rule's own examples and either discriminated or did
  not, so there is no judgement left for a reader to supply -- the line `Executable` and
  `Provenance` already sit on.
  **A rule with no checks is not reported**, because `Sound` declines to and canonizer must
  not add a "rules should carry checks" opinion on top. That is a separate policy, and no
  ruleset in this corpus carries a check yet, so it would fire on every rule.
  The rejected proxy this entry recorded is now moot and worth keeping anyway: the converse
  of `Executable`'s one-directional test would have flagged `✗ conn.Close()` /
  `✓ defer conn.Close()`, which is the shape a real fix takes. That case is now a **test**
  -- "a check passing on both examples is unsound" -- rather than a warning in prose.
  Original entry: **BLOCKED on skillet, established
  2026-09-05.** gnosis does this over a *pattern table*, where a regex either matches its
  positive example or it does not. canonizer's rules are prose statements for a model:
  `ruleset.Rule` carries `Statement`, `Bad`, `Good` and **no executable predicate**, so there
  is nothing to run against the two cases. `judge` has the matcher ops (`OpRegex`,
  `OpContains`, …) — the machinery exists; the field to hold per-rule checks does not, and
  canonizer has no `replace` directive.
  **A cheap proxy was considered and rejected, and the rejection is the useful part.**
  `Executable` already checks one direction (the ✓ does not appear inside the ✗); the
  converse looks free. It is wrong: `✗ conn.Close()` / `✓ defer conn.Close()` has the ✓
  containing the ✗, and that is the most natural shape a fix takes. Shipping it would flag
  correct rules — the false-alarm failure *this entry itself* warns about, that a rule firing
  on ordinary work gets the tool switched off.
  Unblocks when `ruleset.Rule` can carry `judge.Check`s. Original entry: `gate.SelfTest`'s planted-defect
  control generalised from the gate to the *rules*: every rule ships a case it must
  flag and a case it must not, and the set refuses to load if any rule fails either.
  gnosis now does this for its §9.3 pattern table, at load rather than in a test — the
  argument being that a test catches the same defect one commit later and only for
  whoever ran it — and it caught a pattern whose own positive example did not match on
  the first run.
  This repository has the control at the gate and not at the rule, which is the
  difference: `SelfTest` proves the gate discriminates, and says nothing about whether
  any individual rule does. **Soundness before completeness**, because trust is more
  sensitive to false alarms than to misses — a rule that fires on ordinary work gets
  the tool switched off, and the negative case is the one an author will not write
  unprompted.
- [ ] **The cache key should carry the rubric edition.** **NOT ACTIONABLE — there is no
  cache.** Established 2026-09-05: `grep -rni cache --include=*.go` returns nothing outside
  tests. There is no key to extend, and building a cache in order to key it correctly would
  be inventing the problem to solve it. Keep as a **constraint on a future cache**, not as
  work.
  **The principle underneath is live and separable, and is proposed rather than built.** A
  verdict that does not say which rubric produced it is silently yesterday's grade under
  today's rules — and canonizer already emits findings a consumer stores, with nothing in
  that document naming the edition of the checks or the critic prompt behind it.
  The entry's hard question — what identifies an edition, given it must change when scoring
  changes and not when prose does — has an exact answer for the critic half: **the prompt
  template *is* the rubric, all of it is instructions, so `identity.Hash(prompt.Critic)`
  changes precisely when the rubric changes.** The deterministic half would need a
  hand-bumped constant, since its "edition" is the set of checks and their severities.
  Not built because it is adjacent to the item rather than the item. Original entry: gnosis's relay key
  deliberately omits its `standards/` version, and the reason it can is that the rubric
  never enters the prompt there: a threshold change cannot stale a model's reply about
  a source. Wherever the rubric **is** what is being applied, a key without it serves
  yesterday's grade under today's rules — and silently, because the score comes back, it
  is a number, and nothing says which edition produced it.
  The design question is what identifies an edition: it has to change when scoring
  changes and not when prose does, or every comment edit invalidates every cached
  result.

______________________________________________________________________

## No Ruleset Has Ever Been Through This Pipeline (2026-09-07)

Noticed while looking for something to exercise the format-3 gates on, and it turned out to
be a larger fact than the one being looked for. Filed here rather than in `go-advice` or
`skillet` because canonizer owns the pipeline the gap is in, and because a corpus item filed
in a repository with no backlog is one nobody will read.

- [ ] **Every canonical-form ruleset in existence is a test fixture.** `grep -rln '^§[0-9]'`
      across `~/Documents/agent-orange` and this repository returns no stored ruleset: the
      `*_rules.md` files in `go-advice` are prose documents with **zero** `§` rule headers,
      and the only canonical-form text is in `cmd/*_test.go`. skillet's own estimate —
      *"roughly ten stored files exist, most of them 1-4 rule prompt examples"* — was
      generous.
      **So the format-3 gates being idle is a symptom, not the problem.** `verify.Limitations`
      flags every ruleset advisory-only and `Soundness` finds nothing to judge, and the
      reason is not that nobody has migrated a ruleset to format 3 — it is that **nobody has
      produced one at all**. `distill` and `synthesize` emit prompts; an agent is supposed to
      run them and write the ruleset back; that last step has never been completed against a
      real source and committed.
      **What this means for every gate here, and it is uncomfortable.** `Executable`,
      `Provenance`, `Drift`, `Canonical`, `Conflicts`, `Specificity`, `Limitations`,
      `Soundness` and the `--against-proof` split are all tested and none has met content it
      did not come packaged with. Their tests are honest about the logic and say nothing
      about whether the *rules a real distillation produces* trip them — the distinction
      between a check that works and a check that is calibrated.
      **The smallest thing that would change that:** run `distill` over one real source,
      have an agent produce the ruleset, and commit it. Everything else follows from having
      a subject — the format-3 features are then an edit to a real document rather than an
      exercise, and the first honest measurement of how noisy `Specificity` is on genuine
      rules becomes possible.
      **Expect the first run to find defects in the gates, not in the ruleset**, and treat
      that as the return on it. Every measured surprise in this backlog came from running
      something over real content: `normalize` deleting rationales, the `anchor-absent`
      conflation, the description predicate flagging the good case. None came from a test.

______________________________________________________________________

## `Specificity` Measures Typography, Not Concreteness (2026-09-07)

The first real distillation run — eight sources under `go-advice/Sources/benbjohnson` —
produced the first content these gates have ever seen, and this is what it found. It is the
return the *No Ruleset Has Ever Been Through This Pipeline* entry predicted: a defect in a
gate rather than in a ruleset.

Two rulesets carried parseable rules. Measured on both:

| ruleset                          | statements with backticks | `unspecific`      |
| -------------------------------- | ------------------------- | ----------------- |
| `real-world-sql-part-one`        | **0 of 29**               | 26 of 26 enforced |
| `structuring-applications-in-go` | **15 of 19**              | 4                 |

19 − 15 = 4, and `unspecific` = 4 exactly. **The check is not erratic: it fires precisely
when a statement holds no code span.** An earlier reading of the first file alone called it
"miscalibrated at 100%", which was too strong and is corrected here.

- [x] **The proxy is wrong whenever concrete prose is unbackticked.** DONE 2026-09-07 as
      `internal/verify.concrete`, which asks `doc.Links` first and falls back to four
      identifier shapes — `pkg.Name`, `pkg/name`, `Foo()`, `*T`.
      **Measured on the 48 real statements, code spans stripped so each pattern is judged on
      bare prose**: `pkg.Name` 7, `Foo()` 5, `*T` 4, `pkg/name` 2. A `snake_case` pattern was
      written and **dropped at 0 matches** — a pattern that fires on nothing is a claim
      nobody has checked.
      **Result: `real-world-sql-part-one` falls 29 → 15, and `structuring-applications-in-go`
      stays at exactly 4.** The second number is the one that matters: widening bought
      accuracy without going blind, which a laxer check would not have.
      Widened locally rather than in skillet: `markdown.Links` is the kernel's and its doc
      records the mixing as deliberate *because four consumers read it*, so what was widened
      is canonizer's question, not skillet's datum.
      **A residual is recorded in the code, because it bounds what this check can mean.** The
      15 that survive name no symbol and are still perfectly actionable — *"Open a
      transaction at the top of every service method"* is a definite instruction with a
      definite target. **Naming an identifier is not the same as being actionable**, and no
      widening of these patterns closes that. It is why the check is advisory and must stay
      so.
- [ ] **The convention it depends on is unstated, so the score is non-deterministic.** Same
      command, same source tree, one run: one distillation backticked throughout, another
      backticked nothing. Nothing in `distill_source_prompt.md` asked for it and its own
      worked examples do not use it, so the same source distilled twice can score 0% or
      100% unspecific. **That is the more serious of the two**, because noise can be
      dismissed and a signal that varies run to run cannot be read at all.
      **Partly addressed 2026-09-07**: the prompt's pre-submission checklist gained item 8,
      "Backtick every identifier", with the reason stated — a rule naming a real symbol in
      plain prose reads to a checker as a rule naming nothing. Left open until a re-run
      measures whether it holds.
      **Also reduced, not removed, by the widened check above.** With identifier shapes read
      out of bare prose, the unbackticked ruleset scores 15 rather than 29 — so typography
      still moves the number, by half as much. The two fixes are independent and both are
      wanted: the prompt makes the input consistent, the widened check makes the score less
      hostage to it. Neither alone would have been enough.
      **Re-measured 2026-09-07 on the full released corpus — 162 rules, eight rulesets — and
      the convention now holds: 94% of the 99 rules that pass carry a code span.** Checklist
      item 8 worked. But the same number read the other way is the finding below: passing
      and being backticked have become nearly the same event.
      **CORRECTION 2026-09-08: that was the wrong measurement, and it was used to argue this
      entry was nearly closeable.** "94% of passing rules carry a code span" is a fact about
      the *check*, not the convention: `Specificity` fires when a code span is absent, so
      passing implies backticked almost by construction. It cannot speak to whether the
      convention is followed uniformly, which is what this entry is about.
      **Measured directly, per ruleset, and the spread is the finding:**

| ruleset                          | rules | with a code span | rate |
| -------------------------------- | ----: | ---------------: | ---: |
| `real_world_sql_part_one`        |    28 |               22 |  78% |
| `failure_is_your_domain`         |    20 |               15 |  75% |
| `standard_package_layout`        |    20 |               12 |  60% |
| `structuring_applications_in_go` |    20 |               12 |  60% |
| `crud`                           |    25 |               14 |  56% |
| `structuring_tests_in_go`        |    20 |               11 |  55% |
| `packages_as_layers`             |    18 |                7 |  38% |
| `wtf_dial`                       |    11 |                1 |   9% |

**9% to 78% across eight distillations from one prompt in one batch.** This entry's original
observation — *"one distillation backticked throughout, another backticked nothing"* — is
**reproduced, not resolved**: `wtf_dial` at 9% is that second case, in the new corpus, after
checklist item 8. The instruction reduced the variance; it did not remove it.
**So the measurement this entry always wanted is a per-ruleset spread, not a per-rule rate**,
and it stays open. What would close it is a repeat distillation of *one* source scored twice,
which needs the agent pipeline and was not run here. Until then the honest statement is that
`unspecific` still varies with which distillation produced the ruleset.

**The measurement this entry wanted now ships (2026-09-08).** `verify` reports `N of M
enforced rule(s) name a symbol a checker can see` on every run, and across the eight
rulesets that reads 11% to 76% — the spread this entry describes, visible without a script.
**It does not close the item.** A spread across eight *different* sources is not the same
evidence as one source distilled twice, which is what would show the score varying while the
input does not. But the number is now in front of anyone who runs `verify`, so the next
batch will produce the comparison as a side effect rather than needing an experiment.

- [x] **When the convention is followed the check looks sound, and that is the argument for
      keeping it.** The 4 it flagged in the well-formatted ruleset are the softest rules
      there — *"a real dependency boundary"*, *"closely related"*, *"by importance"*,
      *"roughly 10,000 SLOC"*. Judgement-laden, naming nothing an agent can match on:
      exactly what the check exists to surface.
      So the fix is **not** to relax it. Confirm this on a second corpus before treating the
      four as evidence — one file is an anecdote, and this entry exists because the first
      file alone produced a wrong conclusion.
      **Strengthened 2026-09-07: all four survived the widening unchanged.** They name no
      identifier in any form, so the accuracy gain that halved the other ruleset did not
      touch them. That is the best evidence so far that the check catches something real.
      **And a hypothesis was formed, tested and refused in the same pass.** It looked as
      though the axis separating these four from the 15 was *hedging* rather than
      identifiers, and that `SofteningPhrases` — which `Specificity` already runs first —
      might make the second signal redundant. It does not: `skilllens.SofteningTerms` is a
      short list of **discretion** phrases (*"as appropriate"*, *"it depends"*, *"at your
      discretion"*), and none of the four matches one. Verified by running the gate: the
      output carries `unspecific` and no `softening` at all. The two signals detect different
      things and both are load-bearing. Recorded because the hypothesis was plausible enough
      to act on and wrong.
      **The second corpus this entry asked for now exists, and it does not confirm the
      entry.** 162 rules across eight rulesets, each joined to its diagnostics by `path`.
      Of the 63 flagged, **not one carries a code span**; of the 99 that pass, **94% do**.
      Hedging — the axis proposed above — separates them 9% against 3%, which is **not a
      signal**, and refutes the hypothesis rather than leaving it open.
      The widening rescues 5 of the 68 unbackticked rules, **7%**, so `concrete` remains at
      corpus scale very close to a backtick detector; the four identifier shapes fire far
      less often on real prose than the 48-statement sample implied.
      **What the flagged rules read like matters more than the percentage**, because many
      are plainly actionable — *"Accept search criteria as one filter struct parameter,
      never as a list of individual filtering arguments"*, *"Never surface an undefined
      error's own text to an end user; show the generic support message instead."* Neither
      names a symbol; both tell a reader exactly what to do.
      **So the residual is confirmed rather than closed, and this entry's optimistic reading
      was drawn from too little data — the error it exists to record, repeated.** The four
      soft rules are still correctly flagged; the claim that being flagged *means* soft does
      not survive 63 of them. What to do is a separate question: accept the check as a
      typography lint and rename it for what it measures, or find a signal for actionability
      that is not lexical. Do not relax it on this evidence alone.
      **DECIDED 2026-09-08 — keep the computation, drop the accusation.** Delete
      `unspecific` as a per-rule finding, and report what it computes once per document
      instead. Not yet implemented.
      **The check's message is false on essentially every rule it flags.** It says the
      statement *"names no object, tool or API a reader could act on"*, and reading all 63:
      one is *"Hand-write these mocks rather than generating them with a mocking library such
      as GoMock"* — which names a tool — and another is *"Accept search criteria as one
      filter struct parameter"* — which names the object. They all name something. What
      separates the four soft rules is **hedging on a threshold or criterion** (*"closely
      related"*, *"by decreasing importance"*, *"roughly 10K SLOC"*), not naming.
      **63 of 147 enforced rules, a 43% fire rate.** An advisory that fires on two rules in
      five is noise a reader learns to scroll past, and renaming it would make the label
      honest without making the signal useful.
      **The per-rule question already has an owner, and it is not a regex.** The cold
      critic's `vague` test 2 is word for word what this claims to measure — *"Could a reader
      act on it without further interpretation? It should name the object, tool, API or step
      involved"* — asked by something that can read, at **error** severity. Two spellings of
      one question, and the deterministic one is the weaker.
      **What the data does support is a document-level statistic.** Zero of the flagged
      rules carry a code span against 94% of those that pass, so what this measures is
      **backtick-convention adherence** — a true property of a document and a false claim
      about a rule. It belongs in the scope line beside *"examined 9 of 11 rule(s)"*.
      **That is the measurement the entry below has been asking for**, which is why this
      decision is worth making for its sake as much as this entry's: the per-ruleset rate
      runs 9% to 78%, and nothing currently reports it.
      **One option is kept on file rather than taken.** Swapping concreteness for a
      *comparative-hedge* vocabulary — distinct from `SofteningTerms`, which is discretion
      phrases only — scores **63 → 12**, retains all four rules this entry defends, and adds
      three plausible catches. It is still a lexical proxy for a semantic property, which is
      the wall five attempts have hit, so it must earn a second corpus before being trusted.
      That discipline is what this entry exists to enforce.
      **Honesty about the argument for deleting**: that the critic covers these 63 is an
      argument from its rubric, not from evidence. Nobody has run `critic` against them. The
      cheap check before deleting is one critic run on one ruleset.
      **DONE 2026-09-08.** `Specificity` is now `Softening` — removing the concreteness
      branch left a function reporting hedging and nothing else, so the name stopped
      describing it. `CategoryUnspecific` is gone. `concrete` and `identifierPatterns` are
      **kept**, because they are the computation the decision preserved; they now feed
      `Scope.Symbolic`, reported once per document.
      **Measured: 63 `unspecific` findings removed and every other category byte-identical
      on all eight rulesets.** The scope line now carries `N of M enforced rule(s) name a
      symbol a checker can see`, printed on every run including at its ceiling, for the
      reason the examined line is: a reader who only ever meets a number at 100% never
      learns what it means.
      **And the spread reproduces through the shipped code**: 11% to 76% across the eight,
      `wtf_dial` lowest and `real_world_sql_part_one` highest. The script measured 9%–78%
      over all 162 rules; this counts the 147 **enforced** ones, which is the right
      denominator because only those are examined.
      **A consequence worth stating rather than discovering later: `Softening` now fires
      zero times on the corpus.** No ruleset carries a `softening` finding, because
      `skilllens.SofteningTerms` is discretion phrases (*"as appropriate"*, *"at your
      discretion"*) and none of the eight uses one. So this check is entirely idle, exactly
      as `sectionOnly` is. That is not a reason to delete it — a hedged rule is a real defect
      and the vocabulary is shared with skillsaw and adh — but it does mean the
      comparative-hedge option on file is the one that would make this check earn its place,
      and it now has a second reason to be tried: it scores 12 where this scores 0.

______________________________________________________________________

## `Provenance` Never Matched a Correctly-Written Anchor (2026-09-07)

Found by running the gates on the first ruleset the pipeline produced end to end —
`crud_rules.md`, 26 rules distilled from Ben Johnson's *Common CRUD Design in Go* with the
source actually read rather than recalled. `Provenance` reported **26 of 26 anchors absent**,
and the ruleset was largely right.

- [x] **The check searched for the whole anchor, and a correct anchor is not all quotation.**
      DONE 2026-09-07. `distill_source_prompt.md` asks for a `↦` line of the form
      `§Section: "the quote"` — its own worked example is
      `↦  §Errors: "never ignore the value returned by a function"` — so a well-formed anchor
      carries a section prefix that is *about* the source rather than *from* it.
      `anchorPresent` did `strings.Contains(source, anchor)` on the whole string, which the
      prefix guarantees will fail.
      **Isolated on one file with one folding, so the number is the change and not the
      corpus: 25 anchors, whole-anchor match 0, quoted-span match 14.** `anchorText` now
      takes the first quoted span and falls back to the anchor as written, so a bare
      quotation — which most of the corpus writes — still matches. A prefix cannot launder a
      fabricated quotation: that case is a test.
      **The first before/after taken for this was confounded and is not the one above.** The
      agent revised the ruleset two minutes after writing it, so an earlier 26 → 11
      comparison spanned two different files. Re-measured against a single file.
- [x] **An anchor may legitimately carry no quotation, and there is no verdict for that.**
      DONE 2026-09-07 as `verify.sectionOnly` + `CategoryAnchorSectionOnly`, advisory and
      `ActionHuman`, asked by both `Provenance` and `Drift.driftOne` through the one
      predicate — presence is asked *after* it, because "is it in the source" is meaningless
      when there is nothing to look for.
      **The predicate is "does it say anything beyond the section name", not "does it carry
      a quotation", and the second was written first and was wrong twice over.** A bare
      quotation with no quote marks would have been reclassified unsearchable, undoing the
      fallback `anchorText` preserves; and `§Errors: every method takes ctx first` carries no
      quotation while being a **paraphrase**, so keying on quote marks would have turned all
      eight paraphrase anchors below into advisories and buried a real finding. Cutting on
      the colon separates them, and does not reject a multi-word `§Transactional boundaries`.
      **Advisory rather than blocking because the prompt permits a section reference**;
      blocking one would fail a ruleset for doing what it was asked. Making anchors
      verbatim-only is a *prompt* change and is the entry two below.
      **The population is unmeasured and the entry should not pretend otherwise.** The "one
      anchor" count came from a ruleset revised two minutes after it was measured and deleted
      two hours later; the conflation is wrong regardless of frequency, but the fix is
      fixture-tested rather than corpus-measured, which is weaker than the prefix fix beside
      it. Original entry:
      The prompt permits *"a short quote **or section reference**"*. A section reference
      cannot be verbatim-matched by anything, so it reports `anchor-absent` — the same
      category as a fabricated quotation, which is the conflation the `Drift` work exists to
      undo, in a new place.
      It wants the `quotecheck.Status` shape already used for the unverifiable third state:
      *checked and missing* and *not checkable* are different answers. One anchor in the
      measured ruleset is of this kind, so the cost today is one false blocking finding —
      small, and the argument is the confusion rather than the count.
- [x] **An elided quotation cannot match, and eliding is what a careful quoter does.**
      DONE 2026-09-08 in `anchorPresent`, which now reads a quotation as a conjunction of
      fragments. **Measured: `anchor-absent` falls 59 → 39 across the eight rulesets, −20,
      and every other category is byte-identical on all eight.** The predicted class size
      was 20, so the fix lands exactly where the measurement said it would.
      **The separator is `"... "`, not `"..."`, and the corpus is why.** Of 31 ellipses in
      its anchors, the 30 marking an elision are followed by whitespace and the one that is
      not is Go variadic syntax inside a code span — `` `tx.QueryContext(ctx, ...)` ``.
      A bare separator cuts that into `` `tx.QueryContext(ctx,` `` and `` `)` ``, and a lone
      `)` is in every source, so a code anchor could pass on a fragment carrying no
      evidence. Both spellings score 117 of 162 today, so this bought no anchors and closed
      a failure channel; two tests fail under the bare separator, which is what keeps it.
      Safe as a plain string because `textnorm.Fold` collapses whitespace and rewrites
      U+2026 first, so `…` and a line-wrapped ellipsis arrive in one spelling.
      **The conjunction is weaker than a whole-span match and the weakness is recorded, not
      hidden.** Fragments may come from anywhere and **order is not checked** — `"B ... A"`
      passes a source reading `"A ... B"`. An offset per fragment would close it; no anchor
      in the corpus exhibits it, so the gap is priced and left open.
      **A trailing elision is refused, and that is pinned by a test rather than implied.**
      `Fold` trims, so `"text ... "` loses the space that makes the mark a separator. The
      corpus has **zero** leading and **zero** trailing elisions and the prompt's convention
      is a gap *between* two spans, so refusing an unbounded remainder is the reading — and
      it is pinned so widening it later has to be deliberate.
      **A latent vacuous pass died with the branch.** Reviewing against §4 showed the
      elided/unelided guard was unnecessary: an unelided quotation splits to one fragment
      and behaves identically, so the two paths merged into one loop. The old single
      `strings.Contains` reported an **empty** quotation as present; the conjunction reports
      it absent, which is the fail-closed default this family keeps rediscovering.
      Original entry: Six of
      26 anchors quote with `...` — *"I rarely expose internal details like transactions to
      the rest of my application ... it's rarely necessary"* — which is a faithful quotation
      of two spans and matches neither.
      Splitting on the ellipsis and requiring each fragment present would accept these
      without accepting a fabrication, since both halves still have to be in the source.
      **Not done with the prefix fix, deliberately:** that fix corrected a check that could
      never pass, and this one widens what passes. They deserve separate measurement, and
      bundling them would make the 0 → 14 number above unattributable.
- [x] **Eight anchors are paraphrase rather than quotation, and that is the ruleset's
      defect.** DONE 2026-09-08 as checklist item 9 in `distill_source_prompt.md`, which
      says quote verbatim, mark a gap with `...` and a space, and make each side verbatim
      on its own.
      **The premise does not reproduce, and the conclusion survives it anyway.** On the
      eight-ruleset corpus **zero** anchors are paraphrase — every one carries a quoted
      span, in double quotes or in backticks. The eight came from `crud_rules.md` as it
      stood at 19:52 on 2026-09-06, a file revised two minutes later and deleted two hours
      after that; it is the third finding in this backlog traced to that vanished file.
      **What is actually there is misquotation: 31 anchors quote a span that is not in the
      source.** Sampled against the nearest source window, 18 of 31 sit at 0.85–0.95
      similarity — near-misses, not inventions. So the prompt still needs to say *verbatim*,
      for a different reason than this entry gave: the anchors are quoting, and quoting
      inexactly. Fix the reason, keep the fix.
      **The prompt's effect is unmeasured on purpose.** Checklist items only bind the next
      distillation, so nothing here can be verified until `pipeline.sh` is re-run. The
      31 misquotations stay open as the residual below.
      Original entry: *"every method in the `DialService` interface takes `ctx context.Context`
      first"* describes the source instead of quoting it. No substring check can validate a
      paraphrase, and it should not try to: this is the `anchor-fabricated` case the gate
      exists for, and the honest fix is upstream — the prompt says the anchor makes
      provenance auditable without saying it must be verbatim.

______________________________________________________________________

## The Backtick Convention Walked the Corpus into a Parser Bug (2026-09-07)

The first full batch: eight rulesets, **162 rules**, every one opening cleanly at `Source:`.
The three invocation fixes hold and the pipeline produces artifacts. Six verify; two do not.

- [x] **Two of eight rulesets are unparseable, and this repository's own prompt change is
      half the cause.** DONE 2026-09-07 — skillet v0.32.0, pinned here. `ruleset.Parse` refuses a rationale line beginning with a backtick —
      `applyBody` rejects any leading Unicode **symbol**, and a backtick is `Sk` while every
      prose opener its doc names as safe (`—`, `“`, `(`) is punctuation. Filed in
      `skillet/TODO.md` with the category table; the guard needs narrowing to the `So`/`Sm`
      categories the markers actually occupy.
      **The interaction is the part that belongs here.** *"Backtick every identifier"* was
      added to `distill_source_prompt.md` yesterday to stop `Specificity` measuring
      typography. It worked — and it raised the rate of rationales opening with a code span,
      which is what walked the corpus into a latent parser bug. **Two correct changes, one
      bug between them**, and neither is worth reverting.
      Cheap to hit and expensive to suffer: **3 of 299 body lines**, but one line fails a
      whole document, so three lines cost two rulesets.
      **Fixed in skillet 2026-09-07** and measured against this exact corpus with a
      temporary `replace`: both rulesets parse (19 of 20 and 25 of 28 rules examined) and the
      six that already parsed produce byte-identical diagnostic counts. The guard now tests
      `So`/`Sm` — the categories the markers occupy — instead of every Unicode symbol.
      **Closed by skillet v0.32.0**, pinned here and re-measured against the released kernel
      rather than a `replace`: all **eight** rulesets parse, **162 rules**, no `PARSE-FAIL`.
      No code changed in this repository — the bump was the whole fix.
      Recorded because the next person to widen a prompt convention should know it can move
      the corpus into a part of the grammar nothing had exercised.
- [x] **Six rulesets now verify, and the gates report a spread rather than a verdict.**
      DONE 2026-09-07 — superseded by the eight-ruleset recalibration below, which is the
      same measurement with the two parser casualties restored.

Recalibrated 2026-09-07 against skillet v0.32.0, with all eight parsing:

| ruleset                          | anchor-absent | unspecific | unexecutable | non-canonical |
| -------------------------------- | ------------- | ---------- | ------------ | ------------- |
| `crud`                           | 8             | 8          | 2            | 1             |
| `failure_is_your_domain`         | 8             | 5          | 8            | 1             |
| `packages_as_layers`             | 6             | 11         | 1            | 1             |
| `real_world_sql_part_one`        | 12            | 6          | 2            | 1             |
| `standard_package_layout`        | 14            | 8          | 4            | 1             |
| `structuring_applications_in_go` | 6             | 8          | 4            | —             |
| `structuring_tests_in_go`        | 5             | 9          | 5            | 1             |
| `wtf_dial`                       | —             | 8          | 3            | —             |

**`anchor-absent` has stopped being 100%.** It was 26 of 26 before the prefix fix; it now
ranges 0–14, and `wtf_dial` verifies every anchor it declares. That is the strongest
evidence yet that the fix was to the check rather than to the corpus.

**Two rulesets carry no `non-canonical` finding** — the first time `Canonical` has passed on
content it did not come packaged with.

- [x] **`unbounded` fires on eight of eight, and the gate is not the thing that is wrong.**
      DONE 2026-09-08 — the prompt now asks. `Limitations:` joins the metadata block with a
      bracketed instruction, joins the allowed-line list that governs the parse, and joins
      checklist item 7; and the instruction says not to write `none`, because
      `verify.Limitations` accepts any non-empty string and its own doc rules out detecting
      an empty answer with a word list.
      **The allowed-line list had to change first, and that ordering is the point.** Line 29
      tells the model any line outside a fixed set *"will corrupt the parse"*. Adding a
      header to the metadata block without adding it there would have told the model to emit
      a line the same document forbids. Verified against the kernel before writing either:
      `ruleset.Parse` has a `Limitations:` case, `Render` emits it, and a round trip through
      both is byte-identical — so this is format 3 and the parse claim is true today.
      **No Go change, and the check stays advisory.** `verify.Limitations` already reads the
      field, and its doc already names the expiry condition — advisory *"because no ruleset
      in the corpus carries the header yet, so blocking would fail every one of them"*.
      Whether to make it blocking is a decision for after a corpus carries it, and the
      corpus will not until `pipeline.sh` is re-run. Original entry:
      No ruleset declares `Limitations:`, so the check reports a missing header on every
      document it has ever seen. **A check that fires on 100% of inputs carries no
      information** — the shape `Specificity` had at 29 of 29 before it was widened.
      **The cause is that the prompt never asks.** `grep -i limitation` over
      `distill_source_prompt.md` and all eight generated prompts returns nothing, so the
      gate demands a section the generator was never told to write. Gate and prompt
      disagree, and the prompt is the side that is missing something.
      Two options, neither obviously right: **teach the prompt to emit `Limitations:`**,
      which makes the check meaningful and costs a re-run of the corpus; or **drop it to
      advisory** until some ruleset carries the header, on the ground that a blocking error
      nobody can clear is a gate in name only. Prefer the first — the header is the one
      place a ruleset says where it stops applying, and the argument for it does not weaken
      because nothing has one yet.
      Do not simply delete the check: the header is wanted and the finding is accurate.

______________________________________________________________________

## Two More Anchor Classes, Measured and Deliberately Not Bundled (2026-09-08)

Found while measuring the elision fix. Both are in the function that fix edited, both are
cheap, and both were left out so the elision result stayed attributable at exactly −20 —
the same reason the elision work was itself held back from the prefix fix.

Anchor failure classes across the eight rulesets, 162 anchors, before this session:

| class                              |    n | outcome                     |
| ---------------------------------- | ---: | --------------------------- |
| ellipsis, every fragment in source |   20 | fixed 2026-09-08            |
| ellipsis, some fragment absent     |   10 | ruleset's own fault         |
| backtick-quoted span, present      |    3 | fixed 2026-09-08            |
| backtick-quoted span, absent       |    1 | ruleset's own fault         |
| double-quoted span, absent         |   31 | 10 were emphasis, now fixed |
| no quotation at all (paraphrase)   |    0 | does not occur              |

**All three fixes have now landed, and `anchor-absent` across the corpus is 26**, from 59
before any of them: elision took 59 → 39, the backtick span 39 → 36, and emphasis folding
36 → 26. What remains is the residual those entries name — misquotation and partial
elision, which are the rulesets' own defects rather than the check's.

- [x] **`anchorText` reads `"` and not `` ` ``, so a code quotation is unsearchable.**
      DONE 2026-09-08. `anchorText` now tries a double-quoted span, then a backtick span,
      then the whole anchor. **Measured: `anchor-absent` 39 → 36 on enforced rules, −3**,
      matching the predicted "4 in the class, 3 of them present".
      **Double quotes win, and the nesting is why**: `` §Helper methods: "`defer
      rows.Close()`" `` puts the backticks *inside* the quotation, so the outer delimiter is
      the one bounding the passage.
      **The scan was extracted rather than copied.** A second hand-rolled search for a
      delimited span beside the first is the Repetition red flag, and the existing body was
      already that function with `"` hardcoded. `firstDelimited(s, delim)` now holds the
      knowledge of *how* to find a span; `anchorText` keeps the knowledge of *which*
      delimiter wins, which is the part that is a decision. Original entry: An
      anchor may quote an identifier rather than prose — `` §Remove dependencies by
      abstracting services: `FindDialByID(ctx context.Context, id int) (*Dial, error)` `` —
      and `anchorText` looks only for a double-quoted span, so it falls back to the whole
      anchor and searches the section prefix along with it. That is the same defect the
      prefix fix corrected, surviving in the delimiter it did not consider.
      **Measured: 4 anchors, 3 of which are present in the source and wrongly reported
      absent.** Small, and the smallness is the argument for doing it rather than against:
      it is one more delimiter in a function that already takes the first quoted span.
      **The design question is which delimiter wins when an anchor carries both**, since
      `` §Helper methods: "`defer rows.Close()`" `` nests one inside the other. Taking the
      outermost — double quotes when present, backticks otherwise — matches what the prompt
      writes and keeps the rule stateable in a sentence.
- [x] **`textnorm.Fold` does not fold markdown emphasis, so faithful quotations of the
      rendered text miss.** DONE 2026-09-08 as `verify.unemphasize`, applied to source and
      anchor alike so an anchor that quoted the markers verbatim also matches.
      **Measured: `anchor-absent` 36 → 26 on enforced rules, −10**, the largest of the three
      anchor fixes. Across all 162 anchors the corpus goes from 45 absent to 31.
      **Only paired double markers are folded, and the narrow rule won on measurement rather
      than on caution.** A rule folding every marker run scores **three anchors worse**,
      because single-marker italic matches across `snake_case`: in `id IN (SELECT dial_id
      FROM dial_memberships`, the span `_memberships FROM dial_` is a legal `_..._` pair.
      Doubling the marker removes that whole class of false pair, and a test pins it.
      **RE2 has no backreference, so it is one pattern per delimiter.** A single
      `(\*\*|__)(.+?)(\*\*|__)` cannot require the closing marker to match the opening
      one and would fold `**text__`, which is not emphasis. Two patterns state what the
      regexp language cannot.
      **No code-span guard, because the corpus says none is needed.** Of 238 code spans in
      the sources, 8 hold a marker *character* — `[]*Dial`, `COUNT(*) OVER()`, `"name_asc"`,
      `*myapp.Error` — and **not one is a paired run**, so a double-marker pattern cannot
      reach them. Building the guard would be machinery for a case that does not occur,
      which is the ground `snake_case` was dropped on at zero matches. The residual is stated
      in the doc instead: a code span holding a genuine `__dunder__` would lose its markers,
      and none exists here.
      **Local to canonizer, on the `markdown.Links` precedent**, with the kernel question
      left filed rather than decided. Original entry: the
      rendered text miss.** The sources are markdown and use `__bold__`; an anchor quotes
      what a reader sees, so `only` in the anchor meets `__only__` in the source. Diffing
      near-miss anchors against their best source window shows this as the single largest
      systematic cause.
      **Measured: 10 anchors, 65 → 55 absent on its own, and it composes — with the
      elision fix and backtick spans it takes the corpus from 65 absent to 31.**
      **The siting is the real question and it is not obviously canonizer's.** `Fold`'s doc
      says it folds *"the characters a book and its plain-text extraction are most likely to
      disagree about"*, and markdown emphasis is squarely that, which argues for skillet.
      Against: four consumers read `Fold`, and widening a kernel datum to answer one
      consumer's question is what the `markdown.Links` decision refused — there the
      consumer's *question* was widened locally instead. Follow that precedent: fold
      emphasis in canonizer where the anchor question lives, and file the kernel question
      separately rather than deciding it from here.
      **Do not fold emphasis inside a code span.** `` `__init__` `` is an identifier whose
      underscores are content, and stripping them would invent a symbol that does not
      exist. Unmeasured in this corpus; named because the fix is a regex and this is the
      case a regex gets wrong.
- [x] **`sectionOnly` fires zero times on the corpus.** The three-way anchor split shipped
      in `b469f8b` has **no instance** in the eight rulesets: every anchor carries a
      quotation, so none is a bare section reference. Its own entry said the population was
      unmeasured; this measures it at 0.
      **Not a reason to remove it.** The conflation it undoes — *searched and not found*
      versus *nothing to search for* — is wrong at any frequency, and the prompt still
      permits a section reference, so a ruleset may produce one tomorrow. But it is a reason
      to stop citing it as load-bearing, and to expect the next distillation to be the first
      test of whether the permission is ever used.
      **The honest options are to keep it as a fixture-tested guard, or to remove the
      permission from the prompt and make the check blocking.** The second is coherent —
      every anchor already quotes, so nothing would break — and it would replace an advisory
      nobody hits with a rule the corpus already follows.
      **DECIDED 2026-09-08 — permit and advise, and count it as provenance not examined.**
      Not yet implemented. Three parts: state the permission plainly in the prompt's
      checklist, keep the diagnostic advisory, and make a section-only anchor count as *not
      examined* in the scope line rather than passing silently.
      **The prompt contradicts itself, and that reframes the decision.** The format spec
      permits *"a short quote or section reference"*; checklist item 9, added 2026-09-08 in
      the same session that shipped this check, requires a quotation to be *"the source's own
      words, character for character"*. So the real question was never keep-or-block but
      **which direction to resolve an inconsistency nobody had noticed**.
      **Blocking was rejected because it removes the only honest answer.** A rule genuinely
      derived from a whole passage has no single sentence to quote, and this corpus has
      already shown what an agent does when it needs a quotation it does not have: **31
      anchors quote text absent from the source, 18 of them near-misses**. Forcing a
      quotation invites a cherry-picked sentence that *looks* verbatim while being worse
      provenance than an honest section reference. Held as a risk rather than a certainty —
      the fallback is available today and misquotation happens anyway — but a gate that turns
      honest imprecision into confident-looking fabrication is the wrong trade.
      **What is actually broken is that a section-only anchor passes silently**, which is
      indistinguishable in the output from a verified one. That is the fail-open shape this
      family keeps closing, and counting it as unexamined fixes it without failing a ruleset
      for doing what the prompt asked.
      **It reuses machinery that already exists and already says the right thing.**
      `reportScope`'s own message is *"an empty result here means unchecked, not clean"*.
      **Deleting the check was considered and refused**: a legitimate section reference would
      then report `anchor-absent` — fabrication — which is exactly the conflation the split
      was built to undo. Trading a correct distinction for 170 lines.
      **Do both accountings at once.** The entry above decided that `unspecific` becomes a
      document-level statistic in the same scope line; section-only anchors belong in that
      same account of how much of a ruleset is machine-checkable. One coherent change to what
      `verify` reports, not two.
      **DONE 2026-09-08**, and one framing above is corrected in the doing.
      **"A section-only anchor passes silently" was wrong.** `Provenance` emits
      `anchor-section-only` as an advisory, so it *is* reported. What was actually wrong is
      narrower: the scope line's *"examined 9 of 11"* counted such a rule as examined when
      its provenance was never searched. The defect was in the **accounting**, not the
      diagnostic — so the fix is `Scope.SectionOnly` and nothing else. Recorded because the
      looser phrasing would have led to adding a second diagnostic nobody needs.
      **Shipped**: `Scope.SectionOnly`, counted only for enforced rules, and a scope line
      reading `N of M anchor(s) name a section only; their provenance was not searched`.
      **That line prints only when non-zero, and the asymmetry has a reason.** Zero is the
      case in all 162 anchors of the corpus, so a line reporting none of them on every run
      costs attention and teaches nothing. The symbol rate beside it is a proportion
      informative anywhere in its range; this one is an exception report.
      **The prompt now says the permission plainly.** Checklist item 9 became *"Quote anchors
      verbatim, or name a section instead"*, stating that a rule drawn from a whole passage
      should name the section and quote nothing, that this is the honest answer, and that it
      is reported as provenance not searched rather than counted against the ruleset. The
      contradiction with the format spec is gone.

______________________________________________________________________

## Two Gaps the Sign-off Refusals Expose (2026-09-08)

- [x] **A non-canonical ruleset cannot be signed and canonizer offers no way to fix it.**
      DONE 2026-09-08 as `canonizer fmt --ruleset PATH [--check]`. Default rewrites,
      `--check` reports and exits 1 without writing — exegesis `normalize`'s split verbatim,
      so a reader who knows one knows the other.
      **Measured on the corpus: `--check` names the 5 non-canonical rulesets, and after a
      rewrite 0 of 8 remain.** An already-canonical file is not rewritten at all, so running
      this across a corpus does not touch mtimes for nothing; a test asserts that.
      **What it changes is wrapping, not text, and that is measured rather than hoped.**
      Reducing each of the eight to its word sequence, stored and rendered are identical on
      every one. A rule header written with three spaces after its level tag becomes two, and
      a rationale a human wrapped across three lines becomes one long line. Stored rulesets
      already carry 265–379 character lines, so that is the format's existing shape — but a
      hand-wrapped document will not come back wrapped, and `--help` says so before anyone
      runs it.
      **One file, not a tree.** exegesis takes a TREE because skills *are* a tree; every
      canonizer command names its ruleset explicitly, so consistency inside this repository
      won. A directory mode is possible and unbuilt.
      **Atomic write**, for `--sign-off`'s reason: it replaces a document a human owns and
      cannot regenerate. Original entry:
      `--sign-off` refuses a ruleset whose stored form differs from its rendering, because
      writing the event re-renders the document and would otherwise reformat the body as a
      side effect. Five of the eight stored rulesets are in that state, so the refusal is
      the common case rather than the corner.
      **`Render` already produces the answer and nothing exposes it.** That is a
      `canonizer fmt --ruleset PATH`, and it is the natural companion to the refusal:
      today a user is told the document is non-canonical and left without the one-line
      command that would make it canonical. exegesis has `normalize` for skills and this is
      the same shape for rulesets.
      **Not bundled with the sign-off deliberately.** A formatter writes to the same file
      the sign-off writes to, and shipping both at once would make it impossible to say
      which one caused a corpus-wide reformat. It also wants its own `--check` mode, on
      exegesis's precedent, and that is a design conversation rather than a subroutine.
- [x] **skillet could export a frontmatter writer, which would remove the refusal entirely.**
      DECLINED 2026-09-08. Kept rather than deleted, because a rejected option with its
      reasoning is worth more than a silent absence.
      **The premise does not survive measurement: `Render` is text-preserving.** Reducing
      each of the eight stored rulesets to its word sequence, stored and rendered are
      *identical* on every one. Only wrapping and inter-token spacing differ — rule headers
      written with three spaces after the level tag where `Render` emits two, and wrapped
      rationales joined onto one line. So "leave the body untouched" protects **whitespace,
      not content**, and the stored files already carry 265–379 character lines, so
      re-flowing does not cost readability that was there.
      **Against that, the cost is a first-of-its-kind kernel API.** Every skillet function
      today *produces* a document; this one would *edit* one, and it would need a release, to
      spare a caller a single `canonizer fmt` before signing.
      **And the refusal is doing work the writer would remove.** `Canonical` blocks because
      canonical form is wanted. A writer that appends signatures to a non-canonical ruleset
      lets a corpus accumulate attestations while never converging on the form the check
      exists to require — so the two-step workflow is the feature, not the friction.
      **What would reopen this**: a ruleset carrying content `Parse` does not model, so that
      re-rendering would genuinely lose something. Nothing in the corpus does today, and the
      round-trip check above is what would notice. Original entry:
      The reason `--sign-off` must re-render the whole document is that skillet's
      frontmatter emitter is unexported, so the alternative — splice a new block into the
      raw bytes and leave the body untouched — would need a second emitter in canonizer.
      That is the same knowledge in two modules, and this family has spent three entries
      avoiding exactly that third copy.
      **An exported writer would be strictly better than the refusal**, because it is what
      adh already does: `RecordVerification` decodes to `json.RawMessage` and re-encodes so
      unmodelled content survives untouched. A ruleset's body is that unmodelled content.
      Signing would then work on a non-canonical ruleset without touching its body, and the
      canonical question would go back to being `Canonical`'s alone.
      **The cost is a kernel API and a release**, and the shape needs thought: a writer that
      takes a raw document and a `[]verification.Event` and returns the document with its
      block replaced is not the same function as `Render`, and it would be the first
      skillet API that edits a document rather than producing one.

______________________________________________________________________

## The Comparative-Hedge Signal Now Has Two Reasons (2026-09-08)

- [ ] **Try the comparative-hedge vocabulary: it scores 12 where `Softening` scores 0.**
      Filed as considered-and-not-taken when `unspecific` was retired, on the ground that a
      lexical proxy for a semantic property is the wall five attempts have hit and must earn
      a second corpus first. That reasoning stands. What changed is the comparison.
      **The first reason was accuracy.** Swapping concreteness for a vocabulary of
      comparative and threshold hedges — *"closely related"*, *"by decreasing importance"*,
      *"roughly 10K SLOC"*, *"expensive enough"* — scores **63 → 12** on the eight rulesets,
      retains **all four** rules the retired entry defended as genuinely soft, and adds three
      plausible catches (*"at the layers a developer considers significant"*, *"when
      assertion verbosity hurts readability"*).
      **The second reason is that the check it would replace is now idle.** `Softening` fires
      **zero times** across all eight rulesets. `skilllens.SofteningTerms` is a short list of
      **discretion** phrases — *"as appropriate"*, *"it depends"*, *"at your discretion"* —
      and no ruleset in the corpus uses one. So the choice is no longer "a noisy signal
      versus a quieter one" but **12 findings versus none at all**, and a check that reports
      nothing on every document it has ever seen is not obviously better than one that
      reports twelve things worth reading.
      **The axis is different from `SofteningTerms`, which is why this is not just widening
      it.** Discretion phrases say *you may choose*; comparative hedges say *some unstated
      amount*. A rule saying "split when a file gets large" commits to an action and refuses
      to say when — which is the defect, and it contains no discretion phrase at all.
      **Siting: canonizer-local, on the `markdown.Links` precedent.** `SofteningTerms` is
      skillet's and skillsaw and adh score against it; widening a shared vocabulary to answer
      one consumer's question is what that decision refused. What would be added is
      canonizer's list, and if a second consumer ever wants it the question moves to skillet
      then.
      **What it must still earn before shipping**: a second corpus. The 12 are measured on
      the same eight rulesets that produced the hypothesis, which is exactly the error the
      retired entry exists to record. Run it against a different batch first.

______________________________________________________________________

## `pipeline.sh` Before the Next Batch (2026-09-08)

Assessed 2026-09-08 against everything that shipped since the first batch ran. Two defects
are fixed here; two changes need a decision first and are not made.

- [x] **The closing hint taught the invocation that under-reports.** DONE 2026-09-08. It
      printed `canonizer verify --ruleset PATH` with no `--source`, and omitting `--source`
      is not a smaller check but a different one: the anchor gates are replaced by an
      advisory, and on the first batch the blocking count fell **17 → 5, 15 → 3 and 8 → 2**.
      The shorter invocation reads as a better result while having examined less, and
      `--sign-off` refuses outright without it. The hint now names `--source` and, beside it,
      the `canonizer fmt` line for a ruleset reported non-canonical.
- [x] **The prompt's own worked examples were non-canonical, which is why 5 of 8 rulesets
      were.** DONE 2026-09-08. Two of four examples put **three** spaces after the level tag
      where `Render` emits two, and every example wrapped its rationale across lines where
      `Render` joins them. Verified by round-tripping the example block through
      `Parse`/`Render`: it did not match itself, and now does.
      **This is the root cause, not `fmt`'s absence.** Agents copy the worked example, so
      the format the prompt teaches was the format `Canonical` rejects. Fixing the example is
      better than running `fmt` over the output afterwards, because a formatter would hide
      that the prompt and the checker disagreed.
- [ ] **A re-run silently overwrites the untracked corpus, and that is the largest risk
      before the next batch.** `RULES_DIR` is `distilled/${D}`, so `./pipeline.sh
      benbjohnson` writes over the eight rulesets already there. Those files have **zero
      commits** — the `rulesets` repository has no commit at all — and every measurement in
      this backlog references them.
      **The fix is to commit the corpus, which is the item above, not to patch the script
      around an uncommitted repository.** A guard here would be protecting evidence that
      should not have been unprotected. Recorded as a dependency: do not re-run until that
      item is done.
- [ ] **Distilling one source twice needs a run label, and nothing supports it.** The
      unstated-convention item can only close by scoring one source distilled twice, and a
      second run currently overwrites the first — the exact opposite of what the measurement
      needs. A second positional argument writing to `distilled/${D}-${LABEL}` would do it,
      and the prompts directory needs the same treatment or the second run reuses the first
      run's prompts and measures nothing.
      **Not built, because the shape depends on how the comparison is wanted**: two runs of a
      whole subdirectory, or two runs of one named source. The second is cheaper and enough
      for the measurement, and it is a different flag.
