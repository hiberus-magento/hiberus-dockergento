#!/usr/bin/env bash
set -euo pipefail
set -a

#
# Change format of properties into project
#
refactor_old_version() {
    local custom_old_properties="$CUSTOM_PROPERTIES_DIR/properties"

    if [[ -f "$custom_old_properties" ]]; then
        cat "$custom_old_properties" | \
            jq -R -s 'split("\n") 
                | map(select(length > 0)) 
                | map(select(startswith("#") | not)) 
                | map(sub("^[[:space:]]+"; "")) 
                | map(split("=")) 
                | map({(.[0]): .[1:] 
                | join("=")}) 
                | add
            ' | sed 's/\\\"//g' > "$CUSTOM_PROPERTIES_DIR"/properties.json
        
        if [[ $? == 0 ]]; then
            rm -f "$custom_old_properties"
        fi
    fi
}

#
# Load colors 
#
load_colors() {
    BLUE="\033[0;34m"
    GREEN="\033[0;32m"
    CYAN="\033[0;36m"
    RED="\033[0;31m"
    PURPLE="\033[0;35m"
    BROWN="\033[0;33m"
    WHITE="\033[1;37m"
    YELLOW='\033[0;33m'
    COLOR_RESET="\033[0m"
}

#
# Load properties (merge between default and custom properties)
#
load_properties() {
    # If exist project properties, use it in 
    local files=""
    if [[ -f "$CUSTOM_PROPERTIES_DIR"/properties.json ]]; then
        files="$CUSTOM_PROPERTIES_DIR/properties.json"
    fi

    # Prepare string in sh format for executing 
    local properties=$(jq -r '
        to_entries[]
        | .key + "=\"" + .value + "\""
    ' "$DATA_DIR"/properties.json $files)

    # Set properties
    eval $properties
}

refactor_old_version
load_colors
load_properties
set +a
