#!/usr/bin/env bash
#
# JSON envelope: shape, escaping and stream separation.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"
source "$COMPONENTS_DIR/print_json.sh"

test_case "json_object builds a valid object"
result=$(json_object domain sports.local project sports)
assert_json "$result"

test_case "json_object keeps values intact"
assert_json_field "$result" '.domain' "sports.local"

test_case "json_object escapes double quotes"
result=$(json_object name 'a "quoted" value')
assert_json_field "$result" '.name' 'a "quoted" value'

test_case "json_object escapes newlines"
result=$(json_object note $'line1\nline2')
assert_json_field "$result" '.note' $'line1\nline2'

test_case "json_object preserves UTF-8"
result=$(json_object city 'A Coruña €')
assert_json_field "$result" '.city' 'A Coruña €'

test_case "json_object with no pairs is an empty object"
assert_json_field "$(json_object)" '.' '{}'

test_case "json_object_raw keeps typed values"
result=$(json_object_raw count 3 enabled true nested '{"a":1}')
assert_json_field "$result" '.count' "3"

test_case "json_object_raw keeps booleans"
assert_json_field "$result" '.enabled' "true"

test_case "json_success carries the envelope"
result=$(json_success describe "$(json_object domain sports.local)")
assert_json_field "$result" '.ok' "true"

test_case "json_success reports the command"
assert_json_field "$result" '.command' "describe"

test_case "json_success is versioned"
assert_json_field "$result" '.schema_version' "1"

test_case "json_success nests the payload under data"
assert_json_field "$result" '.data.domain' "sports.local"

test_case "json_error goes to stderr, never stdout"
stdout=$(json_error start 3 docker_unavailable "Docker is not running" "Start Docker" 2>/dev/null)
assert_empty "$stdout" "expected stderr only"

test_case "json_error is valid JSON"
stderr=$(json_error start 3 docker_unavailable "Docker is not running" "Start Docker" 2>&1 >/dev/null)
assert_json "$stderr"

test_case "json_error marks failure"
assert_json_field "$stderr" '.ok' "false"

test_case "json_error carries a numeric code"
assert_json_field "$stderr" '.error.code' "3"

test_case "json_error carries the type"
assert_json_field "$stderr" '.error.type' "docker_unavailable"

test_case "json_error carries the hint"
assert_json_field "$stderr" '.error.hint' "Start Docker"

test_case "is_json_output follows HM_OUTPUT_FORMAT"
HM_OUTPUT_FORMAT=json
is_json_output && r=yes || r=no
assert_equals "yes" "$r"

test_case "is_json_output is false for text"
HM_OUTPUT_FORMAT=text
is_json_output && r=yes || r=no
assert_equals "no" "$r"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
