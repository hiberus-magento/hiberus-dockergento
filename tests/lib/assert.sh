#!/usr/bin/env bash
#
# Minimal assertion helpers. Pure Bash on purpose: the project has no build chain and no
# package manager, so the test suite must run on any machine that can run `hm` itself.
#

# Allow a suite to run standalone (`bash tests/unit/foo_test.sh`) as well as through
# tests/run.sh, by bootstrapping the directory variables the components expect.
if [ -z "${COMPONENTS_DIR:-}" ]; then
    _hm_test_root="$(cd -P "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
    export COMMAND_BIN_NAME="hm"
    export COMMAND_TOOLNAME="Hiberus Dockergento"
    export COMMAND_BIN_DIR="$_hm_test_root"
    export COMMANDS_DIR="$_hm_test_root/console/commands"
    export TASKS_DIR="$_hm_test_root/console/tasks"
    export HELPERS_DIR="$_hm_test_root/console/helpers"
    export COMPONENTS_DIR="$_hm_test_root/console/components"
    export DATA_DIR="$_hm_test_root/data"
    export HM_TEST_PROJECT_ROOT="$_hm_test_root"
fi

HM_TESTS_RUN=0
HM_TESTS_FAILED=0
HM_CURRENT_TEST=""

_fail() {
    HM_TESTS_FAILED=$((HM_TESTS_FAILED + 1))
    printf '  \033[0;31m✗\033[0m %s\n' "$HM_CURRENT_TEST"
    printf '      %s\n' "$1"
}

_pass() {
    printf '  \033[0;32m✓\033[0m %s\n' "$HM_CURRENT_TEST"
}

#
# assert_equals <expected> <actual> [label]
#
assert_equals() {
    HM_TESTS_RUN=$((HM_TESTS_RUN + 1))
    if [ "$1" == "$2" ]; then
        _pass
    else
        _fail "${3:-expected} '$1', got '$2'"
    fi
}

#
# assert_contains <haystack> <needle>
#
assert_contains() {
    HM_TESTS_RUN=$((HM_TESTS_RUN + 1))
    if [[ "$1" == *"$2"* ]]; then
        _pass
    else
        _fail "expected to contain '$2', got '$1'"
    fi
}

#
# assert_not_contains <haystack> <needle>
#
assert_not_contains() {
    HM_TESTS_RUN=$((HM_TESTS_RUN + 1))
    if [[ "$1" != *"$2"* ]]; then
        _pass
    else
        _fail "expected NOT to contain '$2', got '$1'"
    fi
}

#
# assert_empty <value> [label]
#
assert_empty() {
    HM_TESTS_RUN=$((HM_TESTS_RUN + 1))
    if [ -z "$1" ]; then
        _pass
    else
        _fail "${2:-expected empty}, got '$1'"
    fi
}

#
# assert_json <string> — valid JSON
#
assert_json() {
    HM_TESTS_RUN=$((HM_TESTS_RUN + 1))
    if printf '%s' "$1" | jq -e . >/dev/null 2>&1; then
        _pass
    else
        _fail "not valid JSON: '$1'"
    fi
}

#
# assert_json_field <json> <jq-filter> <expected>
#
assert_json_field() {
    HM_TESTS_RUN=$((HM_TESTS_RUN + 1))
    local actual
    actual=$(printf '%s' "$1" | jq -r "$2" 2>/dev/null)
    if [ "$actual" == "$3" ]; then
        _pass
    else
        _fail "$2 expected '$3', got '$actual'"
    fi
}

#
# Run a command with a hard time limit, with stdin attached to a FIFO nobody writes to,
# so that any prompt blocks forever and is caught as a failure instead of passing because
# stdin happened to be at EOF.
#
# run_blocking_check <seconds> <command...>  -> 142 when it blocked
#
run_blocking_check() {
    local limit="$1"
    shift

    local fifo
    fifo=$(mktemp -u)
    mkfifo "$fifo"
    exec 9<>"$fifo"

    # The extra subshell keeps the shell's "Alarm clock" job-control notice out of the
    # test output when the time limit kills the command.
    local status=0
    ( exec 2>/dev/null; perl -e 'alarm shift; exec @ARGV' "$limit" "$@" <"$fifo" >/dev/null 2>&1 ) || status=$?

    exec 9>&-
    rm -f "$fifo"
    return $status
}

#
# assert_does_not_block <seconds> <command...>
#
assert_does_not_block() {
    HM_TESTS_RUN=$((HM_TESTS_RUN + 1))
    local limit="$1"
    shift

    local status=0
    run_blocking_check "$limit" "$@" || status=$?

    if [ "$status" -eq 142 ]; then
        _fail "'$*' blocked waiting for input"
    else
        _pass
    fi
}

#
# test_case <name> — sets the label used by the next assertion
#
test_case() {
    HM_CURRENT_TEST="$1"
}
