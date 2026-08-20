#!/usr/bin/env bash

#
# Git worktree awareness.
#
# A worktree is a second working directory of the same repository. Because the compose
# project name lives in config/docker/properties.json, which is committed, a worktree
# inherits it: Docker Compose then treats it as *the same project* and, on `up`, recreates
# the existing containers with the bind mounts of whichever directory invoked it. That
# silently repoints the main environment at the worktree, and `down -v` destroys it.
#
# So from a worktree everything is resolved against the main checkout, and the commands
# that would recreate or destroy the environment are refused.
#

#
# Resolves HM_ROOT, HM_IS_WORKTREE and HM_WORKTREE in the *current* shell.
#
# It assigns instead of printing on purpose: called as `HM_ROOT=$(hm_resolve_project_root)`
# the flags would be set inside a command substitution subshell and thrown away, leaving
# the guardrails switched off while the resolved path looked perfectly correct.
#
hm_resolve_project_root() {
    HM_IS_WORKTREE=false
    HM_WORKTREE=""
    HM_ROOT="$PWD"

    # Explicit override wins: useful where git is unavailable, and the hook the per
    # worktree environments will need later
    if [ -n "${HM_PROJECT_DIR:-}" ]; then
        HM_ROOT="$HM_PROJECT_DIR"
        return 0
    fi

    local common_dir toplevel main_root git_output

    # One git call instead of three. Asking "am I in a repository?", "where is the common
    # .git?" and "what is this checkout's root?" separately cost 132ms; asked together they
    # cost 46ms, and outside a repository the call simply fails, which is the same answer
    # the first question used to give.
    #
    # --path-format needs git 2.31; older versions fall back to the relative form.
    git_output=$(git rev-parse --path-format=absolute --git-common-dir --show-toplevel 2>/dev/null) ||
        git_output=$(git rev-parse --git-common-dir --show-toplevel 2>/dev/null) ||
        return 0

    { read -r common_dir; read -r toplevel; } <<< "$git_output"

    if [ -z "$common_dir" ] || [ -z "$toplevel" ]; then
        return 0
    fi

    # The fallback form can return a relative path
    case "$common_dir" in
        /*) ;;
        *)  common_dir=$(cd "$common_dir" 2>/dev/null && pwd) || return 0 ;;
    esac

    main_root="${common_dir%/.git}"

    # A bare repository or an unexpected layout: nothing to resolve
    if [ "$main_root" == "$common_dir" ] || [ -z "$toplevel" ]; then
        return 0
    fi

    # Same directory: this is the main checkout
    if [ "$main_root" == "$toplevel" ]; then
        return 0
    fi

    # A worktree of a repository that is not a Dockergento project is none of our business
    if [ ! -f "$main_root/docker-compose.yml" ] ||
       [ ! -f "$main_root/config/docker/properties.json" ]; then
        return 0
    fi

    HM_IS_WORKTREE=true
    HM_WORKTREE=$(basename "$toplevel")
    HM_ROOT="$main_root"
}

#
# Commands that create, recreate or destroy the environment
#
hm_alters_environment() {
    case "$1" in
        start | stop | restart | rebuild | down | setup | install | create-project | docker-stop-all)
            return 0
            ;;
        *)
            return 1
            ;;
    esac
}

#
# Commands that operate on the code, which lives in the main checkout, not here
#
hm_operates_on_code() {
    case "$1" in
        magento | composer | npm | grunt | exec | bash | test-unit | test-integration | n98-magerun)
            return 0
            ;;
        *)
            return 1
            ;;
    esac
}
