#!/usr/bin/env bash
set -euo pipefail

DOCKER_CONFIG_DIR="config/docker"
# shellcheck source=/dev/null
source "$COMPONENTS_DIR"/print_message.sh
source "$COMPONENTS_DIR"/input_info.sh

#
# Sanitize path
#
sanitize_path() {
    sanitized_path=${1#/}
    sanitized_path=${sanitized_path#./}
    sanitized_path=${sanitized_path%/}
    echo "$sanitized_path"
}

#
# Create docker-compose files
#
create_docker_compose() {
    if [[ -f "$MAGENTO_DIR/docker-compose.yml" ]]; then
        if [[ -z "$(cat "$MAGENTO_DIR/docker-compose.yml" | grep "hiberus-magento")" ]]; then
          "$TASKS_DIR"/version_manager.sh
        fi
    else
      "$TASKS_DIR"/version_manager.sh
    fi
}

# Prepare enviroment
get_domain
get_magento_root_directory
create_docker_compose

 # Start services
"$TASKS_DIR"/start_service_if_not_running.sh "$SERVICE_APP"

# Magento instalation
"$TASKS_DIR"/magento_installation.sh

print_info "\nSetup completed!\n"

print_info "Open "
print_link "https://$DOMAIN/\n"