#!/usr/bin/env bash
set -euo pipefail

source "$COMPONENTS_DIR"/print_message.sh
source "$HELPERS_DIR"/docker.sh

container_id=$(get_container_id phpfpm)

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

#
# Copy 
#
copy_some_to_container() {
    # Copy each file and folder into container
    for path_to_mirror in "$@"; do
        # Check if file/directory not exists
        if [[ ! -e $path_to_mirror ]] ; then
            continue
        fi
    
        if [ -f "$MAGENTO_DIR/$path_to_mirror" ]; then
            print_processing "Copying $path_to_mirror -> phpfpm:$path_to_mirror'"
            docker cp "$MAGENTO_DIR/$path_to_mirror" "$container_id":/var/www/html/"$path_to_mirror"
        else
            dest_path=$(dirname "$path_to_mirror")
            print_processing "Copying $path_to_mirror -> phpfpm:$dest_path"
            docker cp "$MAGENTO_DIR/$path_to_mirror" "$container_id":/var/www/html/"$dest_path"
        fi
        "$COMMANDS_DIR"/exec.sh -r sh -c "chown -R $USER_PHP:$GROUP_PHP $WORKDIR_PHP/$path_to_mirror"
    done
}

#
# Execute copy file to container
#
copy_to_container_exceute() {
    print_info "Start copy of host into container\n"
    print_info "----------------------------------------\n"

    if [ "$1" == "--all" ]; then
        docker cp "$MAGENTO_DIR/./" "$container_id":/var/www/html/
        echo "Completed copying all files from host to container"
        "$COMMANDS_DIR"/exec.sh -r sh -c "chown -R $USER_PHP:$GROUP_PHP $WORKDIR_PHP/./"
    else
        copy_some_to_container "$@"
    fi
}

copy_to_container_exceute "$@"