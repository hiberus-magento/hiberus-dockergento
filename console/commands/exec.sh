#!/usr/bin/env bash
set -euo pipefail

: "${exec_options:=""}"

if [[ "$1" == "--root" ]]; then
    shift
    exec_options="$exec_options -u root"
fi

docker_eompose_exec="$DOCKER_COMPOSE exec"

if [ "$exec_options" != "" ]; then
    docker_eompose_exec="$docker_eompose_exec $exec_options"
fi

$docker_eompose_exec phpfpm "$@"
