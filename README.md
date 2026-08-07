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
canonizer distill --source ./docs --out ./prompts
#    the agent runs each *_prompt.md and writes a canonical *_rules.md

# 2. Merge the per-source rulesets into one synthesis prompt; the agent produces the
#    single candidate ruleset R.
canonizer synthesize --rulesets ./rulesets --out ./synthesize_prompt.md

# 3. Deterministic checks: executability (the ✗/✓ pair) and, with --source, provenance.
canonizer verify --ruleset R.md --source S.md --out findings.json

# 4. Cold critic: emit a prompt giving a fresh grader only S and R; the agent runs it and
#    writes its findings JSON.
canonizer critic --source S.md --ruleset R.md --out critic_prompt.md
#    the agent runs critic_prompt.md and writes critic_findings.json

# 5. Gate on the findings: exit non-zero while anything blocks.
canonizer gate --findings findings.json
```

## Commands

- **`distill --source DIR --out DIR`** — Fill a distillation prompt for every source in a
  tree.
- **`synthesize --rulesets DIR [--out FILE]`** — Assemble one synthesis prompt from
  distilled rulesets.
- **`verify --ruleset PATH [--source PATH] [--proof PATH] [--out FILE]`** — Check
  executability and provenance, then emit findings JSON. `--proof` writes a packet binding
  the ruleset (and source) to their exact bytes.
- **`critic --source PATH --ruleset PATH [--out FILE]`** — Emit a cold-critic prompt for a
  fresh grader.
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
