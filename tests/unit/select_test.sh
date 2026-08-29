#!/usr/bin/env bash
#
# Moving through a list, and drawing it: the two halves that do not need a terminal.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"
source "$COMPONENTS_DIR/print_message.sh"
source "$COMPONENTS_DIR/select.sh"

OPTS=("Guardar y destruir" "Destruir" "Cancelar")

# ---------------------------------------------------------------- moving

test_case "down moves down"
assert_equals "1" "$(hm_select_move 0 1 3)"

test_case "up moves up"
assert_equals "0" "$(hm_select_move 1 -1 3)"

#
# Somebody at the last option pressing down means the first one, not a beep.
#
test_case "down from the last wraps to the first"
assert_equals "0" "$(hm_select_move 2 1 3)"

test_case "up from the first wraps to the last"
assert_equals "2" "$(hm_select_move 0 -1 3)"

test_case "a list of one goes nowhere"
assert_equals "0" "$(hm_select_move 0 1 1)"
assert_equals "0" "$(hm_select_move 0 -1 1)"

test_case "an empty list is not a division by zero"
assert_equals "0" "$(hm_select_move 0 1 0)"

# ---------------------------------------------------------------- drawing

test_case "every option is drawn, numbered"
rendered=$(hm_select_render 0 "${OPTS[@]}")
assert_equals "3" "$(printf '%s\n' "$rendered" | grep -c .)"
assert_contains "$rendered" "1) Guardar y destruir"
assert_contains "$rendered" "3) Cancelar"

test_case "the marker is on the selected one"
assert_contains "$(hm_select_render 0 "${OPTS[@]}" | head -1)" "❯"
assert_not_contains "$(hm_select_render 0 "${OPTS[@]}" | tail -1)" "❯"

test_case "and it moves with the selection"
assert_contains "$(hm_select_render 2 "${OPTS[@]}" | tail -1)" "❯"
assert_not_contains "$(hm_select_render 2 "${OPTS[@]}" | head -1)" "❯"

#
# The marker is the first thing on the line rather than a colour, because a monochrome terminal
# swallows the colour and leaves three identical lines.
#
test_case "the marker survives without colour"
assert_contains "$(BOLD='' COLOR_RESET='' hm_select_render 1 "${OPTS[@]}" | sed -n 2p)" "❯"

test_case "the list is as many lines as there are options, so it can be rewritten in place"
assert_equals "5" "$(hm_select_render 0 a b c d e | grep -c .)"

# ---------------------------------------------------------------- choosing a way to ask

test_case "a pipe cannot be drawn on"
hm_select_can_draw && r=dibuja || r=no
assert_equals "no" "$r"

test_case "and neither can a dumb terminal"
TERM=dumb hm_select_can_draw && r=dibuja || r=no
assert_equals "no" "$r"

test_case "the non-interactive refusal is untouched"
assert_contains "$(cat "$COMPONENTS_DIR/input_info.sh")" "Non-interactive mode cannot choose"

test_case "and the numbered list is still there for terminals that cannot be drawn on"
assert_contains "$(cat "$COMPONENTS_DIR/input_info.sh")" "select REPLY in"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
