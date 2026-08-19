#!/usr/bin/env bash

source "${HELPERS_DIR}"/exit_codes.sh

#
# Check if docker is running
#
is_docker_service_running() {
    if [[ ! $(docker info >/dev/null 2>&1; echo $?) -eq 0 ]]; then
        hm_fail "$HM_EXIT_DOCKER" "docker_unavailable" \
            "Docker is not running" \
            "Start Docker and try again"
    fi
}

#
# Check if container of services is running
#
is_run_service() {
    is_docker_service_running
    local container_id service
    service="${1:-phpfpm}"
    container_id=$(docker ps -qf name="$COMPOSE_PROJECT_NAME"-"$service" -qf name="$COMPOSE_PROJECT_NAME"_"$service")
    
    if [ -z "$container_id" ]; then
        hm_fail "$HM_EXIT_SERVICE" "service_not_running" \
            "Service '$service' is not running" \
            "$COMMAND_BIN_NAME start $service"
    fi
}
