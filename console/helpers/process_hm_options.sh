#!/usr/bin/env bash
set -euo pipefail

#
# Process help option
#
process_help() {
    if [ "$#" -eq "0" ]; then
        set -- -h
    fi

    # If there are arguments and the first argument is --help
    if [ "$#" -gt 0 ]; then
        # List of commands
        if [ "$1" = "--help" ] || [ "$1" = "-h" ]; then
            "$HELPERS_DIR"/print_help.sh
            exit 0
        # Description specific command
        elif [ "$#" -gt 1 ] &&
            { [ "$2" = "--help" ] || [ "$2" = "-h" ]; }; then
            "$HELPERS_DIR"/print_help.sh "$1"
            exit 0
        fi
    fi
}

#
# Return hm version
#
process_version() {
    if [ "$#" -eq "0" ]; then
        set -- -v
    fi

    if [ "$#" -gt 0 ]; then
        # List of commands
        if [ "$1" = "--version" ] || [ "$1" = "-v" ]; then
            pushd ${COMMANDS_DIR} >/dev/null 2>&1
            IFS=$'\n'
            version=($(git describe --tags --abbrev=0))
            echo "$COMMAND_BIN_NAME v$version"
            exit 0
        fi
    fi
}

execute_process_hm_options() {
    process_help "$@"
    process_version "$@"
}