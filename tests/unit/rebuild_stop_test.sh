#!/usr/bin/env bash
#
# rebuild.sh stops through the binary (`hm_run_binary stop`) before `docker compose up --build`.
#
# Two things only prose said: the services are passed on to that stop, and a stop the binary fails
# or refuses ends the rebuild there, under set -e, before anything is built. Nothing real is
# reached: the binary, Compose and the tasks rebuild.sh calls are stand-ins that record what they
# were asked, and no Docker is involved.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

LAB="$HM_TEST_HOME/rebuild-stop"
CALLS="$LAB/calls"
mkdir -p "$LAB/bin" "$LAB/tasks"

# The binary answers stop with whatever STOP_STATUS says, and records what it was asked
cat > "$LAB/bin/hm" <<FAKE
#!/usr/bin/env bash
printf 'hm %s\n' "\$*" >> "$CALLS"
[ "\${1:-}" == "stop" ] && exit "\${STOP_STATUS:-0}"
exit 0
FAKE

cat > "$LAB/compose" <<FAKE
#!/usr/bin/env bash
printf 'compose %s\n' "\$*" >> "$CALLS"
FAKE

printf '#!/usr/bin/env bash\nprintf "validate_bind_mounts\\n" >> "%s"\n' "$CALLS" \
    > "$LAB/tasks/validate_bind_mounts.sh"
chmod +x "$LAB/bin/hm" "$LAB/compose" "$LAB/tasks/validate_bind_mounts.sh"

run_rebuild() {
    : > "$CALLS"
    COMMAND_BIN_DIR="$LAB" TASKS_DIR="$LAB/tasks" DOCKER_COMPOSE="$LAB/compose" MACHINE=mac \
        STOP_STATUS="${STOP_STATUS:-0}" \
        bash "$HM_TEST_PROJECT_ROOT/console/commands/rebuild.sh" "$@" >/dev/null 2>&1
    STATUS=$?
}

test_case "rebuild with no services stops through the binary, then builds nginx"
run_rebuild
assert_equals "0" "$STATUS"
assert_equals "hm stop
compose up --build -d nginx
validate_bind_mounts" "$(cat "$CALLS")"

test_case "the services are passed on to the stop and to the build"
run_rebuild phpfpm nginx
assert_equals "hm stop phpfpm nginx
compose up --build -d phpfpm nginx
validate_bind_mounts" "$(cat "$CALLS")"

test_case "a stop the binary refuses ends the rebuild with its code, before anything is built"
STOP_STATUS=6 run_rebuild
assert_equals "6" "$STATUS"
assert_equals "hm stop" "$(cat "$CALLS")"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
