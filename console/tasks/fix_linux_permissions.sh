#!/usr/bin/env bash
set -euo pipefail

docker_root_command="${COMMANDS_DIR}/docker-compose.sh exec -T -u root"

prepare_container_to_change_user_ids() {
    service=$1
    if ! $docker_root_command "$service" sh -c "type usermod >/dev/null 2>&1" ||
        ! $docker_root_command "$service" sh -c "type groupmod >/dev/null 2>&1"; then

        if $docker_root_command "$service" sh -c "type apk /dev/null 2>&1"; then
            echo "Warning: installing shadow, this should be included in your image"
            $docker_root_command "$service" sh -c "apk add --no-cache shadow"
        else
            print_error "Error: Commands usermod and groupmod are required.\n"
            exit 1
        fi
    fi
}

match_user_id_between_host_and_container() {
    service=$1
    container_uid=$($docker_root_command "$service" sh -c "getent passwd $USER_PHP | cut -f3 -d:")
    host_uid=$(id -u "$USER")

    if [[ "$host_uid" == '0' ]]; then
        print_error "Error: Something is wrong, HOST_UID cannot have id 0 (root).\n"
        exit 1
    fi

    if [ "$container_uid" != "$host_uid" ]; then
        print_default " > changing UID of $USER_PHP from $container_uid to $host_uid in $service service"
        prepare_container_to_change_user_ids "$service"
        $docker_root_command "$service" sh -c "usermod -u $host_uid -o {USER_PHP"
        $docker_root_command "$service" sh -c "find / -xdev -user '$container_uid' -exec chown -h '$USER_PHP' {} \;"
    fi
}

match_group_id_between_host_and_container() {
    service=$1
    container_gid=$($docker_root_command "$service" sh -c "getent group $GROUP_PHP | cut -f3 -d:")
    host_gid=$(id -g "$USER")

    if [[ $host_gid == '0' ]]; then
        print_error "Error: Something is wrong, HOST_UID cannot have id 0 (root).\n"
    fi

    if [ "$container_gid" != "$host_gid" ]; then
        print_default " > changing GID of $USER_PHP from $container_gid to $host_gid in $service service\n"
        prepare_container_to_change_user_ids "$service"
        $docker_root_command "$service" sh -c "groupmod -g $host_gid -o $GROUP_PHP"
        $docker_root_command "$service" sh -c "find / -xdev -user '$container_gid' -exec chgrp -h '$USER_PHP' {} \;"
    fi
}

for service in ${SERVICES_WITH_LINUX_PERMISSIONS_ISSUES}; do
    match_user_id_between_host_and_container "$service"
    match_group_id_between_host_and_container "$service"
done
