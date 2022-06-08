#!/usr/bin/env bash
set -euo pipefail

REGEX=""

#
# 
#
compose_regex() {
    local services=$(echo "${REQUERIMENTS}" | jq -r 'keys|join(" ")')

    for index in ${services}
    do
        value=$(echo "${REQUERIMENTS}" | jq -r '.'${index}'')
        REGEX+="s/<${index}_version>/${value}/g; "
    done
}

#
# Adapt docker-compose template with requeriments
#
wirte_docker_compose() {
    compose_regex

    sed "${REGEX}" "${COMMAND_BIN_DIR}/docker-compose/docker-compose.template.yml" > "${MAGENTO_DIR}/docker-compose.yml"
    mkdir -p $(dirname ${DOCKER_COMPOSE_FILE_LINUX})
    cp "${COMMAND_BIN_DIR}/docker-compose/docker-compose.dev.linux.template.yml" "${DOCKER_COMPOSE_FILE_LINUX}"
    cp "${COMMAND_BIN_DIR}/docker-compose/docker-compose.dev.mac.template.yml" "${DOCKER_COMPOSE_FILE_MAC}"
}

REQUERIMENTS=$1
wirte_docker_compose