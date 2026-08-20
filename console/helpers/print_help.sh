#!/bin/bash
set -euo pipefail

command_info=""

# Help is the output of this command, not decoration around it: all of it belongs on
# stdout. Without this, JSON mode would route the headings to stderr and `hm --help > file`
# would lose them. There is no JSON rendering of help text.
HM_OUTPUT_FORMAT="text"

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
    local custom=false

    if [ $# -gt 0 ] && [ "$1" == 'custom' ]; then
        command_path="$CUSTOM_COMMANDS_DIR"
        command_color="$PURPLE"
        custom=true

        if [ -f "$command_path/command_descriptions.json" ]; then
            file_content=$(cat "$command_path/command_descriptions.json")
        fi
    fi

    if [ ! -d "$command_path" ]; then
        return 0
    fi

    # Parameter expansion instead of `find -exec basename`, which spawned one process per
    # command file. Glob order, not name order: the glob sorts by file name, so
    # `cloud-login.sh` comes before `cloud.sh`, and that has been the order for years.
    local names="" script basename
    for script in "$command_path"/*.sh; do
        [ -f "$script" ] || continue
        basename="${script##*/}"
        names+="${basename%.sh}"$'\n'
    done

    if [ -z "$names" ]; then
        return 0
    fi

    # Still one jq for the whole table, now carrying the group as well. Grouping by asking
    # jq once per group would undo the work that took `hm --help` from 5.7s to 0.4s.
    #
    # Fields are separated by the unit separator, not a tab: `read` treats tabs as
    # whitespace and collapses consecutive ones, which would shift every column of a
    # command with an empty description.
    local rows
    rows=$(printf '%s' "$file_content" | jq -r --arg names "$names" '
        . as $descriptions
        | (($descriptions._groups // []) | map(.id)) as $order
        | (($descriptions._groups // []) | map({(.id): .title}) | add // {}) as $titles
        | $names
        | split("\n")
        | map(select(length > 0))
        | to_entries
        | map({
            position: .key,
            name: .value,
            description: ($descriptions[.value].description // ""),
            mac: (($descriptions[.value].mac // false) | tostring),
            group: ($descriptions[.value].group // "other")
          })
        | map(.group as $group | . + {rank: (($order | index($group)) // 999)})
        | sort_by([.rank, .position])
        | .[]
        | [.group, ($titles[.group] // "Other"), .name, .description, .mac]
        | join("\u001f")')

    local group title name description mac_only current_group=""
    while IFS=$'\037' read -r group title name description mac_only; do
        [ -z "$name" ] && continue

        if [[ "$MACHINE" != "mac" && "$mac_only" == "true" ]]; then
            continue
        fi

        if [ "$group" != "$current_group" ]; then
            current_group="$group"

            if $custom; then
                title="Custom commands"
            fi

            printf "\n"
            print_heading "$title\n"
        fi

        printf "  $command_color%-22s$COLOR_RESET%s\n" "$name" "$description"
    done <<< "$rows"
}

#
# Print native commands and custom commands info
#
print_all_commands_help_info() {
    print_logo
    print_usage_line
    print_commands_info
    print_commands_info "custom"
    print_examples
    print_global_options
    print_help_footer
}

#
# How the tool is invoked
#
print_usage_line() {
    print_heading "Usage: "
    print_default "$COMMAND_BIN_NAME <command> [options]\n"
}

#
# The tasks people actually do, declared in the data file rather than hardcoded here:
# what counts as common is a team decision and it will change
#
print_examples() {
    local rows command description

    rows=$(jq -r '
        ._examples // []
        | .[]
        | [(.command // ""), (.description // "")]
        | join("\u001f")' < "$DATA_DIR"/command_descriptions.json)

    if [ -z "$rows" ]; then
        return 0
    fi

    printf "\n"
    print_heading "Examples\n"

    while IFS=$'\037' read -r command description; do
        [ -z "$command" ] && continue
        printf "  $GREEN%-30s$COLOR_RESET%s\n" "$command" "$description"
    done <<< "$rows"
}

#
# Where to go next
#
print_help_footer() {
    print_default "Run "
    print_code "$COMMAND_BIN_NAME <command> --help"
    print_default " for details on a single command.\n\n"
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

    printf "\n"
    print_heading "Global options\n"

    while IFS=$'\037' read -r name description; do
        [ -z "$name" ] && continue
        printf "  $BROWN%-22s$COLOR_RESET%s\n" "--$name" "$description"
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
