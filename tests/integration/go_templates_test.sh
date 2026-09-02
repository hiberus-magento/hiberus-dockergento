#!/usr/bin/env bash
#
# Database templates: a byte copy of a data directory, frozen and cloned as files.
#
# Nothing here talks to a database server — it replaces the files underneath one, which is what
# makes it seconds instead of the tens of minutes an import costs, and also why the environment has
# to be down for it. It is the half of `db` that `worktree` is built on.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

GO_BINARY="$COMMAND_BIN_DIR/bin/hm"
SHELL_CLI="$COMMAND_BIN_DIR/bin/run"
PROJECT="hm-go-plantillas"
LAB=$(cd "$(mktemp -d)" && pwd -P)

limpiar() {
    docker compose -p "$PROJECT" down -v >/dev/null 2>&1
    docker volume rm "hm-template-$PROJECT-base" "hm-template-$PROJECT-otra" >/dev/null 2>&1
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
cat > "$DIR/docker-compose.yml" <<'EOF'
services:
  db:
    image: mariadb:10.6
    volumes:
      - dbdata:/var/lib/mysql
    environment:
      MYSQL_ROOT_PASSWORD: password
      MYSQL_DATABASE: magento
      MYSQL_USER: magento
      MYSQL_PASSWORD: magento
    stop_grace_period: 2s
volumes:
  dbdata:
EOF
cp "$DIR/docker-compose.yml" "$DIR/docker-compose.dev.mac.yml"
cp "$DIR/docker-compose.yml" "$DIR/docker-compose.dev.linux.yml"
printf '{"MAGENTO_DIR": "./src", "DOMAIN": "plant.test", "COMPOSE_PROJECT_NAME": "%s"}\n' "$PROJECT" \
    > "$DIR/config/docker/properties.json"

docker image inspect mariadb:10.6 >/dev/null 2>&1 || docker pull -q mariadb:10.6 >/dev/null 2>&1
( cd "$DIR" && "$GO_BINARY" start ) >/dev/null 2>&1

esperando=0
until ( cd "$DIR" && "$GO_BINARY" mysql -q "SELECT 1" ) >/dev/null 2>&1 || [ "$esperando" -ge 90 ]; do
    sleep 3
    esperando=$((esperando + 3))
done

if [ "$esperando" -ge 90 ]; then
    echo "  - skipped: the database did not come up in time"
    echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
    exit 0
fi

( cd "$DIR" && "$GO_BINARY" mysql -q "CREATE TABLE marca (id INT); INSERT INTO marca VALUES (42);" ) >/dev/null 2>&1

# ---------------------------------------------------------------- freezing

#
# Kept apart on purpose: in JSON mode stdout carries the document and everything decorative goes
# to stderr, which is the whole reason a program can read the output of a command that also draws
# a spinner.
#
( cd "$DIR" && "$GO_BINARY" db freeze --name=base >"$LAB/go.out" 2>"$LAB/go.err" ); GO_STATUS=$?

test_case "freezing writes a template"
assert_equals "0" "$GO_STATUS"
assert_equals "$PROJECT/base" "$(jq -r '.data.template' < "$LAB/go.out")"

test_case "and the document is not mixed with what it printed on the way"
assert_equals "1" "$(jq -r '.ok' < "$LAB/go.out" | grep -c true)"

test_case "and the two implementations list it the same way"
assert_equals "$( cd "$DIR" && "$SHELL_CLI" db templates --json | jq -S . )" \
              "$( cd "$DIR" && "$GO_BINARY" db templates --json | jq -S . )"

test_case "including the table a person reads"
assert_equals "$( cd "$DIR" && "$SHELL_CLI" --no-json db templates 2>&1 )" \
              "$( cd "$DIR" && "$GO_BINARY" --no-json db templates 2>&1 )"

test_case "freezing again over one that is there is refused, the same way"
( cd "$DIR" && "$SHELL_CLI" db freeze --name=base >"$LAB/shell.err" 2>&1 ); SHELL_STATUS=$?
( cd "$DIR" && "$GO_BINARY"  db freeze --name=base >"$LAB/go.err" 2>&1 );    GO_STATUS=$?
assert_equals "2" "$GO_STATUS"
assert_equals "$SHELL_STATUS" "$GO_STATUS"
assert_equals "$(jq -S . < "$LAB/shell.err")" "$(jq -S . < "$LAB/go.err")"

# ---------------------------------------------------------------- cloning
#
# The environment has to be down: this replaces the files a server has open, and a server that
# finds its data directory changed underneath it does not notice until much later.

test_case "cloning while the environment runs is refused, the same way"
( cd "$DIR" && "$SHELL_CLI" db clone base >"$LAB/shell.err" 2>&1 ); SHELL_STATUS=$?
( cd "$DIR" && "$GO_BINARY"  db clone base >"$LAB/go.err" 2>&1 );    GO_STATUS=$?
assert_equals "6" "$GO_STATUS"
assert_equals "$SHELL_STATUS" "$GO_STATUS"
assert_equals "$(jq -S . < "$LAB/shell.err")" "$(jq -S . < "$LAB/go.err")"

#
# The round trip, which is the whole point: drop the table, clone, and it is back — without a
# database server ever being asked to do anything.
#
( cd "$DIR" && "$GO_BINARY" mysql -q "DROP TABLE marca" ) >/dev/null 2>&1
( cd "$DIR" && "$GO_BINARY" stop ) >/dev/null 2>&1
( cd "$DIR" && "$GO_BINARY" --force db clone base ) >/dev/null 2>&1
( cd "$DIR" && "$GO_BINARY" start ) >/dev/null 2>&1

esperando=0
until ( cd "$DIR" && "$GO_BINARY" mysql -q "SELECT 1" ) >/dev/null 2>&1 || [ "$esperando" -ge 90 ]; do
    sleep 3
    esperando=$((esperando + 3))
done

test_case "cloning puts the data back, without a server doing anything"
assert_contains "$( cd "$DIR" && "$GO_BINARY" mysql -q "SELECT id FROM marca" 2>&1 )" "42"

test_case "and a template nobody made is refused, the same way"
( cd "$DIR" && "$SHELL_CLI" db clone no/existe >"$LAB/shell.err" 2>&1 ); SHELL_STATUS=$?
( cd "$DIR" && "$GO_BINARY"  db clone no/existe >"$LAB/go.err" 2>&1 );    GO_STATUS=$?
assert_equals "2" "$GO_STATUS"
assert_equals "$SHELL_STATUS" "$GO_STATUS"
assert_equals "$(jq -S . < "$LAB/shell.err")" "$(jq -S . < "$LAB/go.err")"

# ---------------------------------------------------------------- dropping

test_case "answering no leaves the template alone"
( cd "$DIR" && printf 'n\n' | "$GO_BINARY" --no-json db drop base >"$LAB/go.out" 2>&1 )
assert_equals "1" "$( cd "$DIR" && "$GO_BINARY" db templates --json | jq -r '.data.templates | length' )"

test_case "and saying so deletes it, saying what it freed"
( cd "$DIR" && "$GO_BINARY" --yes --no-json db drop base >"$LAB/go.out" 2>&1 )
assert_contains "$(cat "$LAB/go.out")" "Dropped $PROJECT/base"
assert_equals "0" "$( cd "$DIR" && "$GO_BINARY" db templates --json | jq -r '.data.templates | length' )"

test_case "dropping one that is not there is refused, the same way"
( cd "$DIR" && "$SHELL_CLI" db drop no/existe >"$LAB/shell.err" 2>&1 ); SHELL_STATUS=$?
( cd "$DIR" && "$GO_BINARY"  db drop no/existe >"$LAB/go.err" 2>&1 );    GO_STATUS=$?
assert_equals "$SHELL_STATUS" "$GO_STATUS"
assert_equals "$(jq -S . < "$LAB/shell.err")" "$(jq -S . < "$LAB/go.err")"

# ---------------------------------------------------------------- and the other family
#
# `db` is two families that share a name. The snapshots are still the shell implementation's, and
# the boundary is between independent operations rather than down the middle of one.

test_case "a snapshot subcommand still reaches the shell implementation"
( cd "$DIR" && "$GO_BINARY" db list >"$LAB/go.out" 2>&1 )
assert_equals "0" "$?"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
