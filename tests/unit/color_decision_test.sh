#!/usr/bin/env bash
#
# The colour decision and its precedence.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

# load_properties.sh runs at source time and touches the project properties, so it is given
# an empty directory to look at
LAB=$(cd "$(mktemp -d)" && pwd -P)
export CUSTOM_PROPERTIES_DIR="$LAB"
trap 'rm -rf "$LAB"' EXIT

source "$TASKS_DIR/load_properties.sh"

decide() {
    # Runs in a subshell so each case starts from a clean environment
    (
        unset HM_NO_COLOR NO_COLOR FORCE_COLOR CLICOLOR_FORCE
        export TERM="xterm-256color"
        while [ "$#" -gt 0 ]; do
            export "$1"
            shift
        done
        should_use_color && echo "color" || echo "plain"
    )
}

test_case "without a terminal there is no colour"
assert_equals "plain" "$(decide)"

test_case "FORCE_COLOR colours a pipe"
assert_equals "color" "$(decide FORCE_COLOR=1)"

test_case "CLICOLOR_FORCE does the same"
assert_equals "color" "$(decide CLICOLOR_FORCE=1)"

test_case "NO_COLOR wins over FORCE_COLOR"
assert_equals "plain" "$(decide FORCE_COLOR=1 NO_COLOR=1)"

test_case "an explicit --no-color wins over everything"
assert_equals "plain" "$(decide FORCE_COLOR=1 HM_NO_COLOR=1)"

test_case "TERM=dumb wins over FORCE_COLOR"
assert_equals "plain" "$(decide FORCE_COLOR=1 TERM=dumb)"

test_case "an empty TERM also means no colour"
assert_equals "plain" "$(decide FORCE_COLOR=1 TERM=)"

test_case "an empty NO_COLOR is not a request"
assert_equals "color" "$(decide FORCE_COLOR=1 NO_COLOR=)"

# TERM is set explicitly: leaving it to the ambient environment made this pass on a laptop
# and die in a container, where TERM is not defined at all
test_case "with colour off every variable is empty"
result=$( ( export TERM=xterm-256color NO_COLOR=1
            load_colors
            echo "[$RED$GREEN$BLUE$COLOR_RESET]" ) )
assert_equals "[]" "$result"

test_case "with colour on the palette is populated"
result=$( ( export TERM=xterm-256color FORCE_COLOR=1
            load_colors
            if [ -n "$RED" ] && [ -n "$COLOR_RESET" ]; then echo populated; else echo empty; fi ) )
assert_equals "populated" "$result"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
