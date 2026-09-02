#!/usr/bin/env bash
set -euo pipefail
set -a

#
# Change format of properties into project
#
refactor_old_version() {
    local custom_old_properties="$CUSTOM_PROPERTIES_DIR/properties"

    if [[ -f "$custom_old_properties" ]]; then
        #
        # Merged over whatever properties.json already holds, and not written over it.
        #
        # A real migration finds no properties.json and the merge is the conversion. What the
        # merge protects against is the other case: anything that writes a line into the old file
        # by accident would otherwise wipe the project's identity on the next command.
        #
        local existing="{}"
        if [ -f "$CUSTOM_PROPERTIES_DIR/properties.json" ]; then
            existing=$(cat "$CUSTOM_PROPERTIES_DIR/properties.json")
        fi

        cat "$custom_old_properties" | \
            jq -R -s --argjson existing "$existing" 'split("\n") 
                | map(select(length > 0)) 
                | map(select(startswith("#") | not)) 
                | map(sub("^[[:space:]]+"; "")) 
                | map(split("=")) 
                | map({(.[0]): .[1:] 
                | join("=")}) 
                | add
                | $existing + .
            ' | sed 's/\\\"//g' > "$CUSTOM_PROPERTIES_DIR"/properties.json
        
        if [[ $? == 0 ]]; then
            rm -f "$custom_old_properties"
        fi
    fi
}

#
# Should the output be coloured?
#
# One decision point, with an explicit precedence: what the user asked for beats the
# environment, and forcing beats autodetection but never beats an explicit refusal.
#
#   1. --no-color / HM_NO_COLOR   the user said no
#   2. NO_COLOR                    the ecosystem standard (no-color.org)
#   3. TERM=dumb or empty          the terminal cannot render it
#   4. FORCE_COLOR / CLICOLOR_FORCE  colour even when piped, for CI logs that render ANSI
#   5. stdout is not a terminal    nobody is looking
#
should_use_color() {
    [ -n "${HM_NO_COLOR:-}" ] && return 1
    [ -n "${NO_COLOR:-}" ] && return 1

    case "${TERM:-}" in
        dumb | "") return 1 ;;
    esac

    [ -n "${FORCE_COLOR:-}" ] && return 0
    [ -n "${CLICOLOR_FORCE:-}" ] && return 0

    [ -t 1 ] || return 1

    return 0
}

#
# Load colors
#
# When colour is off every variable is empty, so no other file has to know: the printers
# interpolate them exactly the same way.
#
load_colors() {
    if ! should_use_color; then
        BLUE=""
        GREEN=""
        CYAN=""
        RED=""
        PURPLE=""
        BROWN=""
        WHITE=""
        YELLOW=""
        BOLD=""
        COLOR_RESET=""
        return 0
    fi

    BLUE="\033[0;34m"
    GREEN="\033[0;32m"
    CYAN="\033[0;36m"
    RED="\033[0;31m"
    PURPLE="\033[0;35m"
    BROWN="\033[0;33m"
    WHITE="\033[1;37m"
    YELLOW='\033[0;33m'
    # Bold with no colour: a heading is structure, not content, so it is distinguished by
    # weight instead of competing in hue with the command names. It also uses whatever
    # foreground colour the user has configured, so it stays readable on a light theme,
    # which a fixed dark blue does not.
    BOLD="\033[1m"
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
        # Prepare string in sh format for executing 
        local properties=$(jq -r '
        to_entries[]
        | .key + "=\"" + .value + "\""
    ' "$DATA_DIR"/properties.json "$files")
    else
        # Prepare string in sh format for executing 
    	local properties=$(jq -r '
        to_entries[]
        | .key + "=\"" + .value + "\""
    ' "$DATA_DIR"/properties.json)
    fi
    

    # Set properties
    eval $properties
}

refactor_old_version
load_colors
load_properties
set +a
