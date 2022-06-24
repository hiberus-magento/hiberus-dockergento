#!/usr/bin/env bash
set -euo pipefail

CONFIG_IS_VALID=$($DOCKER_COMPOSE config -q && echo true || echo false)
if [[ ${CONFIG_IS_VALID} == false ]]; then
    if [[ $MACHINE == "mac" ]]; then
        printf "\n${RED}Docker is not properly configured or docker is not runnig. Please execute:${COLOR_RESET}\n"
    else
        printf "\n${RED}Docker is not properly configured. Please execute:${COLOR_RESET}\n\n\n"
    fi
    printf "  $COMMAND_BIN_NAME setup\n"
    exit 1
fi
