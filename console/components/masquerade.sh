#!/usr/bin/env bash

platform="magento2"
database=$($DOCKER_COMPOSE exec db bash -c "echo -n \$MYSQL_DATABASE")
username=$($DOCKER_COMPOSE exec db bash -c "echo -n \$MYSQL_USER")
password=$($DOCKER_COMPOSE exec db bash -c "echo -n \$MYSQL_PASSWORD")
port="3306"
driver="mysql"
locale="es_ES"
masquerade_project_config_folder="./config/docker/masquerade"
masquerade_config_folder="/app/masquerade"
volume_config=""
config=""

# Prepare volume config
[ -d ${masquerade_project_config_folder} ] && volume_config="--volume ${masquerade_project_config_folder}:${masquerade_config_folder}"

# Prepare config argument
[ -d ${masquerade_project_config_folder} ] && config="--config=${masquerade_config_folder}"

masquerade_run() {
    #
    # A terminal only when there is one. `-t -i` unconditionally meant `the input device is not a
    # TTY` from CI, from a script and from an agent — so the command that anonymises has never
    # been usable by anything except a person at a keyboard, which is the opposite of what it is
    # for.
    #
    local terminal=""
    [ -t 0 ] && [ -t 1 ] && terminal="-t -i"

    docker run \
    --network=$(docker ps --filter id="$($DOCKER_COMPOSE ps -q db)" --format '{{ json .Networks }}' | tr -d '"') $volume_config \
    $terminal --rm hiberusmagento/masquerade\
    masquerade run \
    --platform=${platform} \
    --database=${database} \
    --username=${username} \
    --password=${password} \
    --host=db \
    --port=${port} \
    --driver=${driver} \
    --locale=${locale} $config
}