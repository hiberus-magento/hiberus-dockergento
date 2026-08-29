#!/usr/bin/env bash

#
# Something on the screen before a hundred milliseconds.
#
# `hm db snapshot` on a real catalogue is four minutes of nothing; `hm mysql -i` is longer; and
# `hm clean` spends twenty-five seconds working out volume sizes without a word. The terminal
# shows a cursor, and the person in front of it cannot tell a slow operation from a hung one.
#
# The interesting part is not the spinner, it is the rule: the label is printed by the same
# statement that begins the work, so the promise is kept by construction rather than by measuring
# anything. The spinner is a detail that says the process is still alive — and it is the wrong
# thing entirely in a log file, which is why nothing animates unless somebody is watching.
#

HM_PROGRESS_PID=""
HM_PROGRESS_LABEL=""
HM_PROGRESS_STARTED=0

#
# One decision, in one place. It answers no unless stdout is a terminal, the output is meant for a
# person, the terminal can render it, and the run is interactive.
#
hm_progress_animates() {
    [ -t 1 ] || return 1
    [ "${HM_OUTPUT_FORMAT:-text}" == "json" ] && return 1
    [ -n "${NO_COLOR:-}" ] && return 1
    [ -n "${HM_NO_COLOR:-}" ] && return 1
    [ -n "${HM_NON_INTERACTIVE:-}" ] && return 1
    [ -n "${HM_NO_PROGRESS:-}" ] && return 1

    case "${TERM:-}" in
        "" | dumb) return 1 ;;
    esac

    return 0
}

#
# Braille where the locale says UTF-8, ASCII otherwise: a terminal that renders the frames as
# mojibake is worse than one that renders a dash.
#
hm_progress_frames() {
    case "${LC_ALL:-${LC_CTYPE:-${LANG:-}}}" in
        *[Uu][Tt][Ff]*8*) printf '⠋ ⠙ ⠹ ⠸ ⠼ ⠴ ⠦ ⠧ ⠇ ⠏' ;;
        *) printf '| / - \\' ;;
    esac
}

hm_progress_elapsed() {
    local seconds="$1"

    if [ "$seconds" -lt 60 ]; then
        printf '%ds' "$seconds"
    else
        printf '%dm%02ds' "$((seconds / 60))" "$((seconds % 60))"
    fi
}

#
# A line, now. For work that prints its own output: the label goes first and the output follows,
# with nothing animated over it.
#
hm_step() {
    print_info "$1\n"
}

#
# A line with a spinner, for silent work whose own output belongs on stdout.
#
hm_start() {
    HM_PROGRESS_LABEL="$1"
    HM_PROGRESS_STARTED=$(date +%s)
    HM_PROGRESS_PID=""

    if ! hm_progress_animates; then
        print_info "$HM_PROGRESS_LABEL\n"
        return 0
    fi

    #
    # One background loop redrawing a single line. It is a `sleep` per tenth of a second, which
    # is the one place in this tool where a subprocess at that rate is affordable — it is not a
    # `jq`, and it ends when the work does.
    #
    (
        frames=$(hm_progress_frames)
        while :; do
            for frame in $frames; do
                printf '\r\033[2K%s %s' "$frame" "$HM_PROGRESS_LABEL"
                sleep 0.1
            done
        done
    ) 2>/dev/null &

    HM_PROGRESS_PID=$!
    disown "$HM_PROGRESS_PID" 2>/dev/null || true
}

#
# Ends it. The label is replaced by the outcome, with the elapsed time when it was long enough to
# be worth knowing: "done" says nothing, and "done in 4m12s" tells somebody what to expect next
# time.
#
hm_stop() {
    local status="${1:-0}" note="${2:-}"
    local seconds=$(( $(date +%s) - HM_PROGRESS_STARTED ))
    local elapsed=""

    [ "$seconds" -ge 2 ] && elapsed=" ($(hm_progress_elapsed "$seconds"))"

    if [ -n "$HM_PROGRESS_PID" ]; then
        kill "$HM_PROGRESS_PID" >/dev/null 2>&1
        wait "$HM_PROGRESS_PID" 2>/dev/null
        HM_PROGRESS_PID=""
        printf '\r\033[2K'
    fi

    if [ "$status" -eq 0 ]; then
        print_info "$HM_PROGRESS_LABEL ${note:-done}$elapsed\n"
    else
        print_warning "$HM_PROGRESS_LABEL failed$elapsed\n"
    fi

    HM_PROGRESS_LABEL=""
    return 0
}

#
# Runs a silent command with the spinner over it, keeping its output and printing it only if the
# command fails. Successful commands that had nothing to say stay quiet; failed ones say
# everything they had.
#
hm_working() {
    local label="$1"; shift
    local output status

    hm_start "$label"

    output=$("$@" 2>&1)
    status=$?

    hm_stop "$status"

    if [ "$status" -ne 0 ] && [ -n "$output" ]; then
        printf '%s\n' "$output" >&2
    fi

    return "$status"
}
