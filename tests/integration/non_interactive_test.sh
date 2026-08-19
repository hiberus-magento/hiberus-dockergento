#!/usr/bin/env bash
#
# No command may block waiting for input when there is nobody to answer.
#
# stdin is attached to a FIFO that nobody ever writes to, so any prompt blocks forever
# and is caught by the time limit. Testing with a closed stdin would pass trivially,
# because `read` returns immediately at EOF.
#
# Commands excluded on purpose, because running them would have real side effects:
#   docker-stop-all  stops every container on the machine
#   update           runs git pull on the hm installation
#   create-project   downloads a Magento project
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

HM="$COMMAND_BIN_DIR/bin/run"
WORKDIR=$(mktemp -d)
trap 'rm -rf "$WORKDIR"' EXIT

check() {
    local label="$1"
    shift
    test_case "$label"
    assert_does_not_block 20 env HM_NON_INTERACTIVE=1 HM_WORKDIR="$WORKDIR" \
        bash -c 'cd "$HM_WORKDIR" && exec "$@"' _ "$HM" "$@"
}

# Commands that skip project validation and therefore reach their own prompts
check "setup does not block"        setup
check "compatibility does not block" compatibility
check "ai-init does not block"      ai-init
check "ai-pull does not block"      ai-pull
check "ai-reset does not block"     ai-reset

# Commands that validate the project first: they must fail fast, never hang
check "bash does not block"         bash
check "start does not block"        start
check "install does not block"      install
check "transfer-db does not block"  transfer-db
check "transfer-media does not block" transfer-media
check "masquerade does not block"   masquerade
check "mysql does not block"        mysql

test_case "a question without a default fails with the usage code"
status=0
( cd "$WORKDIR" && HM_NON_INTERACTIVE=1 "$HM" setup >/dev/null 2>&1 ) || status=$?
assert_equals "2" "$status"

test_case "the failure explains what is missing"
stderr=$( ( cd "$WORKDIR" && HM_NON_INTERACTIVE=1 "$HM" setup 2>&1 >/dev/null ) || true )
assert_contains "$stderr" "input_required"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
