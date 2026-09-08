#!/usr/bin/env bash
#
# pipeline.sh — distill one source subdirectory into canonical rulesets.
#
# Usage: ./pipeline.sh SUBDIR [LABEL]
#
# SUBDIR names a directory under $SRC_DIR. Prompts are written under
# rulesets/prompts/SUBDIR, and each prompt is told to write its ruleset into
# rulesets/distilled/SUBDIR via canonizer distill --rulesout.
#
# LABEL suffixes both directories -- prompts/SUBDIR-LABEL and distilled/SUBDIR-LABEL -- so
# one source tree can be distilled twice and both results kept. Two runs of the same sources
# are what tells a property of the source apart from nondeterminism in the run, which no
# number taken across different sources can do.
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

if [ "$#" -lt 1 ] || [ "$#" -gt 2 ]; then
    printf 'usage: %s SUBDIR [LABEL]\n' "$(basename "$0")" >&2
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

SRC_DIR="${HOME}/Documents/agent-orange/go-advice/Sources"
OUT_ROOT="${HOME}/Documents/git/rulesets"
PROMPT_DIR="${OUT_ROOT}/prompts/${D}${SUFFIX}"
RULES_DIR="${OUT_ROOT}/distilled/${D}${SUFFIX}"

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

# --source is named here because omitting it is not a smaller check, it is a different one.
# Without it the anchor gates are replaced by an advisory, and the blocking count on the
# first real batch fell from 17 to 5, 15 to 3 and 8 to 2 -- so the shorter invocation reads
# as a better result while having examined less. --sign-off refuses outright without it.
printf 'verify them with:\n' >&2
printf '  canonizer verify --ruleset %s/NAME_rules.md \\\n' "$RULES_DIR" >&2
printf '    --source %s/NAME.md\n' "${SRC_DIR}/${D}" >&2
printf 'a ruleset reported non-canonical is made canonical by:\n' >&2
printf '  canonizer fmt --ruleset %s/NAME_rules.md\n' "$RULES_DIR" >&2
