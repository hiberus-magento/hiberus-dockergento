#!/usr/bin/env bash
set -uo pipefail

source "$COMPONENTS_DIR"/print_message.sh
source "$HELPERS_DIR"/exit_codes.sh

#
# The logs of the project, or of the services named.
#
# A wrapper over Compose, and deliberately a thin one: the output is somebody else's, it can be
# infinite with -f, and nothing here should get between the reader and it. What the wrapper adds
# is not having to know that Compose is down there, and a readable error for a service that does
# not exist.
#

services=()
options=()

# Anything that is not an option, and not the value of one, is a service name. The options that
# take a separate value have to be known by name: without that, `hm logs --tail 3` reads the 3 as
# a service and refuses to run.
expecting_value=false

for argument in "$@"; do
    if $expecting_value; then
        options[${#options[@]}]="$argument"
        expecting_value=false
        continue
    fi

    case "$argument" in
        --since | --until | --tail | -n | --index)
            options[${#options[@]}]="$argument"
            expecting_value=true
            ;;
        -*)
            options[${#options[@]}]="$argument"
            ;;
        *)
            services[${#services[@]}]="$argument"
            ;;
    esac
done

if $expecting_value; then
    hm_fail "$HM_EXIT_USAGE" "missing_value" \
        "${options[$(( ${#options[@]} - 1 ))]} needs a value" \
        "$COMMAND_BIN_NAME logs --tail 100"
fi

#
# Compose answers a wrong service name with an error about YAML files. The CLI knows which
# services the project has, so it can say what is available instead.
#
# Only when a service is named: `hm logs` on its own has nothing to validate, and paying a call
# to Compose to validate nothing would be a tax on the common case.
#
if [ "${#services[@]}" -gt 0 ]; then
    available=$($DOCKER_COMPOSE config --services 2>/dev/null | sort)

    for service in "${services[@]}"; do
        if ! printf '%s\n' "$available" | grep -qx "$service"; then
            hm_fail "$HM_EXIT_SERVICE" "unknown_service" \
                "This project has no service called '$service'" \
                "$COMMAND_BIN_NAME logs $(printf '%s' "$available" | tr '\n' ' ')"
        fi
    done
fi

exec $DOCKER_COMPOSE logs "${options[@]+"${options[@]}"}" "${services[@]+"${services[@]}"}"
