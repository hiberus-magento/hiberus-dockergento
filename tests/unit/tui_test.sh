#!/usr/bin/env bash
#
# Terminal primitives: the pure parts, and that nothing is emitted without a terminal.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"
source "$COMPONENTS_DIR/tui.sh"

# ---------------------------------------------------------------- size parsing

test_case "a valid stty size is parsed"
assert_equals "40 120" "$(tui_parse_size "40 120")"

test_case "extra whitespace does not matter"
assert_equals "40 120" "$(tui_parse_size "  40   120  ")"

test_case "empty input is rejected"
tui_parse_size "" >/dev/null 2>&1 && r=accepted || r=rejected
assert_equals "rejected" "$r"

test_case "non numeric input is rejected"
tui_parse_size "rows cols" >/dev/null 2>&1 && r=accepted || r=rejected
assert_equals "rejected" "$r"

test_case "a zero dimension is rejected"
tui_parse_size "0 80" >/dev/null 2>&1 && r=accepted || r=rejected
assert_equals "rejected" "$r"

test_case "a partial size is rejected"
tui_parse_size "40" >/dev/null 2>&1 && r=accepted || r=rejected
assert_equals "rejected" "$r"

test_case "without a terminal the size falls back to 24x80"
assert_equals "24 80" "$( (unset LINES COLUMNS; tui_size) )"

test_case "the shell variables are used when stty cannot answer"
assert_equals "50 200" "$( (export LINES=50 COLUMNS=200; tui_size) )"

# ---------------------------------------------------------------- key naming

test_case "the up arrow is named"
assert_equals "up" "$(tui_key_name $'\033[A')"

test_case "the down arrow is named"
assert_equals "down" "$(tui_key_name $'\033[B')"

test_case "the application mode arrows are named too"
assert_equals "right" "$(tui_key_name $'\033OC')"

test_case "a lone escape is named, not returned raw"
assert_equals "esc" "$(tui_key_name $'\033')"

test_case "enter is named"
assert_equals "enter" "$(tui_key_name $'\n')"

test_case "carriage return is also enter"
assert_equals "enter" "$(tui_key_name $'\r')"

test_case "an empty read is enter, which is what bash gives for it"
assert_equals "enter" "$(tui_key_name '')"

test_case "ctrl-c is named"
assert_equals "ctrl-c" "$(tui_key_name $'\003')"

test_case "backspace is named"
assert_equals "backspace" "$(tui_key_name $'\177')"

test_case "a printable key comes back as itself"
assert_equals "q" "$(tui_key_name 'q')"

# ---------------------------------------------------------------- escape timeout

test_case "the escape timeout defaults to the running shell"
timeout=$(tui_escape_timeout)
case "$timeout" in 1 | 0.05) r=ok ;; *) r="$timeout" ;; esac
assert_equals "ok" "$r"

test_case "the escape timeout is a whole second on bash 3.2"
assert_equals "1" "$(tui_escape_timeout 3)"

test_case "and fractional from bash 4 on"
assert_equals "0.05" "$(tui_escape_timeout 5)"

# ---------------------------------------------------------------- harmless without a terminal

test_case "nothing is emitted without a terminal"
output=$( tui_enter_screen; tui_hide_cursor; tui_move 1 1; tui_clear_line; tui_clear_screen
          tui_suspend; tui_resume; tui_leave_screen; tui_show_cursor )
assert_empty "$output"

test_case "and those calls still succeed"
tui_enter_screen && tui_hide_cursor && tui_move 2 2 && tui_leave_screen && r=ok || r=failed
assert_equals "ok" "$r"

test_case "restoring twice is not an error"
tui_restore >/dev/null 2>&1 && tui_restore >/dev/null 2>&1 && r=ok || r=failed
assert_equals "ok" "$r"

test_case "the library loads under bash 3.2"
/bin/bash -c "COMPONENTS_DIR='$COMPONENTS_DIR' source '$COMPONENTS_DIR/tui.sh' && tui_key_name 'q'" >/dev/null 2>&1 && r=ok || r=failed
assert_equals "ok" "$r"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
