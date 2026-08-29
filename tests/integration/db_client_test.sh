#!/usr/bin/env bash
#
# The launchers against a real project: what they print, and what they refuse.
#
# Nothing is opened here. Whether TablePlus starts is TablePlus's business; what this checks is
# that the connection is the project's own and that a database nothing can reach is said so
# rather than handed to a client that will sit there failing.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

HM="$COMMAND_BIN_DIR/bin/run"
LAB=$(cd "$(mktemp -d)" && pwd -P)
PROJECT="hm-dbclient-selftest"

cleanup() {
    ( cd "$LAB/$PROJECT" 2>/dev/null && docker compose -p "$PROJECT" down -v --remove-orphans ) >/dev/null 2>&1
    rm -rf "$LAB"
}
trap cleanup EXIT

if ! docker info >/dev/null 2>&1; then
    echo "  - skipped: docker is not available"
    echo "RESULT 0 0"
    exit 0
fi

DIR="$LAB/$PROJECT"
mkdir -p "$DIR/config/docker"
cat > "$DIR/docker-compose.yml" <<'YAML'
services:
  db:
    image: mariadb:10.6
    ports:
      - 13399:3306
    environment:
      MYSQL_ROOT_PASSWORD: password
      MYSQL_DATABASE: tienda_db
      MYSQL_USER: tienda
      MYSQL_PASSWORD: secreta
YAML
cp "$DIR/docker-compose.yml" "$DIR/docker-compose.dev.mac.yml"
cp "$DIR/docker-compose.yml" "$DIR/docker-compose.dev.linux.yml"
printf '{"MAGENTO_DIR": ".", "DOMAIN": "dbc.test", "COMPOSE_PROJECT_NAME": "%s"}\n' "$PROJECT" \
    > "$DIR/config/docker/properties.json"

run() { ( cd "$DIR" && "$HM" "$@" >"$LAB/out" 2>"$LAB/err" ); STATUS=$?; STDOUT=$(cat "$LAB/out"); STDERR=$(cat "$LAB/err"); return 0; }

# ---------------------------------------------------------------- the environment is down

run tableplus --print
test_case "a database that is not running is said so"
assert_equals "5" "$STATUS"

( cd "$DIR" && docker compose -p "$PROJECT" up -d ) >/dev/null 2>&1
waited=0
until [ -n "$( cd "$DIR" && docker compose -p "$PROJECT" ps -q db )" ] || [ "$waited" -gt 30 ]; do
    sleep 1; waited=$((waited + 1))
done

# ---------------------------------------------------------------- printing

run --no-json tableplus --print
test_case "the connection is printed, and it is the project's own"
assert_equals "0" "$STATUS"
assert_contains "$STDOUT" "mysql://tienda:secreta@127.0.0.1:13399/tienda_db"

test_case "and nothing was opened"
assert_not_contains "$STDOUT" "Opened"

run tableplus --print
test_case "as data, it is fields rather than a string to parse"
assert_equals "13399" "$(printf '%s' "$STDOUT" | jq -r '.data.port')"
assert_equals "tienda" "$(printf '%s' "$STDOUT" | jq -r '.data.user')"
assert_equals "tienda_db" "$(printf '%s' "$STDOUT" | jq -r '.data.database')"

test_case "the three launchers agree about the connection"
run --no-json sequelace --print; sequelace_url="$STDOUT"
run --no-json dbeaver --print;   dbeaver_url="$STDOUT"
assert_equals "$sequelace_url" "$dbeaver_url"

# ---------------------------------------------------------------- an unknown option

run tableplus --nonsense
test_case "an option nobody declared is a usage error"
assert_equals "2" "$STATUS"

# ---------------------------------------------------------------- no published port

python3 - "$DIR/docker-compose.yml" <<'PY'
import sys, pathlib
p = pathlib.Path(sys.argv[1])
p.write_text(p.read_text().replace("    ports:\n      - 13399:3306\n", ""))
PY
cp "$DIR/docker-compose.yml" "$DIR/docker-compose.dev.mac.yml"
cp "$DIR/docker-compose.yml" "$DIR/docker-compose.dev.linux.yml"

run tableplus --print
test_case "a database nothing can reach is refused, not handed to a client"
assert_equals "6" "$STATUS"
assert_contains "$STDERR" "tunnel db"

run --no-json tableplus --print --port=13399
test_case "and a tunnel's port is used when there is one"
assert_equals "0" "$STATUS"
assert_contains "$STDOUT" ":13399/tienda_db"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
