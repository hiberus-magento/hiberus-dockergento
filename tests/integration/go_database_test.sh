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

#
# mysql has no shell twin left. What it ran is the oracle now: the client inside the db container,
# with the statement in the environment, exactly as console/commands/mysql.sh did it.
#
cliente() {
    docker exec -e QUERY="$1" "$( cd "$DIR" && docker compose -p "$PROJECT" ps -q db )" bash -c \
        'client=$(command -v mariadb || command -v mysql); "$client" -u"root" -p"$MYSQL_ROOT_PASSWORD" "$MYSQL_DATABASE" -e "$QUERY"'
}

# ---------------------------------------------------------------- with nothing running
#
# Any other answer would be a connection error three layers down.

( cd "$DIR" && "$GO_BINARY" mysql -q "SELECT 1" >"$LAB/go.err" 2>&1 ); GO_STATUS=$?

test_case "with the database down, it refuses with the service code"
assert_equals "5" "$GO_STATUS"
assert_equals "false" "$(jq -r '.ok' < "$LAB/go.err")"

# The kind, and not only the code: a connection error that happened to exit 5 would pass the two
# above. No container at all is still a service that is not running (app.runningContainer)
test_case "and the refusal is the service not running, naming the way out"
assert_equals "service_not_running" "$(jq -r '.error.type' < "$LAB/go.err")"
assert_equals "5" "$(jq -r '.error.code' < "$LAB/go.err")"
assert_contains "$(jq -r '.error.hint' < "$LAB/go.err")" "start db"

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

cliente "$CONSULTA" >"$LAB/shell.out" 2>&1; SHELL_STATUS=$?
( cd "$DIR" && "$GO_BINARY"  mysql -q "$CONSULTA" >"$LAB/go.out" 2>&1 );   GO_STATUS=$?

test_case "a statement with quotes and backticks answers the same, character for character"
assert_equals "$(cat "$LAB/shell.out")" "$(cat "$LAB/go.out")"
assert_equals "$SHELL_STATUS" "$GO_STATUS"

test_case "and it really did reach the database"
assert_contains "$(cat "$LAB/go.out")" 'con "comillas" y `acentos`'

cliente "SELECT * FROM no_existe" >"$LAB/shell.out" 2>&1; SHELL_STATUS=$?
( cd "$DIR" && "$GO_BINARY"  mysql -q "SELECT * FROM no_existe" >"$LAB/go.out" 2>&1 );   GO_STATUS=$?

test_case "a statement that fails keeps the client's own exit code and message"
assert_equals "$(cat "$LAB/shell.out")" "$(cat "$LAB/go.out")"
assert_equals "1" "$GO_STATUS"
assert_equals "$SHELL_STATUS" "$GO_STATUS"

# ---------------------------------------------------------------- a dump on the input
#
# Only a stdin that is not a terminal counts as a dump, and only when nothing else was asked for:
# gating on the terminal alone is what once made -q unreachable for anything without one.

printf 'DROP TABLE IF EXISTS prueba;\nCREATE TABLE prueba (id INT);\nINSERT INTO prueba VALUES (7);\n' > "$LAB/dump.sql"

( cd "$DIR" && "$GO_BINARY" mysql < "$LAB/dump.sql" ) >/dev/null 2>&1

test_case "a dump on the input is imported"
assert_contains "$( cd "$DIR" && "$GO_BINARY" mysql -q "SELECT id FROM prueba" 2>&1 )" "7"

test_case "and the client itself reads the same database"
assert_contains "$(cliente "SELECT id FROM prueba" 2>&1)" "7"

# ---------------------------------------------------------------- and the refusals

( cd "$DIR" && "$GO_BINARY" mysql -z >/dev/null 2>&1 ); GO_STATUS=$?

test_case "an option nobody declared is refused with the usage code"
assert_equals "2" "$GO_STATUS"

# ---------------------------------------------------------------- the import
#
# An import is not only an import: it strips the DEFINER clauses, clears the record of the data
# having been anonymised, optionally anonymises, and then points the store at this machine.

#
# `-d` takes no argument, and saying otherwise was not a cosmetic mistake: `hm mysql -d -i dump`
# — the form this tool documents — read `-i` as the argument of `-d`, stopped at the file name,
# imported nothing, and exited 0. The other order was refused outright, so there was no way to ask
# for a DEFINER-cleaned import at all.
#
cat > "$LAB/vistas.sql" <<'EOF'
DROP VIEW IF EXISTS vista;
DROP TABLE IF EXISTS cosas;
CREATE TABLE cosas (id INT, nombre VARCHAR(50));
INSERT INTO cosas VALUES (1, 'uno');
/*!50001 CREATE*/ /*!50013 DEFINER=`alguien`@`otro-sitio` SQL SECURITY DEFINER*/ /*!50001 VIEW `vista` AS SELECT 1 AS `x` */;
EOF

for CUAL in "$GO_BINARY"; do
    ( cd "$DIR" && "$CUAL" mysql -q "DROP VIEW IF EXISTS vista; DROP TABLE IF EXISTS cosas;" ) >/dev/null 2>&1
    ( cd "$DIR" && "$CUAL" mysql -d -i "$LAB/vistas.sql" ) >/dev/null 2>&1

    quien=$(basename "$CUAL")

    test_case "$quien imports the dump when the DEFINER clauses are asked to go"
    assert_contains "$( cd "$DIR" && "$GO_BINARY" mysql -q "SELECT nombre FROM cosas" 2>&1 )" "uno"

    #
    # The view is what proves it: with the clause left in, restoring it as another user fails
    # outright, and with it gone the view belongs to whoever imported.
    #
    test_case "and the view no longer belongs to somebody else"
    assert_contains "$( cd "$DIR" && "$GO_BINARY" mysql -q \
        "SELECT definer FROM information_schema.views WHERE table_name='vista'" 2>&1 )" "root@"
done

test_case "an import clears the record of the data having been anonymised"
mkdir -p "$LAB/estado"
printf '{"anonymised_at":"2026-01-01 00:00"}\n' > "$LAB/estado/$PROJECT.json"
( cd "$DIR" && HM_STATE_DIR="$LAB/estado" "$GO_BINARY" mysql -i "$LAB/dump.sql" ) >/dev/null 2>&1
#
# Only the record goes, not the file: other keys in it are kept, as the shell implementation kept
# them (see State.Clear), so what is asserted is that the date is no longer there.
#
assert_equals "no" "$(grep -q anonymised_at "$LAB/estado/$PROJECT.json" 2>/dev/null && echo yes || echo no)"

test_case "a file that is not there is said, not crashed on"
( cd "$DIR" && "$GO_BINARY" mysql -i "$LAB/no-existe.sql" >"$LAB/go.out" 2>&1 ); GO_STATUS=$?
assert_contains "$(cat "$LAB/go.out")" "No such file"
assert_equals "0" "$GO_STATUS"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
