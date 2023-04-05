#!/usr/bin/env bash

#
# Check if container of services is running
#
is_run() {
    container=${1:="phpfpm"}
    if [ -z "$(docker ps | grep $container)" ]; then
        print_error "Error: $container service is not running!\n"
        exit 2
    fi
}

is_docker_service_running() {
    if [[ ! $(docker info >/dev/null 2>&1; echo $?) -eq 0 ]]; then
        print_warning "Docker is not eunning!\n"
        exit 1
    fi
}
