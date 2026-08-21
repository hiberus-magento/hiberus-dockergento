#!/usr/bin/env bash
#
# The dashboard's layout, checked as pure functions with made-up data.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"
source "$TASKS_DIR/tui_render.sh"

FLEET='{"data":{"environments":[
  {"name":"alpha","status":"running","containers":{"running":9,"total":9},"branch":"main",
   "root":"/Users/someone/projects/alpha","worktree":"","orphan":false,"has_metadata":true},
  {"name":"a-very-long-project-name-indeed","status":"stopped","containers":{"running":0,"total":9},
   "branch":"feature/something-long","root":"/Users/someone/very/deep/path/to/beta",
   "worktree":"wt-x","orphan":true,"has_metadata":false}
]}}'

DOCTOR='{"data":{"checks":[
  {"id":"ports","scope":"global","severity":"error","message":"Port 80 is taken","action":"hm stop"},
  {"id":"disk","scope":"global","severity":"warning","message":"Lots of volumes","action":""},
  {"id":"daemon","scope":"global","severity":"ok","message":"Docker is running","action":""}
]}}'

DETAIL='{"data":{"project":{"name":"alpha","status":"running","domain":"alpha.local",
  "root":"/Users/someone/projects/alpha","urls":{"base":"https://alpha.local/","admin":"","mailhog":"http://localhost:8025"}},
  "magento":{"version":"2.4.9","mode":"developer"},
  "services":[{"name":"phpfpm","image":"php:8.5","state":"running","ports":[]}],
  "paths":{},"tooling":{}}}'

# ---------------------------------------------------------------- truncation

test_case "text shorter than the width is untouched"
assert_equals "short" "$(tui_truncate "short" 20)"

test_case "long text is cut and marked"
assert_equals "abcd~" "$(tui_truncate "abcdefgh" 5)"

test_case "a path is cut from the left, keeping the end that identifies it"
assert_equals "~/to/beta" "$(tui_truncate_path "/very/deep/path/to/beta" 9)"

test_case "a short path is untouched"
assert_equals "/tmp/x" "$(tui_truncate_path "/tmp/x" 20)"

# ---------------------------------------------------------------- columns

test_case "at a comfortable width every column is shown"
columns=$(tui_fleet_columns 120)
path=$(printf '%s' "$columns" | awk '{print $5}')
[ "$path" -gt 0 ] && r=shown || r=hidden
assert_equals "shown" "$r"

test_case "the path is the first column to go"
columns=$(tui_fleet_columns 70)
path=$(printf '%s' "$columns" | awk '{print $5}')
assert_equals "0" "$path"

test_case "and the branch goes next on a very narrow terminal"
columns=$(tui_fleet_columns 40)
branch=$(printf '%s' "$columns" | awk '{print $4}')
assert_equals "0" "$branch"

test_case "the name and the status never disappear"
columns=$(tui_fleet_columns 30)
name=$(printf '%s' "$columns" | awk '{print $1}')
status=$(printf '%s' "$columns" | awk '{print $2}')
{ [ "$name" -gt 0 ] && [ "$status" -gt 0 ]; } && r=kept || r=lost
assert_equals "kept" "$r"

# ---------------------------------------------------------------- fleet rows

test_case "one row per environment"
assert_equals "2" "$(tui_fleet_rows "$FLEET" 120 | sed '/^$/d' | wc -l | tr -d ' ')"

test_case "a row carries the name, the status and the counts"
row=$(tui_fleet_rows "$FLEET" 120 | head -1)
assert_contains "$row" "alpha"

test_case "and the running counts"
assert_contains "$row" "9/9"

test_case "a worktree is labelled"
row=$(tui_fleet_rows "$FLEET" 120 | tail -1)
assert_contains "$row" "[wt-x]"

test_case "an orphan is marked"
assert_contains "$row" "!"

test_case "no row is wider than the terminal"
too_wide=$(tui_fleet_rows "$FLEET" 60 | awk 'length($0) > 60' | wc -l | tr -d ' ')
assert_equals "0" "$too_wide"

test_case "the header lines up with the rows"
header=$(tui_fleet_header 120)
assert_contains "$header" "PROJECT"

test_case "the count comes from the payload"
assert_equals "2" "$(tui_fleet_count "$FLEET")"

test_case "an empty payload counts zero"
assert_equals "0" "$(tui_fleet_count '{"data":{"environments":[]}}')"

test_case "a field of the selected environment can be read"
assert_equals "/Users/someone/projects/alpha" "$(tui_fleet_field "$FLEET" 0 root)"

test_case "reading past the end gives nothing"
assert_empty "$(tui_fleet_field "$FLEET" 9 root)"

# ---------------------------------------------------------------- doctor

test_case "only the problems are shown"
assert_equals "2" "$(tui_doctor_lines "$DOCTOR" 120 | sed '/^$/d' | wc -l | tr -d ' ')"

test_case "errors come before warnings"
assert_contains "$(tui_doctor_lines "$DOCTOR" 120 | head -1)" "ERROR"

test_case "a healthy check is not shown"
assert_not_contains "$(tui_doctor_lines "$DOCTOR" 120)" "daemon"

# ---------------------------------------------------------------- detail

test_case "the detail names the project"
assert_contains "$(tui_detail_lines "$DETAIL" 120)" "alpha"

test_case "and reports the Magento version and mode"
assert_contains "$(tui_detail_lines "$DETAIL" 120)" "2.4.9"

test_case "and lists the URLs that exist"
assert_contains "$(tui_detail_lines "$DETAIL" 120)" "https://alpha.local/"

test_case "empty URLs are skipped"
assert_not_contains "$(tui_detail_lines "$DETAIL" 120)" "admin      "

test_case "and lists the services"
assert_contains "$(tui_detail_lines "$DETAIL" 120)" "phpfpm"

test_case "no detail line is wider than the terminal"
too_wide=$(tui_detail_lines "$DETAIL" 50 | awk 'length($0) > 50' | wc -l | tr -d ' ')
assert_equals "0" "$too_wide"

test_case "a broken payload does not break the layout"
assert_empty "$(tui_detail_lines 'not json at all' 80 | sed '/^$/d')"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
