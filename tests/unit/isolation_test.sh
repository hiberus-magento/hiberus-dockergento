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

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
