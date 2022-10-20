#!/usr/bin/env bash

set -euo pipefail

source "$COMPONENTS_DIR"/print_message.sh
source "$COMPONENTS_DIR"/input_info.sh

#
# Create docker-compose files
#
create_docker_compose() {
    if [[ -f "docker-compose.yml" ]]; then
        if [[ -z "$(cat "docker-compose.yml" | grep "hiberus-magento")" ]]; then
          "$TASKS_DIR"/version_manager.sh
        fi
    else
      "$TASKS_DIR"/version_manager.sh
    fi
}

# Prepare environment
get_project_name
get_domain
get_magento_root_directory
create_docker_compose

 # Start services
"$TASKS_DIR"/start_service_if_not_running.sh "$SERVICE_APP"

# Magento installation
"$TASKS_DIR"/magento_installation.sh "setup"

print_info "\nSetup completed!\n"

print_info "Open "
print_link "https://$DOMAIN/\n"