#!/usr/bin/env bash
set -euo pipefail

#
# Check if container of services is running
#
is_run_service() {
    is_docker_service_running

    service=${1:="phpfpm"}
    container_id=$(docker ps -qf "name=$COMPOSE_PROJECT_NAME-$service")
    if [ -z "$container_id" ]; then
        print_warning "Error: $service service is not running!\n"
        exit 1
    fi
}

#
# Chack if docker is running
#
is_docker_service_running() {
    if [[ ! $(docker info >/dev/null 2>&1; echo $?) -eq 0 ]]; then
        print_warning "Docker is not running!\n"
        exit 1
    fi
}

is_docker_service_running() {
    if [[ ! $(docker info >/dev/null 2>&1; echo $?) -eq 0 ]]; then
        print_warning "Docker is not eunning!\n"
        exit 1
    fi
}
