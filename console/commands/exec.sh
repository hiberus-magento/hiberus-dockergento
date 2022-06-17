#!/usr/bin/env bash
set -euo pipefail

: ${EXEC_OPTIONS:=""}

if [ ${TTY_DISABLE} == true ];
then
    EXEC_OPTIONS="${EXEC_OPTIONS} -T"
fi

if [[ "$1" == "--root" ]];
then
    shift
    EXEC_OPTIONS="${EXEC_OPTIONS} -u root"
fi

DOCKER_COMPOSE_EXEC="${DOCKER_COMPOSE} exec"
if [ "${EXEC_OPTIONS}" != "" ];
then
    DOCKER_COMPOSE_EXEC="${DOCKER_COMPOSE_EXEC} ${EXEC_OPTIONS}"
fi

${DOCKER_COMPOSE_EXEC} ${SERVICE_PHP} "$@"