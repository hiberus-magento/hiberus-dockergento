#!/usr/bin/env bash
set -euo pipefail

REGEX=""

#
# Compose regex with requeriments
#
compose_regex() {
    local services
    services=$(echo "$requeriments" | jq -r 'keys|join(" ")')

    for index in ${services}; do
        value=$(echo "$requeriments" | jq -r '.'"${index}"'')
        REGEX+="s/<${index}_version>/${value}/g; "
    done
}

#
# Adapt docker-compose template with requeriments
#
wirte_docker_compose() {
    compose_regex

    sed "${REGEX}" "${COMMAND_BIN_DIR}/docker-compose/docker-compose.template.yml" >"$MAGENTO_DIR/docker-compose.yml"
    COMPOSER_DIR_NAME=$(dirname "$DOCKER_COMPOSE_FILE_LINUX")
    mkdir -p "$COMPOSER_DIR_NAME"
    cp "${COMMAND_BIN_DIR}/docker-compose/docker-compose.dev.linux.template.yml" "${DOCKER_COMPOSE_FILE_LINUX}"
    cp "${COMMAND_BIN_DIR}/docker-compose/docker-compose.dev.mac.template.yml" "${DOCKER_COMPOSE_FILE_MAC}"
}

requeriments=$1
wirte_docker_compose
