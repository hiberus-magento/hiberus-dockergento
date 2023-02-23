#!/usr/bin/env bash
set -euo pipefail

source "$COMPONENTS_DIR"/input_info.sh
source "$COMPONENTS_DIR"/print_message.sh

"$TASKS_DIR"/start_service_if_not_running.sh phpfpm

if [[ "$MACHINE" == 'linux' && "${XDEBUG_HOST:-}" == "" ]]; then
    source "$TASKS_DIR"/set_xdebug_host_property.sh
fi

if [[ "${XDEBUG_HOST:-}" != "" ]]; then
    "$COMMANDS_DIR"/exec.sh sed -i "s/xdebug\.remote_host\=.*/xdebug\.remote_host\=$XDEBUG_HOST/g" /usr/local/etc/php/conf.d/xdebug.ini
    "$COMMANDS_DIR"/exec.sh sed -i "s/xdebug\.client_host\=.*/xdebug\.client_host\=$XDEBUG_HOST/g" /usr/local/etc/php/conf.d/xdebug.ini
fi

"$COMMANDS_DIR"/exec.sh sed -i -e 's/^\;zend_extension/zend_extension/g' /usr/local/etc/php/conf.d/docker-php-ext-xdebug.ini

if [[ "$MACHINE" == "mac" ]]; then
    print_warning "Copying generated code into host \n"
    "$COMMANDS_DIR"/copy-from-container.sh -f generated
fi

"$COMMANDS_DIR"/restart.sh phpfpm

print_warning "xdebug configuration:\n"
print_warning "------------------------------------------------\n"
"$COMMANDS_DIR"/exec.sh php -i | grep -e "xdebug.idekey" -e "xdebug.client_host" -e "xdebug.client_port" | cut -d= -f1-2
print_warning "------------------------------------------------\n"
