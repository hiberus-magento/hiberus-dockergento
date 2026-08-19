#!/usr/bin/env bash
set -euo pipefail

source "$COMPONENTS_DIR"/print_message.sh
source "$HELPERS_DIR"/exit_codes.sh

# Capture the Compose error instead of letting it reach stderr: in JSON mode stderr
# carries the error envelope, and stray output would make it unparseable.
COMPOSE_ERROR=$($DOCKER_COMPOSE config -q 2>&1 >/dev/null) && CONFIG_IS_VALID=true || CONFIG_IS_VALID=false

if ! $CONFIG_IS_VALID ; then
    MESSAGE="This directory is not a configured $COMMAND_TOOLNAME project, or its Docker configuration is invalid"

    if [ -n "$COMPOSE_ERROR" ]; then
        MESSAGE="$MESSAGE: $COMPOSE_ERROR"
    fi

    hm_fail "$HM_EXIT_PROJECT" "project_not_configured" \
        "$MESSAGE" \
        "$COMMAND_BIN_NAME setup"
fi
