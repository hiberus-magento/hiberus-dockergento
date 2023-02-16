#!/usr/bin/env bash

#
# Check if exist in array
#
in_array() {
    local match="$1"
    local array="$2"
    shift

    for command in $array; do
        [[ "$command" == "$match" ]] && return 0;
    done
    
    return 1
}
