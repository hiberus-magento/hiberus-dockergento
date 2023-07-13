#!/usr/bin/env bash

platform="magento2"
database=$(docker-compose exec db bash -c "echo -n \$MYSQL_database")
username=$(docker-compose exec db bash -c "echo -n \$MYSQL_USER")
password=$(docker-compose exec db bash -c "echo -n \$MYSQL_password")
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
    docker run \
    --network=$(docker ps --filter id="$(docker-compose ps -q db)" --format '{{ json .Networks }}' | tr -d '"') $volume_config \
    -t -i --rm hiberusmagento/masquerade\
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