#!/usr/bin/env bash
set -euo pipefail

echo -e "${GREEN}Starting containers in detached mode${COLOR_RESET}\n"

if [ "$#" == 0 ]; then
    ${DOCKER_COMPOSE} up -d
else
    ${DOCKER_COMPOSE} up -d "$@"
fi

"${TASKS_DIR}"/validate_bind_mounts.sh

if [[ "${MACHINE}" == "linux" ]]; then
    echo "Waiting for everything to spin up..."
    sleep 5
    echo " > fixing permissions"
    "${TASKS_DIR}"/fix_linux_permissions.sh
    echo " > permissions fix finished"
fi
