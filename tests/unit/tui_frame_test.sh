#!/usr/bin/env bash
#
# The frame: what the dashboard writes to the terminal, checked without one.
#
# Composing and painting were split precisely so this file can exist. A frame is a string, and
# the properties that matter — that it never erases the screen, that it fills the height, that
# the selection is where it should be — are properties of that string.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"
source "$TASKS_DIR/tui_render.sh"

fleet_of() {
    local count="$1" i out=""
    for ((i = 0; i < count; i++)); do
        [ -n "$out" ] && out="$out,"
        out="$out{\"name\":\"env-$i\",\"status\":\"stopped\",\"containers\":{\"running\":0,\"total\":9},"
        out="$out\"branch\":\"main\",\"root\":\"/projects/env-$i\",\"worktree\":\"\",\"orphan\":false,\"has_metadata\":true}"
    done
    printf '{"data":{"environments":[%s]}}' "$out"
}

FLEET_JSON=$(fleet_of 4)
DOCTOR_JSON='{"data":{"checks":[{"id":"disk","scope":"global","severity":"warning","message":"Lots of volumes","action":""}]}}'
DETAIL_JSON='{"data":{"project":{"name":"env-0","status":"stopped","domain":"env0.local","root":"/projects/env-0","urls":{"base":"https://env0.local/"}},"magento":{"version":"2.4.9","mode":"developer"},"services":[{"name":"phpfpm","image":"php:8.4","state":"exited","ports":[]}]}}'

source "$TASKS_DIR/tui_frame.sh"

TUI_COLS=100
TUI_ROWS=24
LOADED_AT="12:00:00"
compose

frame() {
    build_frame
    frame_bytes
    printf '%s' "$TUI_BYTES"
}

ESC=$(printf '\033')

# ---------------------------------------------------------------- the shape of a frame

test_case "the frame is exactly as tall as the terminal"
build_frame
assert_equals "24" "$FRAME_N"

test_case "the last line carries the keys"
assert_contains "${FRAME[23]}" "q quit"

test_case "the line above it says how old the data is"
assert_contains "${FRAME[22]}" "data from 12:00:00"

test_case "every line is positioned on its own row"
assert_equals "24" "$(frame | LC_ALL=C grep -o "${ESC}\[[0-9]*;1H" | wc -l | tr -d ' ')"

test_case "and clears whatever was to its right"
assert_equals "24" "$(frame | LC_ALL=C grep -o "${ESC}\[K" | wc -l | tr -d ' ')"

test_case "the frame never erases the screen"
assert_not_contains "$(frame)" "${ESC}[2J"

# ---------------------------------------------------------------- no leftovers

test_case "a screen with nothing on it is still filled to the bottom"
saved="$FLEET_JSON"
FLEET_JSON='{"data":{"environments":[]}}'
compose
build_frame
assert_equals "24" "$FRAME_N"

test_case "so nothing of the previous screen can survive under it"
assert_equals "24" "$(frame | LC_ALL=C grep -o "${ESC}\[K" | wc -l | tr -d ' ')"

FLEET_JSON="$saved"
compose

# ---------------------------------------------------------------- the selection

test_case "the selected environment is marked"
SELECTED=0
build_frame
assert_equals "1" "$(printf '%s\n' "${FRAME[@]}" | LC_ALL=C grep -c "> env-" | tr -d ' ')"

test_case "and the mark follows the selection"
SELECTED=2
build_frame
marked=$(printf '%s\n' "${FRAME[@]}" | LC_ALL=C grep "> env-" | LC_ALL=C sed "s/${ESC}\[[0-9;]*m//g")
assert_contains "$marked" "env-2"

test_case "only one row is ever marked"
assert_equals "1" "$(printf '%s\n' "${FRAME[@]}" | LC_ALL=C grep -c "> env-" | tr -d ' ')"

# ---------------------------------------------------------------- more environments than rows

FLEET_JSON=$(fleet_of 40)
compose

test_case "a fleet taller than the terminal still fits the frame"
SELECTED=0
FIRST_VISIBLE=0
build_frame
assert_equals "24" "$FRAME_N"

test_case "the selection stays visible when it moves past the bottom"
SELECTED=39
build_frame
visible=$(printf '%s\n' "${FRAME[@]}" | LC_ALL=C sed "s/${ESC}\[[0-9;]*m//g")
assert_contains "$visible" "> env-39"

test_case "and the frame says what part of the list is on screen"
assert_contains "$visible" "of 40"

test_case "coming back up brings the first ones back"
SELECTED=0
build_frame
visible=$(printf '%s\n' "${FRAME[@]}" | LC_ALL=C sed "s/${ESC}\[[0-9;]*m//g")
assert_contains "$visible" "> env-0"

# ---------------------------------------------------------------- the detail view

test_case "the detail view names the environment"
FLEET_JSON=$(fleet_of 4)
compose
SELECTED=0
VIEW="detail"
build_frame
visible=$(printf '%s\n' "${FRAME[@]}" | LC_ALL=C sed "s/${ESC}\[[0-9;]*m//g")
assert_contains "$visible" "env-0"

test_case "and shows what describe reported"
assert_contains "$visible" "2.4.9"

test_case "the detail view scrolls rather than cutting off what does not fit"
DETAIL_JSON='{"data":{"project":{"name":"env-0","status":"stopped","domain":"env0.local","root":"/projects/env-0","urls":{"base":"https://env0.local/","admin":"https://env0.local/admin","mailhog":"http://localhost:8025","rabbitmq":"http://localhost:15672","search":"http://localhost:9200"}},"magento":{"version":"2.4.9","mode":"developer"},"services":[
 {"name":"db","image":"mariadb","state":"exited","ports":[]},{"name":"hitch","image":"hitch","state":"exited","ports":[]},
 {"name":"mailhog","image":"mailhog","state":"exited","ports":[]},{"name":"nginx","image":"nginx","state":"exited","ports":[]},
 {"name":"phpfpm","image":"php","state":"exited","ports":[]},{"name":"rabbitmq","image":"rabbitmq","state":"exited","ports":[]},
 {"name":"redis","image":"redis","state":"exited","ports":[]},{"name":"search","image":"search","state":"exited","ports":[]},
 {"name":"varnish","image":"varnish","state":"exited","ports":[]}]}}'
read_into_detail_rows "$(tui_detail_lines "$DETAIL_JSON" "$((TUI_COLS - 4))")"
DETAIL_NAME="env-0"
DETAIL_OFFSET=0
build_frame
visible=$(printf '%s\n' "${FRAME[@]}" | LC_ALL=C sed "s/${ESC}\[[0-9;]*m//g")
assert_contains "$visible" "of ${#DETAIL_ROWS[@]}"

test_case "and says how to scroll it"
assert_contains "$visible" "j/k to scroll"

test_case "scrolling reaches the end of the list"
DETAIL_OFFSET=99
build_frame
visible=$(printf '%s\n' "${FRAME[@]}" | LC_ALL=C sed "s/${ESC}\[[0-9;]*m//g")
assert_contains "$visible" "varnish"

test_case "and cannot scroll past it"
assert_contains "$visible" "-${#DETAIL_ROWS[@]} of ${#DETAIL_ROWS[@]}"

test_case "the detail is titled with the environment its data came from"
SELECTED=3
build_frame
assert_contains "${FRAME[0]}" "env-0"

test_case "the detail view is also exactly as tall as the terminal"
assert_equals "24" "$FRAME_N"

test_case "with its own keys at the bottom"
assert_contains "${FRAME[23]}" "esc back"

# ---------------------------------------------------------------- a narrow terminal

test_case "no line of the frame is wider than the terminal"
VIEW="fleet"
TUI_COLS=60
compose
build_frame
too_wide=$(printf '%s\n' "${FRAME[@]}" | LC_ALL=C sed "s/${ESC}\[[0-9;]*m//g" | awk 'length($0) > 60' | wc -l | tr -d ' ')
assert_equals "0" "$too_wide"

# ---------------------------------------------------------------- the footer

test_case "the footer uses the long form when it fits"
VIEW="fleet"
TUI_COLS=140
assert_contains "$(footer_keys)" "g refresh"

test_case "and the short one when it does not, rather than a truncated long one"
TUI_COLS=100
assert_contains "$(footer_keys)" "q quit"

test_case "the form that is chosen always fits the width"
long=$(footer_keys)
[ "${#long}" -le 98 ] && r=fits || r="${#long} chars in 98"
assert_equals "fits" "$r"

test_case "a resize recomposes on the next paint"
TUI_COLS=120
assert_equals "60" "$COMPOSED_COLS"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
