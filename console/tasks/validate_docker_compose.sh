#!/usr/bin/env bash
set -euo pipefail

source "$COMPONENTS_DIR"/print_message.sh
source "$HELPERS_DIR"/exit_codes.sh
source "$HELPERS_DIR"/cache.sh

#
# Validating the configuration costs a round trip to Compose: 72ms warm, 325ms cold, on
# nearly every command, while the files themselves change once in a blue moon.
#
# The cache key is the project root and the validity token is the modification time and size of
# the compose files, so any edit invalidates it. It errs on the safe side: it revalidates more
# often than strictly needed, never less. `hm doctor` skips project validation altogether
# and runs the real check itself, so there is always a way to verify for real.
#
CACHE_KEY="validated${HM_ROOT//\//-}"
CACHE_TOKEN=""

for compose_file in "$DOCKER_COMPOSE_FILE" "$DOCKER_COMPOSE_FILE_MACHINE"; do
    if [ -f "$compose_file" ]; then
        CACHE_TOKEN="$CACHE_TOKEN:$(hm_file_token "$compose_file")"
    fi
done

if [ -n "$CACHE_TOKEN" ] && hm_cache_read "$CACHE_KEY" "$CACHE_TOKEN" >/dev/null 2>&1; then
    exit 0
fi

# Capture the Compose error instead of letting it reach stderr: in JSON mode stderr
# carries the error envelope, and stray output would make it unparseable.
COMPOSE_ERROR=$($DOCKER_COMPOSE config -q 2>&1 >/dev/null) && CONFIG_IS_VALID=true || CONFIG_IS_VALID=false

if $CONFIG_IS_VALID && [ -n "$CACHE_TOKEN" ]; then
    hm_cache_write "$CACHE_KEY" "$CACHE_TOKEN" "valid"
fi

if ! $CONFIG_IS_VALID ; then
    MESSAGE="This directory is not a configured $COMMAND_TOOLNAME project, or its Docker configuration is invalid"

    if [ -n "$COMPOSE_ERROR" ]; then
        MESSAGE="$MESSAGE: $COMPOSE_ERROR"
    fi

    hm_fail "$HM_EXIT_PROJECT" "project_not_configured" \
        "$MESSAGE" \
        "$COMMAND_BIN_NAME setup"
fi
