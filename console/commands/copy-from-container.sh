#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=/dev/null
source "$COMPONENTS_DIR"/input_info.sh
source "$COMPONENTS_DIR"/print_message.sh


# IMPORTANT:
# mirror-container supports only dir copies for now.
# Because container src is not running, we cannot know whether the source is a file or a dir.
# For this reason and to simplify the logic, we support only dir copies.

#
# Clear destination directory
#
clear_dest_dir() {
    dest_path=$1

    if [ "$answer_remove_dest" != "y" ]; then
        print_question "Confirm removing '$dest_path/*' in host (y/n [n])? "
        read -r answer_remove_dest
    fi

    if [ "$answer_remove_dest" == "y" ]; then
        print_default "rm -rf $dest_path/*\n"
        rm -rf "${dest_path:?}"/*
    else
        print_procesing "Deletion skipped"
    fi
}

if [[ "$MACHINE" != "mac" ]]; then
    print_error " This command is only for mac system.\n"
    exit 1
fi

answer_remove_dest=""
if [[ "$1" == "--force" || "$1" == "-f" ]]; then
    answer_remove_dest='y'
    shift
fi

$COMMAND_BIN_NAME stop

# shellcheck source=/dev/null
source "$TASKS_DIR"/mirror_path.sh

print_info "Start mirror copy of container into host\n"
print_info "----------------------------------------\n"
container_id=$($DOCKER_COMPOSE ps -q phpfpm)


for path_to_mirror in "$@"; do
    path_to_mirror=$(sanitize_mirror_path "$path_to_mirror")

    SRC_PATH="$path_to_mirror/."
    dest_path="$path_to_mirror"

    clear_dest_dir "$dest_path"
    mkdir -p "$dest_path"

    print_procesing "Copying phpfpm:$path_to_mirror -> $path_to_mirror'"
    docker cp "$container_id:$WORKDIR_PHP/$SRC_PATH" "$dest_path"
done

print_info "----------------------------------------\n\n"

# Start containers again because we needed to stop them before mirroring
$COMMAND_BIN_NAME start
