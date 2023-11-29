#!/usr/bin/env bash

get_container_id() {
    local container_id service
    service="${1:-phpfpm}"
    container_id=$(docker ps -qf "name=$COMPOSE_PROJECT_NAME-$service" -qf "name=$COMPOSE_PROJECT_NAME_$service")

    echo "$container_id"
}

#
# Check if docker is running
#
is_docker_service_running() {
    if [[ ! $(docker info >/dev/null 2>&1; echo $?) -eq 0 ]]; then
        print_warning "Docker is not running!\n"
        exit 1
    fi
}

#
# Check if container of services is running
#
is_run_service() {
    is_docker_service_running
    local container_id service
    service="${1:-phpfpm}"
    container_id=$(get_container_id "$service")
    
    if [ -z "$container_id" ]; then
        print_warning "Error: $service service is not running!\n"
        exit 1
    fi
}
