#!/usr/bin/env bash

#
# Change long options to short options and return new line mapped
#
parseToShortArguments() {
    local command_name="$1"
    shift # Remove command name of $@
    local arguments=("$@")
    local file="$(cat "$DATA_DIR/command_descriptions.json")"
    local output=()

    # Get number of options for current command
    local length=$(echo "$file" | jq -r '.["'$command_name'"].opts | length')

    if [[ -n $* ]]; then
        # Itera sobre argumento de entrada
        for argument in "${arguments[@]}"; do

            # Map argument
            for ((i = 0; i < length; i++)); do
                local short_name=$(echo "$file" | jq -r '.["'$command_name'"].opts['$i'].name.short')
                local long_name=$(echo "$file" | jq -r '.["'$command_name'"].opts['$i'].name.long')
                local array_pos=${#output[@]}

                # Replace long by short
                processed=$(echo "$argument" |\

                    sed s/--$long_name/-$short_name/ |\
                    sed 's/-'$short_name'=/-'$short_name' /')
            done

            if [[ -z ${processed:=0} ]]; then
               # Save mapped argument
                for arg in "${processed[@]}"; do
                    output[$array_pos]="$arg"
                    array_pos=$[array_pos + 1]
                done  
            fi
            
        done
        export ARGS_PROCESSED="${output:-$}"
    else
        export ARGS_PROCESSED=false
    fi
}