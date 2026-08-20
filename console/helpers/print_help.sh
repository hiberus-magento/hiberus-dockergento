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

    # Parameter expansion instead of `find -exec basename`, which spawned one process per
    # command file
    local names="" script basename
    for script in "$command_path"/*.sh; do
        [ -f "$script" ] || continue
        basename="${script##*/}"
        names+="${basename%.sh}"$'\n'
    done

    # Glob order, not name order: the glob sorts by file name, so `cloud-login.sh` comes
    # before `cloud.sh`. Sorting by command name would reorder those two and change output
    # that has been stable for years, which is a separate decision from making this fast.

    if [ -z "$names" ]; then
        return 0
    fi

    echo -e "$command_color\n$title\n$underline$COLOR_RESET"

    # One jq for the whole table instead of three per command. Listing 45 commands used to
    # spawn 143 jq processes against the same 13 KB file, which was 5 of the 5.7 seconds
    # `hm --help` took.
    #
    # Fields are separated by the unit separator, not a tab: `read` treats tabs as
    # whitespace and collapses consecutive ones, which would shift every column of a
    # command whose description is empty.
    local rows
    rows=$(printf '%s' "$file_content" | jq -r --arg names "$names" '
        . as $descriptions
        | $names
        | split("\n")
        | map(select(length > 0))
        | .[]
        | [
            .,
            ($descriptions[.].description // ""),
            (($descriptions[.].mac // false) | tostring)
          ]
        | join("\u001f")')

    local name description mac_only
    while IFS=$'\037' read -r name description mac_only; do
        [ -z "$name" ] && continue

        if [[ "$MACHINE" == "mac" || "$mac_only" != "true" ]]; then
            printf "\t$command_color%-20s$COLOR_RESET %s\n" "$name" "$description"
        fi
    done <<< "$rows"

    printf "\n\n"
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
    print_global_options
}

#
# Print the options accepted by every command
#
print_global_options() {
    local rows name description

    # One jq for the whole block: the previous version spent two processes per option plus
    # two more, ten in total, to print three lines
    rows=$(jq -r '
        ._global.opts // []
        | .[]
        | [(.name.long // ""), (.description // "")]
        | join("\u001f")' < "$DATA_DIR"/command_descriptions.json)

    if [ -z "$rows" ]; then
        return 0
    fi

    echo -e "$GREEN\nGlobal options\n--------------\n$COLOR_RESET"

    while IFS=$'\037' read -r name description; do
        [ -z "$name" ] && continue
        printf "\t$BROWN%-20s$COLOR_RESET%s\n" "--$name" " $description"
    done <<< "$rows"

    printf "\n"
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
