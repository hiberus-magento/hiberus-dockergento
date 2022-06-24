#!/usr/bin/env bash
set -euo pipefail

pushd ${COMMANDS_DIR} >/dev/null 2>&1

# If it's a GIT installation, pull last changes
if git rev-parse --git-dir >/dev/null 2>&1; then

    git pull origin $(git rev-parse --abbrev-ref HEAD) >/dev/null 2>&1 &&
        echo -e "${GREEN}${COMMAND_TOOLNAME} updated!${COLOR_RESET}\n" ||
        echo -e "${RED}Error during update.${COLOR_RESET}"

# If it isn't a GIT installation, throw error
else
    echo -e "${RED}Not a valid GIT installation of ${COMMAND_TOOLNAME}.${COLOR_RESET}"
fi

popd >/dev/null 2>&1
