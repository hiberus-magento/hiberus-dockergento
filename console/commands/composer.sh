#!/usr/bin/env bash
set -euo pipefail

source "$COMPONENTS_DIR"/print_message.sh
source "$HELPERS_DIR"/exit_codes.sh
source "$HELPERS_DIR"/binary.sh

#
# A worktree that reads the main checkout's dependencies cannot write to them.
#
# The mount is read-only, so Composer would fail on a read-only filesystem — an error about
# permissions, three layers down, for something that is a decision rather than an accident. It is
# said here instead, with the way out.
#
refuse_on_shared_dependencies() {
    [ "${HM_WORKTREE_ENVIRONMENT:-false}" == "true" ] || return 0

    case "${1:-}" in
        install | update | require | remove) ;;
        *) return 0 ;;
    esac

    source "$TASKS_DIR"/worktree_env.sh
    hm_worktree_load "${HM_PARENT_PROJECT:-}" "${HM_WORKTREE:-}" || return 0

    [ "${WORKTREE_VENDOR:-own}" == "shared" ] || return 0

    hm_fail "$HM_EXIT_BLOCKED" "shared_dependencies" \
        "This worktree reads the dependencies of $HM_MAIN_ROOT, so Composer cannot write to them" \
        "Change composer.json in this branch and run $COMMAND_BIN_NAME worktree remove/add, or work on dependencies in the main checkout"
}

#
# From a worktree with no environment of its own, the macOS dance would restart phpfpm and rewrite
# the dependencies of the main checkout.
#
# The binary refuses the restart anyway, but its way out says "repeat with --force", and after
# `composer` that flag is Composer's: `hm composer install --force` never reaches hm. So it is said
# here, before the restart, with the form hm does read.
#
refuse_from_main_checkouts_worktree() {
    [ "${HM_IS_WORKTREE:-false}" == "true" ] || return 0
    [ "${HM_WORKTREE_ENVIRONMENT:-false}" != "true" ] || return 0
    [ "${HM_FORCE:-}" != "1" ] || return 0

    hm_fail "$HM_EXIT_BLOCKED" "blocked_in_worktree" \
        "'composer $1' is refused from a worktree on macOS: it would restart phpfpm and rewrite the dependencies of $HM_ROOT" \
        "Run it from $HM_ROOT, or repeat as $COMMAND_BIN_NAME --force composer $*"
}

#
# Copy vendor and magento defaults files into container
#
copy_vendor_to_container() {
    if [ ! -d "$MAGENTO_DIR"/vendor ]; then
        mkdir -p "$MAGENTO_DIR/vendor"
    fi
    hm_run_binary copy-to-container "vendor"
}

#
# Synchronize al content from container to host
#
sync_all_from_container_to_host() {
    container_id=$($DOCKER_COMPOSE ps -q phpfpm)

    # Copy all default files magento into container
    default_files_magento=$(cat < "$DATA_DIR/default_files_magento.json" | jq -r 'keys | join(" ")')

    hm_run_binary copy-to-container $default_files_magento
    hm_run_binary stop "phpfpm"

    print_info "Copying all files from container to host\n"
    print_processing "Removing vendor in host: $MAGENTO_DIR/vendor/*"
    rm -rf "$MAGENTO_DIR"/vendor/*

    print_processing "Copying phpfpm:${WORKDIR_PHP}/. into $MAGENTO_DIR"
    
    docker cp "$container_id":"$WORKDIR_PHP"/. "$MAGENTO_DIR"

    # Start containers again because we needed to stop them before mirroring
    "$COMMANDS_DIR"/start.sh "phpfpm"
}

#
# Exit when user tries composer create-project
#
check_create_project() {
    if [[ "$#" != 0 && "$1" == "create-project" ]]; then
        print_error "create-project is not compatible with $COMMAND_BIN_NAME. Please use:\n"
        print_code "\n  $COMMAND_BIN_NAME create-project\n\n"
        exit "$HM_EXIT_USAGE"
    fi
}

#
# Manage composer commands
#
composer_excute() {
    check_create_project "$@"
    refuse_on_shared_dependencies "$@"

    if [[ "$#" != 0 && 
        ("$1" == "install" || 
        "$1" == "update" || 
        "$1" == "require" || 
        "$1" == "remove") ]]; then
        
        # Execute install con mirror wrapper
        if [[ "$#" != 0 && ("$MACHINE" == "mac") ]]; then
            refuse_from_main_checkouts_worktree "$@"
            hm_run_binary restart "phpfpm"
            copy_vendor_to_container
            "$COMMANDS_DIR"/exec.sh composer "$@"
            sync_all_from_container_to_host
        else
            "$COMMANDS_DIR"/exec.sh composer "$@"
        fi
    else
        "$COMMANDS_DIR"/exec.sh composer "$@"
    fi
}

composer_excute "$@"