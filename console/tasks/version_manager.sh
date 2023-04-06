#!/usr/bin/env bash
set -euo pipefail

export REQUIREMENTS=""

source "$COMPONENTS_DIR"/print_message.sh
source "$COMPONENTS_DIR"/input_info.sh
source "$HELPERS_DIR"/properties.sh

#
# Copy File
#
copy() {
    local source_path=$1
    local target_path=$2
    local target_dir=$(dirname "$target_path")

    mkdir -p "$target_dir"
    cp -Rf "$source_path" "$target_path"
}

#
# Changes specific service value
#
edit_version() {
    service_name=$1
    opts=$(jq -r '[.[] | .'["'$service_name'"]'] | unique  | join(" ")' < "$DATA_DIR/requirements.json")

    custom_select "$service_name version:\n" ${opts[@]}
    select_result=$REPLY

    REQUIREMENTS=$(echo "$REQUIREMENTS" | jq -r '.["'$service_name'"]='$select_result'')
}

#
# Select editable services and changes her value
#
edit_versions() {
    opts=($(echo "$REQUIREMENTS" | jq -r 'keys[]'))
    options=("composer" "php" "search" "mariadb" "redis" "varnish")

    echo $opts
    
    custom_select "Choose service:" "${options[@]}"

    edit_version "$REPLY"
    change_requirements
    
exit 2
}

#
# Print current requirements
#
print_requirements() {
    services=$(echo "$REQUIREMENTS" | jq -r 'keys|join(" ")')

    print_table "\n\n-------------------------------\n"
    print_table "          REQUIREMENTS"
    print_table "\n-------------------------------\n"
    for index in $services; do
        value=$(echo "$REQUIREMENTS" | jq -r '.["'$index'"]')
        print_table "   $index: "
        print_default "$value\n"
    done
    print_table "-------------------------------\n\n"
}

#
# Ask if user wants to change requirements
#
change_requirements() {
    print_requirements
    state="continue"
    while [[ $state == "continue" ]]; do
        
        read -rp "$(print_question "Are you satisfied with these versions? [Y/n] ")" yn
        if [ -z "$yn" ]; then
            yn="y"
        fi
        case $yn in
        [Yy]*) state="exit" ;;
        [Nn]*)
            edit_versions
            break
            ;;
        *) print_warning "Please answer yes or no.\n" ;;
        esac
    done
}

#
# Get equivalent version from configuration
#
get_equivalent_version_if_exit() {
    equivalent_version=$("$HELPERS_DIR"/get_equivalent_version.sh "$1")

    if [[ "$equivalent_version" = "null" ]]; then
        print_warning "\nWe don´t have support for the version $1\n"
        print_info "Please, write any version between all versions supported or press Ctrl - C to exit"

        "$COMMANDS_DIR"/compatibility.sh
        read -r MAGENTO_VERSION
        get_equivalent_version_if_exit "$MAGENTO_VERSION"
    fi

    get_requirements "$equivalent_version"
}

#
# If there are arguments
#
get_requirements() {
    if [ "$#" -gt 0 ]; then
        REQUIREMENTS=$(jq -r '.["'$1'"]' "$DATA_DIR/requirements.json")
        if ! $USE_DEAFULT_SETTINGS; then
            change_requirements
        fi
        export REQUIREMENTS=$REQUIREMENTS
    else
        if [ -f "$MAGENTO_DIR/composer.lock" ]; then
            MAGENTO_VERSION=$(jq -r '.packages |
                map(select(.name == "magento/product-community-edition"))[].version' < "$MAGENTO_DIR/composer.lock")
            print_warning "\nVersion detected: $MAGENTO_VERSION\n"
        else
            print_error "\n------------------------------------------------------\n"
            print_error "\n      Not found composer.lock in $MAGENTO_DIR/ directory\n"
            print_default "\n You can clone a project and after execute "
            print_code "$COMMAND_BIN_NAME setup"
            print_default "\n or create a new magento project with "
            print_code "$COMMAND_BIN_NAME create-project"
            print_error "\n------------------------------------------------------\n"
            exit 1
        fi

        get_equivalent_version_if_exit "$MAGENTO_VERSION"
    fi
}

#
# Add git bind paths in file
#
add_git_bind_paths_in_file() {
    local git_files=$1
    local file_to_edit=$2
    local suffix_bind_path=$3
    local bind_paths=""

    while read -r filename_in_git; do
        if [[ "$MAGENTO_DIR" == "$filename_in_git" ]] ||
            [[ "$MAGENTO_DIR" == "$filename_in_git/"* ]] ||
            [[ "$filename_in_git" == "vendor" ]] ||
            [[ "$filename_in_git" == "${DOCKER_COMPOSE_FILE%.*}"* ]]; then
            continue
        fi

        new_path="$MAGENTO_DIR/${filename_in_git}:/var/www/html/$filename_in_git"
        bind_path_exits=$(grep -q -e "$new_path" "$file_to_edit" && echo true || echo false)
        default_file_magento=$(jq -r '.["'$filename_in_git'"]' < "$DATA_DIR/default_files_magento.json")

        if [[ "$bind_path_exits" == true ]] || [[ $default_file_magento == true ]]; then
            continue
        fi

        if [ "$bind_paths" != "" ]; then
            bind_paths="$bind_paths\\      "
        fi

        bind_paths="$bind_paths- ${new_path}$suffix_bind_path\n"

    done <<< "${git_files}"

    print_warning "------ $file_to_edit ------\n"
    sed_in_file "s|# {FILES_IN_GIT}|$bind_paths|w /dev/stdout" "$file_to_edit"
    print_warning "--------------------\n"
}

#
# Update git setting in docker-compose
#
set_settings() {
    print_info "Setting up docker config files PENDDING\n"
    print_info "Setting bind configuration for files in git repository\n"

    sed_in_file "s|{MAGENTO_DIR}|$MAGENTO_DIR|w /dev/stdout" "$DOCKER_COMPOSE_FILE_MAC"

    if [[ -f "$MAGENTO_DIR/.git/HEAD" ]]; then
        git_files=$(git --git-dir="$MAGENTO_DIR/.git" ls-files | awk -F / '{print $1}' | uniq)

        if [[ "$git_files" != "" ]]; then
            add_git_bind_paths_in_file "$git_files" "$DOCKER_COMPOSE_FILE_MAC" ":delegated"
        else
            print_processing "Skipped. There are no files added in this repository"
        fi
    else
        print_processing "Skipped. This is not a git repository"
    fi
}

get_requirements "$@"
"$TASKS_DIR"/write_from_docker-compose_templates.sh
set_settings
save_properties