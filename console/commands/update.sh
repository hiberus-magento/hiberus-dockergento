#!/usr/bin/env bash
set -euo pipefail

source "$COMPONENTS_DIR"/print_message.sh

pushd ${COMMANDS_DIR} >/dev/null 2>&1

# If it's a GIT installation, pull last changes
if git rev-parse --git-dir >/dev/null 2>&1; then

    git pull origin $(git rev-parse --abbrev-ref HEAD) >/dev/null 2>&1 &&
        print_info "${COMMAND_TOOLNAME} updated!\n" ||
        print_error "Error during update.\n"

# If it isn't a GIT installation, throw error
else
    print_error "Not a valid GIT installation of ${COMMAND_TOOLNAME}.\n"
fi

popd >/dev/null 2>&1
