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
# Commands that can create or recreate containers, and therefore need every label resolved
# before compose interpolates them
#
hm_creates_containers() {
    case "$1" in
        start | restart | rebuild | setup | install | create-project | docker-compose)
            return 0
            ;;
        *)
            return 1
            ;;
    esac
}

#
# Export the cheap labels, which every invocation can afford
#
set_environment_labels() {
    export HM_PROJECT="${COMPOSE_PROJECT_NAME:-}"
    export HM_ROOT="${HM_ROOT:-$PWD}"
    export HM_WORKTREE="${HM_WORKTREE:-}"
    export HM_PROFILE="${HM_PROFILE:-full}"
    export HM_AGENT="${HM_AGENT:-}"
    export HM_VERSION="${HM_VERSION:-}"
    export HM_MAGENTO="${HM_MAGENTO:-}"
}

#
# Resolve the expensive labels. `git describe` costs ~50ms and reading the Magento version
# out of a 1.6MB composer.lock costs ~77ms, and only the containers being created care, so
# this is called for those commands instead of on every invocation.
#
set_environment_labels_full() {
    [ -z "${HM_VERSION:-}" ] && export HM_VERSION="$(hm_installed_version)"
    [ -z "${HM_MAGENTO:-}" ] && export HM_MAGENTO="$(hm_magento_version)"

    return 0
}
