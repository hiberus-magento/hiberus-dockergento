#!/bin/bash
set -euo pipefail

source "$COMPONENTS_DIR"/print_message.sh

#
# Get usege with options and arguments
#
get_usage() {
    local command_name=$1
    local custom_command_descriptions=""
    local command_info=$(jq -r '.["'$command_name'"]' "$DATA_DIR/command_descriptions.json")

    if [[ $command_info == null && -f "$CUSTOM_COMMANDS_DIR/command_descriptions.json" ]]; then
        command_info=$(jq -r 'try (.["'$command_name'"]) catch false' "$CUSTOM_COMMANDS_DIR/command_descriptions.json")
    fi

    local usage_property=$(echo "$command_info" | jq -r '.usage')
    local params=$(echo "$command_info" | jq -r '. | if length > 0 then keys[] else false end')

    if [[ $usage_property != null ]]; then
        print_info "Usage: "
        print_code "$COMMAND_BIN_NAME $usage_property\n"
    else
        if [[ "$params" == *"args"* || "$params" == *"opts"* ]]; then
            # Compose options concatenation string
            local opts=$(echo "$command_info" | jq -r 'if .opts 
                then .opts 
                    | map("[-" + .name.short + "|--" + .name.long + "]")
                    | join(" ") + " "
                else "" end')
            # Compose arguments concatenation string
            local args=$(echo "$command_info" | jq -r 'if .args
                then .args
                    | map(if .multiple
                        then "<" + .name + "1>...<" + .name + "N>"
                        else "<" + .name + ">" end) 
                    | join(" ") 
                else "" end')
            # Compose complete command 
            print_info "Usage: "
            print_code "$COMMAND_BIN_NAME $command_name $opts$args\n"
        fi
    fi
}