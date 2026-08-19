#!/usr/bin/env bash

#
# Exit codes used across the CLI.
#
# A caller (script, CI job or AI agent) must be able to tell *why* a command failed
# without parsing its message. Codes 7-9 are reserved for upcoming changes.
#
# Commands run as child processes, so functions sourced in bin/run are not inherited.
# Pull in the printers when they are not already defined.
if ! declare -F is_json_output >/dev/null 2>&1; then
    source "${COMPONENTS_DIR}"/print_json.sh
fi

if ! declare -F print_error >/dev/null 2>&1; then
    source "${COMPONENTS_DIR}"/print_message.sh
fi

export HM_EXIT_OK=0        # success
export HM_EXIT_ERROR=1     # generic error
export HM_EXIT_USAGE=2     # invalid arguments or unknown command
export HM_EXIT_DOCKER=3    # docker daemon unavailable
export HM_EXIT_PROJECT=4   # not a configured Dockergento project
export HM_EXIT_SERVICE=5   # required service is not running
export HM_EXIT_BLOCKED=6   # refused on purpose: it would damage another environment

#
# Fail with a structured error in the active output format and exit with the given code:
#   hm_fail <code> <type> <message> [hint]
#
hm_fail() {
    local code="$1"
    local type="$2"
    local message="$3"
    local hint="${4:-}"

    if is_json_output; then
        json_error "${HM_COMMAND:-${COMMAND_BIN_NAME:-hm}}" "$code" "$type" "$message" "$hint"
    else
        # Errors belong on stderr in both formats, so that `hm cmd > file` never captures
        # a failure as if it were output.
        {
            print_error "\n$message\n"
            if [ -n "$hint" ]; then
                print_default "\n  "
                print_code "$hint"
                print_default "\n\n"
            fi
        } >&2
    fi

    exit "$code"
}
