#!/bin/bash
set -euo pipefail

source "$COMPONENTS_DIR"/input_info.sh
source "$COMPONENTS_DIR"/print_message.sh

project_name=""
domain=""
version=""
edition=""
root_directory=""

export USE_DEFAULT_SETTINGS=false

#
# Overwrite file consent
#
overwrite_file_consent() {
    local target_file=$1

    if [[ -f "$target_file" ]]; then
        
        read -rp "$(print_question "Overwrite $target_file? [Y/n]? ")" answer_overwrite_target
        if [ -z "$answer_overwrite_target" ]; then
            answer_overwrite_target="y"
        fi
        if [ "$answer_overwrite_target" != "y" ]; then
            print_error "Setup interrupted. This commands needs to overwrite this file."
            exit 1
        fi
    fi
}

#
# Initialize command script
#
create_project_execute() {
    # Get magento version information
    get_magento_edition "$edition"
    get_magento_version "$version"
    get_project_name "$project_name"
    get_domain "$domain"

    if ! [ -d "$root_directory" ]; then
        mkdir -p "$root_directory"
    fi
    
    # Create docker environment
    get_magento_root_directory "$root_directory"
    "$TASKS_DIR"/version_manager.sh "$MAGENTO_VERSION"
    docker-compose -f docker-compose.yml up -d
    container_id=$($DOCKER_COMPOSE ps -q phpfpm)

    # Also make sure alternate auth.json is setup (Magento uses this internally)
    "$COMMANDS_DIR"/exec.sh [ -d "./var/composer_home" ] && \
    "$COMMANDS_DIR"/exec.sh cp /var/www/.composer/auth.json ./var/composer_home/auth.json
    
    # Execute composer create-project and copy composer.json
    "$COMMANDS_DIR"/exec.sh composer create-project \
        --no-install \
        --repository=https://repo.magento.com/ \
        magento/project-"$MAGENTO_EDITION"-edition="$MAGENTO_VERSION" "."

    # Copy all to host
    docker cp "$container_id":"$WORKDIR_PHP"/composer.json "$MAGENTO_DIR"

    # Create empty composer.lock
    echo "{}" > "$MAGENTO_DIR"/composer.lock
    
    # Run docker-compose specified files of OS
    "$COMMANDS_DIR"/restart.sh "phpfpm"

    # Magento installation
    "$TASKS_DIR"/magento_installation.sh

    print_info "Open "
    print_link "https://$DOMAIN/\n"
}

while getopts ":p:e:v:r:u" options; do
    case "$options" in
        p)
            # Project name
            project_name="$OPTARG"
            domain="$project_name.local"
        ;;
        e)
            # Edition
            edition="$OPTARG"
        ;;
        v)
            # Version
            version="$OPTARG"
        ;;
        r)
            # Magento root 
            root_directory="$OPTARG"
        ;;
        u)
            # use default settings
            suggested_name=$(basename "$PWD")
            last_version="$(get_last_version)"
            project_name=${project_name:-$suggested_name}
            domain="$project_name.local"
            edition=${edition:="community"}
            version=${version:-$last_version}
            root_directory=${root_directory:="."}
            export USE_DEFAULT_SETTINGS=true
        ;;
        ?)
            source "$HELPERS_DIR"/print_usage.sh
            print_error "The command is not correct\n"
            print_info "Use this format\n"
            get_usage "$(basename ${0%.sh})"
            exit 1
        ;;
    esac
done

create_project_execute "$@"
