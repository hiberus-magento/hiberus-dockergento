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

HM="$COMMAND_BIN_DIR/bin/run"

if [ -n "${HM_SKIP_PERF:-}" ]; then
    echo "  - skipped: HM_SKIP_PERF is set"
    echo "RESULT 0 0"
    exit 0
fi

if ! docker info >/dev/null 2>&1; then
    echo "  - skipped: docker is not available, timings would not be comparable"
    echo "RESULT 0 0"
    exit 0
fi

WORKDIR=$(cd "$(mktemp -d)" && pwd -P)
trap 'rm -rf "$WORKDIR"' EXIT

# Somewhere that is not a project, so the budget measures the CLI and not a Magento install
cd "$WORKDIR" || exit 1

assert_faster_than 800 "listing the commands stays fast" "$HM" --help
assert_faster_than 600 "the startup floor stays low" "$HM" --version
assert_faster_than 2500 "the diagnosis stays fast" "$HM" doctor --json

test_case "listing the commands spawns few processes"
spawned=$(count_spawned jq "$HM" --help)
if [ "$spawned" -le 5 ]; then
    printf '  \033[0;32m✓\033[0m %s \033[0;90m(%s jq processes)\033[0m\n' "$HM_CURRENT_TEST" "$spawned"
else
    HM_TESTS_FAILED=$((HM_TESTS_FAILED + 1))
    printf '  \033[0;31m✗\033[0m %s\n      spawned %s jq processes, expected 5 or fewer\n' \
        "$HM_CURRENT_TEST" "$spawned"
fi
HM_TESTS_RUN=$((HM_TESTS_RUN + 1))

test_case "the startup floor does not call Docker"
spawned=$(count_spawned docker "$HM" --version)
assert_equals "0" "$spawned"

# ---------------------------------------------------------------- the dashboard frame

#
# The dashboard redraws on every keystroke, so its frame has a budget of its own and it is far
# tighter than a command's: what a keystroke costs is what the tool feels like.
#
# The baseline this replaced was 404 ms per frame with ten environments, because the layout was
# recomputed from JSON on every key. The budget is set at 20 ms with twenty environments, which
# is roughly ten times what it measures — loose enough to survive a loaded laptop, tight enough
# that recomputing anything from JSON in the paint path would break it.
#
frame_harness() {
    local count="$1"
    local script="$WORKDIR/frame.sh"

    cat > "$script" <<'HARNESS'
set -uo pipefail
source "$TASKS_DIR/tui_render.sh"

count="$1"
rows=""
for ((i = 0; i < count; i++)); do
    [ -n "$rows" ] && rows="$rows,"
    rows="$rows{"name":"env-$i","status":"stopped","containers":{"running":0,"total":9},"branch":"main","root":"/projects/env-$i","worktree":"","orphan":false,"has_metadata":true}"
done

FLEET_JSON="{"data":{"environments":[$rows]}}"
DOCTOR_JSON='{"data":{"checks":[]}}'
source "$TASKS_DIR/tui_frame.sh"

TUI_COLS=120
TUI_ROWS=40
LOADED_AT="12:00:00"
compose

# A hundred frames, so the measurement is of frames and not of the shell starting up
for ((i = 0; i < 100; i++)); do
    SELECTED=$((i % count))
    build_frame
    frame_bytes
done
HARNESS

    bash "$script" "$count"
}

test_case "a dashboard frame stays under its budget"
start=$(python3 -c 'import time; print(time.time())')
frame_harness 20 >/dev/null 2>&1
end=$(python3 -c 'import time; print(time.time())')
per_frame=$(python3 -c "print(int((($end - $start) * 1000) / 100))")

if [ "$per_frame" -le 20 ]; then
    printf '  [0;32m✓[0m %s [0;90m(%s ms per frame, budget 20 ms)[0m
'         "$HM_CURRENT_TEST" "$per_frame"
else
    HM_TESTS_FAILED=$((HM_TESTS_FAILED + 1))
    printf '  [0;31m✗[0m %s
      %s ms per frame, budget is 20 ms
'         "$HM_CURRENT_TEST" "$per_frame"
fi
HM_TESTS_RUN=$((HM_TESTS_RUN + 1))

#
# Composing is allowed to call `jq`; painting is not. Counting processes cannot tell them apart
# inside one shell, so `jq` is shadowed by a function right after composing: anything that
# reaches for it while a frame is being built says so out loud.
#
test_case "painting a frame runs no jq at all"
export TASKS_DIR
noise=$(bash -c '
    source "$TASKS_DIR/tui_render.sh"
    FLEET_JSON="{\"data\":{\"environments\":[{\"name\":\"a\",\"status\":\"stopped\",\"containers\":{\"running\":0,\"total\":1},\"branch\":\"main\",\"root\":\"/a\",\"worktree\":\"\",\"orphan\":false,\"has_metadata\":true}]}}"
    DOCTOR_JSON="{\"data\":{\"checks\":[]}}"
    source "$TASKS_DIR/tui_frame.sh"
    TUI_COLS=100; TUI_ROWS=24
    compose

    jq() { echo "jq in the paint path"; }
    awk() { echo "awk in the paint path"; }

    for i in 1 2 3 4 5 6 7 8 9 10; do build_frame; frame_bytes; done
    printf "%s" "$TUI_BYTES" >/dev/null
' 2>&1)
assert_empty "$noise"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
