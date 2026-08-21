#!/usr/bin/env bash
#
# The dashboard end to end, driven through a pseudo-terminal.
#
# Keys are fed from a file so the run is deterministic: script(1) needs a terminal on its own
# stdin, so the dashboard reads its keys from the file instead.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

HM="$COMMAND_BIN_DIR/bin/run"
LAB=$(cd "$(mktemp -d)" && pwd -P)
trap 'rm -rf "$LAB"' EXIT

# ------------------------------------------------------- refuses without a terminal

test_case "the dashboard refuses when the output is not a terminal"
( cd "$LAB" && "$HM" tui --json >"$LAB/out" 2>"$LAB/err" )
status=$?
assert_equals "2" "$status"

test_case "and says what to use instead"
assert_json_field "$(cat "$LAB/err")" '.error.type' "requires_terminal"

test_case "and writes no control sequences"
assert_empty "$(cat "$LAB/out")"

# ------------------------------------------------------- the CLI still decides

# The dashboard has no privileges of its own: actions are the CLI's commands, run in the
# environment's directory. What is refused on the command line stays refused from the panel,
# and that is worth checking against a real refusal rather than trusting the wiring.
test_case "an action runs the CLI in the environment's directory"
assert_contains "$(LC_ALL=C grep -A3 'tui_suspend' "$COMMANDS_DIR/tui.sh")" 'cd "$root" && "$HM" "$@"'

mkdir -p "$LAB/main/config/docker"
(
    cd "$LAB/main" || exit 1
    git init -q -b main
    git config user.email "test@example.com"
    git config user.name "test"
    printf 'services:\n  phpfpm:\n    image: alpine:latest\n    command: ["sleep", "5"]\n' \
        > docker-compose.yml
    cp docker-compose.yml docker-compose.dev.mac.yml
    cp docker-compose.yml docker-compose.dev.linux.yml
    echo '{"MAGENTO_DIR": ".", "COMPOSE_PROJECT_NAME": "hm-tui-selftest"}' \
        > config/docker/properties.json
    git add -A && git commit -qm init
    git worktree add --detach "$LAB/wt" HEAD
) >/dev/null 2>&1

test_case "an action forbidden by the CLI is refused when launched the same way"
root="$LAB/wt"
( cd "$root" && "$HM" start >/dev/null 2>&1 )
assert_equals "6" "$?"

test_case "and allowed where the CLI allows it"
root="$LAB/main"
( cd "$root" && "$HM" stop >/dev/null 2>&1 )
assert_equals "0" "$?"

# ------------------------------------------------------- the interactive path

# script(1) comes in two flavours with incompatible argument order: BSD (macOS) takes the
# command after the file, util-linux (Linux) takes it with -c. Its own stdin must be closed:
# it refuses to run when that is not a terminal, which is exactly the case here.
run_tui() {
    local command="cd '$LAB' && '$HM' tui < '$LAB/keys'"
    printf '%s' "$1" > "$LAB/keys"

    if script -q /dev/null bash -c "$command" >"$LAB/pty" 2>/dev/null </dev/null ||
        script -qec "bash -c \"$command\"" /dev/null >"$LAB/pty" 2>/dev/null </dev/null; then
        cat "$LAB/pty"
        return 0
    fi

    echo "unsupported"
}

probe=$(run_tui "q")

if [ "$probe" == "unsupported" ] || [ -z "$probe" ]; then
    echo "  - skipped: no usable script(1) to allocate a pseudo-terminal"
    echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
    exit 0
fi

# The transcript as readable lines.
#
# A frame positions every line by absolute row instead of writing newlines, so the newlines have
# to be put back from those positions before the transcript can be read line by line. The escape
# is passed in as a literal character because neither sed nor awk understands \x1b in a pattern.
ESC=$(printf '\033')
plain() {
    printf '%s' "$1" | LC_ALL=C awk -v esc="$ESC" '{
        gsub(esc "\\[[0-9]+;1H", "\n")
        gsub(esc "\\[[0-9;?]*[a-zA-Z]", "")
        print
    }'
}

test_case "it enters its own screen"
assert_contains "$probe" $'\033[?1049h'

test_case "and leaves it when quitting"
assert_contains "$probe" $'\033[?1049l'

test_case "the cursor is hidden while drawing and restored on the way out"
assert_contains "$probe" $'\033[?25h'

test_case "it draws the fleet table"
assert_contains "$(plain "$probe")" "PROJECT"

test_case "it shows the keys available"
assert_contains "$(plain "$probe")" "q quit"

test_case "the first frame says it is reading, not that there is nothing"
assert_contains "$(plain "$probe")" "Reading the environments"

test_case "it says how old the data is"
assert_contains "$(plain "$probe")" "data from"

# The transcript holds every frame the dashboard drew, so the selection to compare is the
# one in the *last* frame, not the first: the first is always the initial draw.
test_case "moving down changes the selection"
selected_after() {
    plain "$1" | LC_ALL=C grep "^> " | tail -1 | awk '{print $2}'
}
moved=$(selected_after "$(run_tui "jq")")
untouched=$(selected_after "$(run_tui "q")")
{ [ -n "$moved" ] && [ -n "$untouched" ] && [ "$moved" != "$untouched" ]; } && r=moved || r="$untouched vs $moved"
assert_equals "moved" "$r"

# ------------------------------------------------------- how it draws

# Erasing the screen and redrawing shows an empty screen for one refresh: that is the flicker.
# A frame instead overwrites each line in place and clears what is left below it.
test_case "no frame erases the whole screen"
assert_not_contains "$probe" $'\033[2J'

test_case "frames are held while they are written and presented in one go"
assert_contains "$probe" $'\033[?2026h'

test_case "and released afterwards"
assert_contains "$probe" $'\033[?2026l'

test_case "each line is written where it belongs"
assert_contains "$probe" $'\033[1;1H'

test_case "and clears what was to its right"
assert_contains "$probe" $'\033[K'

test_case "what is left below the frame is cleared"
assert_contains "$probe" $'\033[J'

test_case "the escape sequences are real bytes, not the two characters that spell them"
assert_not_contains "$probe" '\033['

test_case "the keys help can be opened"
output=$(run_tui "?q")
assert_contains "$(plain "$output")" "move through the list"

test_case "quitting restores the terminal even after opening a sub screen"
assert_contains "$output" $'\033[?1049l'

test_case "an interruption also restores the terminal"
output=$(run_tui $'\003')
assert_contains "$output" $'\033[?1049l'

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
