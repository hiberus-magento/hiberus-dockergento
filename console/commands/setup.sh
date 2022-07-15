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
        tool_name=$("$TASKS_DIR"/look_at_yml.sh "$MAGENTO_DIR/docker-compose.yml" "x-toolname")
        
        if [[ tool_name != "hiberus-magento" ]]; then
            while true; do
                print_error "\n----------------------------------------------------------------------\n"
                print_error "             ¡¡¡WE HAVE DETECTED DOCKER COMPOSE FILES!!! \n\n"
                print_error "    If you continue with this proccess these files will be removed\n"
                print_error "----------------------------------------------------------------------\n\n"
                print_question "Do you want continue? [Y/n] "

                read -r yn
                if [ -z "$yn" ]; then
                    yn="y"
                fi
                case $yn in
                [Yy]*) break ;;
                [Nn]*) exit 1;;
                *) echo "Please answer yes or no." ;;
                esac
            done
        else
            exit
        fi
    fi
    "$TASKS_DIR"/version_manager.sh
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