#!/usr/bin/env bash
set -uo pipefail

source "$COMPONENTS_DIR"/print_message.sh
source "$COMPONENTS_DIR"/input_info.sh
source "$HELPERS_DIR"/exit_codes.sh

#
# Stop and remove the environment.
#
# Without -v this destroys nothing that cannot be rebuilt, and it stays the one-line pass-through
# it always was. With -v it deletes the volumes, and with them the database — one letter of
# difference, no warning, no way back. An environment on this machine was lost exactly that way
# while this version was being written.
#
# So: it asks, it says which volumes, and it offers the cheap way out now that `hm db snapshot`
# exists to provide one.
#

removes_volumes=false
for argument in "$@"; do
    case "$argument" in
        -v | --volumes) removes_volumes=true ;;
    esac
done

destroy() {
    $DOCKER_COMPOSE down "$@"
}

if ! $removes_volumes || is_non_interactive; then
    destroy "$@"
    exit $?
fi

volumes=$($DOCKER_COMPOSE config --volumes 2>/dev/null | sed '/^$/d')
existing=""

for volume in $volumes; do
    name="${COMPOSE_PROJECT_NAME}_${volume}"
    docker volume inspect "$name" >/dev/null 2>&1 && existing="$existing$name\n"
done

if [ -z "$existing" ]; then
    destroy "$@"
    exit $?
fi

printf '\n'
print_warning "This deletes the volumes of '$COMPOSE_PROJECT_NAME', and the database with them:\n\n"
printf "$existing" | while IFS= read -r name; do
    [ -n "$name" ] && printf '  %s\n' "$name"
done
printf '\n'

# Three answers, not two. Saving first is the default because it is the one nobody regrets, and
# taking the copy automatically would leave a pile of them in the projects that are destroyed on
# purpose several times a day.
options=("Save a database snapshot, then destroy" "Destroy without saving" "Cancel")
custom_select "What should happen?" "${options[@]}"

case "$REPLY" in
    "Save a database snapshot, then destroy")
        if ! "$COMMANDS_DIR"/db.sh snapshot; then
            hm_fail "$HM_EXIT_ERROR" "snapshot_failed" \
                "The snapshot failed, so nothing was destroyed" \
                "$COMMAND_BIN_NAME down ${*}   # to destroy anyway"
        fi
        destroy "$@"
        ;;
    "Destroy without saving")
        destroy "$@"
        ;;
    *)
        print_info "Nothing was destroyed.\n"
        exit 0
        ;;
esac
