#!/usr/bin/env bash
#
# Every action the dashboard triggers must be a command the CLI actually has.
#
# The dashboard runs real commands instead of talking to Docker itself, which is what keeps
# the CLI's protections in place — but it also means a wrong command name fails only when
# the key is pressed, on someone else's machine. This checks the names without running them.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

TUI="$COMMANDS_DIR/tui.sh"

# The command name is the first word after `run_action`
actions=$(LC_ALL=C grep -o 'run_action [a-z-]*' "$TUI" | awk '{print $2}' | sort -u)

test_case "the dashboard triggers at least the four state actions"
assert_equals "4" "$(printf '%s\n' "$actions" | grep -c .)"

for action in $actions; do
    test_case "'$action' is a real command"
    [ -f "$COMMANDS_DIR/$action.sh" ] && result="exists" || result="missing $action.sh"
    assert_equals "exists" "$result"

    test_case "'$action' is documented"
    assert_equals "true" "$(jq --arg c "$action" 'has($c)' "$DATA_DIR/command_descriptions.json")"
done

test_case "the browser action does not invent a command either"
assert_not_contains "$(LC_ALL=C grep -A6 '^open_in_browser' "$TUI")" '"$HM" launch'

test_case "the dashboard itself is documented as a command"
assert_equals "true" "$(jq 'has("tui")' "$DATA_DIR/command_descriptions.json")"

test_case "and grouped with the rest of the environment commands"
assert_equals "environment" "$(jq -r '.tui.group' "$DATA_DIR/command_descriptions.json")"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
