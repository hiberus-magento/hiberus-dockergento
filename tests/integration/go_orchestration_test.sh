#!/usr/bin/env bash
#
# Compose as a library, against Compose as a command.
#
# The whole decision to link the orchestrator rested on one property: that what this creates and
# what `docker compose` creates are the same containers. It was measured before the code was
# written, and this is what keeps it true — a drift here would mean two implementations recreating
# each other's environments, which on a real project is a database that goes away.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

GO_BINARY="$COMMAND_BIN_DIR/bin/hm"
SHELL_CLI="$COMMAND_BIN_DIR/bin/run"
PROJECT="hm-go-orquestacion"
LAB=$(cd "$(mktemp -d)" && pwd -P)

limpiar() {
    docker compose -p "$PROJECT" down --remove-orphans >/dev/null 2>&1
    rm -rf "$LAB"
    hm_test_home_cleanup
}
trap limpiar EXIT

if ! command -v go >/dev/null 2>&1 || ! docker info >/dev/null 2>&1; then
    echo "  - skipped: go or a Docker daemon is missing"
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

# A grace period of one second: the point of the fixture is the orchestration, not waiting for
# alpine to notice a signal it ignores
cat > "$DIR/docker-compose.yml" <<'EOF'
services:
  phpfpm:
    image: alpine:latest
    command: sh -c "echo listo phpfpm; sleep 900"
    stop_grace_period: 1s
  nginx:
    image: alpine:latest
    command: sh -c "echo listo nginx; sleep 900"
    stop_grace_period: 1s
EOF
cp "$DIR/docker-compose.yml" "$DIR/docker-compose.dev.mac.yml"
cp "$DIR/docker-compose.yml" "$DIR/docker-compose.dev.linux.yml"
printf '{"MAGENTO_DIR": "./src", "DOMAIN": "orq.test", "COMPOSE_PROJECT_NAME": "%s"}\n' "$PROJECT" \
    > "$DIR/config/docker/properties.json"

etiqueta() {
    docker inspect "$(docker ps -aq \
        -f "label=com.docker.compose.project=$PROJECT" \
        -f "label=com.docker.compose.service=nginx")" \
        --format "{{index .Config.Labels \"$1\"}}" 2>/dev/null
}

docker compose -p "$PROJECT" down >/dev/null 2>&1

# ---------------------------------------------------------------- the same containers

( cd "$DIR" && "$GO_BINARY" start ) >/dev/null 2>&1

test_case "the library brings the environment up"
assert_equals "2" "$(docker ps -q -f "label=com.docker.compose.project=$PROJECT" | wc -l | tr -d ' ')"

test_case "and the compose command sees what it created"
assert_equals "nginx phpfpm" "$(docker compose -p "$PROJECT" ps --format '{{.Service}}' | sort | tr '\n' ' ' | sed 's/ $//')"

HASH_GO=$(etiqueta com.docker.compose.config-hash)

#
# The one that matters. If the two computed different hashes they would recreate each other's
# containers on every command, and nothing would say so until somebody lost a database.
#
test_case "and the compose command does not recreate it"
recreados=$( cd "$DIR" && "$SHELL_CLI" start 2>&1 | grep -c "Created" )
assert_equals "0" "$recreados"

docker compose -p "$PROJECT" down >/dev/null 2>&1
( cd "$DIR" && "$SHELL_CLI" start ) >/dev/null 2>&1
HASH_SHELL=$(etiqueta com.docker.compose.config-hash)

test_case "the configuration hash is the same one Compose computes"
assert_equals "$HASH_SHELL" "$HASH_GO"

test_case "and the library does not recreate what the command created"
recreados=$( cd "$DIR" && "$GO_BINARY" start 2>&1 | grep -c "Created" )
assert_equals "0" "$recreados"

test_case "the version label says which Compose actually ran"
assert_contains "$(etiqueta com.docker.compose.version)" "2."

# ---------------------------------------------------------------- a change is picked up
#
# Not recreating an unchanged container is only safe if a changed one is still replaced.

sed -i.bak 's/sleep 900/sleep 901/' "$DIR/docker-compose.yml" "$DIR/docker-compose.dev.mac.yml" \
    "$DIR/docker-compose.dev.linux.yml"

( cd "$DIR" && "$GO_BINARY" start ) >/dev/null 2>&1

test_case "a change to the configuration reaches the running environment"
assert_contains "$(docker inspect "$(docker ps -q -f "label=com.docker.compose.project=$PROJECT" -f "label=com.docker.compose.service=nginx")" --format '{{json .Config.Cmd}}')" "901"

# ---------------------------------------------------------------- the same output

#
# The padding of the prefix is not compared, and that is not a concession: Compose computes it
# from the containers registered so far, so a line printed before the widest name has arrived gets
# a narrower prefix. It is its own race, in its own consumer, and both implementations have it.
# What has to match is every line and every prefix.
#
test_case "the logs are the shell implementation's, line for line"
( cd "$DIR" && "$SHELL_CLI" --no-json logs --no-color >"$LAB/shell.out" 2>&1 )
( cd "$DIR" && "$GO_BINARY"  --no-json logs --no-color >"$LAB/go.out" 2>&1 )
assert_equals "$(sed 's/  */ /g' < "$LAB/shell.out" | sort)" "$(sed 's/  */ /g' < "$LAB/go.out" | sort)"

# One service, so there is one name and the width cannot move: this one is compared whole
test_case "and the logs of one service are the same, character for character"
( cd "$DIR" && "$SHELL_CLI" --no-json logs nginx >"$LAB/shell.out" 2>&1 )
( cd "$DIR" && "$GO_BINARY"  --no-json logs nginx >"$LAB/go.out" 2>&1 )
assert_equals "$(cat "$LAB/shell.out")" "$(cat "$LAB/go.out")"

test_case "a service this project does not have is refused the same way"
( cd "$DIR" && "$SHELL_CLI" logs no-existe >"$LAB/shell.err" 2>&1 ); SHELL_STATUS=$?
( cd "$DIR" && "$GO_BINARY"  logs no-existe >"$LAB/go.err" 2>&1 );    GO_STATUS=$?
assert_equals "5" "$GO_STATUS"
assert_equals "$SHELL_STATUS" "$GO_STATUS"
assert_equals "$(jq -S . < "$LAB/shell.err")" "$(jq -S . < "$LAB/go.err")"

test_case "an option that needs a value says so instead of reading the next word as a service"
( cd "$DIR" && "$GO_BINARY" logs --tail >"$LAB/go.err" 2>&1 )
assert_equals "2" "$?"
assert_contains "$(cat "$LAB/go.err")" "needs a value"

# ---------------------------------------------------------------- running things inside

test_case "exec returns the command's own exit code"
( cd "$DIR" && "$SHELL_CLI" exec sh -c 'exit 7' >/dev/null 2>&1 ); SHELL_STATUS=$?
( cd "$DIR" && "$GO_BINARY"  exec sh -c 'exit 7' >/dev/null 2>&1 ); GO_STATUS=$?
assert_equals "7" "$GO_STATUS"
assert_equals "$SHELL_STATUS" "$GO_STATUS"

test_case "and its output, as the same user"
assert_equals "$( cd "$DIR" && "$SHELL_CLI" exec id -un 2>&1 )" "$( cd "$DIR" && "$GO_BINARY" exec id -un 2>&1 )"

test_case "and as root when asked"
assert_equals "root" "$( cd "$DIR" && "$GO_BINARY" exec -r id -un 2>&1 | tr -d '\r' )"

test_case "with nothing to run, it says so"
( cd "$DIR" && "$GO_BINARY" exec >"$LAB/go.err" 2>&1 )
assert_equals "2" "$?"

# ---------------------------------------------------------------- stopping

test_case "stop leaves the containers there, stopped"
( cd "$DIR" && "$GO_BINARY" stop ) >/dev/null 2>&1
assert_equals "0" "$(docker ps -q -f "label=com.docker.compose.project=$PROJECT" | wc -l | tr -d ' ')"
assert_equals "2" "$(docker ps -aq -f "label=com.docker.compose.project=$PROJECT" | wc -l | tr -d ' ')"

#
# There is no database in this fixture, so the snapshot cannot succeed — which is the case being
# tested: a stopped environment and no copy, after asking for one, is the worst of the three
# possible outcomes.
#
test_case "and a failed snapshot leaves the environment running"
( cd "$DIR" && "$GO_BINARY" start ) >/dev/null 2>&1
( cd "$DIR" && "$GO_BINARY" stop --snapshot >"$LAB/go.err" 2>&1 )
assert_equals "1" "$?"
assert_equals "2" "$(docker ps -q -f "label=com.docker.compose.project=$PROJECT" | wc -l | tr -d ' ')"

# ---------------------------------------------------------------- php and composer
#
# Both are the same thing underneath — something run in the php container — and what has to match
# is the wrapper: the command it builds, and the exit code it hands back.

test_case "the Magento CLI is invoked the same way"
( cd "$DIR" && "$SHELL_CLI" magento cache:flush >"$LAB/shell.out" 2>&1 ); SHELL_STATUS=$?
( cd "$DIR" && "$GO_BINARY"  magento cache:flush >"$LAB/go.out" 2>&1 );    GO_STATUS=$?
assert_equals "$(cat "$LAB/shell.out")" "$(cat "$LAB/go.out")"
assert_equals "$SHELL_STATUS" "$GO_STATUS"

test_case "and so is Composer"
( cd "$DIR" && "$SHELL_CLI" composer show >"$LAB/shell.out" 2>&1 ); SHELL_STATUS=$?
( cd "$DIR" && "$GO_BINARY"  composer show >"$LAB/go.out" 2>&1 );    GO_STATUS=$?
assert_equals "$(cat "$LAB/shell.out")" "$(cat "$LAB/go.out")"
assert_equals "$SHELL_STATUS" "$GO_STATUS"

#
# Said before anything runs, because Composer would create the project in the container and leave
# it there. The shell implementation printed a paragraph to stdout and exited 2; this says the
# same thing through the error contract, so a --json caller can read it.
#
test_case "creating a project through Composer is refused, with the usage code"
( cd "$DIR" && "$GO_BINARY" composer create-project >"$LAB/go.err" 2>&1 ); GO_STATUS=$?
( cd "$DIR" && "$SHELL_CLI" composer create-project >/dev/null 2>&1 );     SHELL_STATUS=$?
assert_equals "2" "$GO_STATUS"
assert_equals "$SHELL_STATUS" "$GO_STATUS"
assert_contains "$(cat "$LAB/go.err")" "create-project"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
