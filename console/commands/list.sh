#!/usr/bin/env bash
set -euo pipefail

source "$COMPONENTS_DIR"/print_message.sh
source "$COMPONENTS_DIR"/print_json.sh
source "$HELPERS_DIR"/exit_codes.sh
source "$TASKS_DIR"/collect_environments.sh

print_text() {
    local environments="$1"
    local count
    count=$(echo "$environments" | jq -r 'length')

    if [ "$count" == "0" ]; then
        print_info "\nNo $COMMAND_TOOLNAME environments found on this machine.\n\n"
        print_default "  Create one with "
        print_code "$COMMAND_BIN_NAME setup"
        print_default " inside a Magento project.\n\n"
        return 0
    fi

    printf "\n"
    printf "   %-26s %-9s %-7s %-22s %s\n" "PROJECT" "STATUS" "SERVICE" "BRANCH" "ROOT"
    printf "   %-26s %-9s %-7s %-22s %s\n" "--------------------------" "---------" "-------" "----------------------" "----"

    echo "$environments" | jq -r '.[] |
        "\(.name)|\(.status)|\(.containers.running)/\(.containers.total)|\(.branch)|\(.root)|\(.orphan)|\(.has_metadata)|\(.worktree)"' |
        while IFS='|' read -r name status services branch root orphan metadata worktree; do
            local_name="$name"
            [ -n "$worktree" ] && local_name="$name (wt: $worktree)"

            marker=""
            [ "$orphan" == "true" ] && marker="  ⚠ orphan"
            [ "$metadata" == "false" ] && marker="$marker  (no metadata)"

            printf "   %-26s %-9s %-7s %-22s %s%s\n" \
                "$local_name" "$status" "$services" "${branch:--}" "${root:--}" "$marker"
        done

    printf "\n"
}

list_execute() {
    local environments
    environments=$(collect_environments)

    if is_json_output; then
        json_success "list" "$(jq -n --argjson environments "$environments" \
            '{environments: $environments, count: ($environments | length)}')"
    else
        print_text "$environments"
    fi
}

if [ "$#" -gt 0 ]; then
    hm_fail "$HM_EXIT_USAGE" "invalid_argument" \
        "Unknown option: $1" \
        "$COMMAND_BIN_NAME list --help"
fi

list_execute
