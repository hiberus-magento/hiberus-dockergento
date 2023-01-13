#!/usr/bin/env bash

function parseToShortArguments() {
    local command_name="$1"
    shift
    local arguments="$@"
    local file="$(cat "$DATA_DIR/command_descriptions.json")"

    LENGTH=$(echo "$file" | jq -r '.'$command_name'.opts | length')
    
    for ((i = 0; i < LENGTH; i++)); do
        short_name=$(echo "$file" | jq -r '.'$command_name'.opts['$i'].name.short')
        long_name=$(echo "$file" | jq -r '.'$command_name'.opts['$i'].name.long')

        arguments=$(echo $arguments |\
            sed s/-$long_name/$short_name/ |\
            sed 's/-'$short_name'=/-'$short_name' /')
    done

   echo "$arguments"
}