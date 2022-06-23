#!/bin/bash
set -euo pipefail

#
# Print al all commands info (native and custom)
#
print_commands_info() {
    local command_path="$COMMANDS_DIR"
    local file_content="$FILE"
    local command_color="$GREEN"
    local title="Command list"
    local underline="------------\n"

    if [ $# -gt 0 ] && [ "$1" == 'custom' ]; then
        command_path="$CUSTOM_COMMANDS_DIR"
        title="Custom command list"
        underline="-------------------\n"
        command_color="$PURPLE"

        if [ -f "$CUSTOM_COMMANDS_DIR/command_descriptions.json" ]; then
            file_content=$(cat "$CUSTOM_COMMANDS_DIR/command_descriptions.json")
        else
            file_content="{}"
        fi
    fi

    if [ ! -d "$command_path" ]; then
        exit 0
    fi

    local FILES
    FILES=$(find "$command_path" -name '*.sh' | wc -l)

    if [ "$FILES" -gt 0 ]; then
        echo -e "${command_color}\n${title}\n${underline}${COLOR_RESET}"

        for script in "$command_path"/*.sh; do
            COMMAND_BASENAME=$(basename "$script")
            COMMAND_NAME=${COMMAND_BASENAME%.sh}
            COMMAND_DESC_PROPERTY=$(echo "$file_content" | jq -r 'if ."'"$COMMAND_NAME"'".description then ."'"$COMMAND_NAME"'".description else "" end')
            printf "   $command_color%-20s$COLOR_RESET %s\n" "$COMMAND_NAME" "$COMMAND_DESC_PROPERTY"
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
    echo "${commands_output}"
    echo "${commands_output_all}"
}

#
# Print options data array
#
print_opts() {
    local LENGTH
    LENGTH=$(echo "$FILE" | jq -r '."'"$1"'".opts | length')

    if [[ $LENGTH -gt 0 ]]; then
        print_info "Options:"
    fi

    for ((i = 0; i < LENGTH; i++)); do
        name=$(echo "$FILE" | jq -r '."'"$1"'".opts['"$i"'].name')
        description=$(echo "$FILE" | jq -r '."'"$1"'".opts['"$i"'].description')
        printf "   ${WHITE}%-20s${COLOR_RESET}%s\n" "${name}" "${description}"
    done

    if [[ $LENGTH -gt 0 ]]; then
        printf "\n"
    fi
}

#
# Print arguments data array
#
print_args() {
    local LENGTH
    LENGTH=$(echo "$FILE" | jq -r '."'"$1"'".args | length')

    if [[ $LENGTH -gt 0 ]]; then
        print_info "Arguments:"
    fi

    for ((i = 0; i < LENGTH; i++)); do
        name=$(echo "$FILE" | jq -r '."'"$1"'".args['"$i"'].name')
        description=$(echo "$FILE" | jq -r '."'"$1"'".args['"$i"'].description')
        printf "   ${WHITE}%-20s${COLOR_RESET}%s\n" "${name}" "${description}"
    done

    if [[ $LENGTH -gt 0 ]]; then
        printf "\n"
    fi
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

        params=$(echo "$FILE" | jq -r '."'"$command_name"'" | if length > 0 then keys[] else false end')

        if [[ $params ]]; then
            # Prinf usage seccion
            if [[ "$params" == *"usage"* ]]; then
                local usage
                usage=$(echo "$FILE" | jq -r '."'"$command_name"'".usage')
                print_info "Usage:"
                printf "%3s${COMMAND_BIN_NAME} ${usage}\n\n"

            fi

            # Prinf example seccion
            if [[ "$params" == *"example"* ]]; then
                local example
                example=$(echo "$FILE" | jq -r '."'"$command_name"'".example')
                print_info "Example:"
                printf "%3s${COMMAND_BIN_NAME} ${example}\n\n"
            fi

            # Prinf description seccion
            if [[ "$params" == *"description"* ]]; then
                local description
                description=$(echo "$FILE" | jq -r '."'"$command_name"'".description')
                print_info "Description:"
                printf "%3s${description}\n\n"
            fi

            # Prinf options seccion
            if [[ "$params" == *"opts"* ]]; then
                print_opts "$command_name"
            fi

            # Prinf options seccion
            if [[ "$params" == *"args"* ]]; then
                print_args "$command_name"
            fi
        fi
    fi
}

FILE="$(cat "${DATA_DIR}/command_descriptions.json")"

# shellcheck source=/dev/null
source "${COMPONENTS_DIR}"/print_message.sh
# Show copy
# shellcheck source=/dev/null
source "${TASKS_DIR}/copyright.sh"

usage "$@"
