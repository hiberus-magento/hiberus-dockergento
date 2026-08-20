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
# Describe the installed version.
#
# `git describe --tags` without --abbrev=0 on purpose: rounded to the nearest tag it said
# "1.4.5" while eleven commits ahead of it, so whoever reported a bug could not say what
# they were reporting it against.
#
hm_version_data() {
    local install_dir="$COMMAND_BIN_DIR"
    local description branch commit tag commits_ahead dirty detached

    description=$(git -C "$install_dir" describe --tags 2>/dev/null || echo "")
    tag=$(git -C "$install_dir" describe --tags --abbrev=0 2>/dev/null || echo "")
    commit=$(git -C "$install_dir" rev-parse --short HEAD 2>/dev/null || echo "")
    branch=$(git -C "$install_dir" rev-parse --abbrev-ref HEAD 2>/dev/null || echo "")

    if [ "$branch" == "HEAD" ]; then
        detached="true"
        branch=""
    else
        detached="false"
    fi

    # Commits since the tag, taken from the description instead of asking git again
    commits_ahead="0"
    case "$description" in
        *-*-g*)
            commits_ahead=$(printf '%s' "$description" | sed 's/.*-\([0-9]*\)-g[0-9a-f]*$/\1/')
            ;;
    esac

    # Tracked changes only: untracked files are the user's own and do not affect which
    # version is installed
    if [ -n "$(git -C "$install_dir" status --porcelain --untracked-files=no 2>/dev/null)" ]; then
        dirty="true"
    else
        dirty="false"
    fi

    printf '%s\037%s\037%s\037%s\037%s\037%s\037%s\037%s\n' \
        "${description:-unknown}" "$tag" "$commits_ahead" "$commit" \
        "$branch" "$detached" "$dirty" "$install_dir"
}

#
# Return hm version
#
process_version() {
    if [ "$#" -eq "0" ]; then
        set -- -v
    fi

    if [ "$#" -eq 0 ] || { [ "$1" != "--version" ] && [ "$1" != "-v" ]; }; then
        return 0
    fi

    local description tag commits_ahead commit branch detached dirty install_dir
    IFS=$'\037' read -r description tag commits_ahead commit branch detached dirty install_dir \
        <<< "$(hm_version_data)"

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
            '$ARGS.named')"
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
    exit 0
}

execute_process_hm_options() {
    process_help "$@"
    process_version "$@"
}