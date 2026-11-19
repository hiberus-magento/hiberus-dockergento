#!/usr/bin/env bash
#
# Nothing refers to a command script that is no longer there.
#
# A shell twin retired while a task still ran it by path fails only when that task runs:
# fix_linux_permissions.sh kept calling console/commands/docker-compose.sh, written as
# ${COMMANDS_DIR}/..., which a search for "$COMMANDS_DIR"/... did not see, and post-start failed on
# every Linux start. A test reading a deleted script with `grep ... || true` passes without
# checking anything. Every spelling of the path is looked for here.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

test_case "every command script referred to by path exists"
dead=""
while IFS=: read -r file line match; do
    name="${match##*/}"
    [ -f "$HM_TEST_PROJECT_ROOT/console/commands/$name" ] || dead="$dead $file:$line:$name"
done < <(cd "$HM_TEST_PROJECT_ROOT" &&
    grep -rnoE '\$\{?COMMANDS_DIR\}?"?/[a-z0-9_-]+\.sh' console bin tests 2>/dev/null)
assert_equals "" "$dead"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
