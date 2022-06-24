#!/usr/bin/env bash
set -euo pipefail

echo -e "${GREEN}Rebuilding and starting containers in detached mode${COLOR_RESET}"

if [ "$#" == 0 ]; then
    ${COMMAND_BIN_NAME} stop
    ${DOCKER_COMPOSE} up --build -d "${SERVICE_APP}"
else
    ${COMMAND_BIN_NAME} stop "$@"
    ${DOCKER_COMPOSE} up --build -d "$@"
fi

${TASKS_DIR}/validate_bind_mounts.sh

if [[ "${MACHINE}" == "linux" ]]; then
    echo "Waiting for everything to spin up..."
    sleep 5
    echo " > fixing permissions"
    ${TASKS_DIR}/fix_linux_permissions.sh
    echo " > permissions fix finished"
fi
