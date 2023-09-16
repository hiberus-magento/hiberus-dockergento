#!/usr/bin/env bash

set -euo pipefail

source "$COMPONENTS_DIR"/input_info.sh
source "$COMPONENTS_DIR"/print_message.sh
source "$HELPERS_DIR"/docker.sh

config_file="$HOME/.hm_resources"

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

#
# Prepare basic configuration for setup:install magento command
#
prepare_basic_config() {
    # Get Magento version
    if [ -z "${MAGENTO_VERSION:-""}" ]; then
        if [ -f "$MAGENTO_DIR/composer.lock" ]; then
            MAGENTO_VERSION=$(cat <"$MAGENTO_DIR/composer.lock" |
            jq -r '.packages | map(select(.name == "magento/product-community-edition"))[].version')
        fi

        if [ -z "${MAGENTO_VERSION:-""}" ]; then
            get_magento_version
        fi
    fi

    # Default configuration
    
    if  [[ $MAGENTO_VERSION != 2.3.* ]]; then
        command_arguments="$command_arguments \
            --elasticsearch-host=search \
            --elasticsearch-port=9200 \
            --elasticsearch-username=admin \
            --elasticsearch-password=admin"
    fi

    if  [[ $MAGENTO_VERSION == 2.4.6* ]]; then
        command_arguments="$command_arguments \
            --search-engine=opensearch \
            --opensearch-host=search \
            --opensearch-port=9200 \
            --opensearch-username=admin \
            --opensearch-password=admin"
    fi
}

#
# Run magento setup:install command
#
run_install_magento_command() {
    print_info "Starting magento setup:install...\n"
    # Remove existing env.php file
    if [ -f "$MAGENTO_DIR/app/etc/env.php" ]; then
        rm -rf "$MAGENTO_DIR/app/etc/env.php"
    fi

    config=$(jq -r 'to_entries | map("--" + .key + "=" + .value ) | join(" ")' "$config_file/config.json")
    
    "$COMMANDS_DIR"/magento.sh setup:install --dry-run $command_arguments $config
    "$COMMANDS_DIR"/magento.sh config:set --scope=default --scope-code=0 system/full_page_cache/caching_application 2
}

#
# Get base url
#
get_base_url() {
    source "$COMPONENTS_DIR"/input_info.sh
    get_domain "${DOMAIN:-}"
    command_arguments="$command_arguments --base-url=https://$DOMAIN/ --base-url-secure=https://$DOMAIN/"
}

#
# Get arguments for setup-install command
#
get_argument_command() {
    argument=$(jq -r '.["'$1'"]' "$config_file/config.json")
  
    custom_question "Define $1" "$argument"
    if [[ $REPLY != '' ]]; then
        argument=$REPLY
    fi

    result=$(cat < "$config_file/config.json" | jq --arg ARGUMENT "$argument" '. | ."'"$1"'"=$ARGUMENT')

    echo "$result" > "$config_file/config.json"
}

#
# Get config and run command
#
get_config() {
    if [[ ! -f "$config_file/config.json" ]]; then
        [[ ! -d "$config_file" ]] && mkdir -p "$config_file"
        
        echo "{}" > "$config_file/config.json"

        conf=$(cat "$config_file"/config.json | jq '{
            "language": "es_ES",
            "currency": "EUR",
            "timezone": "Europe/Madrid",
            "admin-firstname": "hiberus",
            "admin-lastname": "hiberus",
            "admin-email": "noreply@hiberus.com",
            "admin-user": "hiberus",
            "admin-password": "Hiberus123"
        }')
        echo $conf | jq '.' > "$config_file"/config.json
    fi

    if ${use_default_settings:-false}; then
        return
    fi
    
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
    is_run_service
    prepare_basic_config
    get_base_url
    get_config
    run_install_magento_command
}

# Process options
while getopts ":u" options; do
    case "$options" in
        u)
            # Force
            use_default_settings=true
        ;;
        ?)
            print_error "The command is not correct\n\n"
            print_info "Use this format\n"
            source "$HELPERS_DIR"/print_usage.sh
            get_usage "$(basename ${0%.sh})"
            exit 1
        ;;
    esac
done

init
