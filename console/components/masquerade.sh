#!/usr/bin/env bash

PLATFORM="magento2"
DATABASE=$(docker-compose exec db bash -c "echo -n \$MYSQL_DATABASE")
USERNAME=$(docker-compose exec db bash -c "echo -n \$MYSQL_USER")
PASSWORD=$(docker-compose exec db bash -c "echo -n \$MYSQL_PASSWORD")
PORT="3306"
DRIVER="mysql"
LOCALE="es_ES"
MASQUERADE_PROJECT_CONFIG_FOLDER="./config/docker/masquerade"
MASQUERADE_CONFIG_FOLDER="/app/masquerade"
VOLUME_CONFIG=""
CONFIG=""

# Prepare volume config
[ -d ${MASQUERADE_PROJECT_CONFIG_FOLDER} ] && VOLUME_CONFIG="--volume ${MASQUERADE_PROJECT_CONFIG_FOLDER}:${MASQUERADE_CONFIG_FOLDER}"

# Prepare config argument
[ -d ${MASQUERADE_PROJECT_CONFIG_FOLDER} ] && CONFIG="--config=${MASQUERADE_CONFIG_FOLDER}"

masquerade_run() {    
    docker run \
    --network=$(docker ps --filter id="$(docker-compose ps -q db)" --format '{{ json .Networks }}' | tr -d '"') $VOLUME_CONFIG \
    -t -i --rm hiberusmagento/masquerade\
    masquerade run \
    --platform=${PLATFORM} \
    --database=${DATABASE} \
    --username=${USERNAME} \
    --password=${PASSWORD} \
    --host=db \
    --port=${PORT} \
    --driver=${DRIVER} \
    --locale=${LOCALE} $CONFIG
}