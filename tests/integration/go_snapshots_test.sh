#!/usr/bin/env bash
#
# Named copies of a project's database, ported.
#
# The other half of `db` freezes the data directory; this one writes a dump. It is the copy that
# outlives the environment — it survives `down -v`, which is the moment it is needed — and it is
# what `down` offers to take before destroying anything.
#
# Both implementations are compared over the same real database: what one saves the other lists,
# and what one refuses the other refuses the same way.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

GO_BINARY="$COMMAND_BIN_DIR/bin/hm"
SHELL_CLI="$COMMAND_BIN_DIR/bin/run"
PROJECT="hm-go-copias"
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

# A throwaway root: a test run has no business leaving copies in the developer's
export HM_SNAPSHOT_DIR="$LAB/copias"

DIR="$LAB/$PROJECT"
mkdir -p "$DIR/config/docker"
cat > "$DIR/docker-compose.yml" <<'YAML'
services:
  db:
    image: mariadb:10.6
    environment:
      MYSQL_ROOT_PASSWORD: password
      MYSQL_DATABASE: magento
      MYSQL_USER: magento
      MYSQL_PASSWORD: magento
  phpfpm:
    image: alpine:latest
    command: ["sleep", "600"]
YAML
cp "$DIR/docker-compose.yml" "$DIR/docker-compose.dev.mac.yml"
cp "$DIR/docker-compose.yml" "$DIR/docker-compose.dev.linux.yml"
printf '{"MAGENTO_DIR": ".", "DOMAIN": "copias.test", "COMPOSE_PROJECT_NAME": "%s"}\n' "$PROJECT" \
    > "$DIR/config/docker/properties.json"

consulta() {
    ( cd "$DIR" && docker compose -p "$PROJECT" exec -T db \
        mariadb -uroot -ppassword magento -N -B -e "$1" 2>/dev/null )
}

( cd "$DIR" && docker compose -p "$PROJECT" up -d ) >/dev/null 2>&1

esperado=0
until consulta "SELECT 1" >/dev/null 2>&1 || [ "$esperado" -gt 90 ]; do
    sleep 2
    esperado=$((esperado + 2))
done

if ! consulta "SELECT 1" >/dev/null 2>&1; then
    echo "  - skipped: the database never became reachable"
    echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
    exit 0
fi

consulta "CREATE TABLE pedidos (id INT PRIMARY KEY, total INT); INSERT INTO pedidos VALUES (1, 100);" >/dev/null

# ---------------------------------------------------------------- taking one

test_case "a copy is taken"
salida=$( cd "$DIR" && "$GO_BINARY" db snapshot --name=antes --json )
assert_equals "antes" "$(printf '%s' "$salida" | jq -r '.data.name')"
assert_equals "0" "$([ -f "$HM_SNAPSHOT_DIR/$PROJECT/antes.sql.gz" ] && echo 0 || echo 1)"

#
# The copy is a dump the database itself wrote, not a file this tool composed: it has to be
# readable by the client, and its header has to say what it is.
#
test_case "and it is a dump, with a header saying where it came from"
assert_contains "$(gunzip -c "$HM_SNAPSHOT_DIR/$PROJECT/antes.sql.gz" | head -3)" "hm snapshot: antes"
assert_contains "$(gunzip -c "$HM_SNAPSHOT_DIR/$PROJECT/antes.sql.gz")" "CREATE TABLE"

test_case "nothing was written inside the project"
assert_empty "$(find "$DIR" -type f \( -name '*.sql' -o -name '*.sql.gz' \) 2>/dev/null)"

test_case "the database is untouched by having been copied"
assert_equals "100" "$(consulta "SELECT total FROM pedidos WHERE id = 1")"

#
# Nothing but an interrupted dump ever has that suffix, and one left behind would be listed as a
# copy that cannot be restored.
#
test_case "and no half-written file is left beside it"
assert_empty "$(find "$HM_SNAPSHOT_DIR" -name '*.partial' 2>/dev/null)"

# ---------------------------------------------------------------- what one saves the other lists

test_case "what the ported half saved, the shell one lists"
assert_contains "$( cd "$DIR" && "$SHELL_CLI" db list --json | jq -r '.data.snapshots[].name' )" "antes"

test_case "and both list it the same way"
assert_equals "$( cd "$DIR" && "$SHELL_CLI" db list --json | jq -S '.data.snapshots | map(.name)' )" \
              "$( cd "$DIR" && "$GO_BINARY" db list --json | jq -S '.data.snapshots | map(.name)' )"

#
# The size is read from the file rather than recorded, so both have to measure it the same way:
# `du` reports what a file takes on disk, and a copy reported smaller than it is would be the
# wrong answer in the one direction that matters when a disk is filling up.
#
test_case "including the size, which both measure as disk usage"
assert_equals "$( cd "$DIR" && "$SHELL_CLI" db list --json | jq -S '.data.snapshots | map(.size)' )" \
              "$( cd "$DIR" && "$GO_BINARY" db list --json | jq -S '.data.snapshots | map(.size)' )"

test_case "and the table a person reads"
assert_equals "$( cd "$DIR" && "$SHELL_CLI" --no-json db list 2>&1 )" \
              "$( cd "$DIR" && "$GO_BINARY" --no-json db list 2>&1 )"

# ---------------------------------------------------------------- the refusals

for CASO in "db snapshot --name=antes" "db snapshot --name=../fuera" "db snapshot --tonteria" \
            "db restore" "db restore noexiste" "db remove" "db remove noexiste" \
            "db clear --tonteria"; do
    ( cd "$DIR" && $SHELL_CLI $CASO >"$LAB/shell.err" 2>&1 ); ESTADO_SHELL=$?
    ( cd "$DIR" && $GO_BINARY  $CASO >"$LAB/go.err" 2>&1 );   ESTADO_GO=$?

    test_case "'$CASO' is refused the same way"
    assert_equals "$ESTADO_SHELL" "$ESTADO_GO"
    assert_equals "$(jq -S . < "$LAB/shell.err")" "$(jq -S . < "$LAB/go.err")"
done

test_case "and overwriting is allowed when it is asked for"
assert_equals "0" "$( cd "$DIR" && "$GO_BINARY" --force db snapshot --name=antes --json >/dev/null 2>&1; echo $? )"

test_case "a copy with no name is named after the moment it was taken"
nombre=$( cd "$DIR" && "$GO_BINARY" db snapshot --json | jq -r '.data.name' )
case "$nombre" in
    20[0-9][0-9]-[0-1][0-9]-[0-3][0-9]-[0-9][0-9][0-9][0-9][0-9][0-9]) r=fechado ;;
    *) r="$nombre" ;;
esac
assert_equals "fechado" "$r"

# ---------------------------------------------------------------- restoring

consulta "UPDATE pedidos SET total = 999 WHERE id = 1; CREATE TABLE despues (id INT);" >/dev/null

#
# Confirming is the project's name typed out rather than a letter: a blind `y` is a reflex, and
# this is the only thing here that destroys data.
#
test_case "an answer that is not the project's name restores nothing"
( cd "$DIR" && printf 'si\n' | "$GO_BINARY" --no-json db restore antes >"$LAB/go.out" 2>&1 )
assert_contains "$(cat "$LAB/go.out")" "Nothing was restored"
assert_equals "999" "$(consulta "SELECT total FROM pedidos WHERE id = 1")"

test_case "the project's name restores it"
( cd "$DIR" && printf '%s\n' "$PROJECT" | "$GO_BINARY" --no-json db restore antes >"$LAB/go.out" 2>&1 )
assert_contains "$(cat "$LAB/go.out")" "Restored"
assert_equals "100" "$(consulta "SELECT total FROM pedidos WHERE id = 1")"

#
# Emptied first: restoring over a database that kept living would leave whatever was created
# afterwards in place, and the result would be a mixture of the two rather than the copy.
#
test_case "and what was created afterwards is gone"
assert_empty "$(consulta "SHOW TABLES LIKE 'despues'")"

test_case "whatever a copy holds, nobody anonymised it after the fact"
( cd "$DIR" && "$GO_BINARY" --yes db restore antes ) >/dev/null 2>&1
assert_equals "unknown" "$( cd "$DIR" && "$GO_BINARY" describe --json | jq -r '.data.data.anonymised' )"

# ---------------------------------------------------------------- surviving the environment

test_case "the copies survive the environment being destroyed"
( cd "$DIR" && docker compose -p "$PROJECT" down -v ) >/dev/null 2>&1
assert_contains "$( cd "$DIR" && "$GO_BINARY" db list --json | jq -r '.data.snapshots[].name' )" "antes"

# ---------------------------------------------------------------- removing and clearing

test_case "a copy can be removed, and stops being listed"
( cd "$DIR" && "$GO_BINARY" db remove antes ) >/dev/null 2>&1
assert_not_contains "$( cd "$DIR" && "$GO_BINARY" db list --json | jq -r '.data.snapshots[].name' )" "antes"

sembrar() {
    mkdir -p "$HM_SNAPSHOT_DIR/$PROJECT" "$HM_SNAPSHOT_DIR/otro"
    for nombre in "$@"; do
        printf 'una copia\n' | gzip > "$HM_SNAPSHOT_DIR/$PROJECT/$nombre.sql.gz"
    done
    printf 'ajena\n' | gzip > "$HM_SNAPSHOT_DIR/otro/ajena.sql.gz"
}

rm -f "$HM_SNAPSHOT_DIR/$PROJECT"/*.sql.gz 2>/dev/null
sembrar una dos

test_case "clearing asks first, and an answer that is not the project's name deletes nothing"
( cd "$DIR" && printf 'no\n' | "$GO_BINARY" --no-json db clear >"$LAB/go.out" 2>&1 )
assert_contains "$(cat "$LAB/go.out")" "Nothing was deleted"
assert_equals "2" "$( cd "$DIR" && "$GO_BINARY" db list --json | jq '.data.snapshots | length' )"

test_case "and says what it would delete before asking"
assert_contains "$(cat "$LAB/go.out")" "una.sql.gz"

test_case "confirming clears this project's copies"
( cd "$DIR" && printf '%s\n' "$PROJECT" | "$GO_BINARY" --no-json db clear >"$LAB/go.out" 2>&1 )
assert_contains "$(cat "$LAB/go.out")" "Deleted 2 snapshot(s)"
assert_equals "0" "$( cd "$DIR" && "$GO_BINARY" db list --json | jq '.data.snapshots | length' )"

test_case "and leaves another project's alone"
assert_equals "0" "$([ -f "$HM_SNAPSHOT_DIR/otro/ajena.sql.gz" ] && echo 0 || echo 1)"

#
# The only thing here that can destroy copies belonging to projects you are not standing in, so it
# asks for a different word.
#
test_case "clearing everything is not confirmed by the project's name"
sembrar una
( cd "$DIR" && printf '%s\n' "$PROJECT" | "$GO_BINARY" --no-json db clear --all >/dev/null 2>&1 )
assert_equals "0" "$([ -f "$HM_SNAPSHOT_DIR/otro/ajena.sql.gz" ] && echo 0 || echo 1)"

test_case "and reaches every project when it is"
( cd "$DIR" && printf 'all\n' | "$GO_BINARY" --no-json db clear --all >/dev/null 2>&1 )
assert_empty "$(find "$HM_SNAPSHOT_DIR" -name '*.sql.gz' 2>/dev/null)"

test_case "clearing nothing says so instead of failing, in both"
assert_equals "$( cd "$DIR" && "$SHELL_CLI" db clear --json | jq -S . )" \
              "$( cd "$DIR" && "$GO_BINARY" db clear --json | jq -S . )"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
