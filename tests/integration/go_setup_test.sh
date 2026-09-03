#!/usr/bin/env bash
#
# Creating a project's environment, ported.
#
# What `setup` writes is what every other command then reads, so this is about the files: which
# images the services run, where the code is mounted from, and the overlay that puts the project
# behind the global proxy — which is the one where a mistake is invisible until something is
# running.
#
# Nothing is installed here. The steps that follow writing the files are recorded against a shell
# tree of this test's own, which is also how their order is checked without a Magento to install.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

GO_BINARY="$COMMAND_BIN_DIR/bin/hm"
LAB=$(cd "$(mktemp -d)" && pwd -P)
trap 'rm -rf "$LAB"' EXIT

if ! command -v go >/dev/null 2>&1; then
    echo "  - skipped: go is missing"
    echo "RESULT 0 0"
    exit 0
fi

export GOCACHE="$LAB/go-build"

( cd "$COMMAND_BIN_DIR" && go build -o "$GO_BINARY" ./cmd/hm ) >/dev/null 2>&1 || {
    echo "  - skipped: the binary does not build here"
    echo "RESULT 0 0"
    exit 0
}

#
# A shell tree that installs nothing and writes down what it was asked to do. Everything after the
# files is still the shell implementation's, and what has to be right here is that it is asked for
# the right things in the right order.
#
mkdir -p "$LAB/fake/bin" "$LAB/fake/console"
cat > "$LAB/fake/bin/run" <<'FAKE'
#!/usr/bin/env bash
printf '%s\n' "$*" >> "$HM_FAKE_LOG"
FAKE
chmod +x "$LAB/fake/bin/run"
export HM_LEGACY_ROOT="$LAB/fake"
export HM_FAKE_LOG="$LAB/pasos.txt"

PROJECT="hm-go-setup"
DIR="$LAB/$PROJECT"
mkdir -p "$DIR/app/code" "$DIR/pub"
printf '{"packages":[{"name":"magento/product-community-edition","version":"2.4.7"}]}\n' \
    > "$DIR/composer.lock"
printf '{}\n' > "$DIR/composer.json"
printf 'algo\n' > "$DIR/app/code/uno.php"
printf 'algo\n' > "$DIR/pub/index.php"
printf 'algo\n' > "$DIR/patches.json"

( cd "$DIR" && git init -q . && git add -A &&
  git -c user.email=t@t -c user.name=t commit -qm inicial ) >/dev/null 2>&1

ejecutar() {
    ( cd "$DIR" && "$GO_BINARY" --yes --no-json setup "$@" >"$LAB/out" 2>"$LAB/err" )
    ESTADO=$?
    SALIDA=$(cat "$LAB/out" "$LAB/err")
    return 0
}

# ---------------------------------------------------------------- the files it writes

ejecutar -p "$PROJECT" --domain=tienda.test -i
assert_equals "0" "$ESTADO" "a project with a composer.lock can be set up"

test_case "the three compose files are written"
for FICHERO in docker-compose.yml docker-compose.dev.mac.yml docker-compose.dev.linux.yml; do
    assert_equals "0" "$([ -f "$DIR/$FICHERO" ] && echo 0 || echo 1)"
done

#
# Which images the services run is the whole point of generating the file: they come from the table
# for the Magento this project resolves to, not from whatever was newest when somebody wrote the
# template.
#
test_case "the services run the images this Magento needs"
esperado=$(jq -r '.["2.4.7"].php' "$COMMAND_BIN_DIR/data/requirements.json")
assert_contains "$(cat "$DIR/docker-compose.yml")" "hiberusmagento/php:$esperado"

test_case "and the mail catcher the project chose"
assert_contains "$(cat "$DIR/docker-compose.yml")" "mailhog"

test_case "no marker is left unreplaced"
assert_not_contains "$(cat "$DIR/docker-compose.yml")" "_version>"
assert_not_contains "$(cat "$DIR/docker-compose.dev.mac.yml")" "{MAGENTO_DIR}"
assert_not_contains "$(cat "$DIR/docker-compose.dev.mac.yml")" "{YML_VERSION}"

#
# On macOS the code lives in a volume and only named paths come from the host, so anything the
# repository tracks and nobody mounted is invisible inside the container — a file that exists in
# git and not in the environment, which no error explains.
#
test_case "what the repository tracks is mounted, and what Magento brings is not"
assert_contains "$(cat "$DIR/docker-compose.dev.mac.yml")" "./patches.json:/var/www/html/patches.json:delegated"
assert_not_contains "$(cat "$DIR/docker-compose.dev.mac.yml")" "/var/www/html/app:delegated"

test_case "and the generated files are not mounted into the container they configure"
assert_not_contains "$(cat "$DIR/docker-compose.dev.mac.yml")" "docker-compose.yml:/var/www/html"

# ---------------------------------------------------------------- what it recorded

test_case "the project's own properties are written outside the code"
assert_equals "tienda.test" "$(jq -r '.DOMAIN' "$DIR/config/docker/properties.json")"
assert_equals "." "$(jq -r '.MAGENTO_DIR' "$DIR/config/docker/properties.json")"

#
# The name is recorded only when it is a decision. This file is committed, so whatever it says
# travels to every clone: writing the name the directory would have given anyway is what made a
# second clone inherit the first one's identity — same containers, same volumes, neither asked for.
#
test_case "and the name only when it is not the one the directory gives"
assert_equals "null" "$(jq -r '.COMPOSE_PROJECT_NAME' "$DIR/config/docker/properties.json")"
ejecutar -p otro-nombre -i
assert_equals "otro-nombre" "$(jq -r '.COMPOSE_PROJECT_NAME' "$DIR/config/docker/properties.json")"

#
# Mailhog is the default, so a project that runs it says nothing rather than saying the default out
# loud: what is in this file travels to every clone.
#
test_case "and says nothing about what it did not choose"
assert_equals "null" "$(jq -r '.MAIL_SERVICE' "$DIR/config/docker/properties.json")"

# ---------------------------------------------------------------- the order of what follows

test_case "the environment is started before anything is installed into it"
assert_equals "start" "$(sed -n 1p "$HM_FAKE_LOG")"
assert_equals "composer install" "$(sed -n 2p "$HM_FAKE_LOG")"

test_case "and the address is set up at the end, once there is something to reach"
assert_equals "ssl tienda.test" "$(grep -n 'ssl ' "$HM_FAKE_LOG" | head -1 | cut -d: -f2-)"
assert_contains "$(cat "$HM_FAKE_LOG")" "set-host tienda.test --no-database"

test_case "with no dump, nothing is imported"
assert_not_contains "$(cat "$HM_FAKE_LOG")" "mysql -i"

# ---------------------------------------------------------------- a dump instead of an install

: > "$HM_FAKE_LOG"
printf -- '-- un volcado\n' > "$LAB/dump.sql"
ejecutar --db-dump="$LAB/dump.sql"

test_case "a dump is imported, and the admin users it brought are removed"
assert_contains "$(cat "$HM_FAKE_LOG")" "mysql -i $LAB/dump.sql"
assert_contains "$(cat "$HM_FAKE_LOG")" "DELETE FROM admin_user"

test_case "a dump that is not there stops the command before anything is created"
ejecutar --db-dump=/no/such/file.sql
assert_equals "2" "$ESTADO"
assert_contains "$SALIDA" "/no/such/file.sql"

# ---------------------------------------------------------------- the proxy overlay

#
# Compared with the shell implementation's, character for character. A repeated key in YAML is not
# a merge — the last one wins and the earlier block disappears without a word — and a service that
# quietly keeps its published ports is a project that cannot be up beside another.
#
jq '. + {USE_PROXY: "true"}' "$DIR/config/docker/properties.json" > "$LAB/p.json" &&
    mv "$LAB/p.json" "$DIR/config/docker/properties.json"

ejecutar -f -i

test_case "a project behind the proxy gets its overlay"
assert_equals "0" "$([ -f "$DIR/docker-compose.proxy.yml" ] && echo 0 || echo 1)"

(
    export MAIL_SERVICE="mailhog" DOMAIN="tienda.test"
    COMPOSE_PROJECT_NAME=$(jq -r '.COMPOSE_PROJECT_NAME' "$DIR/config/docker/properties.json")
    export COMPOSE_PROJECT_NAME
    export HM_PROXY_NETWORK="hm-gateway" DOCKER_COMPOSE_FILE="$LAB/shell/docker-compose.yml"
    mkdir -p "$LAB/shell"
    source "$TASKS_DIR/proxy.sh"
    hm_proxy_write_overlay
) >/dev/null 2>&1

test_case "and it is the one the shell implementation writes"
assert_equals "$(cat "$LAB/shell/docker-compose.proxy.yml" 2>/dev/null)" \
              "$(cat "$DIR/docker-compose.proxy.yml")"

#
# Removing it matters as much as writing it: a project that stopped using the proxy and kept the
# file would have its ports removed by a file nobody remembers, and answer on nothing.
#
jq '. + {USE_PROXY: "false"}' "$DIR/config/docker/properties.json" > "$LAB/p.json" &&
    mv "$LAB/p.json" "$DIR/config/docker/properties.json"

ejecutar -f -i

test_case "a project that stops using the proxy loses the overlay"
assert_equals "1" "$([ -f "$DIR/docker-compose.proxy.yml" ] && echo 0 || echo 1)"

# ---------------------------------------------------------------- what it refuses

test_case "an option nobody declared is a usage error"
ejecutar --tonteria
assert_equals "2" "$ESTADO"

test_case "and so is a mail catcher this tool does not know"
ejecutar --mail=paloma
assert_equals "2" "$ESTADO"

#
# A compose file of ours is left alone: regenerating it recreates the containers, which is a
# morning of somebody's work for no change.
#
ejecutar -i
assert_contains "$(cat "$DIR/docker-compose.yml")" "hiberus-magento" "ours is left in place"
marca=$(md5 -q "$DIR/docker-compose.yml" 2>/dev/null || md5sum "$DIR/docker-compose.yml" | cut -d" " -f1)
ejecutar -i
assert_equals "$marca" "$(md5 -q "$DIR/docker-compose.yml" 2>/dev/null || md5sum "$DIR/docker-compose.yml" | cut -d" " -f1)" \
    "and running setup again does not touch it"

#
# One that is not ours is a project that has not been set up yet — an empty checkout, or a compose
# file from somewhere else — and writing one is what setup is for.
#
test_case "a file that is not ours is what setup is for"
printf 'services: {}\n' > "$DIR/docker-compose.yml"
ejecutar -i
assert_contains "$(cat "$DIR/docker-compose.yml")" "hiberus-magento"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
