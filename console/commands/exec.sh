#!/usr/bin/env bash
set -euo pipefail

# Set root option
if [[ "$1" == "-r" ]]; then
    shift
    "$COMMANDS_DIR"/docker-compose.sh exec -u root phpfpm "$@"
    exit
fi

# Execute docker-compose exec command
"$COMMANDS_DIR"/docker-compose.sh exec phpfpm "$@"
