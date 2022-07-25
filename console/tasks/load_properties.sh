#!/usr/bin/env bash
set -euo pipefail

set -a # Enable export all variables

# shellcheck source=/dev/null
source "${PROPERTIES_DIR}"/color_properties
source "${PROPERTIES_DIR}"/docker_properties

root_dir=$PWD

for properties_root_dir in $root_dir $root_dir/.. $root_dir/../..; do
    custom_properties=$properties_root_dir/config/$COMMAND_BIN_NAME/properties
    if [ -f "$custom_properties" ]; then
        source "$custom_properties"
    fi
done

set +a # Disable export all variables
