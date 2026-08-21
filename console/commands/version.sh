#!/usr/bin/env bash
set -euo pipefail

source "$COMPONENTS_DIR"/print_message.sh
source "$COMPONENTS_DIR"/print_json.sh
source "$HELPERS_DIR"/exit_codes.sh
source "$HELPERS_DIR"/process_hm_options.sh
source "$HELPERS_DIR"/version.sh

#
# Versions: this tool, and the container tooling it drives.
#
# `hm --version` answers the first part and stops there, on purpose: it is the shortest path in
# the CLI and it has a performance budget watched by a test, so it does not call Docker. This
# command exists for the other half, which is what a bug report needs.
#
# It does not require a project, because the problem being reported may be that there is no
# project.
#

while [ "$#" -gt 0 ]; do
    case "$1" in
        *)
            hm_fail "$HM_EXIT_USAGE" "invalid_argument" \
                "Unknown option: $1" \
                "$COMMAND_BIN_NAME version"
            ;;
    esac
done

IFS=$'\037' read -r description tag commits_ahead commit branch detached dirty install_dir \
    <<< "$(hm_version_data)"

# Docker may not be installed, or not running. That is a fact worth reporting, not a reason to
# fail: a report that says "docker: not available" is more useful than no report.
docker_version=$(docker version --format '{{.Server.Version}}' 2>/dev/null || echo "")
[ -z "$docker_version" ] && docker_version=$(docker --version 2>/dev/null | sed 's/[^0-9]*\([0-9.]*\).*/\1/' || echo "")

compose_command=$(get_docker_compose_cmd 2>/dev/null || echo "")
compose_version=""
[ -n "$compose_command" ] && compose_version=$(get_docker_compose_version 2>/dev/null || echo "")

if is_json_output; then
    json_success "version" "$(jq -n \
        --arg version "$description" \
        --arg tag "$tag" \
        --argjson commits_ahead "${commits_ahead:-0}" \
        --arg commit "$commit" \
        --arg branch "$branch" \
        --argjson detached "$detached" \
        --argjson dirty "$dirty" \
        --arg path "$install_dir" \
        --arg docker "$docker_version" \
        --arg compose "$compose_version" \
        --arg compose_command "$compose_command" \
        '{
            version: $version, tag: $tag, commits_ahead: $commits_ahead, commit: $commit,
            branch: $branch, detached: $detached, dirty: $dirty, path: $path,
            docker: {version: $docker, compose: $compose, compose_command: $compose_command}
        }')"
    exit 0
fi

print_heading "$COMMAND_BIN_NAME $description\n"

if [ "$detached" == "true" ]; then
    printf "  %-12s %s\n" "version" "${tag:-unknown} (detached checkout)"
else
    printf "  %-12s %s\n" "branch" "$branch"
fi

printf "  %-12s %s\n" "commit" "${commit:-unknown}"
printf "  %-12s %s\n" "installed" "$install_dir"

if [ "$dirty" == "true" ]; then
    print_warning "  uncommitted changes in the installation directory\n"
fi

printf "\n"
printf "  %-12s %s\n" "docker" "${docker_version:-not available}"
printf "  %-12s %s\n" "compose" "${compose_version:-not available}"
printf "\n"
