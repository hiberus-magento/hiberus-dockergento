#!/usr/bin/env bash
set -euo pipefail

# add check if all containers is up

: "${exec_options:=""}"

docker_compose_exec="$DOCKER_COMPOSE exec"
# Set root option
if [[ -n "$1" && "$1" == "-r" ]]; then
    shift
    exec_options="$exec_options -u root"
fi

# Save exec option for final command
if [ "$exec_options" != "" ]; then
    docker_compose_exec="$docker_compose_exec $exec_options"
fi

# Execute docker-compose exec command
$docker_compose_exec phpfpm "$@"
