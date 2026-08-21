#!/usr/bin/env bash

#
# Commands whose stdout is data or the output of a child process. Global flags are only
# honoured *before* the command name for these, so that a flag meant for the child
# process (e.g. `hm composer show --format=json`) is never swallowed by the router.
#
is_transparent_command() {
    case "$1" in
        exec | bash | magento | composer | npm | n98-magerun | grunt | \
        test-unit | test-integration | cloud | cloud-login | masquerade | \
        mysql | mysqldump | docker-compose | copy-to-container | copy-from-container | logs)
            return 0
            ;;
        *)
            return 1
            ;;
    esac
}

#
# Extract global options and leave the command and its own arguments in HM_ARGS
#
parse_global_options() {
    local arg command_seen=false transparent=false
    HM_ARGS=()

    for arg in "$@"; do
        if $command_seen && $transparent; then
            HM_ARGS+=("$arg")
            continue
        fi

        case "$arg" in
            --json)
                HM_OUTPUT_FORMAT="json"
                ;;
            --no-json)
                HM_OUTPUT_FORMAT="text"
                ;;
            --yes)
                HM_NON_INTERACTIVE="1"
                ;;
            --force)
                HM_FORCE="1"
                ;;
            --no-color)
                HM_NO_COLOR="1"
                ;;
            -*)
                HM_ARGS+=("$arg")
                ;;
            *)
                if ! $command_seen; then
                    command_seen=true
                    is_transparent_command "$arg" && transparent=true
                fi
                HM_ARGS+=("$arg")
                ;;
        esac
    done
}

#
# Resolve the output format: explicit flag > environment variable > TTY detection.
# Without a terminal the caller is a machine, so JSON is the sensible default.
#
resolve_output_format() {
    if [ -z "${HM_OUTPUT_FORMAT:-}" ]; then
        if [ -t 1 ]; then
            HM_OUTPUT_FORMAT="text"
        else
            HM_OUTPUT_FORMAT="json"
        fi
    fi

    export HM_OUTPUT_FORMAT
    export HM_NON_INTERACTIVE="${HM_NON_INTERACTIVE:-}"

    # --force applies to one invocation only: no variable and no configuration file can
    # turn the guardrails off for good
    export HM_FORCE="${HM_FORCE:-}"
    export HM_NO_COLOR="${HM_NO_COLOR:-}"

    # USE_DEFAULT_SETTINGS predates this contract and covers part of the setup/install
    # flow. Non-interactive mode is a superset of it, so it turns it on as well.
    if [ -n "$HM_NON_INTERACTIVE" ]; then
        export USE_DEFAULT_SETTINGS=true
    fi
}
