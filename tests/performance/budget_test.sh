#!/usr/bin/env bash
#
# Performance budgets.
#
# These are not micro-benchmarks: they exist to catch an order-of-magnitude regression, the
# kind that turns `hm --help` into a six second wait again. The budgets are deliberately
# loose, because the same call measured 72ms warm and 325ms cold on the same machine.
#
# Skip with HM_SKIP_PERF=1 on a machine too slow or too loaded to measure anything.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"
source "$(dirname "${BASH_SOURCE[0]}")/../lib/measure.sh"

# The binary answers the help and the version now, so that is what the budgets measure
HM="$COMMAND_BIN_DIR/bin/hm"

if [ -n "${HM_SKIP_PERF:-}" ]; then
    echo "  - skipped: HM_SKIP_PERF is set"
    echo "RESULT 0 0"
    exit 0
fi

# Process counts are not timings: they are the same on any machine, with or without Docker, so they
# are asserted before the timings are skipped. The shell tool's budget was 5 jq for the help; the
# binary parses its own JSON and spawns none, and a budget above that would let one creep back.
test_case "listing the commands spawns no jq"
spawned=$(count_spawned jq "$HM" --help)
if [ "$spawned" -eq 0 ]; then
    printf '  \033[0;32m✓\033[0m %s \033[0;90m(%s jq processes)\033[0m\n' "$HM_CURRENT_TEST" "$spawned"
else
    HM_TESTS_FAILED=$((HM_TESTS_FAILED + 1))
    printf '  \033[0;31m✗\033[0m %s\n      spawned %s jq processes, expected none: the binary reads its JSON itself\n' \
        "$HM_CURRENT_TEST" "$spawned"
fi
HM_TESTS_RUN=$((HM_TESTS_RUN + 1))

test_case "the startup floor does not call Docker"
spawned=$(count_spawned docker "$HM" --version)
assert_equals "0" "$spawned"

if ! docker info >/dev/null 2>&1; then
    echo "  - skipped: docker is not available, timings would not be comparable"
    echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
    exit 0
fi

WORKDIR=$(cd "$(mktemp -d)" && pwd -P)
trap 'rm -rf "$WORKDIR"' EXIT

# Somewhere that is not a project, so the budget measures the CLI and not a Magento install
cd "$WORKDIR" || exit 1

assert_faster_than 800 "listing the commands stays fast" "$HM" --help
# 900 rather than 600: the floor measures around 580 ms here, and a budget three per cent above
# the measurement fails on a loaded laptop instead of on a regression, which is the opposite of
# what these tests are for.
assert_faster_than 900 "the startup floor stays low" "$HM" --version
assert_faster_than 2500 "the diagnosis stays fast" "$COMMAND_BIN_DIR/bin/hm" doctor --json

# The dashboard's frame is the binary's now (internal/cli/tui_frame.go): it is composed once per
# size and painted from strings with no process spawned, which the Go tests in tui_test.go cover.
# The shell renderer these budgets measured is gone.

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
