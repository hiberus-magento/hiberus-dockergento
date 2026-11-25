#!/usr/bin/env bash
#
# What tests/run.sh does with its argument. A typo used to run every suite instead of failing, so
# nobody ever noticed it.
#
# The runner is exercised as a copy inside a fixture tree, with two suites of its own and a `go`
# that only says it ran: the real one would run this very suite again, and the Go packages with it.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

FIXTURE="$HM_TEST_HOME/run-fixture"
mkdir -p "$FIXTURE/tests/unit" "$FIXTURE/tests/integration" "$FIXTURE/bin"
cp "$HM_TEST_PROJECT_ROOT/tests/run.sh" "$FIXTURE/tests/run.sh"

printf '#!/usr/bin/env bash\necho "RESULT 1 0"\n' > "$FIXTURE/tests/unit/first_test.sh"
printf '#!/usr/bin/env bash\necho "RESULT 2 0"\n' > "$FIXTURE/tests/integration/second_test.sh"
printf '#!/bin/sh\necho "ok  fake/package"\n' > "$FIXTURE/bin/go"
chmod +x "$FIXTURE/bin/go"

# run_runner <args...>: sets OUT, ERR and STATUS
run_runner() {
    local err_file="$FIXTURE/stderr"
    OUT=$(cd "$FIXTURE" && PATH="$FIXTURE/bin:$PATH" bash tests/run.sh "$@" 2>"$err_file")
    STATUS=$?
    ERR=$(cat "$err_file")
}

run_runner nonsense

test_case "an argument it does not know fails"
assert_equals "2" "$STATUS"

test_case "and names the argument"
assert_contains "$ERR" "nonsense"

test_case "and runs no shell suite"
assert_not_contains "$OUT" "RESULT"

test_case "and no Go suite"
assert_not_contains "$OUT" "go test"

run_runner

test_case "no argument runs everything and passes"
assert_equals "0" "$STATUS"

test_case "the Go suites"
assert_contains "$OUT" "1 packages passed"

test_case "and every shell suite"
assert_contains "$OUT" "3 assertions passed"

run_runner unit

test_case "unit runs the unit suites"
assert_equals "0 1 assertions passed" "$STATUS $(printf '%s' "$OUT" | tail -1 | sed 's/\x1b\[[0-9;]*m//g')"

test_case "and not the Go ones"
assert_not_contains "$OUT" "go test"

run_runner tests/integration/second_test.sh

test_case "a suite path runs that suite alone"
assert_equals "0 2 assertions passed" "$STATUS $(printf '%s' "$OUT" | tail -1 | sed 's/\x1b\[[0-9;]*m//g')"

run_runner --shell

test_case "--shell runs every shell suite"
assert_equals "0 3 assertions passed" "$STATUS $(printf '%s' "$OUT" | tail -1 | sed 's/\x1b\[[0-9;]*m//g')"

test_case "and not the Go ones"
assert_not_contains "$OUT" "go test"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
