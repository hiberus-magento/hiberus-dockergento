#!/usr/bin/env bash
#
# What the platform needs after an environment comes up.
#
# It is its own command because `start` is no longer the only thing that brings an environment up:
# the Go implementation does the Compose part and hands this back. One copy of the steps, or the
# two callers drift.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

COMANDO="$COMMAND_BIN_DIR/console/commands/post-start.sh"

# ---------------------------------------------------------------- the platform decides

test_case "on macOS there is nothing to do, and it says so by doing nothing"
MACHINE=mac bash "$COMANDO" >/dev/null 2>&1
assert_equals "0" "$?"

test_case "and an unknown platform is not treated as Linux"
MACHINE= bash "$COMANDO" >/dev/null 2>&1
assert_equals "0" "$?"

# ---------------------------------------------------------------- one copy of the steps

test_case "start does not carry its own copy of them"
assert_equals "" "$(grep -c 'fix_linux_permissions' "$COMMAND_BIN_DIR/console/commands/start.sh" | grep -v '^0$')"

test_case "it calls the command instead"
assert_contains "$(cat "$COMMAND_BIN_DIR/console/commands/start.sh")" "post-start.sh"

# ---------------------------------------------------------------- a project with no TLS terminator
#
# The self-routing entries point at Hitch, and a project routed through the global proxy has no
# Hitch: the proxy overlay deletes it. Demanding it anyway made `hm start` bring the whole
# environment up on Linux and then fail with "Service 'hitch' is not running" — every time, for
# every project on the proxy.
#
# Checked by reading rather than by running: the step needs a Linux host with a stack up, which
# this suite does not have. What it can hold in place is that nothing demands the service any more.

ENTRADAS="$COMMAND_BIN_DIR/console/tasks/set_etc_hosts.sh"

test_case "the entries no longer demand a TLS terminator"
assert_equals "" "$(grep -o 'is_run_service "hitch"' "$ENTRADAS")"

test_case "the php container is still required, because that is where they are written"
assert_contains "$(cat "$ENTRADAS")" 'is_run_service "phpfpm"'

test_case "and with no terminator the entries are skipped rather than failed"
assert_contains "$(cat "$ENTRADAS")" 'if [ -z "$HITCH_CONTAINER" ]'

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
