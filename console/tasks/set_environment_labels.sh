#!/usr/bin/env bash

#
# Environment metadata exported for the `hm.*` labels of docker-compose.template.yml.
#
# Only stable identity is stamped on a container. Anything that changes while the
# container lives (current branch, git state, last activity) is derived at read time from
# hm.root, so a label never lies.
#

#
# Version of the hm installation itself
#
hm_installed_version() {
    git -C "$COMMAND_BIN_DIR" describe --tags --abbrev=0 2>/dev/null || true
}

#
# Magento version of the current project, read from composer.lock (~55ms on a 1.6MB file,
# which is noise next to the `docker compose config` every command already runs)
#
hm_magento_version() {
    local lock="${MAGENTO_DIR:-.}/composer.lock"

    if [ ! -f "$lock" ]; then
        return 0
    fi

    jq -r '.packages
        | map(select(.name == "magento/product-community-edition" or
                     .name == "magento/product-enterprise-edition"))[0].version // empty' \
        < "$lock" 2>/dev/null || true
}

#
# Export everything the compose template interpolates
#
set_environment_labels() {
    export HM_PROJECT="${COMPOSE_PROJECT_NAME:-}"
    export HM_ROOT="${HM_ROOT:-$PWD}"
    export HM_WORKTREE="${HM_WORKTREE:-}"
    export HM_PROFILE="${HM_PROFILE:-full}"
    export HM_AGENT="${HM_AGENT:-}"
    export HM_VERSION="${HM_VERSION:-$(hm_installed_version)}"
    export HM_MAGENTO="${HM_MAGENTO:-$(hm_magento_version)}"
}
