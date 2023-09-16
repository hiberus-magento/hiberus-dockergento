#!/usr/bin/env bash
set -euo pipefail

source "$COMPONENTS_DIR"/input_info.sh
source "$COMPONENTS_DIR"/print_message.sh
source "$HELPERS_DIR"/docker.sh

#
# Excute copy_from_container command
#
copy_from_container_exceute() {
    print_info "Start mirror copy of container into host\n"
    print_info "----------------------------------------\n"
    container_id=$(get_container_id phpfpm)


    for path_to_mirror in "$@"; do
        src_path=$([[ -d $path_to_mirror ]] && echo "$path_to_mirror/." || echo  "$path_to_mirror")

        path_to_mirror=${path_to_mirror%/}
        dest_path="$path_to_mirror"

        print_processing "Copying phpfpm:$path_to_mirror -> $path_to_mirror'"
        docker cp "$container_id:/var/www/html/$src_path" "$dest_path"
    done

    print_info "----------------------------------------\n\n"
}

# Check if command recives arguments
if [[ $# -eq 0 ]]; then
    source "$HELPERS_DIR"/print_usage.sh
    print_error "The command is not correct\n"
    print_info "Use this format\n"
    get_usage "$(basename ${0%.sh})"
    exit 1
fi

# Command only available in Mac OS
if [[ "$MACHINE" != "mac" ]]; then
    print_error " This command is only for mac system.\n"
    exit 1
fi

copy_from_container_exceute "$@"