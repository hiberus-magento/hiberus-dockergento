#!/usr/bin/env bash
#
# Moving a project's files between this machine and its container.
#
# It exists because of macOS: there the code is copied into a volume rather than mounted, which is
# what makes PHP fast enough to work in, and the price is that the two sides are two places. What
# is checked here is that what goes in arrives, that what comes out lands where it was asked for,
# and that a path which is the same file on both sides is refused rather than copied over itself.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

GO_BINARY="$COMMAND_BIN_DIR/bin/hm"
PROJECT="hm-go-copias-fichero"
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
mkdir -p "$DIR/config/docker" "$DIR/app/code" "$DIR/montado"

#
# One directory mounted and the rest not, which is the shape this command exists for: what is
# mounted is already the same file on both sides, and what is not has to be carried.
#
cat > "$DIR/docker-compose.yml" <<YAML
services:
  phpfpm:
    image: alpine:latest
    working_dir: /var/www/html
    command: ["sleep", "600"]
    volumes:
      - $DIR/montado:/var/www/html/montado
YAML
cp "$DIR/docker-compose.yml" "$DIR/docker-compose.dev.mac.yml"
cp "$DIR/docker-compose.yml" "$DIR/docker-compose.dev.linux.yml"
printf '{"MAGENTO_DIR": ".", "DOMAIN": "copias.test", "COMPOSE_PROJECT_NAME": "%s", "WORKDIR_PHP": "/var/www/html"}\n' \
    "$PROJECT" > "$DIR/config/docker/properties.json"

( cd "$DIR" && docker compose -p "$PROJECT" up -d ) >/dev/null 2>&1

dentro() {
    ( cd "$DIR" && docker compose -p "$PROJECT" exec -T phpfpm sh -c "$1" 2>/dev/null )
}

# ---------------------------------------------------------------- into the container

printf 'uno\n' > "$DIR/app/code/uno.php"
mkdir -p "$DIR/app/code/vendor-suyo"
printf 'dos\n' > "$DIR/app/code/vendor-suyo/dos.php"

test_case "a directory is copied in, with everything under it"
( cd "$DIR" && "$GO_BINARY" copy-to-container app/code ) >/dev/null 2>&1
assert_equals "uno" "$(dentro 'cat app/code/uno.php')"
assert_equals "dos" "$(dentro 'cat app/code/vendor-suyo/dos.php')"

test_case "and a single file lands beside its neighbours, not inside itself"
printf 'tres\n' > "$DIR/suelto.txt"
( cd "$DIR" && "$GO_BINARY" copy-to-container suelto.txt ) >/dev/null 2>&1
assert_equals "tres" "$(dentro 'cat suelto.txt')"

#
# The two sides are already the same file there, and copying one onto the other is a way to lose
# whichever was newer.
#
test_case "a path that is a bind mount is refused rather than copied over itself"
printf 'montado\n' > "$DIR/montado/algo.txt"
( cd "$DIR" && "$GO_BINARY" copy-to-container montado >"$LAB/go.err" 2>&1 ); ESTADO=$?
assert_equals "6" "$ESTADO"
assert_contains "$(cat "$LAB/go.err")" "bind mount"

test_case "something that is not there is skipped rather than failing"
( cd "$DIR" && "$GO_BINARY" copy-to-container no-existe app/code >/dev/null 2>&1 ); ESTADO=$?
assert_equals "0" "$ESTADO"

test_case "and with nothing to copy it says so"
( cd "$DIR" && "$GO_BINARY" copy-to-container >/dev/null 2>&1 ); ESTADO=$?
assert_equals "2" "$ESTADO"

# ---------------------------------------------------------------- out of the container

dentro 'mkdir -p generated/code && printf "generado\n" > generated/code/hecho.php' >/dev/null 2>&1

test_case "a directory is copied out"
( cd "$DIR" && "$GO_BINARY" copy-from-container generated ) >/dev/null 2>&1
assert_equals "generado" "$(cat "$DIR/generated/code/hecho.php" 2>/dev/null)"

#
# Copying it again over what is already there replaces its contents rather than nesting a second
# copy inside the first, which is what a mirror has to do to be a mirror.
#
dentro 'printf "otro\n" > generated/code/hecho.php' >/dev/null 2>&1

test_case "and copying it again replaces what is there rather than nesting it"
( cd "$DIR" && "$GO_BINARY" copy-from-container generated ) >/dev/null 2>&1
assert_equals "otro" "$(cat "$DIR/generated/code/hecho.php" 2>/dev/null)"
assert_equals "1" "$([ -d "$DIR/generated/generated" ] && echo 0 || echo 1)"

test_case "a single file comes out too"
dentro 'printf "solo\n" > salida.txt' >/dev/null 2>&1
( cd "$DIR" && "$GO_BINARY" copy-from-container salida.txt ) >/dev/null 2>&1
assert_equals "solo" "$(cat "$DIR/salida.txt" 2>/dev/null)"

test_case "and with nothing to copy it says so"
( cd "$DIR" && "$GO_BINARY" copy-from-container >/dev/null 2>&1 ); ESTADO=$?
assert_equals "2" "$ESTADO"

# ---------------------------------------------------------------- with nothing running

( cd "$DIR" && docker compose -p "$PROJECT" stop phpfpm ) >/dev/null 2>&1

test_case "with the container stopped it is refused, not attempted"
( cd "$DIR" && "$GO_BINARY" copy-to-container app/code >"$LAB/go.err" 2>&1 ); ESTADO=$?
assert_equals "5" "$ESTADO"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
