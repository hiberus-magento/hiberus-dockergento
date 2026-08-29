#!/usr/bin/env bash
#
# The options `hm setup` refuses, against the real command.
#
# Nothing is set up here: what is checked is that a bad instruction stops the command before it
# has created anything, which is the difference between a pipeline that fails and one that hangs.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

HM="$COMMAND_BIN_DIR/bin/run"
LAB=$(cd "$(mktemp -d)" && pwd -P)
trap 'rm -rf "$LAB"' EXIT

mkdir -p "$LAB/proyecto"
printf -- '-- un volcado\n' > "$LAB/dump.sql"

run() { ( cd "$LAB/proyecto" && "$HM" "$@" >"$LAB/out" 2>"$LAB/err" ); STATUS=$?; STDOUT=$(cat "$LAB/out"); STDERR=$(cat "$LAB/err"); return 0; }

test_case "a dump that does not exist stops the command"
run setup --dump=/no/such/file.sql
assert_equals "2" "$STATUS"
assert_contains "$STDERR" "/no/such/file.sql"

test_case "and nothing was created on the way"
assert_equals "" "$(ls -A "$LAB/proyecto")"

test_case "an option nobody declared is a usage error"
run setup --nonsense
assert_equals "2" "$STATUS"
assert_contains "$STDERR" "nonsense"

test_case "the Warden spelling is not an unknown option"
run --yes setup --clean-install --domain=x.test
assert_equals "0" "$([ "$STATUS" != "2" ] && echo 0 || echo 1)"

test_case "nor is the long form of the dump"
run --yes setup --db-dump="$LAB/dump.sql"
assert_equals "0" "$([ "$STATUS" != "2" ] && echo 0 || echo 1)"

#
# The one question with no safe default: choosing wrong either wipes a database or spends twenty
# minutes installing something nobody wanted.
#
test_case "non-interactive with no database mode refuses rather than choosing"
run --yes setup --domain=x.test
assert_equals "2" "$STATUS"
assert_contains "$STDERR" "cannot choose"

test_case "hm install takes the long form of its option"
run install --use-default
assert_equals "0" "$([ "$STATUS" != "2" ] && echo 0 || echo 1)"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
