#!/usr/bin/env bash
#
# The file the old format used, and why it is not inert.
#
# `config/docker/properties` is converted into properties.json and removed on every invocation.
# So anything that writes a line into it decides what the project's properties.json will say next
# time somebody runs any command — and until now that conversion replaced the file rather than
# merging into it.
#
# Importing a dump into a project with no domain configured did write a line into it. The next
# command left that project with a properties.json holding nothing but the domain: no
# COMPOSE_PROJECT_NAME, which is the name its containers, its volumes and its database answer to.
# They were still running, under a name nothing pointed at any more.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

#
# Sourcing this file also runs it: the conversion, the palette and the properties happen at the
# bottom of it. So the directories it needs are set first, and the function is called again per
# case with a fresh one.
#
preparar() {
    CUSTOM_PROPERTIES_DIR=$(mktemp -d)
    DOCKER_CONFIG_DIR="$CUSTOM_PROPERTIES_DIR"
    export CUSTOM_PROPERTIES_DIR DOCKER_CONFIG_DIR
}

preparar
source "$COMMAND_BIN_DIR/console/tasks/load_properties.sh"

# ---------------------------------------------------------------- the conversion

test_case "an old properties file becomes properties.json"
preparar
printf '  COMPOSE_PROJECT_NAME="tienda"\n  MAGENTO_DIR="./src"\n' > "$CUSTOM_PROPERTIES_DIR/properties"
refactor_old_version
assert_equals "tienda" "$(jq -r '.COMPOSE_PROJECT_NAME' "$CUSTOM_PROPERTIES_DIR/properties.json")"

test_case "and the old file is removed once it has been read"
assert_equals "no" "$([ -f "$CUSTOM_PROPERTIES_DIR/properties" ] && echo yes || echo no)"

test_case "what properties.json already said is kept"
preparar
printf '{"COMPOSE_PROJECT_NAME": "tienda", "MAGENTO_DIR": "./src", "WORKDIR_PHP": "/var/www/html"}\n' \
    > "$CUSTOM_PROPERTIES_DIR/properties.json"
printf '  DOMAIN="recuperado.test"\n' > "$CUSTOM_PROPERTIES_DIR/properties"
refactor_old_version
assert_equals "tienda" "$(jq -r '.COMPOSE_PROJECT_NAME' "$CUSTOM_PROPERTIES_DIR/properties.json")"
assert_equals "./src" "$(jq -r '.MAGENTO_DIR' "$CUSTOM_PROPERTIES_DIR/properties.json")"

test_case "and the new value is there too"
assert_equals "recuperado.test" "$(jq -r '.DOMAIN' "$CUSTOM_PROPERTIES_DIR/properties.json")"

# ---------------------------------------------------------------- and nothing writes to it now

test_case "the domain read from the database goes into properties.json"
preparar
printf '{"COMPOSE_PROJECT_NAME": "tienda", "MAGENTO_DIR": "./src"}\n' > "$CUSTOM_PROPERTIES_DIR/properties.json"

# The database is stubbed: what is being tested is where the answer is written, not how it is asked
query() {
    case "$1" in
        *core_config_data*LIKE*|*"SHOW TABLES"*) echo "core_config_data" ;;
        *base_url*)                              echo "https://recuperado.test/" ;;
    esac
}
unset DOMAIN
source "$COMMAND_BIN_DIR/console/tasks/set_magento_configs.sh"
set_current_domain

assert_equals "recuperado.test" "$(jq -r '.DOMAIN' "$CUSTOM_PROPERTIES_DIR/properties.json")"

test_case "and the project keeps its identity"
assert_equals "tienda" "$(jq -r '.COMPOSE_PROJECT_NAME' "$CUSTOM_PROPERTIES_DIR/properties.json")"

test_case "no file in the old format is left behind"
assert_equals "no" "$([ -f "$CUSTOM_PROPERTIES_DIR/properties" ] && echo yes || echo no)"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
