#!/usr/bin/env bash
set -euo pipefail

if [[ "${MACHINE}" != "mac" ]] && [[ "${MACHINE}" != "windows" ]];
then
    printf "${RED} This command is only for mac and windows systems.${COLOR_RESET}\n"
    exit 1
fi

PATH_ARGS=""
for WATCH_PATH in $@
do
    PATH_ARGS="${PATH_ARGS} -path ${WATCH_PATH}"
done

${DOCKER_COMPOSE} run --rm ${SERVICE_UNISON} watch ${PATH_ARGS}