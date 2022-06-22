#!/usr/bin/env bash
set -euo pipefail

"${TASKS_DIR}"/start_service_if_not_running.sh phpfpm
${COMMAND_BIN_NAME} exec sed -i -e 's/^\zend_extension/;zend_extension/g' /usr/local/etc/php/conf.d/docker-php-ext-xdebug.ini
${DOCKER_COMPOSE} restart phpfpm

echo -e "  ${GREEN}xdebug disabled${COLOR_RESET}"