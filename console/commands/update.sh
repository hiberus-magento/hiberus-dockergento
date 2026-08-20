#!/usr/bin/env bash
set -euo pipefail

source "$COMPONENTS_DIR"/print_message.sh
source "$HELPERS_DIR"/exit_codes.sh

install_dir="$COMMAND_BIN_DIR"

if ! git -C "$install_dir" rev-parse --git-dir >/dev/null 2>&1; then
    hm_fail "$HM_EXIT_ERROR" "not_a_git_installation" \
        "This is not a git installation of $COMMAND_TOOLNAME" \
        "Reinstall with the installer to be able to update"
fi

branch=$(git -C "$install_dir" rev-parse --abbrev-ref HEAD 2>/dev/null || echo "")

# A detached checkout means someone is validating a specific version. `git pull origin HEAD`
# does not fail there: it brings the remote's default branch and takes them off the version
# they were testing, without saying anything. Refusing is the whole point.
if [ "$branch" == "HEAD" ]; then
    version=$(git -C "$install_dir" describe --tags --abbrev=0 2>/dev/null || echo "unknown")

    hm_fail "$HM_EXIT_BLOCKED" "detached_installation" \
        "The installation is pinned to $version, so there is nothing to update" \
        "$COMMAND_BIN_NAME switch --stable   # or switch to another version"
fi

if git -C "$install_dir" pull origin "$branch" >/dev/null 2>&1; then
    print_info "$COMMAND_TOOLNAME updated!\n"
    "$COMMAND_BIN_DIR"/generate_completion.sh
else
    hm_fail "$HM_EXIT_ERROR" "update_failed" \
        "Could not update from origin/$branch" \
        "Check the installation directory: $install_dir"
fi
