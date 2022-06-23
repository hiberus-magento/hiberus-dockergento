#!/usr/bin/env bash
set -euo pipefail

RUNNING_CONTAINERS=$(docker ps -q)
if [[ ${RUNNING_CONTAINERS} != "" ]]; then
    echo -e "${GREEN}Stopping running containers${COLOR_RESET}"
    docker stop "${RUNNING_CONTAINERS}"
else
    echo "No containers running"
fi
