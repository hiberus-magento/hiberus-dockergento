#!/usr/bin/env bash
#
# The commands that are one thing run in one container. There is no shell twin left to compare
# against, so each case is pinned against something that is not the binary itself: the container
# it reached, the database it dumped, or git in the installation directory — never the binary
# compared with its own other rendering, which would prove nothing.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

GO_BINARY="$COMMAND_BIN_DIR/bin/hm"
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
# A recorder in place of each real binary, so what is checked is the container's own behaviour —
# not two CLIs agreeing with each other. `printf '%s\n' "$#"` then `printf '[%s]\n' "$@"` is the
# unambiguous shape: `echo "$@"` or `"$*"` would render three separate arguments identically to
# one already-joined string, which is exactly the defect class these two cases exist to catch.
#
cat > "$LAB/npm-recorder" <<'RECORDER'
#!/bin/sh
pwd                  >  /tmp/npm.argv
printf '%s\n' "$#"   >> /tmp/npm.argv
printf '[%s]\n' "$@" >> /tmp/npm.argv
exit 42
RECORDER
chmod +x "$LAB/npm-recorder"
( cd "$DIR" && docker compose -p "$PROJECT" cp "$LAB/npm-recorder" phpfpm:/usr/local/bin/npm ) >/dev/null 2>&1

test_case "npm is run in the php container, and its failure is its own"
( cd "$DIR" && "$GO_BINARY" npm --version ) >/dev/null 2>&1; ESTADO_GO=$?
assert_equals "$(printf '/var/www/html\n1\n[--version]')" "$(en_contenedor 'cat /tmp/npm.argv')"
assert_equals "42" "$ESTADO_GO"

#
# `wrappers.go:55` builds `["bash", "-c", "n98-magerun " + strings.Join(args, " ")]` — three
# elements, the third one already joined. The recorder replaces `bash` itself, which is the
# executable the container is asked to run: a recorder logging `"$@"` or `"$*"` alone would render
# that identically whether the arguments had been passed separately or already joined, so it must
# count arguments explicitly. Had they been passed separately, `$#` would read 3 and the last line
# would read `[n98-magerun]` / `[--version]` instead of one joined string — the regression this
# case exists to catch. (`$#` is 2, not 3: `$0` is the recorder's own name.)
#
cat > "$LAB/bash-recorder" <<'RECORDER'
#!/bin/sh
printf '%s\n' "$#"   >  /tmp/bash.argv
printf '[%s]\n' "$@" >> /tmp/bash.argv
exit 43
RECORDER
chmod +x "$LAB/bash-recorder"
( cd "$DIR" && docker compose -p "$PROJECT" cp "$LAB/bash-recorder" phpfpm:/usr/local/bin/bash ) >/dev/null 2>&1

test_case "and n98-magerun composes one already-joined command line, not separate arguments"
( cd "$DIR" && "$GO_BINARY" n98-magerun --version ) >/dev/null 2>&1; ESTADO_GO=$?
assert_equals "$(printf '2\n[-c]\n[n98-magerun --version]')" "$(en_contenedor 'cat /tmp/bash.argv')"
assert_equals "43" "$ESTADO_GO"

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
# Exact equality of the whole recorded invocation, not a substring: a single argument with spaces
# in it never invokes the recorder at all, which leaves the log empty rather than merely wrong —
# `echo "$@"` cannot tell one such argument from three separate ones, but an empty log can.
#
test_case "and the whole invocation reaches phpunit exactly, nothing appended"
en_contenedor ': > /tmp/llamadas' >/dev/null 2>&1
( cd "$DIR" && "$GO_BINARY" test-unit ) >/dev/null 2>&1
assert_equals "$(printf '/var/www/html\n--config ./dev/tests/unit/phpunit.xml.dist')" \
              "$(en_contenedor 'cat /tmp/llamadas')"

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
    mariadb -uroot -ppassword magento -e "CREATE TABLE pedidos (id INT); INSERT INTO pedidos VALUES (7),(8),(9);" ) >/dev/null 2>&1

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

#
# Completeness against the database itself, which is stronger than a `grep -c` against another
# dump: extended inserts can make both counts read 1 regardless of what actually happened. The
# dump is restored into a second database and every row is counted there.
#
test_case "and every row it dumped, restored and counted independently"
( cd "$DIR" && docker compose -p "$PROJECT" exec -T db \
    mariadb -uroot -ppassword -e "CREATE DATABASE IF NOT EXISTS verificacion" ) >/dev/null 2>&1
( cd "$DIR" && docker compose -p "$PROJECT" exec -T db \
    mariadb -uroot -ppassword verificacion ) < "$LAB/salida.sql" >/dev/null 2>&1
assert_equals "3" "$( cd "$DIR" && docker compose -p "$PROJECT" exec -T db \
    mariadb -uroot -ppassword verificacion -N -B -e "SELECT COUNT(*) FROM pedidos" )"

# ---------------------------------------------------------------- version

#
# Which build of the tool this is, and what is underneath it: not "1.4.5", but 1.4.5 and eleven
# commits, on this branch, with uncommitted changes — which is the difference between a report
# somebody can act on and one naming the version somebody happened to have tagged. There is no
# shell implementation left to compare against, so git in the installation directory is the
# oracle: it is the only thing independent of both the JSON and the text the binary prints.
#

COMMIT=$(git -C "$COMMAND_BIN_DIR" rev-parse --short HEAD 2>/dev/null)
DESCRIBED=$(git -C "$COMMAND_BIN_DIR" describe --tags 2>/dev/null)

test_case "the installation directory is a checkout, or nothing below is an oracle"
assert_equals "checkout" "$([ -n "$COMMIT" ] && echo checkout || echo "not a checkout")"

TAG=$(git -C "$COMMAND_BIN_DIR" describe --tags --abbrev=0 2>/dev/null)
DIRTY_GIT=false
[ -n "$(git -C "$COMMAND_BIN_DIR" status --porcelain --untracked-files=no 2>/dev/null)" ] && DIRTY_GIT=true

EXPECTED_VERSION="$DESCRIBED"
[ -z "$EXPECTED_VERSION" ] && EXPECTED_VERSION="unknown"

# The `<n>` of `<tag>-<n>-g<hash>`, or 0 when there is no such suffix (`core/installation.go:35-53`).
EXPECTED_AHEAD=0
case "$DESCRIBED" in
    *-g*)
        REST="${DESCRIBED%-g*}"
        EXPECTED_AHEAD="${REST##*-}"
        ;;
esac

VERSION_JSON=$( "$GO_BINARY" version --json )

test_case "the commit in the document is git's own, not a prefix of the long hash"
assert_json_field "$VERSION_JSON" '.data.commit' "$COMMIT"

test_case "the installation path is where the checkout is"
assert_json_field "$VERSION_JSON" '.data.path' "$COMMAND_BIN_DIR"

test_case "the version is what git describes, or unknown when it has nothing to say"
assert_json_field "$VERSION_JSON" '.data.version' "$EXPECTED_VERSION"

test_case "the tag is the nearest one alone, not the full description with its commit count"
assert_json_field "$VERSION_JSON" '.data.tag' "$TAG"

test_case "dirty means uncommitted tracked changes, and nothing else"
assert_json_field "$VERSION_JSON" '.data.dirty' "$DIRTY_GIT"

test_case "commits ahead is read from the same description git gives"
assert_json_field "$VERSION_JSON" '.data.commits_ahead' "$EXPECTED_AHEAD"

#
# The one field with no git oracle, because it is not about the checkout: which build of the
# binary itself is running. It is why the diagnostic that used to answer it is gone.
#
test_case "and the ported half says which build of it is running"
assert_equals "0" "$(echo "$VERSION_JSON" | jq -e '.data.binary != null' >/dev/null; echo $?)"

#
# The text rendering, checked against the same git oracle and not against the JSON: a cross-format
# comparison runs one binary against itself and cannot catch a value wrong in both renderings.
#
TEXT=$( "$GO_BINARY" --no-json version 2>&1 )

test_case "the words a person reads name the same commit"
assert_contains "$(grep '^  commit' <<< "$TEXT")" "$COMMIT"

test_case "and the same installation path"
assert_contains "$(grep '^  installed' <<< "$TEXT")" "$COMMAND_BIN_DIR"

test_case "and the header carries the same description git gives"
assert_contains "$TEXT" "$EXPECTED_VERSION"

test_case "it needs no project, because the problem may be that there is no project"
assert_equals "0" "$( cd "$LAB" && "$GO_BINARY" version >/dev/null 2>&1; echo $? )"

test_case "and an option nobody declared is a usage error"
assert_equals "2" "$( "$GO_BINARY" version --tonteria >/dev/null 2>&1; echo $? )"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
