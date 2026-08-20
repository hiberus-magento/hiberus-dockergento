#!/usr/bin/env bash
#
# How the CLI presents itself: colour end to end, credentials and the question flow.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

HM="$COMMAND_BIN_DIR/bin/run"
LAB=$(cd "$(mktemp -d)" && pwd -P)
trap 'rm -rf "$LAB"' EXIT

# Escape sequences survive a pipe, so they can be looked for directly
has_ansi() {
    printf '%s' "$1" | LC_ALL=C grep -q -- $'\033\\['
}

strip_ansi() {
    printf '%s' "$1" | LC_ALL=C perl -pe 's/\e\[[0-9;]*m//g'
}

run_help() {
    ( cd "$LAB" && env "$@" "$HM" --help 2>/dev/null )
}

test_case "a piped run has no colour"
has_ansi "$(run_help)" && r=color || r=plain
assert_equals "plain" "$r"

test_case "FORCE_COLOR colours a pipe"
has_ansi "$(run_help FORCE_COLOR=1)" && r=color || r=plain
assert_equals "color" "$r"

test_case "NO_COLOR wins over FORCE_COLOR"
has_ansi "$(run_help FORCE_COLOR=1 NO_COLOR=1)" && r=color || r=plain
assert_equals "plain" "$r"

test_case "TERM=dumb wins over FORCE_COLOR"
has_ansi "$(run_help FORCE_COLOR=1 TERM=dumb)" && r=color || r=plain
assert_equals "plain" "$r"

test_case "--no-color wins over everything"
output=$( cd "$LAB" && FORCE_COLOR=1 "$HM" --help --no-color 2>/dev/null )
has_ansi "$output" && r=color || r=plain
assert_equals "plain" "$r"

test_case "the text is the same with and without colour"
assert_equals "$(strip_ansi "$(run_help FORCE_COLOR=1)")" "$output"

test_case "--no-color is not passed on to the command"
output=$( cd "$LAB" && "$HM" noexiste --no-color 2>&1 >/dev/null )
assert_json_field "$output" '.command' "noexiste"

test_case "the logo follows the colour decision too"
has_ansi "$(run_help)" && r=color || r=plain
assert_equals "plain" "$r"

# ---------------------------------------------------------------- credentials

# Structural, not interactive: `read -s` disables echo by definition, and what a test can
# genuinely protect is that nobody adds a credential prompt without it. Driving a real
# terminal is unreliable here, because `script` echoes the piped stdin itself.
test_case "every credential prompt disables echo"
offenders=""
while IFS= read -r line; do
    [ -z "$line" ] && continue
    case "$line" in
        *"read -rs"* | *"read -s"* | *"read -sr"*) ;;
        *) offenders="$offenders $line" ;;
    esac
done <<< "$(LC_ALL=C grep -rn -i -E 'read +-[a-z]*p? *"[^"]*(password|secret|token)' \
            "$COMMAND_BIN_DIR/console" 2>/dev/null || true)"
assert_empty "$offenders" "these prompts echo a credential:"

# ---------------------------------------------------------------- questions

test_case "asking does not clear the screen"
offenders=$(LC_ALL=C grep -n -A3 -E '^(custom_question|custom_select)\(\)' \
    "$COMMAND_BIN_DIR/console/components/input_info.sh" | LC_ALL=C grep -c "clear_screen" || true)
assert_equals "0" "$offenders"

# A bare `clear` as a command, which is what wipes the user's scrollback. The one inside
# clear_screen is the implementation of the helper itself and is allowed.
test_case "no command clears the screen behind the user's back"
offenders=$(LC_ALL=C grep -rn -E '^[[:space:]]*clear[[:space:]]*$' \
    "$COMMAND_BIN_DIR/console" 2>/dev/null |
    LC_ALL=C grep -v "input_info.sh" || true)
assert_empty "$offenders" "these still clear the screen:"

test_case "the only clear left is inside the helper kept for the TUI"
remaining=$(LC_ALL=C grep -c -E '^[[:space:]]*clear[[:space:]]*$' \
    < "$COMMAND_BIN_DIR/console/components/input_info.sh" || true)
assert_equals "1" "$remaining"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
