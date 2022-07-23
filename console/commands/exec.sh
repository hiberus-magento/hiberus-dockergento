#!/usr/bin/env bash
set -euo pipefail

# add check if all containers is up

: "${exec_options:=""}"

if [[ "$1" == "--root" ]]; then
    shift
    exec_options="$exec_options -u root"
fi

docker_compose_exec="$DOCKER_COMPOSE exec"

if [ "$exec_options" != "" ]; then
    docker_compose_exec="$docker_compose_exec $exec_options"
fi
$docker_compose_exec phpfpm "$@"
