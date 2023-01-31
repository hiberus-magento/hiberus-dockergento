#!/usr/bin/env bash

in_array() {
    set -x
    local match="$1"
    local array="$2"
    shift

    for command in $array; do
        [[ "$command" == "$match" ]] && return 0;
    done
    
    return 1
    set +x
}
