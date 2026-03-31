#!/usr/bin/env bash
set -euo pipefail

source "$COMPONENTS_DIR"/print_message.sh
source "$HELPERS_DIR"/docker.sh

is_run_service "phpfpm"

$DOCKER_COMPOSE exec phpfpm bash -c "magento-cloud $@"
