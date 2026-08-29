#!/usr/bin/env bash
#
# The selector driven through a real terminal.
#
# Keys come from a file: script(1) needs a terminal on its own stdin, so the selector reads its
# keys from the file instead — the same arrangement the dashboard's suite uses.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

LAB=$(cd "$(mktemp -d)" && pwd -P)
trap 'rm -rf "$LAB"' EXIT

#
# fzf is skipped on purpose: it is what a person who installed it wants, and a test cannot press
# its keys. What is tested is our own selector, which is what everybody else gets.
#
ask() {
    printf '%s' "$1" > "$LAB/keys"

    cat > "$LAB/ask.sh" <<'INNER'
export COMPONENTS_DIR HELPERS_DIR
source "$COMPONENTS_DIR/print_message.sh"
source "$COMPONENTS_DIR/select.sh"
hm_select_interactive "¿Qué hacemos?" "Guardar y destruir" "Destruir" "Cancelar" < "$KEYS"
printf 'ELEGIDO:%s\n' "$REPLY"
INNER

    if KEYS="$LAB/keys" script -q /dev/null bash "$LAB/ask.sh" >"$LAB/pty" 2>/dev/null </dev/null ||
        KEYS="$LAB/keys" script -qec "bash '$LAB/ask.sh'" /dev/null >"$LAB/pty" 2>/dev/null </dev/null; then
        sed -n 's/.*ELEGIDO:\(.*\)/\1/p' "$LAB/pty" | tail -1 | tr -d '\r'
        return 0
    fi

    echo "unsupported"
}

probe=$(ask $'\n')

if [ "$probe" == "unsupported" ]; then
    echo "  - skipped: script(1) is not available in a usable form"
    echo "RESULT 0 0"
    exit 0
fi

# ---------------------------------------------------------------- the default

#
# The point of the exercise: the safest option of `hm down -v` is now the one you get by pressing
# Enter, instead of a number you have to read and type.
#
test_case "Enter takes the first option"
assert_equals "Guardar y destruir" "$probe"

# ---------------------------------------------------------------- moving

test_case "the down arrow moves"
assert_equals "Destruir" "$(ask $'\033[B\n')"

test_case "twice moves twice"
assert_equals "Cancelar" "$(ask $'\033[B\033[B\n')"

test_case "the up arrow comes back"
assert_equals "Guardar y destruir" "$(ask $'\033[B\033[A\n')"

test_case "and it wraps at the end"
assert_equals "Guardar y destruir" "$(ask $'\033[B\033[B\033[B\n')"

test_case "up from the first wraps to the last"
assert_equals "Cancelar" "$(ask $'\033[A\n')"

test_case "j and k move as well"
assert_equals "Destruir" "$(ask 'j'$'\n')"
assert_equals "Cancelar" "$(ask 'k'$'\n')"

# ---------------------------------------------------------------- digits

test_case "a digit chooses without pressing Enter"
assert_equals "Destruir" "$(ask '2')"

test_case "a digit past the end is ignored"
assert_equals "Guardar y destruir" "$(ask '9'$'\n')"

# ---------------------------------------------------------------- escape

#
# Escape does nothing on purpose: callers read REPLY and act on it, so a cancel that returned an
# empty answer would have them carry on with nothing chosen — for a destructive question that is
# the wrong branch.
#
test_case "escape does not choose anything on its own"
assert_equals "Guardar y destruir" "$(ask $'\033\n')"

# ---------------------------------------------------------------- redrawing

test_case "the list is rewritten in place rather than scrolled"
ask $'\033[B\n' >/dev/null
assert_contains "$(cat "$LAB/pty")" $'\033[3A'

test_case "and the question is printed once"
assert_equals "1" "$(grep -c '¿Qué hacemos?' "$LAB/pty")"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
