#!/usr/bin/env bash
set -euo pipefail

source "$COMPONENTS_DIR"/input_info.sh
source "$COMPONENTS_DIR"/print_message.sh

"$TASKS_DIR"/start_service_if_not_running.sh phpfpm

if [[ "$MACHINE" == 'linux' && "${XDEBUG_HOST:-}" == "" ]]; then
    source "$TASKS_DIR"/set_xdebug_host_property.sh
fi

if [[ "${XDEBUG_HOST:-}" != "" ]]; then
    $COMMAND_BIN_NAME exec sed -i "s/xdebug\.remote_host\=.*/xdebug\.remote_host\=$XDEBUG_HOST/g" /usr/local/etc/php/conf.d/xdebug.ini
    $COMMAND_BIN_NAME exec sed -i "s/xdebug\.client_host\=.*/xdebug\.client_host\=$XDEBUG_HOST/g" /usr/local/etc/php/conf.d/xdebug.ini
fi

$COMMAND_BIN_NAME exec sed -i -e 's/^\;zend_extension/zend_extension/g' /usr/local/etc/php/conf.d/docker-php-ext-xdebug.ini

if [[ "$MACHINE" == "mac" ]]; then
    print_warning "Copying generated code into host \n"
    $COMMAND_BIN_NAME copy-from-container -f generated
else
    $DOCKER_COMPOSE restart phpfpm
fi

print_warning "xdebug configuration:\n"
print_warning "------------------------------------------------\n"
$COMMAND_BIN_NAME exec php -i | grep -e "xdebug.idekey" -e "xdebug.client_host" -e "xdebug.client_port" | cut -d= -f1-2
print_warning "------------------------------------------------\n"
