#!/usr/bin/env bash
set -euo pipefail

regex=""

#
# Compose regex with requirements
#
compose_regex() {
    local services
    services=$(echo "$requirements" | jq -r 'keys|join(" ")')

    for index in $services; do
        value=$(echo "$requirements" | jq -r '.'"$index"'')
        regex+="s/<${index}_version>/${value}/g; "
    done
}

#
# Adapt docker-compose template with requirements
#
wirte_docker_compose() {
    compose_regex
    local composer_dir_name

    sed "$regex" "$COMMAND_BIN_DIR/docker-compose/docker-compose.template.yml" >"$MAGENTO_DIR/docker-compose.yml"
    composer_dir_name=$(dirname "$DOCKER_COMPOSE_FILE_LINUX")
    mkdir -p "$composer_dir_name"
    cp "$COMMAND_BIN_DIR/docker-compose/docker-compose.dev.linux.template.yml" "$DOCKER_COMPOSE_FILE_LINUX"
    cp "$COMMAND_BIN_DIR/docker-compose/docker-compose.dev.mac.template.yml" "$DOCKER_COMPOSE_FILE_MAC"
}

requirements=$1
wirte_docker_compose
