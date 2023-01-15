#!/usr/bin/env bash
set -euo pipefail


function execute_process_help() {
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