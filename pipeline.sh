#!/usr/bin/env bash
#
# pipeline.sh — distill one source subdirectory into canonical rulesets.
#
# Usage: ./pipeline.sh [--src-dir DIR] [--out-root DIR] [--max-attempts N] SUBDIR [LABEL]
#
# Run with --help for the flags and their defaults. Every path is a flag or an environment
# variable because the values built in below describe one workspace and nothing else could
# use them; a flag wins over CANONIZER_SRC_DIR / CANONIZER_OUT_ROOT / CANONIZER_MAX_ATTEMPTS,
# which win over the default.
#
# SUBDIR names a directory under --src-dir. Prompts are written under
# rulesets/prompts/SUBDIR, and each prompt is told to write its ruleset into
# rulesets/distilled/SUBDIR via canonizer distill --rulesout.
#
# LABEL suffixes every directory below, so one source tree can be run twice and both results
# kept. Two runs of the same sources are what tells a property of the source apart from
# nondeterminism in the run, which no number taken across different sources can do.
#
# The stages, and the directory each writes:
#
#   prompts/           distill prompts, one per source        (agent writes distilled/)
#   distilled/         one ruleset per source
#   synthesis_prompts/ the single merge prompt                (agent writes synthesis/)
#   synthesis/         the candidate ruleset, one per attempt
#   findings/          verify findings, one per attempt
#   critique/          cold-critic prompts and their findings, one per attempt
#
# **Nothing is overwritten.** Every artifact after synthesis carries its attempt number, so a
# refine loop leaves the whole history on disk rather than the last state of it.
#
# **The refine loop is the driver `canonizer loop` documents itself as needing**: canonizer
# calls no model, so the critic and the rework are the agent's steps and the attempt counter
# is held here.
#
# This is step 1 of canonizer's pipeline (see README, "A worked run"): distill emits one
# prompt per source and an agent runs each prompt.
#
# **The agent writes the ruleset; this script does not capture it.** Each prompt ends by
# naming the file it should produce, so the artifact is the agent's to write and whatever
# it prints is progress for a person to read. Redirecting its stdout into a *_rules.md was
# how the first run produced eight files of plan prose: the reply is a report, not the
# document.
#
# **The agent runs from the prompt's own directory**, because every link inside a prompt is
# relative to it. A Markdown link carries no anchor, so an agent started anywhere else
# resolves the "../" from the wrong place and reads the wrong source.
#
# **That directory is also the sandbox, which is why --add-dir is not optional.** Claude
# Code confines a session to its working directory, so running from the prompt's directory
# makes the links resolve and then denies the reads they resolve to. The source subtree and
# the rules directory are granted explicitly -- those two and nothing wider, so a prompt
# cannot reach a sibling source it was not asked to distil.
#
# **--add-dir grants reads; writing needs acceptEdits as well.** Measured: with only
# --add-dir the agent read the source and produced a correct 23-rule ruleset, then reported
# that saving it "require[s] a permission grant, and this session is non-interactive so the
# prompt can't be answered". The ruleset went to stdout and no file was written. acceptEdits
# answers that prompt in advance, and is scoped by the two --add-dir grants above rather
# than opening the machine.
set -euo pipefail

# has_cmd NAME — true if NAME is an executable file on $PATH.
# Ignores shell functions, aliases, and builtins of the same name.
has_cmd() {
    if [ -n "${ZSH_VERSION:-}" ]; then
        builtin whence -p -- "$1" >/dev/null 2>&1
    elif [ -n "${BASH_VERSION:-}" ]; then
        builtin type -P -- "$1" >/dev/null 2>&1
    else
        command -v -- "$1" >/dev/null 2>&1
    fi
}

# Defaults, overridable by environment and then by flag.
#
# **These two are one workspace's layout, not a convention**, which is the whole reason the
# flags exist: nothing but this checkout can use the paths baked in below. They stay the
# defaults so the invocation the author types keeps working; a relative default would be
# more portable and would break the only current caller, which buys nothing.
#
# **The environment names are prefixed and the plain ones are not read.** `SRC_DIR` is a name
# a CI job or a sourced profile may already hold, and a script that silently picks up someone
# else's variable is worse than one that ignores it.
SRC_DIR="${CANONIZER_SRC_DIR:-${HOME}/Documents/agent-orange/go-advice/Sources}"
OUT_ROOT="${CANONIZER_OUT_ROOT:-${HOME}/Documents/git/rulesets}"

# MAX_ATTEMPTS bounds the refine loop. It is the --max canonizer budget reads, and the reason
# a bound exists at all: a loop that reworks until it passes will eventually pass by attrition
# rather than by the ruleset improving.
#
# The bare name is still honoured because it was the only name until now, and silently
# ignoring an existing MAX_ATTEMPTS=1 would change a run without saying so.
MAX_ATTEMPTS="${CANONIZER_MAX_ATTEMPTS:-${MAX_ATTEMPTS:-3}}"

usage() {
    printf 'usage: %s [--src-dir DIR] [--out-root DIR] [--max-attempts N] SUBDIR [LABEL]\n' \
        "$(basename "$0")"
    printf '\n'
    printf '  --src-dir DIR       source tree holding SUBDIR      (default %s)\n' "$SRC_DIR"
    printf '  --out-root DIR      where every artifact is written (default %s)\n' "$OUT_ROOT"
    printf '  --max-attempts N    refine rounds before escalating (default %s)\n' "$MAX_ATTEMPTS"
    printf '\n'
    printf '  CANONIZER_SRC_DIR, CANONIZER_OUT_ROOT and CANONIZER_MAX_ATTEMPTS set the same\n'
    printf '  values; a flag wins over the environment, which wins over the default.\n'
}

# Flags are parsed before the positionals, which stay positional because SUBDIR and LABEL are
# what actually gets typed.
#
# An unknown flag is an error rather than a positional: treating `--src-dr` as SUBDIR would
# fail later with "no such source directory: .../--src-dr", which reads as a typo in the
# wrong place. `--help` exits 0 where a usage error exits 2, so a caller checking status can
# tell a question from a mistake.
while [ "$#" -gt 0 ]; do
    case "$1" in
        --src-dir)      SRC_DIR="${2-}"; shift 2 ;;
        --src-dir=*)    SRC_DIR="${1#*=}"; shift ;;
        --out-root)     OUT_ROOT="${2-}"; shift 2 ;;
        --out-root=*)   OUT_ROOT="${1#*=}"; shift ;;
        --max-attempts) MAX_ATTEMPTS="${2-}"; shift 2 ;;
        --max-attempts=*) MAX_ATTEMPTS="${1#*=}"; shift ;;
        -h | --help)    usage; exit 0 ;;
        --)             shift; break ;;
        -*)
            printf '%s: unknown flag: %s\n' "$(basename "$0")" "$1" >&2
            usage >&2
            exit 2
            ;;
        *) break ;;
    esac
done

if [ "$#" -lt 1 ] || [ "$#" -gt 2 ]; then
    usage >&2
    exit 2
fi
D="$1"
LABEL="${2-}"

# A label suffixes both output directories, so the same sources can be distilled more than
# once without the second run overwriting the first. That is the only way to see whether a
# score varies while the input does not: the corpus's backtick rate ranged 11% to 78% across
# eight *different* sources, which cannot distinguish a property of the source from
# nondeterminism in the run.
#
# **The prompts directory is suffixed too, and that is not tidiness.** Each prompt names the
# file its agent should write, so a prompt generated for one label points at that label's
# rules directory. Sharing one prompts directory would send both runs' output to the same
# place and measure nothing.
#
# Validated as a single path segment: a label reaching this from a shell history could
# otherwise walk out of the output tree, and these two variables are used unquoted nowhere
# but they are used to build paths that get created.
if [ -n "$LABEL" ]; then
    case "$LABEL" in
        *[!A-Za-z0-9._-]* | -* | .* )
            printf '%s: LABEL must be letters, digits, dot, underscore or dash, and may not begin with a dot or dash: %s\n' \
                "$(basename "$0")" "$LABEL" >&2
            exit 2
            ;;
    esac
    SUFFIX="-${LABEL}"
else
    SUFFIX=""
fi

PROMPT_DIR="${OUT_ROOT}/prompts/${D}${SUFFIX}"
RULES_DIR="${OUT_ROOT}/distilled/${D}${SUFFIX}"
SYNTH_PROMPT_DIR="${OUT_ROOT}/synthesis_prompts/${D}${SUFFIX}"
SYNTH_DIR="${OUT_ROOT}/synthesis/${D}${SUFFIX}"
FINDINGS_DIR="${OUT_ROOT}/findings/${D}${SUFFIX}"
CRITIQUE_DIR="${OUT_ROOT}/critique/${D}${SUFFIX}"

for cmd in canonizer claude; do
    if ! has_cmd "$cmd"; then
        printf '%s: %s is not on PATH\n' "$(basename "$0")" "$cmd" >&2
        exit 1
    fi
done

# Both paths are checked up front, and each message names the flag that set the value.
# A mistyped --out-root otherwise surfaced much later as a mkdir failure deep in the run,
# and "no such directory" without the flag name is a hunt rather than a fix.
if [ ! -d "$SRC_DIR" ]; then
    printf '%s: --src-dir is not a directory: %s\n' "$(basename "$0")" "$SRC_DIR" >&2
    exit 1
fi
if [ ! -d "${SRC_DIR}/${D}" ]; then
    printf '%s: no such source directory: %s\n' "$(basename "$0")" "${SRC_DIR}/${D}" >&2
    printf '  (SUBDIR is resolved under --src-dir %s)\n' "$SRC_DIR" >&2
    exit 1
fi
# The output root may not exist yet -- the run creates it -- but its parent must, or mkdir
# would fail after the distillation had already written prompts somewhere else.
if [ ! -d "$OUT_ROOT" ] && [ ! -d "$(dirname "$OUT_ROOT")" ]; then
    printf '%s: --out-root %s does not exist and neither does its parent\n' \
        "$(basename "$0")" "$OUT_ROOT" >&2
    exit 1
fi

mkdir -p "$PROMPT_DIR" "$RULES_DIR" "$SYNTH_PROMPT_DIR" "$SYNTH_DIR" \
    "$FINDINGS_DIR" "$CRITIQUE_DIR"

# run_agent NAME PROMPT EXTRA_DIR... -- run one prompt from its own directory.
#
# Factored out because the distill stage and the two synthesis-stage agents need identical
# treatment and had drifted apart when it was written twice: run from the prompt's directory
# so its relative links resolve, grant the directories it must read and write, and show the
# output rather than capturing it. The artifact is the file the agent writes; its reply is a
# report about the work.
run_agent() {
    _label="$1"; _prompt="$2"; shift 2
    _grants=()
    for _d in "$@"; do _grants+=(--add-dir "$_d"); done
    printf '\n=== %s\n' "$_label" >&2
    if ! ( cd "$(dirname "$_prompt")" \
        && claude -p --permission-mode acceptEdits "${_grants[@]}" \
            < "$(basename "$_prompt")" ); then
        printf '%s: claude failed on %s\n' "$(basename "$0")" "$_prompt" >&2
        return 1
    fi
}

canonizer distill --source "${SRC_DIR}/${D}" --out "$PROMPT_DIR" --rulesout "$RULES_DIR"

# Collect the prompts before looping. nullglob makes a directory with no prompts an empty
# array rather than one iteration over the literal pattern, which is what an unquoted glob
# does by default and is indistinguishable from a real file named "*_prompt.md".
shopt -s nullglob
prompts=("${PROMPT_DIR}"/*_prompt.md)
shopt -u nullglob

if [ "${#prompts[@]}" -eq 0 ]; then
    printf '%s: distill wrote no prompts to %s\n' "$(basename "$0")" "$PROMPT_DIR" >&2
    exit 1
fi

for p in "${prompts[@]}"; do
    run_agent "$(basename "$p")" "$p" "${SRC_DIR}/${D}" "$RULES_DIR" || exit 1
done

shopt -s nullglob
rulesets=("${RULES_DIR}"/*_rules.md)
shopt -u nullglob
if [ "${#rulesets[@]}" -eq 0 ]; then
    printf '%s: no rulesets in %s; the distill agents wrote nothing\n' \
        "$(basename "$0")" "$RULES_DIR" >&2
    exit 1
fi
printf '\ndistilled %d ruleset(s)\n' "${#rulesets[@]}" >&2

# ---------------------------------------------------------------------------------------
# Stage 2: merge the per-source rulesets into one candidate.
# ---------------------------------------------------------------------------------------

SYNTH_PROMPT="${SYNTH_PROMPT_DIR}/${D}_synthesis_prompt.md"

# --rulesout writes the destination into the prompt, the same way distill's does for each
# per-source ruleset. Without it the prompt names no output path and an agent prints the
# merged ruleset instead of writing one.
canonizer synthesize --rulesets "$RULES_DIR" --rulesout "$SYNTH_DIR" --out "$SYNTH_PROMPT"

run_agent "synthesis" "$SYNTH_PROMPT" "$RULES_DIR" "$SYNTH_DIR" || exit 1

# The candidate is discovered rather than named. `synthesize --rulesout` derives the
# filename through skillet's naming rules, and spelling that out again here is how the two
# would start disagreeing -- the label form alone (`benbjohnson-run2`) already normalises
# differently than a shell substitution would.
shopt -s nullglob
candidates=("${SYNTH_DIR}"/*_rules.md)
shopt -u nullglob
if [ "${#candidates[@]}" -ne 1 ]; then
    printf '%s: expected exactly one ruleset in %s, found %d\n' \
        "$(basename "$0")" "$SYNTH_DIR" "${#candidates[@]}" >&2
    exit 1
fi
CANDIDATE="${candidates[0]}"
printf 'candidate: %s\n' "$CANDIDATE" >&2

# ---------------------------------------------------------------------------------------
# Stage 3: the sources.
#
# verify, critic and loop each take a repeatable --source, so a synthesized ruleset is
# checked against every document it was merged from. An anchor is present when any source
# contains it.
#
# **This replaced a concatenation, and the difference is a false positive.** The union was
# the honest answer while only one --source existed, but textnorm.Fold collapses whitespace,
# so joining two documents let an anchor match text spanning the seam between them -- a
# quotation no source contains. Iterating cannot produce that, and it keeps each source
# hashable on its own, which --proof and --against-proof depend on.
# ---------------------------------------------------------------------------------------

shopt -s nullglob
source_files=("${SRC_DIR}/${D}"/*.md)
shopt -u nullglob
if [ "${#source_files[@]}" -eq 0 ]; then
    printf '%s: no sources in %s\n' "$(basename "$0")" "${SRC_DIR}/${D}" >&2
    exit 1
fi
SOURCE_FLAGS=()
for s in "${source_files[@]}"; do SOURCE_FLAGS+=(--source "$s"); done
printf 'sources: %d document(s)\n' "${#source_files[@]}" >&2

# ---------------------------------------------------------------------------------------
# Stage 4: the refine loop -- verify, critique, decide, rework.
#
# canonizer calls no model, so the critic and the rework are the agent's steps and the
# attempt counter is held here. That division is `canonizer loop`'s own description of the
# driver it needs, and this is that driver.
#
# **Every artifact carries its attempt number.** A loop that overwrote its findings would
# leave only the last round on disk, and the question worth asking afterwards is whether the
# rework improved the ruleset -- which needs both rounds.
# ---------------------------------------------------------------------------------------

verdict="unknown"
attempt=1
while [ "$attempt" -le "$MAX_ATTEMPTS" ]; do
    printf '\n───── attempt %d of %d ─────\n' "$attempt" "$MAX_ATTEMPTS" >&2

    verify_out="${FINDINGS_DIR}/verify_${attempt}.json"
    critic_prompt="${CRITIQUE_DIR}/critic_prompt_${attempt}.md"
    critic_out="${CRITIQUE_DIR}/critic_findings_${attempt}.json"

    # Deterministic checks first: they cost nothing and a ruleset that fails them has
    # defects a grader should not have to spend a model call finding.
    canonizer verify --ruleset "$CANDIDATE" "${SOURCE_FLAGS[@]}" --out "$verify_out"

    # The cold critic sees only the source and the candidate -- never the distilled rulesets
    # or this script's opinion of them -- which is what makes its finding independent.
    canonizer critic "${SOURCE_FLAGS[@]}" --ruleset "$CANDIDATE" \
        --findingsout "$critic_out" --out "$critic_prompt"
    run_agent "critic (attempt ${attempt})" "$critic_prompt" \
        "${SRC_DIR}/${D}" "$SYNTH_DIR" "$CRITIQUE_DIR" || exit 1

    if [ ! -s "$critic_out" ]; then
        printf '%s: the critic agent wrote no findings to %s\n' \
            "$(basename "$0")" "$critic_out" >&2
        exit 1
    fi

    # loop merges the deterministic and the graded findings and returns the verdict:
    # 0 ship, 2 rework, 1 needs-human. Captured rather than allowed to abort, because a
    # rework verdict is a normal outcome of this loop and not an error in it.
    code=0
    canonizer loop "${SOURCE_FLAGS[@]}" --ruleset "$CANDIDATE" --findings "$critic_out" \
        --attempt "$attempt" --max "$MAX_ATTEMPTS" || code=$?

    case "$code" in
        0) verdict="ship"; break ;;
        2) verdict="rework" ;;
        1) verdict="needs-human"; break ;;
        *) printf '%s: loop exited %d\n' "$(basename "$0")" "$code" >&2; exit "$code" ;;
    esac

    # Keep this attempt's candidate before the agent edits it, so the loop leaves a history
    # rather than the last state of one file.
    cp "$CANDIDATE" "${SYNTH_DIR}/${D}_rules.attempt${attempt}.md"

    # The revision prompt is `canonizer rework`'s, not this script's. It used to be a
    # heredoc here -- unversioned, untested, and invisible to the template tests that guard
    # every other prompt against silently losing the canonical form.
    rework_prompt="${CRITIQUE_DIR}/rework_prompt_${attempt}.md"
    canonizer rework --ruleset "$CANDIDATE" \
        --findings "$verify_out" --findings "$critic_out" --out "$rework_prompt"

    run_agent "rework (attempt ${attempt})" "$rework_prompt" \
        "${SRC_DIR}/${D}" "$SYNTH_DIR" "$CRITIQUE_DIR" "$FINDINGS_DIR" || exit 1

    attempt=$((attempt + 1))
done

# ---------------------------------------------------------------------------------------
# Stage 5: the gate has the last word.
#
# It re-reads the final deterministic findings rather than trusting the loop's verdict: the
# loop decided under a budget, and the gate asks the one question a budget cannot soften --
# does anything still block.
# ---------------------------------------------------------------------------------------

final_verify="${FINDINGS_DIR}/verify_final.json"
canonizer verify --ruleset "$CANDIDATE" "${SOURCE_FLAGS[@]}" --out "$final_verify"

gate_code=0
canonizer gate --findings "$final_verify" || gate_code=$?

printf '\n───── result ─────\n' >&2
printf 'candidate : %s\n' "$CANDIDATE" >&2
printf 'verdict   : %s after %d attempt(s)\n' "$verdict" "$attempt" >&2
printf 'findings  : %s\n' "$FINDINGS_DIR" >&2
printf 'critiques : %s\n' "$CRITIQUE_DIR" >&2

if [ "$gate_code" -ne 0 ]; then
    printf '\ngate: the candidate still has blocking findings; it is not shippable.\n' >&2
    printf 'every attempt is on disk, so the rounds can be compared rather than re-run.\n' >&2
    exit "$gate_code"
fi

printf '\ngate: clean. The refined ruleset is %s\n' "$CANDIDATE" >&2

# The per-source rulesets are inputs to the synthesis rather than the deliverable, and this
# script no longer tells a caller how to verify them by hand -- it verifies the candidate
# itself. To inspect one of the distilled inputs:
printf '\nto inspect a distilled input:\n' >&2
printf '  canonizer verify --ruleset %s/NAME_rules.md --source %s/NAME.md\n' \
    "$RULES_DIR" "${SRC_DIR}/${D}" >&2
