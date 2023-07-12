#!/bin/bash
set -euo pipefail

command_info=""

source "$COMPONENTS_DIR"/print_message.sh
source "$TASKS_DIR"/copyright.sh
source "$HELPERS_DIR"/print_usage.sh

#
# Print al all commands info (native and custom)
#
print_commands_info() {
    local command_path="$COMMANDS_DIR"
    local file_content="$(cat "$DATA_DIR"/command_descriptions.json)"
    local command_color="$GREEN"
    local title="Command list"
    local underline="------------\n"

    if [ $# -gt 0 ] && [ "$1" == 'custom' ]; then
        command_path="$CUSTOM_COMMANDS_DIR"
        title="Custom command list"
        underline="-------------------\n"
        command_color="$PURPLE"

        if [ -f "$command_path/command_descriptions.json" ]; then
            file_content=$(cat "$command_path/command_descriptions.json")
        fi
    fi

    if [ ! -d "$command_path" ]; then
        exit 0
    fi

    local files=$(find "$command_path" -name '*.sh' | wc -l)

    if [ "$files" -gt 0 ]; then
        echo -e "$command_color\n$title\n$underline$COLOR_RESET"

        for script in "$command_path"/*.sh; do
            command_basename=$(basename "$script")
            command_name=${command_basename%.sh}
            command_information=$(echo "$file_content" | jq -r '.["'$command_name'"]')
            command_desc_property=$(echo "$command_information" | jq -r 'if .description then .description else "" end')
            mac=$(echo "$command_information" | jq -r '.mac')
            
            if [[ "$MACHINE" == "mac" || $mac != true ]]; then
            

                printf "\t$command_color%-20s$COLOR_RESET %s\n" "$command_name" "$command_desc_property"
            fi
        done

        printf "\n\n"
    fi
}

#
# Print native commands and custom commands info
#
print_all_commands_help_info() {
    local commands_output
    local commands_output_all
    commands_output=$(print_commands_info)
    commands_output_all=$(print_commands_info "custom")
    echo "$commands_output"
    echo "$commands_output_all"
}

#
# Print options data array
#
print_opts() {
    local command_opts=$(echo "$command_info" | jq -r '.opts')
    local length=$(echo "$command_opts" | jq -r 'length')

    if [[ $length -gt 0 ]]; then
        print_info "Options:\n"
    fi

    for ((i = 0; i < length; i++)); do

        name=$(echo "$command_opts" | jq -r '.['$i'].name | "-" + .short + "|--" + .long')
        description=$(echo "$command_opts" | jq -r '.['$i'].description')
        printf "   $BROWN%-16s$COLOR_RESET%s\n" "[$name]" " $description"
    done
}

#
# Print arguments data array
#
print_args() {
    local command_args=$(echo "$command_info" | jq -r '.args')
    local length=$(echo "$command_args" | jq -r 'length')

    if [[ $length -gt 0 ]]; then
        print_info "Arguments:\n"
    fi

    for ((i = 0; i < length; i++)); do
        name=$(echo "$command_args" | jq -r '.['$i'].name')
        description=$(echo "$command_args" | jq -r '.['$i'].description')
        printf "   $BROWN%-16s$COLOR_RESET%s\n" "<$name>" "$description"
    done
}

#
# Define usage
#
usage() {
    if [ $# == 0 ]; then
        print_all_commands_help_info
    else
        local params
        local command_name=$1

        command_info=$(jq -r '.["'$command_name'"]' < "$DATA_DIR/command_descriptions.json")
        if [[ $command_info == null && -f "$CUSTOM_COMMANDS_DIR/command_descriptions.json" ]]; then
            command_info=$(jq -r '.["'$command_name'"]' "$CUSTOM_COMMANDS_DIR/command_descriptions.json")
        fi
        params=$(echo "$command_info" | jq -r '. | if length > 0 then keys[] else false end')

        if [[ $params ]]; then
            # Print usage section
            get_usage "$command_name"

            # Print example section
            if [[ "$params" == *"example"* ]]; then
                local example
                example=$(echo "$command_info" | jq -r '.example')
                print_info "Example: "
                print_code "$COMMAND_BIN_NAME $example\n"
            fi

            # Print description section
            if [[ "$params" == *"description"* ]]; then
                local description
                description=$(echo "$command_info" | jq -r '.description')
                print_info "Description:"
                printf "%1s$description\n"
            fi

            # Print options section
            if [[ "$params" == *"opts"* ]]; then
                print_opts "$command_name"
            fi

            # Print options section
            if [[ "$params" == *"args"* ]]; then
                print_args "$command_name"
            fi
            printf "\n"
        fi
    fi
}

usage "$@"
