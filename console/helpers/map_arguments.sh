#!/usr/bin/env bash

#
# Change long options to short options and return new line mapped
#
parseToShortArguments() {
    local command_name="$1"
    shift # Remove command name of $@
    local arguments="$@"
    local file="$(cat "$DATA_DIR/command_descriptions.json")"

    # Get number of options for current command
    length=$(echo "$file" | jq -r '.["'$command_name'"].opts | length')
    
    for ((i = 0; i < length; i++)); do
        short_name=$(echo "$file" | jq -r '.["'$command_name'"].opts['$i'].name.short')
        long_name=$(echo "$file" | jq -r '.["'$command_name'"].opts['$i'].name.long')

        # Replace long by short
        arguments=$(echo "$arguments" |\
            sed s/--$long_name/-$short_name/ |\
            sed 's/-'$short_name'=/-'$short_name' /')
    done

   echo "$arguments"
}