#!/usr/bin/env bash
#
# The Linux half of post-start: matching the container's user and group ids to the host's.
#
# It ran Compose through console/commands/docker-compose.sh, which was retired with the rest of
# that command's shell twin; on Linux post-start then failed at its first step, and with it every
# `hm start`, since the binary hands post-start back on Linux. It runs Compose through
# $DOCKER_COMPOSE now, as set_etc_hosts.sh — the other half — already did. No Docker here: Compose is
# a stand-in that records what it was asked and answers the ids the host already has.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

LAB="$HM_TEST_HOME/linux-permissions"
CALLS="$LAB/calls"
mkdir -p "$LAB"

cat > "$LAB/compose" <<FAKE
#!/usr/bin/env bash
printf '%s\n' "\$*" >> "$CALLS"
case "\$*" in
    *"getent passwd"*) id -u ;;
    *"getent group"*)  id -g ;;
esac
exit 0
FAKE
chmod +x "$LAB/compose"

: > "$CALLS"
DOCKER_COMPOSE="$LAB/compose" USER_PHP=app GROUP_PHP=app USER="$(id -un)" \
    bash "$HM_TEST_PROJECT_ROOT/console/tasks/fix_linux_permissions.sh" >/dev/null 2>&1
STATUS=$?

test_case "with the ids already matching, it finishes without error"
assert_equals "0" "$STATUS"

test_case "it asks Compose, as root, for the ids in both services"
assert_contains "$(cat "$CALLS")" "exec -T -u root nginx sh -c getent passwd app | cut -f3 -d:"
assert_contains "$(cat "$CALLS")" "exec -T -u root phpfpm sh -c getent group app | cut -f3 -d:"

test_case "and changes nothing when they already match"
assert_equals "" "$(grep -E 'usermod|groupmod' "$CALLS" || true)"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
