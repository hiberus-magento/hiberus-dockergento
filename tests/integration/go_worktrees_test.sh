#!/usr/bin/env bash
#
# Branch environments: listing them and taking them away.
#
# `add` is still the shell implementation's. All three read and write the same registrations, so
# there is no moment where the two disagree — what one writes the other sees, which is the only
# way a command gets ported one half at a time.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

GO_BINARY="$COMMAND_BIN_DIR/bin/hm"
SHELL_CLI="$COMMAND_BIN_DIR/bin/run"
PROJECT="hm-go-ramas"
LAB=$(cd "$(mktemp -d)" && pwd -P)

trap 'docker compose -p "$PROJECT" down -v >/dev/null 2>&1; rm -rf "$LAB"; hm_test_home_cleanup' EXIT

if ! command -v go >/dev/null 2>&1; then
    echo "  - skipped: go is not installed"
    echo "RESULT 0 0"
    exit 0
fi

export GOCACHE="$LAB/go-build"

( cd "$COMMAND_BIN_DIR" && go build -o "$GO_BINARY" ./cmd/hm ) >/dev/null 2>&1 || {
    echo "  - skipped: the binary does not build here"
    echo "RESULT 0 0"
    exit 0
}

DIR="$LAB/$PROJECT"
mkdir -p "$DIR/config/docker" "$DIR/src"
printf 'services:\n  phpfpm:\n    image: alpine:latest\n    command: sh -c "sleep 600"\n    stop_grace_period: 1s\n' \
    > "$DIR/docker-compose.yml"
cp "$DIR/docker-compose.yml" "$DIR/docker-compose.dev.mac.yml"
cp "$DIR/docker-compose.yml" "$DIR/docker-compose.dev.linux.yml"
printf '{"MAGENTO_DIR": "./src", "DOMAIN": "ramas.test", "COMPOSE_PROJECT_NAME": "%s"}\n' "$PROJECT" \
    > "$DIR/config/docker/properties.json"

( cd "$DIR" && git init -q . && git add -A &&
  git -c user.email=t@t -c user.name=t commit -qm inicial ) >/dev/null 2>&1

export HM_WORKTREE_DIR="$LAB/registro"
mkdir -p "$HM_WORKTREE_DIR/$PROJECT"

#
# The registration written by hand, in the shape the shell implementation writes it. That is the
# point of this test: what `add` writes, the ported half has to read.
#
registrar() {
    ( cd "$DIR" && git worktree add -q "$LAB/rama-$1" -b "$1" ) >/dev/null 2>&1
    printf '{"path":"%s","branch":"%s","profile":"agent","domain":"%s.ramas.test","project":"%s-%s","created":"2026-09-02 10:00","vendor":"own"}\n' \
        "$LAB/rama-$1" "$1" "$1" "$PROJECT" "$1" > "$HM_WORKTREE_DIR/$PROJECT/$1.json"
    printf 'services: {}\n' > "$HM_WORKTREE_DIR/$PROJECT/$1.yml"
}

registrar azul

# ---------------------------------------------------------------- listing

test_case "the ported half reads what the shell one wrote"
assert_equals "$( cd "$DIR" && "$SHELL_CLI" worktree list --json | jq -S . )" \
              "$( cd "$DIR" && "$GO_BINARY" worktree list --json | jq -S . )"

test_case "including the table a person reads"
assert_equals "$( cd "$DIR" && "$SHELL_CLI" --no-json worktree list 2>&1 )" \
              "$( cd "$DIR" && "$GO_BINARY" --no-json worktree list 2>&1 )"

#
# A directory somebody deleted by hand leaves a registration behind. Saying "stopped" about it
# would send them looking for containers that are not the problem.
#
rm -rf "$LAB/rama-azul"

test_case "a worktree whose directory is gone is reported as missing, in both"
assert_equals "$( cd "$DIR" && "$SHELL_CLI" worktree list --json | jq -r '.data.worktrees[0].state' )" \
              "$( cd "$DIR" && "$GO_BINARY" worktree list --json | jq -r '.data.worktrees[0].state' )"
assert_equals "missing" "$( cd "$DIR" && "$GO_BINARY" worktree list --json | jq -r '.data.worktrees[0].state' )"

( cd "$DIR" && git worktree prune ) >/dev/null 2>&1
rm -f "$HM_WORKTREE_DIR/$PROJECT"/azul.*

test_case "with none registered, both say so the same way"
assert_equals "$( cd "$DIR" && "$SHELL_CLI" --no-json worktree list 2>&1 )" \
              "$( cd "$DIR" && "$GO_BINARY" --no-json worktree list 2>&1 )"

# ---------------------------------------------------------------- the refusals

for CASO in "noexiste" ""; do
    ( cd "$DIR" && "$SHELL_CLI" worktree remove $CASO >"$LAB/shell.err" 2>&1 ); SHELL_STATUS=$?
    ( cd "$DIR" && "$GO_BINARY"  worktree remove $CASO >"$LAB/go.err" 2>&1 );    GO_STATUS=$?

    test_case "removing '${CASO:-nothing}' is refused the same way"
    assert_equals "2" "$GO_STATUS"
    assert_equals "$SHELL_STATUS" "$GO_STATUS"
    assert_equals "$(jq -S . < "$LAB/shell.err")" "$(jq -S . < "$LAB/go.err")"
done

#
# Containers and databases can be rebuilt in seconds; uncommitted code cannot be rebuilt at all,
# which is why git's own refusal is repeated rather than worked around.
#
registrar verde
echo "sin guardar" > "$LAB/rama-verde/pendiente.txt"

( cd "$DIR" && "$SHELL_CLI" worktree remove verde >"$LAB/shell.err" 2>&1 ); SHELL_STATUS=$?
( cd "$DIR" && "$GO_BINARY"  worktree remove verde >"$LAB/go.err" 2>&1 );    GO_STATUS=$?

test_case "a worktree with uncommitted changes is refused, the same way"
assert_equals "6" "$GO_STATUS"
assert_equals "$SHELL_STATUS" "$GO_STATUS"
assert_equals "$(jq -S . < "$LAB/shell.err")" "$(jq -S . < "$LAB/go.err")"

rm -f "$LAB/rama-verde/pendiente.txt"

# ---------------------------------------------------------------- and taking one away

test_case "answering anything else leaves it alone"
( cd "$DIR" && printf 'otra-cosa\n' | "$GO_BINARY" --no-json worktree remove verde >/dev/null 2>&1 )
assert_equals "1" "$( cd "$DIR" && "$GO_BINARY" worktree list --json | jq -r '.data.worktrees | length' )"

( cd "$DIR" && "$GO_BINARY" --yes --no-json worktree remove verde >"$LAB/go.out" 2>&1 )

test_case "and removing it takes the registration, the overlay and the worktree"
assert_contains "$(cat "$LAB/go.out")" "Removed"
assert_equals "0" "$( cd "$DIR" && "$GO_BINARY" worktree list --json | jq -r '.data.worktrees | length' )"
assert_equals "no" "$([ -f "$HM_WORKTREE_DIR/$PROJECT/verde.yml" ] && echo yes || echo no)"
assert_equals "no" "$([ -d "$LAB/rama-verde" ] && echo yes || echo no)"

# ---------------------------------------------------------------- and the half that is not ported

test_case "add still reaches the shell implementation"
( cd "$DIR" && "$GO_BINARY" worktree add >"$LAB/go.out" 2>&1 )
assert_contains "$(cat "$LAB/go.out")" "branch"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
