#!/usr/bin/env bash
set -euo pipefail

source "$COMPONENTS_DIR"/print_message.sh
source "$COMPONENTS_DIR"/print_json.sh
source "$HELPERS_DIR"/exit_codes.sh
source "$TASKS_DIR"/collect_project_info.sh

#
# Open one of the project's addresses in the browser.
#
# The addresses come from the same place `hm describe` reports them, so there is one definition
# of what this project's URL is. This command does not start anything: opening a browser against
# a stopped environment is not what was asked for, and starting it by surprise is worse.
#

target="base"
chosen=""

while [ "$#" -gt 0 ]; do
    case "$1" in
        --admin | --mail | --mailhog | --mailpit | --rabbitmq | --search | --base)
            # Two destinations in one invocation is a typo, not a request for two tabs
            if [ -n "$chosen" ]; then
                hm_fail "$HM_EXIT_USAGE" "conflicting_options" \
                    "Pick one destination: $chosen and $1 cannot both be opened" \
                    "$COMMAND_BIN_NAME launch --admin"
            fi
            chosen="$1"
            target="${1#--}"

            # The mailbox is one destination whichever catcher is behind it
            case "$target" in
                mailhog | mailpit) target="mail" ;;
            esac

            shift
            ;;
        *)
            hm_fail "$HM_EXIT_USAGE" "invalid_argument" \
                "Unknown option: $1" \
                "$COMMAND_BIN_NAME launch --help"
            ;;
    esac
done

data=$(collect_project_info false)
url=$(printf '%s' "$data" | jq -r --arg target "$target" '.project.urls[$target] // ""')

if [ -z "$url" ]; then
    case "$target" in
        base | admin)
            hm_fail "$HM_EXIT_PROJECT" "no_domain" \
                "This project has no domain configured, so it has no address to open" \
                "$COMMAND_BIN_NAME setup --domain=project.local"
            ;;
        *)
            hm_fail "$HM_EXIT_SERVICE" "no_address" \
                "The $target service publishes no port, so it has no address to open" \
                "$COMMAND_BIN_NAME describe"
            ;;
    esac
fi

#
# Where there is nowhere to open, the address itself is the useful answer: that is the case in a
# script, over SSH, and on a machine with no desktop.
#
if is_json_output; then
    json_success "launch" "$(jq -n --arg target "$target" --arg url "$url" \
        '{target: $target, url: $url, opened: false}')"
    exit 0
fi

if ! opener=$(command -v open || command -v xdg-open); then
    print_warning "No browser opener on this machine. The address is:\n"
    printf '%s\n' "$url"
    exit 0
fi

"$opener" "$url" >/dev/null 2>&1 || hm_fail "$HM_EXIT_ERROR" "open_failed" \
    "Could not open $url" \
    "$COMMAND_BIN_NAME launch --json"

print_info "Opened "
print_link "$url\n"
