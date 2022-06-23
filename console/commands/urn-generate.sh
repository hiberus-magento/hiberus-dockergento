#!/usr/bin/env bash
set -euo pipefail

echo " > Generating urn-catalog in container"
MISC_PATH=$1
${COMMAND_BIN_NAME} exec sh -c "mkdir -p $(dirname .idea/misc.xml)"
${COMMAND_BIN_NAME} magento dev:urn-catalog:generate "${MISC_PATH}"

ABSOLUTE_HOST_DIR=$(pwd)
echo " > Replacing paths: '${WORKDIR_PHP} -> ${ABSOLUTE_HOST_DIR}'"
${COMMAND_BIN_NAME} exec sh -c "sed -i s#${WORKDIR_PHP}#${ABSOLUTE_HOST_DIR}#g ${MISC_PATH}"

echo " > Copying generated urn from container into host"
CONTAINER_ID=$(${DOCKER_COMPOSE} ps -q phpfpm)
docker cp ${CONTAINER_ID}:${WORKDIR_PHP}/${MISC_PATH} ${MISC_PATH}
