#!/usr/bin/env bash
#
# Every suite runs with a HOME of its own, run alone or through tests/run.sh.
#
# lib/assert.sh is where that happens: it swaps HOME for a throwaway directory and puts the cache
# under it. A suite that does not source it, or that hands the binary the real HOME back, writes
# into the developer's ~/.hm/cache — `validated-*` entries from throwaway projects piled up there,
# one per run.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

# not_isolated <suite...>: the suites that would reach the real HOME, one per line
not_isolated() {
    local suite
    for suite in "$@"; do
        if ! grep -q 'lib/assert\.sh' "$suite" || grep -qE '(^|[[:space:]])HOME=("?\$\{?REAL_HOME|~)' "$suite"; then
            printf '%s\n' "${suite#"$HM_TEST_PROJECT_ROOT"/}"
        fi
    done
}

suites=()
while IFS= read -r suite; do suites+=("$suite"); done < <(find "$HM_TEST_PROJECT_ROOT/tests" -name '*_test.sh' | sort)

test_case "there are suites to check"
[ "${#suites[@]}" -gt 10 ] && r=found || r="${#suites[@]} suites"
assert_equals "found" "$r"

test_case "every suite runs with a HOME of its own"
assert_empty "$(not_isolated "${suites[@]}")" "suites that reach the real HOME"

# The check itself, against suites made to fail it: a guard that never fires proves nothing
FIXTURE="$HM_TEST_HOME/isolation-fixture"
mkdir -p "$FIXTURE"
printf '#!/usr/bin/env bash\necho "RESULT 0 0"\n' > "$FIXTURE/bare_test.sh"
printf '#!/usr/bin/env bash\nsource lib/assert.sh\nHOME="$REAL_HOME" hm list\n' > "$FIXTURE/real_home_test.sh"
printf '#!/usr/bin/env bash\nsource lib/assert.sh\nhm list\n' > "$FIXTURE/isolated_test.sh"

flagged=$(not_isolated "$FIXTURE/bare_test.sh" "$FIXTURE/real_home_test.sh" "$FIXTURE/isolated_test.sh")

test_case "a suite that does not source lib/assert.sh is caught"
assert_contains "$flagged" "bare_test.sh"

test_case "a suite that hands the binary the real HOME is caught"
assert_contains "$flagged" "real_home_test.sh"

test_case "an isolated suite is not"
assert_not_contains "$flagged" "isolated_test.sh"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
