#!/usr/bin/env bash
set -euo pipefail

echo "${DOCKER_COMPOSE}"
CONFIG_IS_VALID=$(${DOCKER_COMPOSE} config -q && echo true || echo false)
if [[ ${CONFIG_IS_VALID} == false ]]; then
    echo ""
    if [[ ${MACHINE} == "mac" ]]; then
        printf "${RED}Docker is not properly configured or docker is not runnig. Please execute:${COLOR_RESET}\n"
    else
        printf "${RED}Docker is not properly configured. Please execute:${COLOR_RESET}\n"
    fi
    echo ""
    echo "  ${COMMAND_BIN_NAME} setup"
    echo ""
    exit 1
fi