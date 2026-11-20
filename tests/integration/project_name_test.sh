#!/usr/bin/env bash
#
# The project's identity, against Docker Compose itself and against real directories.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

HM="$COMMAND_BIN_DIR/bin/hm"
GO_BINARY="$HM"
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

# The name the binary resolved: a configured one, or the directory's when none is configured
hm_name() {
    ( cd "$1" && "$HM" describe --json 2>/dev/null ) | jq -r '.data.project.name // ""'
}

# ------------------------------------------------- the rule matches Compose, not a table of ours

for directory in "UPPER" "con espacio" "punto.com" "acentúado" "my_project-1" "2fast" "--weird--"; do
    test_case "'$directory' derives the same name as Compose gives it"
    make_project "$LAB/$directory"
    assert_equals "$(compose_name "$LAB/$directory")" "$(hm_name "$LAB/$directory")"
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

# A project command is the window onto the refusal bin/run raised for every command
# (no_project_name): describe answers any unresolvable project with the generic
# project_not_configured, which would let this case pass for an unrelated reason.
test_case "a directory with no usable name is refused, not passed to Compose"
make_project "$LAB/___"
mkdir -p "$LAB/___/config/hm/commands"
printf '#!/bin/sh\necho ran\n' > "$LAB/___/config/hm/commands/probe.sh"
chmod +x "$LAB/___/config/hm/commands/probe.sh"
( cd "$LAB/___" && "$HM" probe --json >"$LAB/out" 2>"$LAB/err" )
assert_equals "4" "$?"
assert_equals "" "$(cat "$LAB/out")"

test_case "and the refusal says what to do about it"
assert_json_field "$(cat "$LAB/err")" '.error.type' "no_project_name"
assert_contains "$(cat "$LAB/err")" "COMPOSE_PROJECT_NAME"

test_case "describe refuses it too"
( cd "$LAB/___" && "$HM" describe --json >"$LAB/out" 2>"$LAB/err" )
assert_equals "4" "$?"
assert_contains "$(jq -r '.error.message' "$LAB/err")" "project name"

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
assert_equals "hmnametest" "$(hm_name "$LAB/otro-directorio")"
assert_equals "hmnametest" "$(hm_name "$LAB/otro-directorio-copia")"

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
assert_equals "principal" "$(hm_name "$LAB/principal")"

# ------------------------------------------------- what setup records
#
# Pinned in Go now, against app.Setup.Save (dockergento/app/setup_test.go): accepting the proposed
# name records nothing, a name of your own is recorded, an existing one is kept, nothing else is
# wiped.

# ------------------------------------------------- the label stops being empty

test_case "the containers of an unnamed project carry the resolved name as a label"
( cd "$LAB/tienda" && "$COMMAND_BIN_DIR/bin/hm" start >/dev/null 2>&1 )
labelled=$(docker ps -aq --filter "label=hm.project=tienda" 2>/dev/null | head -1)
[ -n "$labelled" ] && r=labelled || r="no container labelled hm.project=tienda"
assert_equals "labelled" "$r"

test_case "and the environment shows up in the inventory under that name"
assert_contains "$("$GO_BINARY" list --json 2>/dev/null | jq -r '.data.environments[].name')" "tienda"

( cd "$LAB/tienda" && "$COMMAND_BIN_DIR/bin/hm" down >/dev/null 2>&1 )

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
