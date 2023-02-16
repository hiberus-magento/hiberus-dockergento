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
