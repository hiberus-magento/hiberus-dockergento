#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=/dev/null
source "$COMPONENTS_DIR"/input_info.sh

running_containers=$(docker ps -q)

if [[ "$running_containers" != "" ]]; then
    print_info "Stopping running containers\n"
    docker stop "$running_containers"
else
    echo "No containers running"
fi
