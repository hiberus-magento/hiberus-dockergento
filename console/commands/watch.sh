#!/usr/bin/env bash
set -euo pipefail

if [[ "${MACHINE}" != "mac" ]]; then
    echo -e "${RED} This command is only for mac system.${COLOR_RESET}"
    exit 1
fi

PATH_ARGS=""
for WATCH_PATH in "$@"; do
    PATH_ARGS="${PATH_ARGS} -path ${WATCH_PATH}"
done

${DOCKER_COMPOSE} run --rm "${SERVICE_UNISON}" watch "${PATH_ARGS}"
