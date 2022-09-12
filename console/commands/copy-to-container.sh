#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=/dev/null
source "$COMPONENTS_DIR"/print_message.sh
source "$TASKS_DIR"/mirror_path.sh

#
# Validate mirror host path
#
validate_mirror_host_path() {
    path_to_mirror=$1

    bind_mount_path=$("$TASKS_DIR"/get_bind_mount_path.sh "$WORKDIR_PHP/$path_to_mirror")
    if [[ $bind_mount_path != false ]]; then
        print_error "\nPath cannot be mirrored. Following path is a bind mount inside container:\n\n"
        print_default "  - ./<host_path>:$bind_mount_path\n"
        exit 1
    fi
}

if [[ "$MACHINE" != "mac" ]]; then
    print_error " This command is only for mac system.\n"
    exit 1
fi

print_info "Start mirror copy of host into container\n"
print_info "----------------------------------------\n"

container_id=$($DOCKER_COMPOSE ps -q phpfpm)

for path_to_mirror in "$@"; do
    path_to_mirror=$(sanitize_mirror_path "$path_to_mirror")
    validate_mirror_host_path "$path_to_mirror"

    # If not exist jump to the next one
    if [[ ! -f $path_to_mirror && ! -d $path_to_mirror ]] ; then
        continue
    fi

    src_path=$path_to_mirror
    dest_path=$path_to_mirror
    dest_dir=$(dirname "$dest_path")

    src_is_dir=$([ -d "$src_path" ] && echo true || echo false)
    
    if [[ $src_is_dir == *true* ]]; then
        $COMMAND_BIN_NAME exec sh -c "rm -rf $dest_path/*"
        src_path="$src_path/."
        dest_dir="$dest_path"
    fi

    $COMMAND_BIN_NAME exec sh -c "mkdir -p $dest_dir"

    if [[ $src_is_dir == *true* && $(find "$src_path" -maxdepth 0 -empty) ]]; then
        print_procesing "Skipping copy. Source dir is empty: '$src_path'"
    else
        print_procesing "Copying $path_to_mirror -> phpfpm:$path_to_mirror'"
        docker cp "$src_path" "$container_id:$WORKDIR_PHP/$dest_path"
    fi

    ownership_command="chown -R $USER_PHP:$GROUP_PHP $WORKDIR_PHP/$dest_path"
    $COMMAND_BIN_NAME exec --root sh -c "$ownership_command"
done

print_info "----------------------------------------\n\n"
