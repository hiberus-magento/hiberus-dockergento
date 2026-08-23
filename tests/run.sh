#!/usr/bin/env bash
#
# Test runner. Usage:
#   tests/run.sh            all suites
#   tests/run.sh unit       only unit suites
#   tests/run.sh <file>     a single suite
#
set -uo pipefail

TESTS_DIR="$(cd -P "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd -P "$TESTS_DIR/.." && pwd)"

# Directory variables the components expect when sourced outside of bin/run
export COMMAND_BIN_NAME="hm"
export COMMAND_TOOLNAME="Hiberus Dockergento"
export COMMAND_BIN_DIR="$PROJECT_ROOT"
export COMMANDS_DIR="$PROJECT_ROOT/console/commands"
export TASKS_DIR="$PROJECT_ROOT/console/tasks"
export HELPERS_DIR="$PROJECT_ROOT/console/helpers"
export COMPONENTS_DIR="$PROJECT_ROOT/console/components"
export DATA_DIR="$PROJECT_ROOT/data"
export HM_TEST_PROJECT_ROOT="$PROJECT_ROOT"

# A HOME for the whole run, shared by every suite. lib/assert.sh does the same when a suite is
# run on its own; see the reasoning there — it is not tidiness, it is that `hm switch`
# regenerates the shell completion and that registers itself in the shell profile.
if [ -z "${HM_TEST_HOME:-}" ]; then
    HM_TEST_HOME="$(mktemp -d)"
    export HM_TEST_HOME
    export DOCKER_CONFIG="${DOCKER_CONFIG:-$HOME/.docker}"
    export HOME="$HM_TEST_HOME"
    printf '# profile belonging to a test run\n' > "$HOME/.zshrc"
    trap 'rm -rf "$HM_TEST_HOME"' EXIT
fi

# The cache belongs inside that HOME, where the tool would put it anyway
HM_CACHE_DIR="${HM_CACHE_DIR:-$HOME/.hm/cache}"
export HM_CACHE_DIR

target="${1:-}"
suites=()

if [ -n "$target" ] && [ -f "$target" ]; then
    suites=("$target")
elif [ -n "$target" ] && [ -d "$TESTS_DIR/$target" ]; then
    while IFS= read -r suite; do suites+=("$suite"); done < <(find "$TESTS_DIR/$target" -name '*_test.sh' | sort)
else
    while IFS= read -r suite; do suites+=("$suite"); done < <(find "$TESTS_DIR" -name '*_test.sh' | sort)
fi

total_run=0
total_failed=0
failed_suites=()

for suite in "${suites[@]}"; do
    printf '\n\033[1;37m%s\033[0m\n' "$(basename "$suite")"

    output=$(bash "$suite" 2>&1)
    status=$?
    printf '%s\n' "$output"

    run=$(printf '%s' "$output" | tail -1 | sed -n 's/^RESULT \([0-9]*\) \([0-9]*\)$/\1/p')
    failed=$(printf '%s' "$output" | tail -1 | sed -n 's/^RESULT \([0-9]*\) \([0-9]*\)$/\2/p')

    total_run=$((total_run + ${run:-0}))
    total_failed=$((total_failed + ${failed:-0}))

    if [ "$status" -ne 0 ] || [ "${failed:-0}" -ne 0 ]; then
        failed_suites+=("$(basename "$suite")")
    fi
done

printf '\n\033[1;37m────────────────────────────\033[0m\n'
if [ "$total_failed" -eq 0 ] && [ ${#failed_suites[@]} -eq 0 ]; then
    printf '\033[0;32m%s assertions passed\033[0m\n' "$total_run"
    exit 0
fi

printf '\033[0;31m%s of %s assertions failed\033[0m\n' "$total_failed" "$total_run"
for suite in "${failed_suites[@]}"; do printf '  - %s\n' "$suite"; done
exit 1
