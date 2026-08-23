#!/usr/bin/env bash
#
# A test run must not be able to touch the developer's own machine.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

test_case "the run has a HOME of its own"
[ -n "${HM_TEST_HOME:-}" ] && [ "$HOME" == "$HM_TEST_HOME" ] && r=isolated || r="HOME is $HOME"
assert_equals "isolated" "$r"

test_case "and it is not the real one"
[ "$HOME" != "$(eval echo ~"$(id -un)")" ] && r=different || r=same
assert_equals "different" "$r"

test_case "the cache is not in the real HOME"
case "$HM_CACHE_DIR" in
    "$HM_TEST_HOME"/*) r=isolated ;;
    /var/folders/*|/tmp/*|/private/*) r=isolated ;;
    *) r="$HM_CACHE_DIR" ;;
esac
assert_equals "isolated" "$r"

# The completion registration writes to the shell profile, so it is the one thing most likely to
# escape into the developer's dotfiles.
test_case "regenerating the completion from another checkout leaves the profile alone"
lab=$(mktemp -d)
mkdir -p "$lab/checkout/console/commands" "$lab/checkout/console/tasks" "$lab/checkout/console/helpers"
cp "$COMMAND_BIN_DIR/generate_completion.sh" "$lab/checkout/"
cp "$COMMAND_BIN_DIR/console/tasks/copyright.sh" "$lab/checkout/console/tasks/"
cp "$COMMAND_BIN_DIR/console/helpers/array_manager.sh" "$lab/checkout/console/helpers/"
touch "$lab/checkout/console/commands/start.sh"
printf '# untouched\n' > "$HOME/.zshrc"
( cd "$lab/checkout" && bash ./generate_completion.sh >/dev/null 2>&1 )
assert_equals "0" "$(LC_ALL=C grep -c 'hm-completion' "$HOME/.zshrc" | tr -d ' ')"

test_case "but it still generates its own completion file"
[ -f "$lab/checkout/console/hm-completion.bash" ] && r=generated || r=missing
assert_equals "generated" "$r"
rm -rf "$lab"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
