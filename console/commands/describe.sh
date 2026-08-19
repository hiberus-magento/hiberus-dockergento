#!/usr/bin/env bash
set -euo pipefail

source "$COMPONENTS_DIR"/print_message.sh
source "$COMPONENTS_DIR"/print_json.sh
source "$HELPERS_DIR"/exit_codes.sh
source "$TASKS_DIR"/collect_project_info.sh

with_secrets=false

#
# Readable output: what people look up most often goes first
#
print_text() {
    local data="$1"
    local label value

    print_header "$(echo "$data" | jq -r '.project.name')"

    printf "\n"
    print_info "Environment\n"
    for label in status domain; do
        value=$(echo "$data" | jq -r ".project.$label // \"\"")
        printf "   %-14s %s\n" "$label" "${value:--}"
    done
    printf "   %-14s %s\n" "worktree" "$(echo "$data" | jq -r '.project.worktree // "" | if . == "" then "-" else . end')"
    printf "   %-14s %s\n" "root" "$(echo "$data" | jq -r '.project.root')"

    printf "\n"
    print_info "URLs\n"
    echo "$data" | jq -r '.project.urls | to_entries[] | select(.value != "") | "\(.key)|\(.value)"' |
        while IFS='|' read -r key value; do
            printf "   %-14s " "$key"
            print_link "$value\n"
        done

    printf "\n"
    print_info "Magento\n"
    printf "   %-14s %s\n" "version" "$(echo "$data" | jq -r '.magento.version // "" | if . == "" then "unknown" else . end')"
    printf "   %-14s %s\n" "mode" "$(echo "$data" | jq -r '.magento.mode // "" | if . == "" then "unknown" else . end')"
    printf "   %-14s %s\n" "xdebug" "$(echo "$data" | jq -r '.tooling.xdebug')"

    printf "\n"
    print_info "Services\n"
    echo "$data" | jq -r '.services[] | "\(.name)|\(.state)|\(.image)"' |
        while IFS='|' read -r name state image; do
            printf "   %-14s %-10s %s\n" "$name" "$state" "$image"
        done

    printf "\n"
    print_info "Paths\n"
    printf "   %-14s %s\n" "magento dir" "$(echo "$data" | jq -r '.paths.magento_dir')"
    printf "   %-14s %s\n" "mounts" "$(echo "$data" | jq -r '.paths.strategy')"

    if echo "$data" | jq -e '.credentials' >/dev/null 2>&1; then
        printf "\n"
        print_warning "Credentials\n"
        echo "$data" | jq -r '.credentials.database | to_entries[] | "db \(.key)|\(.value)"' |
            while IFS='|' read -r key value; do
                printf "   %-14s %s\n" "$key" "$value"
            done
    fi

    printf "\n"
}

describe_execute() {
    local data
    data=$(collect_project_info "$with_secrets")

    if is_json_output; then
        json_success "describe" "$data"
    else
        print_text "$data"
    fi
}

while [ "$#" -gt 0 ]; do
    case "$1" in
        --with-secrets)
            with_secrets=true
            shift
            ;;
        *)
            hm_fail "$HM_EXIT_USAGE" "invalid_argument" \
                "Unknown option: $1" \
                "$COMMAND_BIN_NAME describe --help"
            ;;
    esac
done

describe_execute
