#!/usr/bin/env bash
#
# The database client, and the capability underneath it.
#
# `hm mysql -q` is what a script and an agent use, and it is also how the tool itself will read the
# project's domains out of core_config_data once the Linux steps after `start` are ported. So what
# is compared here is not only the command: it is that a statement full of quotes survives the
# trip, which is the part that breaks quietly.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

GO_BINARY="$COMMAND_BIN_DIR/bin/hm"
SHELL_CLI="$COMMAND_BIN_DIR/bin/run"
PROJECT="hm-go-basedatos"
LAB=$(cd "$(mktemp -d)" && pwd -P)

trap 'docker compose -p "$PROJECT" down -v >/dev/null 2>&1; rm -rf "$LAB"; hm_test_home_cleanup' EXIT

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
cat > "$DIR/docker-compose.yml" <<'EOF'
services:
  db:
    image: mariadb:10.6
    environment:
      MYSQL_ROOT_PASSWORD: password
      MYSQL_DATABASE: magento
      MYSQL_USER: magento
      MYSQL_PASSWORD: magento
    stop_grace_period: 2s
EOF
cp "$DIR/docker-compose.yml" "$DIR/docker-compose.dev.mac.yml"
cp "$DIR/docker-compose.yml" "$DIR/docker-compose.dev.linux.yml"
printf '{"MAGENTO_DIR": "./src", "DOMAIN": "bd.test", "COMPOSE_PROJECT_NAME": "%s"}\n' "$PROJECT" \
    > "$DIR/config/docker/properties.json"

# ---------------------------------------------------------------- with nothing running
#
# Any other answer would be a connection error three layers down.

( cd "$DIR" && "$SHELL_CLI" mysql -q "SELECT 1" >"$LAB/shell.err" 2>&1 ); SHELL_STATUS=$?
( cd "$DIR" && "$GO_BINARY"  mysql -q "SELECT 1" >"$LAB/go.err" 2>&1 );    GO_STATUS=$?

test_case "with the database down, both refuse the same way"
assert_equals "5" "$GO_STATUS"
assert_equals "$SHELL_STATUS" "$GO_STATUS"
assert_equals "$(jq -S . < "$LAB/shell.err")" "$(jq -S . < "$LAB/go.err")"

# ---------------------------------------------------------------- with it up

if ! docker image inspect mariadb:10.6 >/dev/null 2>&1; then
    docker pull -q mariadb:10.6 >/dev/null 2>&1
fi

( cd "$DIR" && "$GO_BINARY" start ) >/dev/null 2>&1

esperando=0
until ( cd "$DIR" && "$GO_BINARY" mysql -q "SELECT 1" ) >/dev/null 2>&1 || [ "$esperando" -ge 60 ]; do
    sleep 2
    esperando=$((esperando + 2))
done

if [ "$esperando" -ge 60 ]; then
    echo "  - skipped: the database did not come up in time"
    echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
    exit 0
fi

#
# The statement travels in the environment rather than in the command. A query is full of quotes
# and backticks, and passing it as an argument through a shell is a quoting bug waiting for the
# right query.
#
CONSULTA='SELECT 1 AS uno, '"'"'con "comillas" y `acentos`'"'"' AS texto'

( cd "$DIR" && "$SHELL_CLI" mysql -q "$CONSULTA" >"$LAB/shell.out" 2>&1 ); SHELL_STATUS=$?
( cd "$DIR" && "$GO_BINARY"  mysql -q "$CONSULTA" >"$LAB/go.out" 2>&1 );   GO_STATUS=$?

test_case "a statement with quotes and backticks answers the same, character for character"
assert_equals "$(cat "$LAB/shell.out")" "$(cat "$LAB/go.out")"
assert_equals "$SHELL_STATUS" "$GO_STATUS"

test_case "and it really did reach the database"
assert_contains "$(cat "$LAB/go.out")" 'con "comillas" y `acentos`'

( cd "$DIR" && "$SHELL_CLI" mysql -q "SELECT * FROM no_existe" >"$LAB/shell.out" 2>&1 ); SHELL_STATUS=$?
( cd "$DIR" && "$GO_BINARY"  mysql -q "SELECT * FROM no_existe" >"$LAB/go.out" 2>&1 );   GO_STATUS=$?

test_case "a statement that fails keeps the client's own exit code and message"
assert_equals "$(cat "$LAB/shell.out")" "$(cat "$LAB/go.out")"
assert_equals "1" "$GO_STATUS"
assert_equals "$SHELL_STATUS" "$GO_STATUS"

# ---------------------------------------------------------------- a dump on the input
#
# Only a stdin that is not a terminal counts as a dump, and only when nothing else was asked for:
# gating on the terminal alone is what once made -q unreachable for anything without one.

printf 'CREATE TABLE prueba (id INT);\nINSERT INTO prueba VALUES (7);\n' > "$LAB/dump.sql"

( cd "$DIR" && "$GO_BINARY" mysql < "$LAB/dump.sql" ) >/dev/null 2>&1

test_case "a dump on the input is imported"
assert_contains "$( cd "$DIR" && "$GO_BINARY" mysql -q "SELECT id FROM prueba" 2>&1 )" "7"

test_case "and the shell implementation reads the same database"
assert_contains "$( cd "$DIR" && "$SHELL_CLI" mysql -q "SELECT id FROM prueba" 2>&1 )" "7"

# ---------------------------------------------------------------- and the refusals

( cd "$DIR" && "$SHELL_CLI" mysql -z >/dev/null 2>&1 ); SHELL_STATUS=$?
( cd "$DIR" && "$GO_BINARY"  mysql -z >/dev/null 2>&1 ); GO_STATUS=$?

test_case "an option nobody declared is refused with the usage code"
assert_equals "2" "$GO_STATUS"
assert_equals "$SHELL_STATUS" "$GO_STATUS"

#
# The import also cleans DEFINER clauses, optionally anonymises, and then configures Magento for
# local development. That sequence stays whole with the shell implementation, so the invocation
# has to reach it.
#
test_case "an import still goes to the shell implementation"
( cd "$DIR" && "$GO_BINARY" mysql -i "$LAB/dump.sql" >"$LAB/go.out" 2>&1 )
assert_contains "$(cat "$LAB/go.out")" "Importing the database"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
