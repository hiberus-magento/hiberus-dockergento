#!/usr/bin/env bash
#
# Stop and remove the environment, ported.
#
# Without `-v` this destroys nothing that cannot be rebuilt. With it the volumes go and the
# database with them: one letter of difference, no warning and no way back. An environment on this
# machine was lost exactly that way, which is why the question exists — and why it is a list of
# three answers rather than a yes.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

GO_BINARY="$COMMAND_BIN_DIR/bin/hm"
SHELL_CLI="$COMMAND_BIN_DIR/bin/run"
PROJECT="hm-go-abajo"
LAB=$(cd "$(mktemp -d)" && pwd -P)

limpiar() {
    ( cd "$LAB/$PROJECT" 2>/dev/null && docker compose -p "$PROJECT" down -v --remove-orphans ) >/dev/null 2>&1
    docker volume rm "${PROJECT}_dbdata" >/dev/null 2>&1
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

export HM_SNAPSHOT_DIR="$LAB/copias"

DIR="$LAB/$PROJECT"
mkdir -p "$DIR/config/docker"
cat > "$DIR/docker-compose.yml" <<'YAML'
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
  phpfpm:
    image: alpine:latest
    command: ["sleep", "600"]
volumes:
  dbdata:
YAML
cp "$DIR/docker-compose.yml" "$DIR/docker-compose.dev.mac.yml"
cp "$DIR/docker-compose.yml" "$DIR/docker-compose.dev.linux.yml"
printf '{"MAGENTO_DIR": ".", "DOMAIN": "abajo.test", "COMPOSE_PROJECT_NAME": "%s"}\n' "$PROJECT" \
    > "$DIR/config/docker/properties.json"

arriba() {
    ( cd "$DIR" && docker compose -p "$PROJECT" up -d ) >/dev/null 2>&1
}

#
# A copy cannot be taken from a database that is not answering yet, and a container that is running
# is not the same thing as one that is. Waiting here is what makes the difference between testing
# the copy and testing the wait.
#
esperar_db() {
    local esperado=0

    until ( cd "$DIR" && docker compose -p "$PROJECT" exec -T db \
                mariadb -uroot -ppassword magento -N -B -e "SELECT 1" ) >/dev/null 2>&1 ||
          [ "$esperado" -gt 90 ]; do
        sleep 2
        esperado=$((esperado + 2))
    done
}

corriendo() {
    docker ps --filter "label=com.docker.compose.project=$PROJECT" -q | grep -c . | tr -d ' '
}

hay_volumen() {
    docker volume inspect "${PROJECT}_dbdata" >/dev/null 2>&1 && echo si || echo no
}

# ---------------------------------------------------------------- the everyday case

arriba

test_case "down stops and removes the environment"
( cd "$DIR" && "$GO_BINARY" --no-json down >"$LAB/go.out" 2>&1 )
assert_equals "0" "$?"
assert_equals "0" "$(corriendo)"

#
# The data is what cannot be rebuilt, so it is what `down` leaves alone unless asked.
#
test_case "and leaves the data where it was"
assert_equals "si" "$(hay_volumen)"

# ---------------------------------------------------------------- the refusals

for CASO in "down --tonteria" "down -t nada"; do
    ( cd "$DIR" && $SHELL_CLI $CASO >"$LAB/shell.err" 2>&1 ); ESTADO_SHELL=$?
    ( cd "$DIR" && $GO_BINARY  $CASO >"$LAB/go.err" 2>&1 );   ESTADO_GO=$?

    test_case "'$CASO' is a usage error"
    assert_equals "2" "$ESTADO_GO"
done

# ---------------------------------------------------------------- the question

arriba

#
# Three answers, and the third one destroys nothing. It is the answer somebody reaches for when
# they read the list of volumes and realised which project they were standing in.
#
test_case "answering 'Cancel' destroys nothing"
( cd "$DIR" && printf '3\n' | "$GO_BINARY" --no-json down -v >"$LAB/go.out" 2>&1 )
assert_contains "$(cat "$LAB/go.out")" "Nothing was destroyed"
assert_equals "si" "$(hay_volumen)"

test_case "and the question names the volumes that are about to go"
assert_contains "$(cat "$LAB/go.out")" "${PROJECT}_dbdata"

test_case "and offers to save a copy first"
assert_contains "$(cat "$LAB/go.out")" "Save a database snapshot"

#
# An environment gone and no copy, after asking for one, is the worst of the three outcomes. So a
# copy that cannot be taken destroys nothing: shown here against a database that is not there to
# be copied.
#
( cd "$DIR" && docker compose -p "$PROJECT" stop db ) >/dev/null 2>&1

test_case "a copy that cannot be taken destroys nothing"
( cd "$DIR" && printf '1\n' | "$GO_BINARY" --no-json down -v >"$LAB/go.out" 2>&1 ); ESTADO=$?
assert_equals "1" "$ESTADO"
assert_contains "$(cat "$LAB/go.out")" "nothing was destroyed"
assert_equals "si" "$(hay_volumen)"

#
# Saving first is offered first because it is the one nobody regrets.
#
arriba
esperar_db

test_case "answering it takes a copy, and then destroys"
( cd "$DIR" && printf '1\n' | "$GO_BINARY" --no-json down -v >"$LAB/go.out" 2>&1 )
assert_equals "0" "$(corriendo)"
assert_equals "no" "$(hay_volumen)"

test_case "and the copy is there afterwards, which is the whole point"
assert_equals "1" "$(find "$HM_SNAPSHOT_DIR/$PROJECT" -name '*.sql.gz' 2>/dev/null | grep -c .)"

# ---------------------------------------------------------------- not interactive

arriba

#
# The flag was explicit, so nothing is asked: a command that hung waiting for an answer nobody can
# give would be worse than the deletion.
#
test_case "a non-interactive run deletes without asking"
( cd "$DIR" && "$GO_BINARY" --yes --no-json down -v >"$LAB/go.out" 2>&1 )
assert_equals "0" "$(corriendo)"
assert_equals "no" "$(hay_volumen)"

test_case "and with nothing left to lose it does not ask either"
( cd "$DIR" && printf '\n' | "$GO_BINARY" --no-json down -v >"$LAB/go.out" 2>&1 )
assert_not_contains "$(cat "$LAB/go.out")" "What should happen"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
