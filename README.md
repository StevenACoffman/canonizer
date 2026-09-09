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
distill ─▶ [agent writes rules] ─▶ synthesize ─▶ [agent merges] ─▶ verify ─┐
                                                             critic ─▶ [agent grades] ─┤
                                                                                       ▼
                                                                        gate / loop / budget
```

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
canonizer critic --source S.md --ruleset R.md --out critic_prompt.md
#    the agent runs critic_prompt.md and writes critic_findings.json

# 5. Gate on the findings: exit non-zero while anything blocks.
canonizer gate --findings findings.json

# 6. If anything blocks and attempts remain, emit the revision prompt and let the agent
#    revise R in place, then repeat from 3. pipeline.sh is that driver: it holds the
#    attempt counter and keeps every round's artifacts.
canonizer rework --ruleset R.md --findings findings.json --findings critic_findings.json \
  --out rework_prompt.md
```

## Commands

- **`distill --source DIR --out DIR`** — Fill a distillation prompt for every source in a
  tree.
- **`synthesize --rulesets DIR [--rulesout DIR] [--out FILE]`** — Assemble one synthesis
  prompt from distilled rulesets. `--rulesout` writes the destination into the prompt, as
  `distill`'s does; without it the prompt names no output path and an agent prints the
  merged ruleset instead of writing one.
- **`verify --ruleset PATH [--source PATH]... [--proof PATH] [--out FILE] [--sign-off]`** — Check
  executability and provenance, then emit findings JSON. `--proof` writes a packet binding
  the ruleset (and source) to their exact bytes. It also reports two kinds of vagueness as
  **warnings that never block**: *hedging*, a rule using a discretion phrase
  (`it depends`), and an *unquantified* amount, a rule that commits to an action while
  leaving the threshold it turns on unstated (`too many files`, `roughly 10K SLOC`). The two
  vocabularies are disjoint — one says *you may choose*, the other *some unstated
  quantity*: such a rule is sometimes correct and a deterministic check cannot tell which,
  so this reports and does not decide.
  It reports two proportions per run rather than per rule: how many enforced rules **name a
  symbol a checker can see**, and how many anchors **name a section only**, whose provenance
  therefore went unsearched. Both were per-rule findings once. The first claimed a rule
  "names no object, tool or API" and fired on 63 of 147 rules where the claim was false —
  what it actually measured is backtick-convention adherence, which is true of a document
  and false of a rule, and which varies 11% to 76% across distillations from one prompt. The
  per-rule version of that question belongs to `critic`, whose `vague` test asks it in the
  same words and blocks on it.
  `--sign-off` appends a verification event to the ruleset's frontmatter (`format: 4`),
  recording who confirmed it. The actor comes from `identity.actor` in `--config`
  (default `.canonizer.yaml`) and **never from a flag** — a caller-supplied actor would let
  anyone mint a human's sign-off. It is **attributable rather than authenticated**: it says
  which actor this checkout was configured as, not who was at the keyboard.
  A sign-off is refused when the run found blocking findings (it would attest to a state
  the same run disproved), when `--source` was omitted (nothing searched for the anchors,
  so the event would vouch for unexamined provenance), and when no actor is configured (an
  event with no actor records nothing). **Every ruleset in the current corpus is refused**,
  each carrying 3–17 blocking findings; that is the gate working, not a defect.
- **`fmt --ruleset PATH [--check]`** — Rewrite a ruleset into the exact form
  `ruleset.Render` emits, which is what `verify`'s `non-canonical` finding measures against.
  `--check` reports without writing and exits 1, the same split exegesis `normalize` uses; an
  already-canonical file is left untouched. No text is lost — the word sequence is identical
  before and after — but wrapping moves: a rationale hand-wrapped across three lines becomes
  one long line. This is the command that clears `--sign-off`'s non-canonical refusal.
- **`critic --source PATH [--source PATH] --ruleset PATH [--out FILE]`** — Emit a
  cold-critic prompt for a fresh grader. `--source` is repeatable, so a synthesized ruleset
  is graded against every document it was merged from.
- **`rework --ruleset PATH --findings FILE [--findings FILE] [--out FILE]`** — Emit the
  prompt an agent runs to revise a ruleset against its findings. `--findings` is repeatable
  because a refine round produces two documents — the deterministic checks and the grader —
  and reworking against one resolves half the round. It is the only prompt canonizer emits
  that asks an agent to *edit* an artifact rather than produce one.
- **`gate [--findings FILE] [--selftest]`** — Block (exit 1) while any finding is blocking.
  `--selftest` runs a planted-defect control.
- **`budget [--findings FILE] --attempt K --max N`** — Decide ship / rework / needs-human,
  exiting 0 / 2 / 1.
- **`loop --source PATH --ruleset PATH [--findings FILE] --attempt K --max N`** — One
  deterministic rework round: verify, merge critic findings, and decide.
- **`calibrate --samples PATH`** — Report the critic's calibration (ECE/MCE/Brier) from a
  review log.
- **`version [--json]`** — Print version information.

Run `canonizer <command> --help` for the full flag surface of any command.

## The Rework Loop

`loop` runs **one deterministic round** (verify the candidate ruleset, merge the agent's
cold-critic findings, and decide under the rework budget) and exits `0` (ship), `2`
(rework), or `1` (needs-human). It holds no state. The attempt counter is passed in, so a
thin driver wraps it and supplies the agent's steps between rounds:

```sh
K=1; MAX=3
while true; do
  canonizer critic --source S.md --ruleset R.md --out critic_prompt.md
  # agent runs critic_prompt.md -> critic_findings.json, and reworks R.md if asked
  canonizer loop --source S.md --ruleset R.md --findings critic_findings.json \
    --attempt "$K" --max "$MAX"
  case $? in
    0) echo "ship"; break ;;                 # adopt R.md
    2) K=$((K+1)) ;;                          # rework and retry
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
