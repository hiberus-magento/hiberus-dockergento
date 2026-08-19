#!/usr/bin/env bash
#
# End-to-end behaviour of the router: formats, exit codes and stream separation.
# Runs from a temporary directory that is deliberately not a Dockergento project.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

HM="$COMMAND_BIN_DIR/bin/run"
WORKDIR=$(mktemp -d)
OUT="$WORKDIR/stdout"
ERR="$WORKDIR/stderr"
trap 'rm -rf "$WORKDIR"' EXIT

# run <args...> — captures stdout, stderr and the exit code in separate files
run() {
    ( cd "$WORKDIR" && "$HM" "$@" >"$OUT" 2>"$ERR" )
    STATUS=$?
    STDOUT=$(cat "$OUT")
    STDERR=$(cat "$ERR")
    return 0
}

test_case "an unknown command exits with the usage code"
run noexiste
assert_equals "2" "$STATUS"

test_case "an unknown command reports the error as JSON when piped"
assert_json "$STDERR"

test_case "an unknown command reports its type"
assert_json_field "$STDERR" '.error.type' "command_not_found"

test_case "an unknown command keeps stdout clean"
assert_empty "$STDOUT"

test_case "--no-json falls back to readable text"
run --no-json noexiste
assert_not_contains "$STDERR" '"schema_version"'

test_case "a readable error still mentions the command"
assert_contains "$STDERR" "noexiste"

test_case "a readable error still exits with the usage code"
assert_equals "2" "$STATUS"

test_case "outside a project the exit code says so"
run bash
assert_equals "4" "$STATUS"

test_case "outside a project the error is machine readable"
assert_json_field "$STDERR" '.error.type' "project_not_configured"

test_case "outside a project stdout stays empty"
assert_empty "$STDOUT"

test_case "the error names the command that failed"
assert_json_field "$STDERR" '.command' "bash"

test_case "--help still works"
run --help
assert_equals "0" "$STATUS"

test_case "--help prints something"
assert_contains "$STDOUT$STDERR" "hm"

test_case "the global flag is not passed on to the command"
run --json noexiste
assert_json_field "$STDERR" '.command' "noexiste"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
