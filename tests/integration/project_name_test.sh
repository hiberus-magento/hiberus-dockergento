#!/usr/bin/env bash
#
# The project's identity, against Docker Compose itself and against real directories.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"
source "$HELPERS_DIR/project_name.sh"

HM="$COMMAND_BIN_DIR/bin/run"
LAB=$(cd "$(mktemp -d)" && pwd -P)

cleanup() {
    for name in $(docker ps -aq --filter "label=com.docker.compose.project=hmnametest" 2>/dev/null); do
        docker rm -f "$name" >/dev/null 2>&1
    done
    rm -rf "$LAB"
}
trap cleanup EXIT

if ! docker info >/dev/null 2>&1; then
    echo "  - skipped: docker is not available"
    echo "RESULT 0 0"
    exit 0
fi

DEFAULT_PROPERTIES='{"MAGENTO_DIR": "."}'

make_project() {
    local dir="$1" properties="${2:-$DEFAULT_PROPERTIES}"
    mkdir -p "$dir/config/docker"
    # The label block is the one from the real template: this is what the resolved name has to
    # end up in
    cat > "$dir/docker-compose.yml" <<'YAML'
services:
  phpfpm:
    image: alpine:latest
    command: ["sleep", "60"]
    labels:
      hm.project: "${HM_PROJECT:-}"
      hm.root: "${HM_ROOT:-}"
      hm.worktree: "${HM_WORKTREE:-}"
YAML
    cp "$dir/docker-compose.yml" "$dir/docker-compose.dev.mac.yml"
    cp "$dir/docker-compose.yml" "$dir/docker-compose.dev.linux.yml"
    printf '%s' "$properties" > "$dir/config/docker/properties.json"
}

compose_name() {
    ( cd "$1" && env -u COMPOSE_PROJECT_NAME docker compose config --format json 2>/dev/null ) |
        jq -r '.name'
}

hm_name() {
    ( cd "$1" && "$HM" describe --json 2>/dev/null ) | jq -r '.data.project.name // ""'
}

# ------------------------------------------------- the rule matches Compose, not a table of ours

for directory in "UPPER" "con espacio" "punto.com" "acentúado" "my_project-1" "2fast" "--weird--"; do
    test_case "'$directory' derives the same name as Compose gives it"
    make_project "$LAB/$directory"
    assert_equals "$(compose_name "$LAB/$directory")" "$(hm_derive_project_name "$directory")"
done

# ------------------------------------------------- resolution through the CLI

test_case "a project with no configured name is known by its directory"
make_project "$LAB/Sin Nombre"
assert_equals "sinnombre" "$(hm_name "$LAB/Sin Nombre")"

test_case "and the CLI agrees with Compose about it"
assert_equals "$(compose_name "$LAB/Sin Nombre")" "$(hm_name "$LAB/Sin Nombre")"

test_case "a configured name wins"
make_project "$LAB/otro-directorio" '{"MAGENTO_DIR": ".", "COMPOSE_PROJECT_NAME": "hmnametest"}'
assert_equals "hmnametest" "$(hm_name "$LAB/otro-directorio")"

test_case "a directory with no usable name is refused, not passed to Compose"
make_project "$LAB/___"
( cd "$LAB/___" && "$HM" describe --json >"$LAB/out" 2>"$LAB/err" )
assert_equals "4" "$?"

test_case "and the refusal says what to do about it"
assert_json_field "$(cat "$LAB/err")" '.error.type' "no_project_name"

# ------------------------------------------------- two copies are two environments

test_case "two copies of a project without a configured name are different environments"
make_project "$LAB/tienda"
cp -R "$LAB/tienda" "$LAB/tienda-copia"
first=$(hm_name "$LAB/tienda")
second=$(hm_name "$LAB/tienda-copia")
[ -n "$first" ] && [ "$first" != "$second" ] && r=different || r="$first vs $second"
assert_equals "different" "$r"

test_case "but a configured name is inherited by a copy, because it is a decision"
cp -R "$LAB/otro-directorio" "$LAB/otro-directorio-copia"
assert_equals "$(hm_name "$LAB/otro-directorio")" "$(hm_name "$LAB/otro-directorio-copia")"

# ------------------------------------------------- a worktree keeps the main checkout's identity

test_case "from a worktree the derived name is the main checkout's"
make_project "$LAB/principal"
(
    cd "$LAB/principal" || exit 1
    git init -q -b main
    git config user.email test@example.com
    git config user.name test
    git add -A && git commit -qm init
    git worktree add --detach "$LAB/una-rama" HEAD
) >/dev/null 2>&1
assert_equals "principal" "$(hm_name "$LAB/una-rama")"

test_case "so a worktree does not invent a second environment"
assert_equals "$(hm_name "$LAB/principal")" "$(hm_name "$LAB/una-rama")"

# ------------------------------------------------- what setup records

# The properties file is committed, so what it says travels to every clone. Recording a name that
# the directory would have produced anyway is what made a clone inherit the original's identity.
record() {
    local dir="$1" existing="$2" chosen="$3"
    mkdir -p "$dir/config/docker"
    printf '%s' "$existing" > "$dir/config/docker/properties.json"
    (
        cd "$dir" || exit 1
        export CUSTOM_PROPERTIES_DIR="$PWD/config/docker" DOCKER_CONFIG_DIR="config/docker"
        export MAGENTO_DIR="." DOMAIN="x.local" COMPOSE_PROJECT_NAME="$chosen" HM_ROOT="$PWD"
        source "$COMPONENTS_DIR/print_message.sh"
        source "$HELPERS_DIR/properties.sh"
        save_properties >/dev/null 2>&1
    )
    jq -c . "$dir/config/docker/properties.json"
}

test_case "accepting the proposed name records nothing"
assert_equals "false" "$(record "$LAB/rec-a" '{}' "rec-a" | jq 'has("COMPOSE_PROJECT_NAME")')"

test_case "a name of your own is recorded"
assert_equals "otro" "$(record "$LAB/rec-b" '{}' "otro" | jq -r '.COMPOSE_PROJECT_NAME')"

test_case "a project that already had a name keeps it"
assert_equals "viejo" "$(record "$LAB/rec-c" '{"COMPOSE_PROJECT_NAME":"viejo"}' "viejo" | jq -r '.COMPOSE_PROJECT_NAME')"

test_case "and other properties are no longer wiped"
assert_equals "custom/bin" "$(record "$LAB/rec-d" '{"BIN_DIR":"custom/bin"}' "rec-d" | jq -r '.BIN_DIR')"

# ------------------------------------------------- the label stops being empty

test_case "the containers of an unnamed project carry the resolved name as a label"
( cd "$LAB/tienda" && "$HM" start >/dev/null 2>&1 )
labelled=$(docker ps -aq --filter "label=hm.project=tienda" 2>/dev/null | head -1)
[ -n "$labelled" ] && r=labelled || r="no container labelled hm.project=tienda"
assert_equals "labelled" "$r"

test_case "and the environment shows up in the inventory under that name"
assert_contains "$("$HM" list --json 2>/dev/null | jq -r '.data.environments[].name')" "tienda"

( cd "$LAB/tienda" && "$HM" down >/dev/null 2>&1 )

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
