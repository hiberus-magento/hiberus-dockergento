#!/usr/bin/env bash
set -euo pipefail

service=$1

set +e # Disable interruption in case of error
$DOCKER_COMPOSE exec -T "$service" sh -c "echo 'check $service service is running'" &>/dev/null
service_running_error=$?
set -e # Enable interruption in case of error

if [ $service_running_error == 1 ]; then
    $COMMAND_BIN_NAME start
fi
