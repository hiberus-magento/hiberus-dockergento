#!/usr/bin/env bash
#
# The signal, through a real terminal and through a pipe.
#
# The rule is what is being tested: something on screen before the work starts, and nothing
# animated when nobody is watching. Both halves matter — escape sequences in a log file are as
# wrong as silence in a terminal.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

LAB=$(cd "$(mktemp -d)" && pwd -P)
trap 'rm -rf "$LAB"' EXIT

WORK="source '$COMPONENTS_DIR/print_message.sh'
source '$COMPONENTS_DIR/progress.sh'
hm_start 'Copiando el directorio de datos'
sleep 0.6
hm_stop 0"

printf '%s\n' "$WORK" > "$LAB/work.sh"

#
# script(1) comes in two flavours with incompatible argument order: BSD (macOS) takes the command
# after the file, util-linux takes it with -c. Its own stdin must be closed: it refuses to run
# when that is not a terminal, which is exactly the case here.
#
through_a_terminal() {
    if script -q /dev/null bash "$LAB/work.sh" >"$LAB/pty" 2>/dev/null </dev/null ||
        script -qec "bash '$LAB/work.sh'" /dev/null >"$LAB/pty" 2>/dev/null </dev/null; then
        cat "$LAB/pty"
        return 0
    fi

    echo "unsupported"
}

# ---------------------------------------------------------------- through a pipe

piped=$(bash "$LAB/work.sh" 2>&1)

test_case "a pipe is told what is happening"
assert_contains "$piped" "Copiando el directorio de datos"

test_case "and told when it finished, with the elapsed time"
assert_contains "$piped" "done"

#
# Looked for by the escape byte itself: busybox `tr` does not take two character classes the way
# GNU and BSD do, and deleting the printable characters to see what is left deletes half the
# alphabet on Alpine.
#
test_case "and receives no escape sequence, nor a carriage return"
case "$piped" in
    *$'\033'* | *$'\r'*) reached=escapes ;;
    *) reached=texto ;;
esac
assert_equals "texto" "$reached"

# ---------------------------------------------------------------- through a terminal

terminal=$(through_a_terminal)

if [ "$terminal" == "unsupported" ]; then
    echo "  - skipped: script(1) is not available in a usable form"
    echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
    exit 0
fi

test_case "a terminal is told the same thing"
assert_contains "$terminal" "Copiando el directorio de datos"

test_case "and it is animated"
assert_equals "true" \
    "$(printf '%s' "$terminal" | LC_ALL=C grep -qc $'\r' >/dev/null 2>&1 && echo true || echo false)"

test_case "the animation is erased rather than left on the screen"
assert_contains "$terminal" $'\033[2K'

test_case "and the line ends as a result, not as a frame"
assert_contains "$(printf '%s' "$terminal" | tail -2)" "done"

# ---------------------------------------------------------------- turning it off

test_case "NO_COLOR turns the animation off, terminal or not"
NO_COLOR=1 script -q /dev/null bash "$LAB/work.sh" >"$LAB/nocolor" 2>/dev/null </dev/null ||
    NO_COLOR=1 script -qec "bash '$LAB/work.sh'" /dev/null >"$LAB/nocolor" 2>/dev/null </dev/null
assert_equals "0" "$(LC_ALL=C grep -c $'\033\[2K' "$LAB/nocolor" || true)"

test_case "and so does a non-interactive run"
HM_NON_INTERACTIVE=1 script -q /dev/null bash "$LAB/work.sh" >"$LAB/noninteractive" 2>/dev/null </dev/null ||
    HM_NON_INTERACTIVE=1 script -qec "bash '$LAB/work.sh'" /dev/null >"$LAB/noninteractive" 2>/dev/null </dev/null
assert_equals "0" "$(LC_ALL=C grep -c $'\033\[2K' "$LAB/noninteractive" || true)"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
