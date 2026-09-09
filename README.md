# Canonizer

canonizer turns source documents into Claude coding rulesets, and puts a **fresh,
independent, deterministic grader** between a candidate ruleset and adoption.

## Why This Exists

The pipeline canonizer reproduces (distill a document into rules, then synthesize them
into one ruleset) has strong *design-time* rigor. But in the original, every quality gate
was self-assessment by the same model that produced the artifact. The writer graded its
own work. A self-assessed score is inflated. A model grading its own rules grades them
generously.

The transferable lesson, and canonizer's whole reason to exist:

> A fresh, independent grader checking against runnable ground truth beats the producer
> grading its own work.

canonizer makes that grader the gate. It runs deterministic checks over a candidate
ruleset (do the rules carry discriminating examples? does each cite a source anchor that
appears in the source?), emits a **cold-critic** prompt that gives a fresh grader only
the source and the ruleset (never the reasoning that produced them), and blocks adoption
on the union of both. Nothing ships while a blocking finding stands.

### The Prompt-Filler Boundary

canonizer **never calls a model.** It fills prompts for an agent to run, applies the
deterministic gate to what comes back, and decides. The division of labor is deliberate:

- **canonizer / [skillet](https://github.com/StevenACoffman/skillet)** own everything
  deterministic: parsing the canonical form, the executability and provenance checks, the
  blocking decision, and the rework budget.
- **The agent** owns only what a deterministic check cannot decide: writing rules from a
  source, and the cold critic's semantic judgment ("this rule isn't supported by the
  source").

A run is a loop. canonizer emits a prompt, an agent runs it, the agent feeds findings
back, and canonizer applies the gate. Everything runs locally and offline.

## Install

```sh
go install github.com/StevenACoffman/canonizer@latest
```

Or build from a checkout:

```sh
git clone https://github.com/StevenACoffman/canonizer && cd canonizer
go build ./...
go run . --help
```

Requires Go 1.26+. Every flag can also be set from a `CANONIZER_`-prefixed environment
variable (uppercase the flag and replace each `-` with `_`). Command-line flags win.

## Concepts

**Canonical ruleset form.** Rulesets use the canonical text form that
[`skillet/ruleset`](https://github.com/StevenACoffman/skillet) renders and parses
round-trip, not free-form Markdown. Each rule is a `§`-numbered header plus an indented
rationale, a discriminating counter-/preferred-example pair, and a source anchor:

```text
Source: The Go Programming Language
Scope:  Go

§1.1  [MUST][CODE]  Always close what you open.
      A leaked file descriptor exhausts the process's table.
      ✗  f, _ := os.Open(p)  // never closed
      ✓  f, _ := os.Open(p); defer f.Close()
      ↦  "defer is commonly used to close a file"
```

`[MUST]` and `[SHOULD]` are enforced. `[CONSIDER]` is advisory. The `↦` line is the
source anchor the provenance check looks for.

**Findings.** Checks and the cold critic speak one JSON schema
([`skillet/finding`](https://github.com/StevenACoffman/skillet)): a list of diagnostics
`{severity, category, path, message}`. An `error`-severity diagnostic is *blocking*. A
`warning` is not.

## The Pipeline

```text
distill ─▶ [agent writes rules] ─▶ synthesize ─▶ [agent merges] ─▶ fmt ─▶ verify ─┐
                                                             critic ─▶ [agent grades] ─┤
                                                                                       ▼
                                                                        gate / loop / budget
                                                                                       │
                                              rework ─▶ [agent revises] ◀──────────────┘
```

`pipeline.sh` is that whole loop as one command, holding the attempt counter and keeping
every round's artifacts. Its paths are flags — `--src-dir`, `--out-root`, `--max-attempts`,
each also settable as `CANONIZER_*` — so it is not tied to one workspace; run it with
`--help` for the defaults. canonizer itself calls no model: the four `[agent …]` steps are
where a model runs, and every command below is deterministic.

A worked run:

```sh
# 1. Distill each source document into a per-source prompt the agent runs to write rules.
canonizer distill --source ./docs --out ./prompts --rulesout ./rulesets
#    Run each prompt from its own directory -- every link inside it is relative to that
#    directory, and a Markdown link carries no anchor, so an agent started anywhere else
#    resolves the "../" from the wrong place:
for p in ./prompts/*_prompt.md; do
  ( cd "$(dirname "$p")" && claude -p < "$(basename "$p")" )
done
#    The agent writes each *_rules.md into --rulesout. Do not capture its stdout as the
#    artifact: the reply is a report about the work, and the ruleset is the file.

# 2. Merge the per-source rulesets into one synthesis prompt; the agent produces the
#    single candidate ruleset R.
canonizer synthesize --rulesets ./rulesets --rulesout ./synthesis --out ./synthesize_prompt.md

# 3. Deterministic checks: executability (the ✗/✓ pair) and, with --source, provenance.
canonizer verify --ruleset R.md --source S.md --out findings.json
#    --source is repeatable. A synthesized ruleset derives from every source it was merged
#    from, so pass them all: an anchor is present when any source contains it. Sources are
#    iterated, never joined, so no anchor can match text spanning two documents.

# 4. Cold critic: emit a prompt giving a fresh grader only S and R; the agent runs it and
#    writes its findings JSON.
canonizer critic --source S.md --ruleset R.md \
  --findingsout critic_findings.json --out critic_prompt.md
#    --findingsout is the file the prompt tells the grader to write. Without it the prompt
#    names no destination and the grader prints its JSON into a transcript.

# 5. Gate on the findings: exit non-zero while anything blocks.
canonizer gate --findings findings.json

# 6. If anything blocks and attempts remain, emit the revision prompt and let the agent
#    revise R in place, then repeat from 3. pipeline.sh is that driver: it holds the
#    attempt counter and keeps every round's artifacts.
canonizer rework --ruleset R.md --findings findings.json --findings critic_findings.json \
  --out rework_prompt.md
```

Every command that hands work to an agent names the file the agent must write —
`distill --rulesout`, `synthesize --rulesout`, `critic --findingsout`. The reply is a report
about the work; the artifact is the file.

## Commands

Every command is deterministic — canonizer calls no model. `--help` on any of them carries
the reasoning; this list is for finding the right one.

**Producing prompts an agent runs.** Each names the file the agent must write, because the
reply is a report about the work and the artifact is the file.

- **`distill --source DIR --out DIR [--rulesout DIR]`** — one distillation prompt per
  source in a tree; `--rulesout` is where each agent writes its ruleset.
- **`synthesize --rulesets DIR [--rulesout DIR] [--out FILE]`** — one prompt merging the
  distilled rulesets into a single candidate.
- **`critic --source PATH... --ruleset PATH [--findingsout FILE] [--out FILE]`** — a
  cold-critic prompt for a fresh grader, which sees only the sources and the ruleset.
- **`rework --ruleset PATH --findings FILE... [--out FILE]`** — the prompt an agent runs to
  revise a ruleset against its findings. The only one that asks an agent to *edit* an
  artifact rather than produce one.

**Checking a ruleset.**

- **`verify --ruleset PATH [--source PATH]... [--proof PATH] [--out FILE] [--sign-off]`** —
  executability and provenance, as findings JSON. Reports hedging and unquantified
  thresholds as warnings that never block, and two per-document proportions: how many rules
  name a symbol a checker can see, and how many anchors name a section only.
  `--sign-off` appends a verification event to the ruleset, attributed to `identity.actor`
  in `--config` and never to a flag. It is refused on a run with blocking findings, on a run
  given no `--source`, and when no actor is configured.
- **`fmt --ruleset PATH [--check]`** — rewrite a ruleset into canonical form, which is what
  `verify`'s `non-canonical` finding measures against. `--check` reports and exits 1 without
  writing. No text is lost; wrapping moves.
- **`gate [--findings FILE] [--selftest]`** — exit 1 while any finding blocks. `--selftest`
  runs a planted-defect control first.

**Deciding what happens next.**

- **`budget [--findings FILE] --attempt K --max N`** — ship / rework / needs-human, exiting
  0 / 2 / 1.
- **`loop --source PATH... --ruleset PATH [--findings FILE] --attempt K --max N`** — one
  round: verify, merge the critic's findings, decide.
- **`calibrate --samples PATH`** — the critic's calibration (ECE/MCE/Brier) from a review log.
- **`version [--json]`** — version information.

`--source` is repeatable on `verify`, `critic` and `loop`: a synthesized ruleset derives from
every document it was merged from, so an anchor is present when any source contains it.
Sources are iterated, never joined, so no anchor can match text spanning two documents.

## The Rework Loop

`loop` runs **one deterministic round** (verify the candidate ruleset, merge the agent's
cold-critic findings, and decide under the rework budget) and exits `0` (ship), `2`
(rework), or `1` (needs-human). It holds no state. The attempt counter is passed in, so a
thin driver wraps it and supplies the agent's steps between rounds:

```sh
K=1; MAX=3
while true; do
  canonizer critic --source S.md --ruleset R.md \
    --findingsout "critic_findings_$K.json" --out "critic_prompt_$K.md"
  # agent runs critic_prompt_$K.md and writes critic_findings_$K.json
  canonizer loop --source S.md --ruleset R.md --findings "critic_findings_$K.json" \
    --attempt "$K" --max "$MAX"
  case $? in
    0) echo "ship"; break ;;                 # adopt R.md
    2) canonizer rework --ruleset R.md --findings "critic_findings_$K.json" \
         --out "rework_prompt_$K.md"          # agent revises R.md, then retry
       K=$((K+1)) ;;
    1) echo "needs human"; break ;;           # budget spent, blocked
  esac
done
```

## Policies

**Model policy (convention).** Run the emitted distill/synthesize/critic prompts on a
reasoning-class model. canonizer fills prompts an agent runs, so it *cannot observe* the
model and does not gate on it. That stays the operator's responsibility. `loop --model`
records the operator's attestation for audit, explicitly labeled unverified.

**Refinement policy (enforced).** The ship gate is the cold critic plus the deterministic
findings gate, never a model self-score. The invariant "a blocked ruleset never ships" is
test-enforced: the decision's only inputs are the blocking state and the attempt count.

**Calibration (audit, not a gate).** `calibrate` reports how well the critic's stated
confidence matched how its flags held up on review (ECE/MCE/Brier over a
`{confidence, correct}` log). It surfaces an over- or under-confident critic. It never
blocks adoption, because the ship gate stays findings-based.

**What a clean gate does and does not license (documentation, not a mechanism).** A run
where `verify` reports nothing blocking and `gate` exits zero means exactly this: *no
deterministic check objected, and one cold critic did not object either.* It is worth
stating what that is not, because the short way to say it — "the ruleset is verified" —
claims all four of the following, and the pipeline supports none of them.

- **Not "the rules are correct."** The deterministic checks are structural: a rule carries a
  discriminating ✗/✓ pair, cites an anchor that appears in the source, and the file
  round-trips through the canonical form. None of them reads the rule for truth.
- **Not "the anchor supports the claim."** `Provenance` finds the quoted text in the source.
  Whether the passage *says what the rule says it says* is the critic's `unsupported`
  judgment, and the critic is one grader running one prompt.
- **Not "the ruleset covers its scope."** Every rule can be individually sound while the set
  omits most of what the `Scope:` line promises. That is the `coverage` category, and it is
  a judgment rather than a check.
- **Not "nothing was flagged."** `Specificity` and `Conflicts` are advisory by design and a
  clean gate may still carry them; so may a critic's coverage record naming what it did not
  examine. A zero exit means *nothing blocking*, not *nothing found*.

The reason to write this down is that the failure is silent. A gate that blocks says why; a
gate that passes says nothing, and the word chosen for that silence in a commit message or a
PR description is where the overclaim enters. Prefer "passed canonizer's structural gate"
over "verified".

## Development

```sh
go test ./...
golangci-lint run ./...
climax lint          # structural drift check for the climax CLI scaffold
```

canonizer is built on [`ff/v4`](https://github.com/peterbourgon/ff) and scaffolded with
[climax](https://github.com/StevenACoffman/climax). The deterministic cores
(ruleset/finding/judge/proof/calibration) live in
[skillet](https://github.com/StevenACoffman/skillet).
