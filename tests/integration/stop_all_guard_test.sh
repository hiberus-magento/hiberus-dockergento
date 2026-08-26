#!/usr/bin/env bash
#
# The command that stops every container on the machine.
#
# Never run for real here: it would stop whatever the developer has running, which is precisely
# the damage it now asks about. `docker` is replaced by a stub that records what it was told to do,
# so the logic is exercised and nothing is touched.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

LAB=$(cd "$(mktemp -d)" && pwd -P)
trap 'rm -rf "$LAB"' EXIT

mkdir -p "$LAB/bin"
cat > "$LAB/bin/docker" <<'STUB'
#!/usr/bin/env bash
# Three containers running: two of this project, one of somebody else's
case "$1 $2" in
    "ps -q")
        for argument in "$@"; do
            case "$argument" in
                label=com.docker.compose.project=*)
                    printf 'aaa1\nbbb2\n'
                    exit 0
                    ;;
            esac
        done
        printf 'aaa1\nbbb2\nccc3\n'
        ;;
    "stop "*)
        shift
        printf 'stopped %s\n' "$*" >> "$STUB_LOG"
        ;;
    *) : ;;
esac
STUB
chmod +x "$LAB/bin/docker"

export STUB_LOG="$LAB/stopped.log"
: > "$STUB_LOG"

run_stop_all() {
    local answer="$1"
    : > "$STUB_LOG"
    printf '%s\n' "$answer" | env PATH="$LAB/bin:$PATH" \
        COMPONENTS_DIR="$COMPONENTS_DIR" HELPERS_DIR="$HELPERS_DIR" \
        COMMAND_BIN_NAME="$COMMAND_BIN_NAME" COMPOSE_PROJECT_NAME="mine" \
        HM_NON_INTERACTIVE="" \
        bash "$COMMANDS_DIR/docker-stop-all.sh" 2>&1
}

test_case "it says how many it would stop"
output=$(run_stop_all "n")
assert_contains "$output" "3 container"

test_case "and how many belong to somebody else"
assert_contains "$output" "1 of them do not belong"

test_case "answering no stops nothing"
assert_empty "$(cat "$STUB_LOG")"

test_case "and says so"
assert_contains "$output" "Nothing was stopped"

test_case "answering yes stops them"
output=$(run_stop_all "y")
assert_contains "$(cat "$STUB_LOG")" "stopped"

test_case "all of them, not just this project's"
assert_contains "$(cat "$STUB_LOG")" "ccc3"

test_case "with --yes there is no question at all"
: > "$STUB_LOG"
output=$(env PATH="$LAB/bin:$PATH" COMPONENTS_DIR="$COMPONENTS_DIR" HELPERS_DIR="$HELPERS_DIR" \
    COMMAND_BIN_NAME="$COMMAND_BIN_NAME" COMPOSE_PROJECT_NAME="mine" HM_NON_INTERACTIVE="1" \
    bash "$COMMANDS_DIR/docker-stop-all.sh" </dev/null 2>&1)
assert_not_contains "$output" "Stop them all"

test_case "and it stops them"
assert_contains "$(cat "$STUB_LOG")" "stopped"

test_case "with nothing running it neither asks nor fails"
cat > "$LAB/bin/docker" <<'STUB'
#!/usr/bin/env bash
exit 0
STUB
chmod +x "$LAB/bin/docker"
output=$(run_stop_all "y")
assert_contains "$output" "No containers running"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
