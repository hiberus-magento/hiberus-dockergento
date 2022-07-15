#!/usr/bin/env bash
set -euo pipefail

export REQUIREMENTS=""

DOCKER_CONFIG_DIR="config/docker"

# shellcheck source=/dev/null
source "$COMPONENTS_DIR"/print_message.sh
source "$COMPONENTS_DIR"/input_info.sh

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
# Changes especific service value
#
edit_version() {
    service_name=$1
    opts="$(cat < "$DATA_DIR/requirements.json" | jq -r '[.[] | .'"$service_name"'] | unique  | join(" ")')"

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

    REQUIREMENTS=$(echo "$REQUIREMENTS" | jq -r '.'"$service_name"'="'"$select_result"'"')
}

#
# Select editable services and changes her value
#
edit_versions() {
    opts=$(echo "$REQUIREMENTS " | jq -r 'keys | join(" ")')

    print_question "Choose service:\n"
    select select_result in $opts; do
        if $($TASKS_DIR/in_list.sh "$select_result" "$opts"); then
            break
        fi

        if $($TASKS_DIR/in_list.sh "$REPLY" "$opts"); then
            select_result=$REPLY
            break
        fi

        echo "invalid option '$REPLY'"
    done

    edit_version "$select_result"
    change_requirements
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
        value=$(echo "$REQUIREMENTS" | jq -r '.'"$index"'')
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
        print_question "Are you satisfied with these versions? [Y/n] "
        read -r yn
        if [ -z "$yn" ]; then
            yn="y"
        fi
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
# Get equivalent version from configuration
#
get_equivalent_version_if_exit() {
    equivalent_version=$("$TASKS_DIR/get_equivalent_version.sh" "$1")

    if [[ "$equivalent_version" = "null" ]]; then
        print_warning "\nWe don´t have support for the version $1 "
        print_info "\nPlease, write any version between all versions supported or press Ctrl - C to exit"

        $COMMAND_BIN_NAME compatibility
        read -r MAGENTO_VERSION
        get_equivalent_version_if_exit "$MAGENTO_VERSION"
    fi

    get_requirements "$equivalent_version"
}

#
# If there are arguments
#
get_requirements() {
    # Check if command "jq" exists
    if ! command -v jq &>/dev/null; then
        print_error "Required 'jq' not found"
        print_question "https://stedolan.github.io/jq/download/"
        exit
    fi

    if [ "$#" -gt 0 ]; then
        REQUIREMENTS=$(cat <"$DATA_DIR/requirements.json" | jq -r '.['\""$1"\"']')
        change_requirements
        export REQUIREMENTS=$REQUIREMENTS
    else
        if [ -f "$MAGENTO_DIR/composer.lock" ]; then
            MAGENTO_VERSION=$(cat < "$MAGENTO_DIR/composer.lock" |
                jq -r '.packages | map(select(.name == "magento/product-community-edition"))[].version')
            print_warning "\nVersion detected: $MAGENTO_VERSION\n"
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
        default_file_magento=$(cat < "$DATA_DIR/default_files_magento.json" | jq -r '.["'"$filename_in_git"'"]')

        if [[ "$bind_path_exits" == true ]] || [[ $default_file_magento == true ]]; then
            continue
        fi

        if [ "$bind_paths" != "" ]; then
            bind_paths="$bind_paths\\      "
        fi

        bind_paths="$bind_paths- ${new_path}$suffix_bind_path\n"

    done <<<"${git_files}"

    print_warning "------ $file_to_edit ------\n"
    sed_in_file "s|# {FILES_IN_GIT}|$bind_paths|w /dev/stdout" "$file_to_edit"
    print_warning "--------------------\n"
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

        if [[ "$git_files" != "" ]]; then
            add_git_bind_paths_in_file "$git_files" "${DOCKER_COMPOSE_FILE_MAC}" ":delegated"
        else
            print_highlight " > Skipped. There are no files added in this repository\n"
        fi
    else
        print_highlight " > Skipped. This is not a git repository\n"
    fi
}

get_requirements "$@"
"$TASKS_DIR"/write_from_docker-compose_templates.sh
set_settings
save_properties
