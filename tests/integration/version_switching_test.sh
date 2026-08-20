#!/usr/bin/env bash
#
# Reporting the installed version, switching between versions and not being taken off one.
#
# Everything runs against a throwaway clone of the repository, so the developer's own
# installation is never touched: this suite moves HEAD around, which is exactly the sort of
# thing you do not want happening to the hm you are using.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

LAB=$(cd "$(mktemp -d)" && pwd -P)
CLONE="$LAB/clone"
trap 'rm -rf "$LAB"' EXIT

if ! git -C "$COMMAND_BIN_DIR" rev-parse --git-dir >/dev/null 2>&1; then
    echo "  - skipped: the installation is not a git checkout"
    echo "RESULT 0 0"
    exit 0
fi

git clone -q "$COMMAND_BIN_DIR" "$CLONE" 2>/dev/null
branch=$(git -C "$COMMAND_BIN_DIR" rev-parse --abbrev-ref HEAD)
git -C "$CLONE" checkout -q "$branch" 2>/dev/null
HM="$CLONE/bin/run"

run() {
    ( cd "$LAB" && "$HM" "$@" >"$LAB/out" 2>"$LAB/err" )
    STATUS=$?
    STDOUT=$(cat "$LAB/out")
    STDERR=$(cat "$LAB/err")
    return 0
}

reference() {
    git -C "$CLONE" rev-parse --abbrev-ref HEAD
}

# Every switch that lands on an older version also lands on *its* code, which has no
# `switch`, no JSON and none of the protections. So the clone is put back on the branch
# under test with plain git before each case, otherwise the suite would be asserting the
# behaviour of whatever version it happened to end up on.
reset_clone() {
    git -C "$CLONE" checkout -q --force "$branch" 2>/dev/null
    git -C "$CLONE" clean -qfd 2>/dev/null
}

# ------------------------------------------------------------------ reporting

test_case "the version is reported in full, not rounded to the tag"
run --version --json
assert_json_field "$STDOUT" '.data.version | test("^[0-9]+\\.[0-9]+\\.[0-9]+")' "true"

test_case "the branch is reported"
assert_json_field "$STDOUT" '.data.branch' "$branch"

test_case "the checkout is not detached on a branch"
assert_json_field "$STDOUT" '.data.detached' "false"

test_case "the commit is reported"
assert_json_field "$STDOUT" '.data.commit | length > 0' "true"

test_case "the install path is reported and normalised"
assert_json_field "$STDOUT" '.data.path' "$CLONE"

test_case "the readable output names the tool and the version"
run --version --no-json
assert_contains "$STDOUT" "hm "

# ------------------------------------------------------------------ listing

test_case "the available versions are listed"
run switch --list --json
assert_json_field "$STDOUT" '.data.versions | length > 0' "true"

test_case "the current reference is reported"
assert_json_field "$STDOUT" '.data.current' "$branch"

# ------------------------------------------------------------------ switching

test_case "switching to a version detaches at that tag"
reset_clone
run switch 1.4.5
assert_equals "1.4.5" "$(git -C "$CLONE" describe --tags 2>/dev/null)"

test_case "a detached checkout is reported as such"
reset_clone
git -C "$CLONE" checkout -q --detach HEAD
run --version --json
assert_json_field "$STDOUT" '.data.detached' "true"

pinned_head=$(git -C "$CLONE" rev-parse HEAD)

test_case "update refuses to move a pinned installation"
run update --json
assert_json_field "$STDERR" '.error.type' "detached_installation"

test_case "refusing to update is not a generic failure"
assert_equals "6" "$STATUS"

test_case "and it changed nothing"
assert_equals "$pinned_head" "$(git -C "$CLONE" rev-parse HEAD)"

test_case "switching back to the stable branch works"
reset_clone
run switch --stable
assert_equals "main" "$(reference)"

test_case "switching to a branch by name works"
reset_clone
run switch "$branch"
assert_equals "$branch" "$(reference)"

test_case "update works normally on a branch"
reset_clone
run update
assert_equals "0" "$STATUS"

test_case "an unknown version is refused"
reset_clone
run switch 9.9.9-does-not-exist --json
assert_json_field "$STDERR" '.error.type' "unknown_reference"

test_case "and the installation did not move"
assert_equals "$branch" "$(reference)"

test_case "uncommitted changes stop the switch"
reset_clone
echo "# local change" >> "$CLONE/README.md"
run switch 1.4.5 --json
assert_json_field "$STDERR" '.error.type' "dirty_installation"

test_case "and again the installation did not move"
assert_equals "$branch" "$(reference)"

test_case "untracked files do not stop it"
reset_clone
printf 'notes\n' > "$CLONE/my-own-notes.txt"
run switch 1.4.5
assert_equals "1.4.5" "$(git -C "$CLONE" describe --tags 2>/dev/null)"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
