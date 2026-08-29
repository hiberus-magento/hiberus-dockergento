#!/usr/bin/env bash
set -uo pipefail

source "$COMPONENTS_DIR"/print_message.sh
source "$COMPONENTS_DIR"/print_json.sh
source "$HELPERS_DIR"/exit_codes.sh
source "$HELPERS_DIR"/docker.sh
source "$TASKS_DIR"/collect_project_info.sh
source "$TASKS_DIR"/db_client.sh

hm_db_client_run "sequelace" "$@"
