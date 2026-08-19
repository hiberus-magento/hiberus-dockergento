#!/usr/bin/env bash
#
# Behaviour from a git worktree.
#
# Everything runs against a synthetic repository built here. Testing guardrails against a
# real project is how you find out they do not work by destroying the environment they
# were meant to protect.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

HM="$COMMAND_BIN_DIR/bin/run"
# Canonical path: on macOS /var is a symlink to /private/var, and git reports the resolved
# path. Comparing against the unresolved one makes assertions pass or fail for the wrong
# reason.
LAB=$(cd "$(mktemp -d)" && pwd -P)
PROJECT="hm-worktree-selftest"

cleanup() {
    ( cd "$LAB/main" 2>/dev/null && docker compose -p "$PROJECT" down -v --remove-orphans >/dev/null 2>&1 )
    rm -rf "$LAB"
}
trap cleanup EXIT

# ------------------------------------------------------------------ the lab

mkdir -p "$LAB/main/config/docker"
cd "$LAB/main" || exit 1

git init -q -b main
git config user.email "test@example.com"
git config user.name "test"

# A stack of alpine containers: this test is about paths and refusals, not about Magento
cat > docker-compose.yml <<'YAML'
services:
  phpfpm:
    image: alpine:latest
    command: ["sleep", "120"]
    volumes:
      - ./app:/var/www/html/app
YAML
cp docker-compose.yml docker-compose.dev.mac.yml
cp docker-compose.yml docker-compose.dev.linux.yml
mkdir -p app
echo '{"MAGENTO_DIR": ".", "COMPOSE_PROJECT_NAME": "hm-worktree-selftest", "DOMAIN": "worktree.local"}' \
    > config/docker/properties.json

git add -A
git commit -qm "init"
git worktree add --detach "$LAB/wt" HEAD >/dev/null 2>&1

run_in() {
    local dir="$1"
    shift
    ( cd "$dir" && "$HM" "$@" >"$LAB/out" 2>"$LAB/err" )
    STATUS=$?
    STDOUT=$(cat "$LAB/out")
    STDERR=$(cat "$LAB/err")
    return 0
}

# ------------------------------------------------------------------ detection

test_case "a worktree is recognised as such"
run_in "$LAB/wt" describe --json
assert_json_field "$STDOUT" '.data.project.worktree' "wt"

test_case "the root points at the main checkout"
assert_json_field "$STDOUT" '.data.project.root' "$LAB/main"

test_case "the main checkout is not treated as a worktree"
run_in "$LAB/main" describe --json
assert_json_field "$STDOUT" '.data.project.worktree' ""

test_case "the project name is the same from both directories"
assert_json_field "$STDOUT" '.data.project.name' "$PROJECT"

# ------------------------------------------------------------------ guardrails

for command in start stop restart rebuild down setup install docker-stop-all; do
    test_case "$command is refused from a worktree"
    run_in "$LAB/wt" "$command"
    assert_equals "6" "$STATUS"
done

test_case "the refusal is machine readable"
run_in "$LAB/wt" down --json -v
assert_json_field "$STDERR" '.error.type' "blocked_in_worktree"

test_case "the refusal names the main checkout"
run_in "$LAB/wt" start --no-json
assert_contains "$STDERR$STDOUT" "$LAB/main"

test_case "the refusal explains how to proceed"
assert_contains "$STDERR$STDOUT" "--force"

test_case "the same commands are allowed from the main checkout"
run_in "$LAB/main" stop
assert_equals "0" "$STATUS"

# ------------------------------------------------------------------ force

test_case "--force runs the command anyway"
run_in "$LAB/wt" stop --force
assert_equals "0" "$STATUS"

test_case "--force does not persist to the next invocation"
run_in "$LAB/wt" stop
assert_equals "6" "$STATUS"

# ------------------------------------------------------------------ allowed commands

test_case "describe works from a worktree"
run_in "$LAB/wt" describe --json
assert_equals "0" "$STATUS"

test_case "list works from a worktree"
run_in "$LAB/wt" list --json
assert_equals "0" "$STATUS"

test_case "doctor works from a worktree"
run_in "$LAB/wt" doctor --json
assert_equals "0" "$STATUS"

test_case "a command touching code warns about whose code it is"
run_in "$LAB/wt" magento --version
assert_contains "$STDERR" "$LAB/main"

# ------------------------------------------------------------------ the mounts

if ! docker info >/dev/null 2>&1; then
    echo "  - skipped: docker is not available for the mount checks"
    echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
    exit 0
fi

mounts_pointing_at() {
    docker inspect "$PROJECT-phpfpm-1" \
        --format '{{range .Mounts}}{{if eq .Type "bind"}}{{.Source}}{{"\n"}}{{end}}{{end}}' 2>/dev/null |
        grep -c "^$1" | tr -d ' '
}

run_in "$LAB/main" start

test_case "the environment mounts the main checkout"
[ "$(mounts_pointing_at "$LAB/main")" -gt 0 ] && r=yes || r=no
assert_equals "yes" "$r"

test_case "allowed commands from a worktree leave the mounts alone"
run_in "$LAB/wt" describe --json
run_in "$LAB/wt" doctor --json
assert_equals "0" "$(mounts_pointing_at "$LAB/wt")"

test_case "even forcing a start does not repoint the environment"
run_in "$LAB/wt" start --force
assert_equals "0" "$(mounts_pointing_at "$LAB/wt")"

test_case "the mounts still point at the main checkout"
[ "$(mounts_pointing_at "$LAB/main")" -gt 0 ] && r=yes || r=no
assert_equals "yes" "$r"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
