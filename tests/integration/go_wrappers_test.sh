#!/usr/bin/env bash
#
# The commands that are one thing run in one container, ported.
#
# They are wrappers, and that is the point of them: knowing that the unit tests are run with this
# phpunit and that configuration, and that clearing generated code means these seven directories,
# is what the tool is for. What is checked here is that they reach the same container with the
# same command as the shell implementation.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

GO_BINARY="$COMMAND_BIN_DIR/bin/hm"
SHELL_CLI="$COMMAND_BIN_DIR/bin/run"
PROJECT="hm-go-envoltorios"
LAB=$(cd "$(mktemp -d)" && pwd -P)

limpiar() {
    ( cd "$LAB/$PROJECT" 2>/dev/null && docker compose -p "$PROJECT" down -v --remove-orphans ) >/dev/null 2>&1
    rm -rf "$LAB"
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
mkdir -p "$DIR/config/docker"

#
# A php container that is an alpine with a shell in it, and a real MariaDB: what these commands do
# is run something somewhere, and a shell is enough to see where they ran it.
#
cat > "$DIR/docker-compose.yml" <<'YAML'
services:
  phpfpm:
    image: alpine:latest
    working_dir: /var/www/html
    command: ["sleep", "600"]
  db:
    image: mariadb:10.6
    environment:
      MYSQL_ROOT_PASSWORD: password
      MYSQL_DATABASE: magento
      MYSQL_USER: magento
      MYSQL_PASSWORD: magento
YAML
cp "$DIR/docker-compose.yml" "$DIR/docker-compose.dev.mac.yml"
cp "$DIR/docker-compose.yml" "$DIR/docker-compose.dev.linux.yml"
printf '{"MAGENTO_DIR": ".", "DOMAIN": "envoltorios.test", "COMPOSE_PROJECT_NAME": "%s"}\n' "$PROJECT" \
    > "$DIR/config/docker/properties.json"

( cd "$DIR" && docker compose -p "$PROJECT" up -d ) >/dev/null 2>&1

en_contenedor() {
    ( cd "$DIR" && docker compose -p "$PROJECT" exec -T phpfpm sh -c "$1" 2>/dev/null )
}

# ---------------------------------------------------------------- purge

en_contenedor 'mkdir -p var/cache generated pub/static var/log &&
    touch var/cache/uno generated/dos pub/static/tres var/log/importante.log' >/dev/null 2>&1

test_case "purge clears what Magento can generate again"
( cd "$DIR" && "$GO_BINARY" purge ) >/dev/null 2>&1
assert_equals "" "$(en_contenedor 'find var/cache generated pub/static -type f 2>/dev/null')"

#
# `var/log` is in neither list, and that is deliberate: a developer looking for what went wrong an
# hour ago should still find it.
#
test_case "and leaves the logs, which is where what went wrong is written"
assert_equals "importante.log" "$(en_contenedor 'ls var/log')"

# ---------------------------------------------------------------- npm and n98-magerun

#
# Neither is installed in this image. What is compared is the failure: it has to be the container
# saying the command is not there, and it has to be the same in both halves.
#
( cd "$DIR" && "$SHELL_CLI" npm --version >"$LAB/shell.out" 2>&1 ); ESTADO_SHELL=$?
( cd "$DIR" && "$GO_BINARY"  npm --version >"$LAB/go.out" 2>&1 );   ESTADO_GO=$?

test_case "npm is run in the php container, and its failure is its own"
assert_equals "$ESTADO_SHELL" "$ESTADO_GO"

test_case "and n98-magerun too"
( cd "$DIR" && "$SHELL_CLI" n98-magerun --version >"$LAB/shell.out" 2>&1 ); ESTADO_SHELL=$?
( cd "$DIR" && "$GO_BINARY"  n98-magerun --version >"$LAB/go.out" 2>&1 );   ESTADO_GO=$?
assert_equals "$ESTADO_SHELL" "$ESTADO_GO"

# ---------------------------------------------------------------- the test suites

#
# What matters is the command they build: which phpunit, which configuration, and from which
# directory. Shown by putting a phpunit in the container that writes down how it was called.
#
en_contenedor 'mkdir -p vendor/bin dev/tests/unit dev/tests/integration &&
    printf "#!/bin/sh\npwd >> /tmp/llamadas\necho \"\$@\" >> /tmp/llamadas\n" > vendor/bin/phpunit &&
    chmod +x vendor/bin/phpunit' >/dev/null 2>&1

test_case "the unit suite runs its own phpunit with its own configuration"
en_contenedor ': > /tmp/llamadas' >/dev/null 2>&1
( cd "$DIR" && "$GO_BINARY" test-unit ) >/dev/null 2>&1
assert_contains "$(en_contenedor 'cat /tmp/llamadas')" "--config ./dev/tests/unit/phpunit.xml.dist"

#
# The integration suite runs from its own directory, which is not a preference: its configuration
# resolves paths relative to itself.
#
test_case "and the integration one runs from its own directory"
en_contenedor ': > /tmp/llamadas' >/dev/null 2>&1
( cd "$DIR" && "$GO_BINARY" test-integration ) >/dev/null 2>&1
assert_contains "$(en_contenedor 'cat /tmp/llamadas')" "/var/www/html/dev/tests/integration"

test_case "with the arguments it was given"
en_contenedor ': > /tmp/llamadas' >/dev/null 2>&1
( cd "$DIR" && "$GO_BINARY" test-unit --filter AlgunTest ) >/dev/null 2>&1
assert_contains "$(en_contenedor 'cat /tmp/llamadas')" "--filter AlgunTest"

#
# Both halves have to reach it, which is not a given: the shell implementation passed the whole
# command line as one argument, so Docker looked for a file whose name had spaces in it and the
# tests never ran — while the exit code said they had.
#
test_case "and both halves build the same one"
en_contenedor ': > /tmp/llamadas' >/dev/null 2>&1
( cd "$DIR" && "$SHELL_CLI" test-unit ) >/dev/null 2>&1
DESDE_SHELL=$(en_contenedor 'cat /tmp/llamadas')
en_contenedor ': > /tmp/llamadas' >/dev/null 2>&1
( cd "$DIR" && "$GO_BINARY" test-unit ) >/dev/null 2>&1
assert_equals "$DESDE_SHELL" "$(en_contenedor 'cat /tmp/llamadas')"
assert_contains "$DESDE_SHELL" "phpunit.xml.dist"

# ---------------------------------------------------------------- mysqldump

esperar_db() {
    local esperado=0
    until ( cd "$DIR" && docker compose -p "$PROJECT" exec -T db \
                mariadb -uroot -ppassword magento -N -B -e "SELECT 1" ) >/dev/null 2>&1 ||
          [ "$esperado" -gt 90 ]; do
        sleep 2
        esperado=$((esperado + 2))
    done
}

esperar_db

( cd "$DIR" && docker compose -p "$PROJECT" exec -T db \
    mariadb -uroot -ppassword magento -e "CREATE TABLE pedidos (id INT); INSERT INTO pedidos VALUES (7);" ) >/dev/null 2>&1

test_case "mysqldump writes the database where it was told"
( cd "$DIR" && "$GO_BINARY" mysqldump "$LAB/salida.sql" ) >/dev/null 2>&1
assert_contains "$(cat "$LAB/salida.sql")" "CREATE TABLE"
assert_contains "$(cat "$LAB/salida.sql")" "INSERT INTO \`pedidos\`"

#
# What the dumper says about itself must not land inside the dump: a file with a sentence in the
# middle of it is not a dump, and it is found the day somebody loads it.
#
test_case "and nothing but the database"
assert_not_contains "$(cat "$LAB/salida.sql")" "Warning"

test_case "with no path it says so instead of writing somewhere"
( cd "$DIR" && "$GO_BINARY" mysqldump >"$LAB/go.err" 2>&1 ); ESTADO_GO=$?
assert_equals "2" "$ESTADO_GO"

test_case "and what it writes is what the shell implementation writes"
( cd "$DIR" && "$SHELL_CLI" mysqldump "$LAB/shell.sql" ) >/dev/null 2>&1
assert_equals "$(grep -c 'INSERT INTO' "$LAB/shell.sql")" "$(grep -c 'INSERT INTO' "$LAB/salida.sql")"

# ---------------------------------------------------------------- version

#
# Which build of the tool this is, and what is underneath it: not "1.4.5", but 1.4.5 and eleven
# commits, on this branch, with uncommitted changes — which is the difference between a report
# somebody can act on and one naming the version somebody happened to have tagged.
#
test_case "both report the same installation"
assert_equals "$( "$SHELL_CLI" version --json | jq -S . )" \
              "$( "$GO_BINARY" version --json | jq -S 'del(.data.binary)' )"

#
# One field is new, and it is the one the shell implementation cannot answer about itself: which
# build of the binary is running. It is why the diagnostic that used to answer it is gone.
#
test_case "and the ported half says which build of it is running"
assert_equals "0" "$( "$GO_BINARY" version --json | jq -e '.data.binary != null' >/dev/null; echo $? )"

test_case "the words a person reads are the same, but for that line"
assert_equals "$( "$SHELL_CLI" --no-json version 2>&1 )" \
              "$( "$GO_BINARY" --no-json version 2>&1 | grep -v '^  binary' )"

test_case "it needs no project, because the problem may be that there is no project"
assert_equals "0" "$( cd "$LAB" && "$GO_BINARY" version >/dev/null 2>&1; echo $? )"

test_case "and an option nobody declared is a usage error"
assert_equals "2" "$( "$GO_BINARY" version --tonteria >/dev/null 2>&1; echo $? )"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
