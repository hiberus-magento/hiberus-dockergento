#!/usr/bin/env bash

source "$COMPONENTS_DIR"/print_message.sh

get_last_version() {
    echo $(jq -r 'keys | last' "$DATA_DIR"/equivalent_versions.json)
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
# Get equivalent version for docker settings
#
get_equivalent_version_if_exit() {
    equivalent_version=$("$HELPERS_DIR"/get_equivalent_version.sh "$1")
    if [[ "$equivalent_version" = "null" ]]; then
        print_warning "\nWe don't have support for the version $1 "
        print_info "\nPlease, write any version between all versions supported or press Ctrl - C to exit"
        "$COMMANDS_DIR"/compatibility.sh
        read -r MAGENTO_VERSION
        get_equivalent_version_if_exit "$MAGENTO_VERSION"
    fi

    export EQUIVALENT_VERSION=$equivalent_version
}

#
# Get magento version
#
get_magento_version() {
    DEFAULT_MAGENTO_VERSION="$(get_last_version)"

    if [[ $# -eq 0 || -z "$1" ]]; then
        read -rp "$(print_question "Magento version: " "$DEFAULT_MAGENTO_VERSION")" MAGENTO_VERSION

        if [[ $MAGENTO_VERSION == '' ]]; then
            MAGENTO_VERSION=$DEFAULT_MAGENTO_VERSION
        fi
    else
        MAGENTO_VERSION=$1
    fi

    get_equivalent_version_if_exit "$MAGENTO_VERSION"
    export MAGENTO_VERSION=$EQUIVALENT_VERSION   
}

#
# Get magento edition
#
get_magento_edition() {
    available_magento_editions=("community" "enterprise")

    if [[ $# -eq 0 || -z "$1" ]]; then
        label="Magento edition:"
        custom_select "$label" "${available_magento_editions[@]}"

        magento_edition=$REPLY
    else
        if [[ $1 == "community" || $1 == "enterprise" ]]; then
            magento_edition=$1
        else
            print_warning "Edition '$1' is not available.\n"
            get_magento_edition
        fi
    fi
    export MAGENTO_EDITION=$magento_edition
}

#
# Get base url
#
get_project_name() {
    local project_name=""

    if [[ $# > 0 ]]; then
        if [[ -n $1 ]]; then
            project_name=$(basename "$PWD" | awk '{print tolower($0)}')
        fi
    fi

    if [[ -z $project_name ]]; then
        suggested_name="$(basename "$PWD" | awk '{print tolower($0)}')"
        read -rp "$(print_question "Define project name " "$suggested_name")" COMPOSE_PROJECT_NAME

        if [[ $COMPOSE_PROJECT_NAME == '' ]]; then
            COMPOSE_PROJECT_NAME=$suggested_name
        fi
    else
        COMPOSE_PROJECT_NAME=$(echo $1 | awk '{print tolower($0)}')
    fi
    
    export COMPOSE_PROJECT_NAME=$COMPOSE_PROJECT_NAME
}

#
# Get base url
#
get_domain() {
    local project_name=""

    if [[ $# > 0 ]] && [[ -n $1 ]]; then
        project_name=$(basename "$PWD" | awk '{print tolower($0)}')
    fi

    if [[ -z $project_name ]]; then
        calculated_name=$(basename "$PWD" | awk '{print tolower($0)}')
        suggested_name=${COMPOSE_PROJECT_NAME:-$calculated_name}.local
        read -rp "$(print_question "Define domain " "$suggested_name")" domain

        if [[ -z $domain ]]; then
            domain="$suggested_name"
        fi
    else
        domain=$1
    fi

    # Transform domain name to lowercase
    domain=$(echo $domain | awk '{print tolower($0)}')
    export DOMAIN=$domain
}

#
# Prepare root path directory
#
process_magento_root_directory() {
    answer_magento_dir=$1

    # Remove last slash
    if [[ $answer_magento_dir == *"/" ]]; then
        length=${#answer_magento_dir}
        answer_magento_dir=${answer_magento_dir:0: length - 1}
    fi

    # Add dot and slash (./) before relative path
    if [[ $answer_magento_dir != './'* &&
        $answer_magento_dir != '/'* &&
        $answer_magento_dir != '.' &&
        -n $answer_magento_dir ]]; then
        answer_magento_dir="./$answer_magento_dir"
    fi

    # Default value if response is empty
    if [[ -z $answer_magento_dir ]]; then
        answer_magento_dir=${answer_magento_dir:-$MAGENTO_DIR}
    fi

    echo "$answer_magento_dir"
}

#
# Ask magento directory
#
get_magento_root_directory() {
    
    if [[ $# -gt 0 && -d $1 ]]; then
        MAGENTO_DIR=$(process_magento_root_directory "$1")
    else
        read -rep "$(print_question "Magento root dir " "$MAGENTO_DIR")" answer_magento_dir
        MAGENTO_DIR=$(process_magento_root_directory "$answer_magento_dir")
    fi

    export MAGENTO_DIR=$MAGENTO_DIR
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

prepare_options() {
    local opts=("$@")

    for i in "${!opts[@]}"; do
        opts[$i]=$(print_info "${opts[$i]}")
    done
    echo "length "${#opts[@]}
    echo "${opts[@]}"
}

#
# Select component
# Example:
#    options=("Import sql Dump" "Magento installation")
#    label="How do you want create database?"
#
#    custom_select "$label" "${options[@]}"
#    
#    if [[ $REPLY == SOME_REPONSE* ]]; then
#        echo "RESPONSE"
#        any_action
#    fi
#
custom_select() {
    local question="$1"
    shift
    local opts=("$@")

    for i in "${!opts[@]}"; do
        opts[$i]=$(print_table "${opts[$i]}")
    done
    COLUMNS=1
    PS3="$(print_default "Option: ")"
    print_question "\n✅ $question\n"
    select REPLY in "${opts[@]}"; do
        if [[ " ${opts[@]} " ==  *" $REPLY "* ]]; then
            # Remove color codes from selected option
            response=$(echo -e "$REPLY" | sed -r "s/\x1B\[([0-9]{1,2}(;[0-9]{1,2})?)?[m|K]//g")
            export REPLY=$response
            break
        fi
        print_warning "\nInvalid option, choose an option\n"
        print_question "\n✅ $question\n"
    done
}