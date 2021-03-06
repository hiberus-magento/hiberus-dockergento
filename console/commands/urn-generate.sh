#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=/dev/null
source "$COMPONENTS_DIR"/print_message.sh

print_default " > Generating urn-catalog in container\n"
misc_path=$1
$COMMAND_BIN_NAME exec sh -c "mkdir -p $(dirname .idea/misc.xml)"
$COMMAND_BIN_NAME magento dev:urn-catalog:generate "$misc_path"

absolute_host_dir=$(pwd)
print_default " > Replacing paths: '$WORKDIR_PHP -> $absolute_host_dir'\n"
$COMMAND_BIN_NAME exec sh -c "sed -i s#${WORKDIR_PHP}#${absolute_host_dir}#g $misc_path"

print_default " > Copying generated urn from container into host\n"
container_id=$($DOCKER_COMPOSE ps -q phpfpm)
docker cp "$container_id:$WORKDIR_PHP/$misc_path $misc_path"
