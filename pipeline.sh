#!/usr/bin/env bash
#
# pipeline.sh — distill one source subdirectory into canonical rulesets.
#
# Usage: ./pipeline.sh SUBDIR
#
# SUBDIR names a directory under $SRC_DIR. Prompts are written under
# rulesets/prompts/SUBDIR, and each prompt is told to write its ruleset into
# rulesets/distilled/SUBDIR via canonizer distill --rulesout.
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

if [ "$#" -ne 1 ]; then
    printf 'usage: %s SUBDIR\n' "$(basename "$0")" >&2
    exit 2
fi
D="$1"

SRC_DIR="${HOME}/Documents/agent-orange/go-advice/Sources"
OUT_ROOT="${HOME}/Documents/git/rulesets"
PROMPT_DIR="${OUT_ROOT}/prompts/${D}"
RULES_DIR="${OUT_ROOT}/distilled/${D}"

for cmd in canonizer claude; do
    if ! has_cmd "$cmd"; then
        printf '%s: %s is not on PATH\n' "$(basename "$0")" "$cmd" >&2
        exit 1
    fi
done

if [ ! -d "${SRC_DIR}/${D}" ]; then
    printf '%s: no such source directory: %s\n' "$(basename "$0")" "${SRC_DIR}/${D}" >&2
    exit 1
fi

mkdir -p "$PROMPT_DIR" "$RULES_DIR"

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
    printf '\n=== %s\n' "$(basename "$p")" >&2
    # Run from the prompt's directory so its relative links resolve, and read the prompt
    # by name from there. Output is shown, not captured: the ruleset is the file the agent
    # writes, and this is the running commentary.
    if ! ( cd "$(dirname "$p")" \
        && claude -p --permission-mode acceptEdits \
            --add-dir "${SRC_DIR}/${D}" --add-dir "$RULES_DIR" \
            < "$(basename "$p")" ); then
        printf '%s: claude failed on %s\n' "$(basename "$0")" "$p" >&2
        exit 1
    fi
done

printf '\nran %d prompt(s); rulesets should be in %s\n' "${#prompts[@]}" "$RULES_DIR" >&2
printf 'verify them with: canonizer verify --ruleset %s/NAME_rules.md\n' "$RULES_DIR" >&2
