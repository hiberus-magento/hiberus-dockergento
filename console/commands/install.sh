#!/usr/bin/env bash

# shellcheck source=/dev/null
source "$COMPONENTS_DIR"/input_info.sh
source "$COMPONENTS_DIR"/print_message.sh

yml_file="$MAGENTO_DIR/docker-compose.yml"
path_to_mysql_keys="services_db_environment_MYSQL"
look_at_task="$TASKS_DIR/look_at_yml.sh"

# Get credentials
user=$($look_at_task "$yml_file" "${path_to_mysql_keys}_USER")
password=$($look_at_task "$yml_file" "${path_to_mysql_keys}_PASSWORD")
database=$($look_at_task "$yml_file" "${path_to_mysql_keys}_DATABASE")

# Get Magento version
if [ -z $MAGENTO_VERSION ]; then
    if [ -f "$MAGENTO_DIR/composer.lock" ]; then
    MAGENTO_VERSION=$(cat <"$MAGENTO_DIR/composer.lock" |
        jq -r '.packages | map(select(.name == "magento/product-community-edition"))[].version')
    fi
    if [ -z $MAGENTO_VERSION ]; then
        get_magento_version
    fi
fi

# Default configuration
command_arguments="--db-host=db \
--backend-frontname=admin \
--use-rewrites=1 \
--db-name=$database \
--db-user=$user \
--db-password=$password \
--session-save=redis \
--session-save-redis-host=redis \
--session-save-redis-db=0 \
--session-save-redis-disable-locking=1 \
--cache-backend=redis \
--cache-backend-redis-server=redis \
--cache-backend-redis-db=1 \
--page-cache=redis \
--page-cache-redis-server=redis \
--page-cache-redis-db=2 \
--amqp-host=rabbitmq \
--amqp-port=5672 \
--amqp-user=user \
--amqp-password=password"

if  [[ $MAGENTO_VERSION != 2.3.* ]]; then
    command_arguments="$command_arguments \
    --elasticsearch-host=search \
    --elasticsearch-port=9200 \
    --elasticsearch-username=admin \
    --elasticsearch-password=admin"
fi

#
# Run magento setup:install command
#
run_install_magento_command() {
    config=$(cat <"$DATA_DIR/config.json" | jq -r 'to_entries | map("--" + .key + "=" + .value ) | join(" ")') 
    $COMMANDS_DIR/magento.sh setup:install $command_arguments $config
    $COMMANDS_DIR/magento.sh config:set --scope=default --scope-code=0 system/full_page_cache/caching_application 2
}

#
# Get base url
#
get_base_url() {
    # shellcheck source=/dev/null 
    source "$COMPONENTS_DIR"/input_info.sh

    get_domain "$@"

    command_arguments="$command_arguments --base-url=https://$DOMAIN/"
}

#
# Get arguments for setup-install command
#
get_argument_command() {
    argument=$(cat <"$DATA_DIR/config.json" | jq -r '."'"$1"'"')

    print_question "Define $1 " "$argument"
    read -r response

    if [[ $response != '' ]]; then
        argument=$response
    fi

    RESULT=$(cat <"$DATA_DIR/config.json" | jq --arg ARGUMENT "$argument" '. | ."'"$1"'"=$ARGUMENT')

    echo "$RESULT" > "$DATA_DIR/config.json"
}

#
# Get config and run comand
#
get_config() {
    get_argument_command "language"
    get_argument_command "currency"
    get_argument_command "timezone"
    get_argument_command "admin-firstname"
    get_argument_command "admin-lastname"
    get_argument_command "admin-email"
    get_argument_command "admin-user"
    get_argument_command "admin-password"
}

#
# Initialize script
#
init() {
    get_base_url "$@"
    get_config
    run_install_magento_command
}

init "$@"
