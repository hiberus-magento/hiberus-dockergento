#!/usr/bin/env bash
set -euo pipefail

DOCKER_CONFIG_DIR="config/docker"
# shellcheck source=/dev/null
source "$COMPONENTS_DIR"/print_message.sh

#
# Ask magento directory
#
get_magento_root_directory() {
    print_question "Magento root dir: [$MAGENTO_DIR] "

    MAGENTO_DIR=${answer_magento_dir:-$MAGENTO_DIR}

    if [ "$MAGENTO_DIR" != "." ]; then
        print_info "Setting custom magento dir: '$MAGENTO_DIR'\n"
        MAGENTO_DIR=$(sanitize_path "$MAGENTO_DIR")
        print_warnning "------ $DOCKER_COMPOSE_FILE ------\n"
        sed_in_file "s#/html/var/composer_home#/html/$MAGENTO_DIR/var/composer_home#gw /dev/stdout" "$DOCKER_COMPOSE_FILE"
        print_warnning "--------------------\n"
        print_warnning "------ $DOCKER_COMPOSE_FILE_MAC ------\n"
        sed_in_file "s#/app:#/$MAGENTO_DIR/app:#gw /dev/stdout" "$DOCKER_COMPOSE_FILE_MAC"
        sed_in_file "s#/vendor#/$MAGENTO_DIR/vendor#gw /dev/stdout" "$DOCKER_COMPOSE_FILE_MAC"
        print_warnning "--------------------\n"
        print_warnning "------ $DOCKER_CONFIG_DIR/nginx/conf/default.conf ------\n"
        sed_in_file "s#/var/www/html#/var/www/html/$MAGENTO_DIR#gw /dev/stdout" "$DOCKER_CONFIG_DIR/nginx/conf/default.conf"
        print_warnning "--------------------\n"
    fi
}

#
# Check if exit docker-compose file in magento root
#
check_if_docker_enviroment_exist() {
    if [[ -f "$MAGENTO_DIR/docker-compose.yml" ]]; then
        while true; do
            print_error "\n----------------------------------------------------------------------\n"
            print_error "             ¡¡¡WE HAVE DETECTED DOCKER COMPOSE FILES!!! \n\n"
            print_error "    If you continue with this proccess these files will be removed\n"
            print_error "----------------------------------------------------------------------\n\n"
            print_question "Do you want continue? [y/n] "
            read -r yn
            case $yn in
            [Yy]*) break ;;
            [Nn]*) exit ;;
            *) echo "Please answer yes or no." ;;
            esac
        done
    fi
}

#
# Copy File
#
copy() {
    local source_path=$1
    local target_path=$2
    local target_dir
    
    target_dir=$(dirname "$target_path")
    mkdir -p "$target_dir"
    cp -Rf "$source_path" "$target_path"
}

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
# replace in file
#
sed_in_file() {
    local sed_regex=$1
    local target_path=$2

    if [[ "$MACHINE" == "mac" ]]; then
        sed -i '' "$sed_regex" "$target_path"
    else
        sed -i "$sed_regex" "$target_path"
    fi
}

#
# Add git bind paths in file
#
add_git_bind_paths_in_file() {
    git_files=$1
    file_to_edit=$2
    suffix_bind_path=$3

    bind_paths=""
    while read -r filename_in_git; do
        if [[ "$MAGENTO_DIR" == "$filename_in_git" ]] ||
            [[ "$MAGENTO_DIR" == "$filename_in_git/"* ]] ||
            [[ "$filename_in_git" == "vendor" ]] ||
            [[ "$filename_in_git" == "${DOCKER_COMPOSE_FILE%.*}"* ]]; then
            continue
        fi

        new_path="./$filename_in_git:/var/www/html/$filename_in_git"
        bind_path_exits=$(grep -q -e "$new_path" "$file_to_edit" && echo true || echo false)

        if [ "$bind_path_exits" == true ]; then
            continue
        fi

        if [ "$bind_paths" != "" ]; then
            bind_paths="$bind_paths\\ "
        fi

        bind_paths="$bind_paths- ${new_path}$suffix_bind_path"

    done <<<"${git_files}"

    print_warnning "------ $file_to_edit ------"
    sed_in_file "s|# {FILES_IN_GIT}|$bind_paths|w /dev/stdout" "$file_to_edit"
    print_warnning "--------------------"
}

get_equivalent_version_if_exit() {
    equivalent_version=$("$TASKS_DIR/get_equivalent_version.sh" "$1")
    if [[ "$equivalent_version" = "null" ]]; then
        print_warnning "\nWe don´t have support for the version $1 "
        print_info "\nPlease, write any version between all versions supported or press Ctrl - C to exit"

        $COMMAND_BIN_NAME compatibility
        read -r MAGENTO_VERSION
        get_equivalent_version_if_exit "$MAGENTO_VERSION"
    fi

    get_requeriments "$equivalent_version"
}

#
# If there are arguments
#
get_requeriments() {
    # Check if command "jq" exists
    if ! command -v jq &>/dev/null; then
        print_error "Required 'jq' not found"
        print_question "https://stedolan.github.io/jq/download/"
        exit
    fi

    if [ "$#" -gt 0 ]; then
        requeriments=$(cat <"$DATA_DIR/requeriments.json" | jq -r '.['\""$1"\"']')
        change_requeriments
    else
        if [ -f "$MAGENTO_DIR/composer.lock" ]; then
            MAGENTO_VERSION=$(cat <"$MAGENTO_DIR/composer.lock" |
                jq -r '.packages | map(select(.name == "magento/product-community-edition"))[].version')
            print_warnning "\nVersion detected: $MAGENTO_VERSION"
        else
            print_error "\n------------------------------------------------------\n"
            print_error "\n       We need a magento project in $MAGENTO_DIR/ path\n"
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
# Update git setting in docker-compose
#
set_settings() {
    print_info "Setting up docker config files\n"
    copy "$COMMAND_BIN_DIR/$DOCKER_CONFIG_DIR/" "$DOCKER_CONFIG_DIR"

    print_info "Setting bind configuration for files in git repository\n"

    if [[ -f ".git/HEAD" ]]; then
        git_files=$(git ls-files | awk -F / '{print $1}' | uniq)

        if [[ "${git_files}" != "" ]]; then
            add_git_bind_paths_in_file "${git_files}" "${DOCKER_COMPOSE_FILE_MAC}" ":delegated"
        else
            print_highlight " > Skipped. There are no files added in this repository\n"
        fi
    else
        print_highlight " > Skipped. This is not a git repository\n"
    fi
}

#
# Set propierties in <root_project>/<docker_config>/propierties
#
save_properties() {
    print_info "Saving custom properties file: '$DOCKER_CONFIG_DIR/properties'\n"
    cat <<EOF >./$DOCKER_CONFIG_DIR/properties
  MAGENTO_DIR="$MAGENTO_DIR"
  BIN_DIR="$BIN_DIR"
  COMPOSE_PROJECT_NAME="$COMPOSE_PROJECT_NAME"
EOF
}

#
# Print current requeriments
#
print_requeriments() {
    services=$(echo "${requeriments}" | jq -r 'keys|join(" ")')

    print_table "\n\n-------------------------------\n"
    print_table "          REQUERIMENTS"
    print_table "\n-------------------------------\n"
    for index in ${services}; do
        value=$(echo "${requeriments}" | jq -r '.'"${index}"'')
        print_table "   $index: "
        print_default "${value}\n"
    done
    print_table "-------------------------------\n\n"
}

#
# Ask if user wants to change requeriments
#
change_requeriments() {
    print_requeriments
    state="continue"
    while [[ $state == "continue" ]]; do
        print_question "Are you satisfied with these versions? [y/n] "
        read -r yn
        case $yn in
        [Yy]*) state="exit" ;;
        [Nn]*)
            edit_versions
            break
            ;;
        *) echo "Please answer yes or no." ;;
        esac
    done
}

#
# Changes especific service value
#
edit_version() {
    service_name=$1
    opts="$(cat <"$DATA_DIR/requeriments.json" | jq -r '[.[] | .'"$service_name"'] | unique  | join(" ")')"

    print_question "$service_name version:\n"
    select select_result in $opts; do
        if $($TASKS_DIR/in_list.sh "$select_result" "$opts"); then
            break
        fi

        if $($TASKS_DIR/in_list.sh "${REPLY}" "$opts"); then
            select_result=${REPLY}
            break
        fi
        echo "invalid option '${REPLY}'"
    done

echo "service_name: $service_name"
echo "select_result: $select_result"
    requeriments=$(echo "$requeriments" | jq -r '.'"$service_name"'="'"$select_result"'"')
}

#
# Select editable services and changes her value
#
edit_versions() {
    opts=$(echo "${requeriments} " | jq -r 'keys | join(" ")')

    print_question "Choose service:\n"
    select select_result in $opts; do
        if $($TASKS_DIR/in_list.sh "$select_result" "$opts"); then
            break
        fi

        if $($TASKS_DIR/in_list.sh "${REPLY}" "$opts"); then
            select_result=${REPLY}
            break
        fi

        echo "invalid option '${REPLY}'"
    done

    edit_version "$select_result"
    change_requeriments
}

get_magento_root_directory
check_if_docker_enviroment_exist
get_requeriments "$@"
"$TASKS_DIR/write_from_docker-compose_templates.sh" "${requeriments}"
set_settings
save_properties

# Stop running containers in case that setup was executed in an already running project
$COMMAND_BIN_NAME stop