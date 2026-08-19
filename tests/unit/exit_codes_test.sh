#!/usr/bin/env bash
#
# Exit codes and hm_fail behaviour in both output formats.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"
source "$HELPERS_DIR/exit_codes.sh"

test_case "exit codes are the documented ones"
assert_equals "0 1 2 3 4 5" "$HM_EXIT_OK $HM_EXIT_ERROR $HM_EXIT_USAGE $HM_EXIT_DOCKER $HM_EXIT_PROJECT $HM_EXIT_SERVICE"

test_case "hm_fail exits with the given code"
status=0
( HM_OUTPUT_FORMAT=text hm_fail "$HM_EXIT_DOCKER" docker_unavailable "boom" "fix it" ) >/dev/null 2>&1 || status=$?
assert_equals "3" "$status"

test_case "hm_fail emits JSON on stderr in json mode"
stderr=$( ( HM_OUTPUT_FORMAT=json HM_COMMAND=start hm_fail "$HM_EXIT_SERVICE" service_not_running "db is down" "hm start db" ) 2>&1 >/dev/null ) || true
assert_json "$stderr"

test_case "hm_fail reports the failing command"
assert_json_field "$stderr" '.command' "start"

test_case "hm_fail reports the error type"
assert_json_field "$stderr" '.error.type' "service_not_running"

test_case "hm_fail keeps stdout clean in json mode"
stdout=$( ( HM_OUTPUT_FORMAT=json hm_fail "$HM_EXIT_ERROR" boom "message" ) 2>/dev/null ) || true
assert_empty "$stdout"

test_case "hm_fail prints a readable message in text mode"
stderr=$( ( HM_OUTPUT_FORMAT=text hm_fail "$HM_EXIT_ERROR" boom "something broke" "run hm doctor" ) 2>&1 >/dev/null ) || true
assert_contains "$stderr" "something broke"

test_case "hm_fail shows the hint in text mode"
assert_contains "$stderr" "run hm doctor"

test_case "hm_fail text output is not JSON"
assert_not_contains "$stderr" '"schema_version"'

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
