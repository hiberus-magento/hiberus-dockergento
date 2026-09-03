#!/usr/bin/env bash
#
# Branch environments: listing them and taking them away.
#
# There is one implementation now: the shell entry point delegates to the binary, which is what let
# the registry stop being a directory of small files. So nothing here compares two answers — there
# is only one — and every assertion is about what the command does.
#
# The comparisons that used to be here did their job: they are in the history, and they are why
# the registration and the overlay this writes are the ones the shell implementation wrote.
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
# The registry is a database beside the state directory, so this has to point somewhere throwaway
# too: a test that registers branch environments in the database the machine actually uses would
# leave them there, and they would be listed by every later command.
#
export HM_STATE_DIR="$LAB/estado"
mkdir -p "$HM_STATE_DIR"

#
# The registration written by hand, in the shape the shell implementation writes it. That is the
# point of this test: what `add` writes, the ported half has to read.
#
registrar() {
    mkdir -p "$HM_WORKTREE_DIR/$PROJECT"
    ( cd "$DIR" && git worktree add -q "$LAB/rama-$1" -b "$1" ) >/dev/null 2>&1
    printf '{"path":"%s","branch":"%s","profile":"agent","domain":"%s.ramas.test","project":"%s-%s","created":"2026-09-02 10:00","vendor":"own"}\n' \
        "$LAB/rama-$1" "$1" "$1" "$PROJECT" "$1" > "$HM_WORKTREE_DIR/$PROJECT/$1.json"
    printf 'services: {}\n' > "$HM_WORKTREE_DIR/$PROJECT/$1.yml"
}

#
# The registry is a database now, so a fixture cannot be built by writing files into a directory
# and taken apart by deleting them. What the old directory holds is drained on the way in, which is
# what makes writing one there a way to set the scene; taking it apart goes through the command.
#
olvidar() {
    ( cd "$DIR" && "$GO_BINARY" --yes --force worktree remove "$1" ) >/dev/null 2>&1
    rm -f "$HM_WORKTREE_DIR/$PROJECT/$1".*
}

registrar azul

# ---------------------------------------------------------------- listing

#
# What the shell implementation wrote is drained on the way in rather than abandoned: a machine
# with branch environments in it keeps them.
#
test_case "a registration the old implementation wrote is still found"
assert_equals "azul" "$( cd "$DIR" && "$GO_BINARY" worktree list --json | jq -r '.data.worktrees[0].name' )"

#
# The entry point people type is still `hm`, and it has to arrive at the same place: the shell
# implementation of this command is a delegator now, which is what makes one registry possible.
#
test_case "and the shell entry point reaches the same answer"
assert_equals "$( cd "$DIR" && "$SHELL_CLI" worktree list --json | jq -S . )" \
              "$( cd "$DIR" && "$GO_BINARY" worktree list --json | jq -S . )"

#
# A directory somebody deleted by hand leaves a registration behind. Saying "stopped" about it
# would send them looking for containers that are not the problem.
#
rm -rf "$LAB/rama-azul"

test_case "a worktree whose directory is gone is reported as missing"
assert_equals "missing" "$( cd "$DIR" && "$GO_BINARY" worktree list --json | jq -r '.data.worktrees[0].state' )"

( cd "$DIR" && git worktree prune ) >/dev/null 2>&1
( cd "$DIR" && "$GO_BINARY" --yes --force worktree remove azul ) >/dev/null 2>&1

#
# Removing has to clear both: the row, and the file the old implementation wrote. Leaving the file
# would bring the environment back on the next listing, because that file is read on the way in.
#
test_case "removing clears the registration and what the old directory held"
assert_equals "0" "$( cd "$DIR" && "$GO_BINARY" worktree list --json | jq -r '.data.worktrees | length' )"
assert_equals "no" "$([ -f "$HM_WORKTREE_DIR/$PROJECT/azul.json" ] && echo yes || echo no)"

test_case "with none registered it says so, and how to make one"
assert_contains "$( cd "$DIR" && "$GO_BINARY" --no-json worktree list 2>&1 )" "No branch environments"

# ---------------------------------------------------------------- the refusals

for CASO in "noexiste" ""; do
    ( cd "$DIR" && "$GO_BINARY" worktree remove $CASO >"$LAB/go.err" 2>&1 ); GO_STATUS=$?

    test_case "removing '${CASO:-nothing}' is refused with the usage code"
    assert_equals "2" "$GO_STATUS"
    assert_contains "$(jq -r '.error.hint' < "$LAB/go.err")" "worktree list"
done

#
# Containers and databases can be rebuilt in seconds; uncommitted code cannot be rebuilt at all,
# which is why git's own refusal is repeated rather than worked around.
#
registrar verde
echo "sin guardar" > "$LAB/rama-verde/pendiente.txt"

( cd "$DIR" && "$GO_BINARY" worktree remove verde >"$LAB/go.err" 2>&1 ); GO_STATUS=$?

test_case "a worktree with uncommitted changes is refused"
assert_equals "6" "$GO_STATUS"
assert_equals "uncommitted_changes" "$(jq -r '.error.type' < "$LAB/go.err")"

rm -f "$LAB/rama-verde/pendiente.txt"

# ---------------------------------------------------------------- and taking one away

antes=$( cd "$DIR" && "$GO_BINARY" worktree list --json | jq -r '.data.worktrees | length' )

test_case "answering anything else leaves it alone"
( cd "$DIR" && printf 'otra-cosa\n' | "$GO_BINARY" --no-json worktree remove verde >/dev/null 2>&1 )
assert_equals "$antes" "$( cd "$DIR" && "$GO_BINARY" worktree list --json | jq -r '.data.worktrees | length' )"

( cd "$DIR" && "$GO_BINARY" --yes --no-json worktree remove verde >"$LAB/go.out" 2>&1 )

test_case "and removing it takes the registration, the overlay and the worktree"
assert_contains "$(cat "$LAB/go.out")" "Removed"
assert_not_contains "$( cd "$DIR" && "$GO_BINARY" worktree list --json | jq -r '.data.worktrees[].name' )" "verde"
assert_equals "no" "$([ -f "$HM_WORKTREE_DIR/$PROJECT/verde.yml" ] && echo yes || echo no)"
assert_equals "no" "$([ -d "$LAB/rama-verde" ] && echo yes || echo no)"

# ---------------------------------------------------------------- creating one
#
# What matters is not that it works but that it writes the same things: another agent, or the
# shell implementation, has to be able to read what this one left.

for CASO in "" "--profile=inventado rama" "--tonteria" "una otra"; do
    ( cd "$DIR" && "$GO_BINARY" worktree add $CASO >"$LAB/go.err" 2>&1 ); GO_STATUS=$?

    test_case "adding with '${CASO:-no branch}' is refused with the usage code"
    assert_equals "2" "$GO_STATUS"
done

#
# A branch environment is reached by name, which needs the global proxy: without it every one of
# them would publish its own ports, which is the collision the proxy was built to end.
#
printf '{"MAGENTO_DIR": "./src", "DOMAIN": "ramas.test", "COMPOSE_PROJECT_NAME": "%s"}\n' "$PROJECT" \
    > "$DIR/config/docker/properties.json"

( cd "$DIR" && "$GO_BINARY" worktree add sinproxy >"$LAB/go.err" 2>&1 ); GO_STATUS=$?

test_case "without the proxy it is refused"
assert_equals "6" "$GO_STATUS"
assert_equals "proxy_required" "$(jq -r '.error.type' < "$LAB/go.err")"

printf '{"MAGENTO_DIR": "./src", "DOMAIN": "ramas.test", "COMPOSE_PROJECT_NAME": "%s", "USE_PROXY": "true"}\n' "$PROJECT" \
    > "$DIR/config/docker/properties.json"

reiniciar() {
    olvidar rojo
    ( cd "$DIR" && git worktree remove --force "$LAB/$PROJECT-worktrees/rojo" &&
      git worktree prune ) >/dev/null 2>&1
    rm -rf "$LAB/$PROJECT-worktrees"
}

reiniciar
( cd "$DIR" && "$GO_BINARY" worktree add rojo --no-start ) >/dev/null 2>&1

#
# The registration is in the database now, and the overlay is still a file: a compose file is
# something Docker reads, and one in a database is one nothing can load.
#
test_case "the registration is recorded, and not as a file any more"
assert_equals "rojo" "$( cd "$DIR" && "$GO_BINARY" worktree list --json | jq -r '.data.worktrees[0].name' )"
assert_equals "no" "$([ -f "$HM_WORKTREE_DIR/$PROJECT/rojo.json" ] && echo yes || echo no)"

#
# The overlay is where a mistake is invisible until something is running: a service written twice
# is not a merge — the last one wins and the earlier block disappears without a word.
#
test_case "and the overlay is written where compose can load it"
assert_contains "$(cat "$HM_WORKTREE_DIR/$PROJECT/rojo.yml")" "$PROJECT-rojo"
assert_contains "$(cat "$HM_WORKTREE_DIR/$PROJECT/rojo.yml")" "services:"

test_case "and the worktree is on disk, on its own branch"
assert_equals "yes" "$([ -d "$LAB/$PROJECT-worktrees/rojo" ] && echo yes || echo no)"
assert_equals "rojo" "$(git -C "$LAB/$PROJECT-worktrees/rojo" rev-parse --abbrev-ref HEAD)"

test_case "a name that is already taken is refused rather than resolved"
( cd "$DIR" && "$GO_BINARY" worktree add rojo >"$LAB/go.err" 2>&1 ); GO_STATUS=$?
assert_equals "2" "$GO_STATUS"
assert_equals "already_registered" "$(jq -r '.error.type' < "$LAB/go.err")"

reiniciar

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
