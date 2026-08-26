#!/usr/bin/env bash
set -uo pipefail

source "$COMPONENTS_DIR"/print_message.sh
source "$HELPERS_DIR"/exit_codes.sh

#
# Stop the environment, optionally saving the database first.
#
# Stopping is an everyday, quick operation, so the copy is asked for rather than taken: a `stop`
# that sometimes takes a minute because it is dumping a database would be an unpleasant surprise.
# The option is named after the subcommand that creates the copy, so it is one thing with one name.
#
snapshot=false
arguments=()

for argument in "$@"; do
    case "$argument" in
        --snapshot) snapshot=true ;;
        *)          arguments[${#arguments[@]}]="$argument" ;;
    esac
done

if $snapshot; then
    # Not stopping on failure is the point: a stopped environment and no copy, after asking for
    # one, is the worst of the three possible outcomes
    if ! "$COMMANDS_DIR"/db.sh snapshot; then
        hm_fail "$HM_EXIT_ERROR" "snapshot_failed" \
            "The snapshot failed, so the environment was left running" \
            "$COMMAND_BIN_NAME stop   # to stop without saving"
    fi
fi

$DOCKER_COMPOSE stop ${arguments[@]+"${arguments[@]}"}
