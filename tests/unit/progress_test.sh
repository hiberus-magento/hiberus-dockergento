#!/usr/bin/env bash
#
# The rule: something on screen before the work starts, and nothing animated when nobody is
# watching.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"
source "$COMPONENTS_DIR/print_message.sh"
source "$COMPONENTS_DIR/progress.sh"

# ---------------------------------------------------------------- when to animate
#
# This test's own stdout is a pipe, so the answer is already no here for the most important
# reason of all: escape sequences in a pipe are noise, and in a JSON document they are a
# corruption.

test_case "a pipe is never animated"
hm_progress_animates && r=anima || r=no
assert_equals "no" "$r"

test_case "and the decision is one function, so nothing can disagree with it"
assert_equals "1" "$(grep -c '^hm_progress_animates()' "$COMPONENTS_DIR/progress.sh")"

test_case "every reason to stay quiet is checked"
for reason in 'HM_OUTPUT_FORMAT' 'NO_COLOR' 'HM_NON_INTERACTIVE' 'dumb' '[ -t 1 ]'; do
    assert_contains "$(cat "$COMPONENTS_DIR/progress.sh")" "$reason"
done

# ---------------------------------------------------------------- the plain shapes

test_case "a step says what is about to happen"
assert_contains "$(hm_step 'Importing the database...' 2>&1)" "Importing the database..."

test_case "an operation announces itself before it runs"
output=$(hm_start 'Freezing...' 2>&1)
assert_contains "$output" "Freezing..."
hm_stop 0 >/dev/null 2>&1

#
# Checked by looking for the escape byte itself rather than by deleting the printable ones:
# busybox `tr` does not take two character classes the way GNU and BSD do, and the test that
# tried it deleted half the alphabet on Alpine instead.
#
test_case "and no escape sequence reaches a pipe"
output=$( { hm_start 'Trabajando'; hm_stop 0; } 2>&1 )
case "$output" in
    *$'\033'* | *$'\r'*) reached=escapes ;;
    *) reached=texto ;;
esac
assert_equals "texto" "$reached"

# ---------------------------------------------------------------- elapsed time
#
# "Done" says nothing. "Done in 4m12s" tells somebody what to expect the next time, which is the
# difference between a tool that feels slow and one that is honest about being slow.

test_case "seconds are seconds"
assert_equals "45s" "$(hm_progress_elapsed 45)"

test_case "and minutes are minutes"
assert_equals "4m12s" "$(hm_progress_elapsed 252)"
assert_equals "1m00s" "$(hm_progress_elapsed 60)"

test_case "something instantaneous does not get a stopwatch"
HM_PROGRESS_LABEL="Rápido"
HM_PROGRESS_STARTED=$(date +%s)
HM_PROGRESS_PID=""
assert_not_contains "$(hm_stop 0 2>&1)" "("

# ---------------------------------------------------------------- wrapping a command

test_case "a command that works says nothing it did not have to"
output=$(hm_working 'Copiando' true 2>&1)
assert_not_contains "$output" "failed"

test_case "a command that fails prints what it said"
output=$(hm_working 'Copiando' bash -c 'echo "no encuentro el volumen" >&2; exit 3' 2>&1) && r=0 || r=$?
assert_contains "$output" "no encuentro el volumen"
assert_contains "$output" "failed"

test_case "and its exit code survives"
assert_equals "3" "$r"

test_case "the output of a command that worked is not printed"
assert_not_contains "$(hm_working 'Copiando' bash -c 'echo mucho ruido' 2>&1)" "mucho ruido"

# ---------------------------------------------------------------- frames

test_case "braille where the locale says UTF-8"
assert_contains "$(LC_ALL=es_ES.UTF-8 hm_progress_frames)" "⠋"

test_case "and ASCII where it does not"
assert_equals "| / - \\" "$(LC_ALL=C LANG=C LC_CTYPE=C hm_progress_frames)"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
