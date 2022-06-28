#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=/dev/null
source "$COMPONENTS_DIR"/input_info.sh

"$TASKS_DIR"/start_service_if_not_running.sh phpfpm
$COMMAND_BIN_NAME exec sed -i -e 's/^\zend_extension/;zend_extension/g' /usr/local/etc/php/conf.d/docker-php-ext-xdebug.ini
$DOCKER_COMPOSE restart phpfpm

print_info "  xdebug disabled\n"