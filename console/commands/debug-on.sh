#!/usr/bin/env bash
set -euo pipefail

"${TASKS_DIR}"/start_service_if_not_running.sh phpfpm

if [[ "${MACHINE}" == 'linux' && "${XDEBUG_HOST:-}" == "" ]]; then
  source "${TASKS_DIR}"/set_xdebug_host_property.sh
fi

if [[ "${XDEBUG_HOST:-}" != "" ]]; then
  ${COMMAND_BIN_NAME} exec sed -i "s/xdebug\.remote_host\=.*/xdebug\.remote_host\=${XDEBUG_HOST}/g" /usr/local/etc/php/conf.d/xdebug.ini
  ${COMMAND_BIN_NAME} exec sed -i "s/xdebug\.client_host\=.*/xdebug\.client_host\=${XDEBUG_HOST}/g" /usr/local/etc/php/conf.d/xdebug.ini
fi

${COMMAND_BIN_NAME} exec sed -i -e 's/^\;zend_extension/zend_extension/g' /usr/local/etc/php/conf.d/docker-php-ext-xdebug.ini

if [[ "${MACHINE}" == "mac" ]]; then
  echo -e "${YELLOW}Copying generated code into host ${COLOR_RESET}"
  ${COMMAND_BIN_NAME} mirror-container -f generated
else
  ${DOCKER_COMPOSE} restart phpfpm
fi

echo -e "\n${YELLOW}xdebug configuration: ${COLOR_RESET}"
echo -e "${YELLOW}------------------------------------------------${COLOR_RESET}"
${COMMAND_BIN_NAME} exec php -i | grep -e "xdebug.idekey" -e "xdebug.client_host" -e "xdebug.client_port" | cut -d= -f1-2
echo -e "${YELLOW}------------------------------------------------${COLOR_RESET}\n"
