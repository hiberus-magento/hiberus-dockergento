#!/usr/bin/env bash
set -euo pipefail

source "$COMPONENTS_DIR"/print_message.sh

CONFIG_IS_VALID=$($DOCKER_COMPOSE config -q && echo true || echo false)

if ! $CONFIG_IS_VALID ; then
    if [[ $MACHINE == "mac" ]]; then
        mac_cause=" or docker is not running"
    fi
    
    print_error "\nDocker is not properly configured${mac_cause:-}. Please execute:\n\n\n"
    print_default "  $COMMAND_BIN_NAME setup\n"
    exit 1
fi
