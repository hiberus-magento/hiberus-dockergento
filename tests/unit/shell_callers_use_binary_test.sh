#!/usr/bin/env bash
#
# Shell commands that ask the tool for something go through the binary, never bin/run.
#
# bin/run only names the binary for a command whose shell twin is retired, so a caller that kept
# asking it got that refusal instead of an answer. The dashboard and the MCP server both swallowed
# it — `|| echo '{}'` and a tool error — and showed an empty fleet and broken tools without saying
# why, for every retired command they used. Their suites need Docker and skip without it, which is
# how it went unnoticed.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

for script in tui mcp; do
    # A script ported to the binary and retired has nothing left to check
    [ -f "$HM_TEST_PROJECT_ROOT/console/commands/$script.sh" ] || continue
    test_case "$script asks the binary for the commands it calls"
    assert_equals 'HM="$COMMAND_BIN_DIR/bin/hm"' \
        "$(grep -E '^HM=' "$HM_TEST_PROJECT_ROOT/console/commands/$script.sh")"
done

test_case "no shell script keeps bin/run in a variable to call commands through"
assert_equals "" "$(grep -rlE '^[[:space:]]*[A-Z_]+="?\$\{?COMMAND_BIN_DIR\}?/bin/run"?[[:space:]]*$' \
    "$HM_TEST_PROJECT_ROOT/console" || true)"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
