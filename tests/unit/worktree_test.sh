#!/usr/bin/env bash
#
# Worktree detection and command classification.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"
source "$HELPERS_DIR/worktree.sh"

LAB=$(cd "$(mktemp -d)" && pwd -P)
trap 'rm -rf "$LAB"' EXIT

make_project() {
    local dir="$1"
    mkdir -p "$dir/config/docker"
    ( cd "$dir" || exit 1
      git init -q -b main
      git config user.email "test@example.com"
      git config user.name "test"
      echo "services: {}" > docker-compose.yml
      echo '{"COMPOSE_PROJECT_NAME": "lab"}' > config/docker/properties.json
      git add -A
      git commit -qm init )
}

resolve_in() {
    ( cd "$1" || exit 1
      hm_resolve_project_root
      printf '%s|%s|%s' "$HM_ROOT" "$HM_IS_WORKTREE" "$HM_WORKTREE" )
}

make_project "$LAB/project"
git -C "$LAB/project" worktree add --detach "$LAB/feature" HEAD >/dev/null 2>&1

test_case "the main checkout resolves to itself"
assert_equals "$LAB/project|false|" "$(resolve_in "$LAB/project")"

test_case "a worktree resolves to the main checkout, flagged and named"
assert_equals "$LAB/project|true|feature" "$(resolve_in "$LAB/feature")"

test_case "a directory outside git resolves to itself"
mkdir -p "$LAB/plain"
assert_equals "$LAB/plain|false|" "$(resolve_in "$LAB/plain")"

test_case "a git repository that is not a project is left alone"
mkdir -p "$LAB/other"
( cd "$LAB/other" && git init -q -b main &&
  git config user.email t@t && git config user.name t &&
  git commit -qm init --allow-empty ) >/dev/null 2>&1
git -C "$LAB/other" worktree add --detach "$LAB/other-wt" HEAD >/dev/null 2>&1
assert_equals "$LAB/other-wt|false|" "$(resolve_in "$LAB/other-wt")"

test_case "HM_PROJECT_DIR overrides the detection"
result=$( cd "$LAB/feature" || exit 1
          export HM_PROJECT_DIR="$LAB/project"
          hm_resolve_project_root
          printf '%s' "$HM_ROOT" )
assert_equals "$LAB/project" "$result"

test_case "commands that recreate the environment are classified as such"
blocked=""
for command in start stop restart rebuild down setup install create-project docker-stop-all; do
    hm_alters_environment "$command" || blocked="$blocked $command"
done
assert_empty "$blocked" "expected all of them to be blocked, these were not:"

test_case "read-only commands are not classified as destructive"
allowed=""
for command in describe list doctor logs mysql exec bash magento composer; do
    hm_alters_environment "$command" && allowed="$allowed $command"
done
assert_empty "$allowed" "these should not be blocked:"

test_case "commands that touch code are recognised"
hm_operates_on_code magento && r=yes || r=no
assert_equals "yes" "$r"

test_case "commands that do not touch code are not"
hm_operates_on_code list && r=yes || r=no
assert_equals "no" "$r"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
