#!/usr/bin/env bash

source "$COMPONENTS_DIR"/input_info.sh
source "$COMPONENTS_DIR"/print_message.sh

# Get Magento version
if [ -z "$MAGENTO_VERSION" ]; then
    if [ -f "$MAGENTO_DIR/composer.lock" ]; then
        MAGENTO_VERSION=$(cat <"$MAGENTO_DIR/composer.lock" |
        jq -r '.packages | map(select(.name == "magento/product-community-edition"))[].version')
    fi

    if [ -z "$MAGENTO_VERSION" ]; then
        get_magento_version
    fi
fi

# Default configuration
command_arguments="--db-host=db \
--backend-frontname=admin \
--use-rewrites=1 \
--db-name=magento \
--db-user=magento \
--db-password=magento \
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
    # Remove existing env.php file
    if [ -f "$MAGENTO_DIR/app/etc/env.php" ]; then
        rm -rf "$MAGENTO_DIR/app/etc/env.php"
    fi

    # If config.php file exists, create a backup and remote it
    if [ -f "$MAGENTO_DIR/app/etc/config.php" ]; then
        mv "$MAGENTO_DIR/app/etc/config.php" "$MAGENTO_DIR/app/etc/_config.php"
    fi

    config=$(cat <"$DATA_DIR/config.json" | jq -r 'to_entries | map("--" + .key + "=" + .value ) | join(" ")') 
    "$COMMANDS_DIR"/magento.sh setup:install $command_arguments $config
    "$COMMANDS_DIR"/magento.sh config:set --scope=default --scope-code=0 system/full_page_cache/caching_application 2

    # If config.php backup file exists, restore it
    if [ -f "$MAGENTO_DIR/app/etc/_config.php" ]; then
        rm "$MAGENTO_DIR/app/etc/config.php"
        mv "$MAGENTO_DIR/app/etc/_config.php" "$MAGENTO_DIR/app/etc/config.php"
    fi
}

#
# Get base url
#
get_base_url() {
    source "$COMPONENTS_DIR"/input_info.sh
    get_domain "$@"
    command_arguments="$command_arguments --base-url=https://$DOMAIN/ --base-url-secure=https://$DOMAIN/"
}

#
# Get arguments for setup-install command
#
get_argument_command() {
    argument=$(jq -r '.["'$1'"]' "$DATA_DIR/config.json")
    
    if ! $USE_DEFAULT_SETTINGS; then
        print_question "Define $1 " "$argument"
        read -r response

        if [[ $response != '' ]]; then
            argument=$response
        fi
    fi

    RESULT=$(cat < "$DATA_DIR/config.json" | jq --arg ARGUMENT "$argument" '. | ."'"$1"'"=$ARGUMENT')

    echo "$RESULT" > "$DATA_DIR/config.json"
}

#
# Get config and run command
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
