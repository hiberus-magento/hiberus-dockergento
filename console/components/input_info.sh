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
    local equivalent_version=$("$HELPERS_DIR"/get_equivalent_version.sh "$1")

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
    local default_magento_version="$(get_last_version)"
    local magento_version

    if [[ $# -eq 0 || -z "$1" ]]; then
        custom_question "Magento version:" "$default_magento_version"
        magento_version=${REPLY:-$default_magento_version}
    else
        magento_version=$1
    fi

    get_equivalent_version_if_exit "$magento_version"
    export MAGENTO_VERSION=$EQUIVALENT_VERSION   
}

#
# Get magento edition
#
get_magento_edition() {
    local available_magento_editions=("community" "enterprise")

    if [ $# -gt 0 ] && [ -n "$1" ]; then
        if [[ $1 == "community" || $1 == "enterprise" ]]; then
            magento_edition=$1
        else
            print_warning "Edition '$1' is not available.\n"
            get_magento_edition
        fi
    else
        label="Magento edition:"
        custom_select "$label" "${available_magento_editions[@]}"

        magento_edition=$REPLY
    fi

    export MAGENTO_EDITION=$magento_edition
}

#
# Get base url
#
get_project_name() {
    local project_name=""
    local suggested_name compose_project_name
    
    if [ $# -gt 0 ] && [ -n "$1" ]; then
        project_name=$(basename "$PWD" | awk '{print tolower($0)}')
    fi

    if [[ -z $project_name ]]; then
        suggested_name="$(basename "$PWD" | awk '{print tolower($0)}')"
        custom_question "Define project name" "$suggested_name"
        compose_project_name=${REPLY:-$suggested_name}
    else
        compose_project_name=$(echo "$1" | awk '{print tolower($0)}')
    fi
    
    export COMPOSE_PROJECT_NAME=$compose_project_name
}

#
# Get base url
#
get_domain() {
    local project_name=""
    local calculated_name suggested_name domain

    if [ $# -gt 0 ] && [ -n "$1" ]; then
        project_name=$(basename "$PWD" | awk '{print tolower($0)}')
    fi

    if [[ -z $project_name ]]; then
        calculated_name=$(basename "$PWD" | awk '{print tolower($0)}')
        suggested_name=${COMPOSE_PROJECT_NAME:-$calculated_name}.local
        custom_question "Define domain" "$suggested_name"
        domain=${REPLY:-$suggested_name}
    else
        domain=$1
    fi

    # Transform domain name to lowercase
    domain=$(echo "$domain" | awk '{print tolower($0)}')
    export DOMAIN=$domain
}

#
# Prepare root path directory
#
process_magento_root_directory() {
    local length answer_magento_dir
    answer_magento_dir="$1"

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
    local magento_dir

    if [ $# -gt 0 ] && [ -d "$1" ]; then
        magento_dir=$(process_magento_root_directory "$1")
    else
        custom_question "Magento root dir" "${magento_dir:-$MAGENTO_DIR}"
        magento_dir=$(process_magento_root_directory "$REPLY")
    fi

    export MAGENTO_DIR=$magento_dir
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
# Input question with specific format for question
#
_custom_read() {
    local question="$1"
    local argument="${2-}"

    read -rp "$(print_question "$question " "$argument")"
}

#
# Input confirm with specific format for question
#
confirm() {
    _custom_read "$@"
}

#
# Input question with specific format for question
#
custom_question() {
    clear
    _custom_read "$@"
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
    clear
    print_question "✅ $question\n"

    COLUMNS=1
    PS3="$(print_default "Option: ")"
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