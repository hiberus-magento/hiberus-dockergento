#!/usr/bin/env bash
set -euo pipefail

source "$COMPONENTS_DIR"/input_info.sh

"$TASKS_DIR"/start_service_if_not_running.sh phpfpm
"$COMMANDS_DIR"/exec.sh sed -i -e 's/^\zend_extension/;zend_extension/g' /usr/local/etc/php/conf.d/docker-php-ext-xdebug.ini
$DOCKER_COMPOSE restart phpfpm

print_info "  xdebug disabled\n"