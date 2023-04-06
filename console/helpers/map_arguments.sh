#!/usr/bin/env bash

#
# Change long options to short options and return new line mapped
#
parseToShortArguments() {
    local command_name arguments file output short_name long_name array_pos length processed set_output
    command_name="$1"
    shift # Remove command name of $@
    arguments=("$@")
    file="$(cat "$DATA_DIR/command_descriptions.json")"
    output=()
    # Get number of options for current command
    length=$(echo "$file" | jq -r '.["'"$command_name"'"].opts | length')

    if [[ -n $* ]]; then
        # Itera sobre argumento de entrada
        for argument in "${arguments[@]}"; do
            # Map argument
            for ((i = 0; i < length; i++)); do
                short_name=$(echo "$file" | jq -r '.["'"$command_name"'"].opts['$i'].name.short')
                long_name=$(echo "$file" | jq -r '.["'"$command_name"'"].opts['$i'].name.long')
                array_pos=${#output[@]}
                
                # Replace long by short
                processed=$(echo "$argument" |\
                    sed 's/--'"$long_name"'/-'"$short_name"'/' |\
                    sed 's/-'"$short_name"'=/-'"$short_name"' /')
            done
            if [[ -n ${processed:=""} ]]; then
               # Save mapped argument
                for arg in "${processed[@]}"; do
                    output[array_pos]="$arg"
                    array_pos=$[array_pos + 1]
                done  
            fi
        done
        if [[ ${#output[@]} -gt 0 ]]; then
            export MAPPED=true
        else
            export MAPPED=false
        fi
        export ARGS_PROCESSED="${output:-$}"
    else
        export MAPPED=false
    fi
}