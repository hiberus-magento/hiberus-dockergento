#!/usr/bin/env bash
#
# The terminal primitives against a real pseudo-terminal.
#
# What can be asserted here is the sequence of control codes the library emits, which is
# what determines whether the user gets their terminal back. Reading the visual result is
# not possible from a test; the order of the codes is.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

LAB=$(cd "$(mktemp -d)" && pwd -P)
trap 'rm -rf "$LAB"' EXIT

# `script` takes different arguments on macOS and on util-linux
pty_run() {
    local body="$1"
    local script_file="$LAB/body.sh"

    cat > "$script_file" <<SCRIPT
COMPONENTS_DIR="$COMPONENTS_DIR"
source "$COMPONENTS_DIR/tui.sh"
$body
SCRIPT

    # stdin is redirected: script(1) puts its own stdin into raw mode and fails outright
    # when that is not a terminal, which is the case in any non-interactive runner
    if script -q /dev/null bash "$script_file" >"$LAB/pty.out" 2>/dev/null </dev/null; then
        :
    elif script -q -c "bash $script_file" /dev/null >"$LAB/pty.out" 2>/dev/null </dev/null; then
        :
    else
        echo "unsupported"
        return 0
    fi

    cat "$LAB/pty.out"
}

probe=$(pty_run 'printf ready')

if [ "$probe" == "unsupported" ] || [ -z "$probe" ]; then
    echo "  - skipped: no usable script(1) to allocate a pseudo-terminal"
    echo "RESULT 0 0"
    exit 0
fi

# The escape character, spelled out so the assertions read
ESC=$'\033'

test_case "entering the alternate screen emits the sequence that preserves the scrollback"
output=$(pty_run 'tui_enter_screen; tui_leave_screen')
assert_contains "$output" "${ESC}[?1049h"

test_case "and leaving it emits the one that gives the terminal back"
assert_contains "$output" "${ESC}[?1049l"

test_case "the cursor is hidden and shown again"
output=$(pty_run 'tui_hide_cursor; tui_show_cursor')
assert_contains "$output" "${ESC}[?25l"

test_case "showing the cursor is emitted too"
assert_contains "$output" "${ESC}[?25h"

# The pseudo-terminal script(1) allocates really is 24x80, so this cannot assert that the
# size differs from the fallback: what it can assert is that two usable numbers came back
test_case "with a terminal the size is reported as two positive numbers"
output=$(pty_run 'tui_update_size; printf "size=%s,%s" "$TUI_ROWS" "$TUI_COLS"')
reported=$(printf '%s' "$output" | LC_ALL=C tr -d '\r' | sed -n 's/.*size=\([0-9]*\),\([0-9]*\).*/\1 \2/p')
rows=$(printf '%s' "$reported" | awk '{print $1}')
cols=$(printf '%s' "$reported" | awk '{print $2}')
{ [ "${rows:-0}" -gt 0 ] && [ "${cols:-0}" -gt 0 ]; } && r=ok || r="$reported"
assert_equals "ok" "$r"

test_case "the terminal is restored when the program simply ends"
output=$(pty_run 'tui_enter_screen; tui_hide_cursor; printf drawing')
assert_contains "$output" "${ESC}[?1049l"

test_case "and the cursor comes back with it"
assert_contains "$output" "${ESC}[?25h"

test_case "the terminal is restored after an interruption"
output=$(pty_run 'tui_enter_screen; tui_hide_cursor; kill -INT $$; printf never')
assert_contains "$output" "${ESC}[?1049l"

test_case "the cursor comes back after an interruption too"
assert_contains "$output" "${ESC}[?25h"

test_case "handing the terminal over lets another command write normally"
output=$(pty_run 'tui_enter_screen; tui_suspend; printf "handed over"; tui_resume; tui_leave_screen')
assert_contains "$output" "handed over"

# Counted with python: grep in this environment is not dependable for a pattern carrying a
# raw escape character
test_case "and it goes back to the alternate screen afterwards"
count=$(printf '%s' "$output" | python3 -c 'import sys; print(sys.stdin.buffer.read().count(b"\x1b[?1049h"))')
[ "$count" -ge 2 ] && r=ok || r="only $count"
assert_equals "ok" "$r"

test_case "an existing trap is not clobbered"
output=$(pty_run 'trap "printf mine" EXIT; tui_enter_screen; tui_hide_cursor')
assert_contains "$output" "mine"

test_case "and the terminal is restored as well"
assert_contains "$output" "${ESC}[?1049l"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
