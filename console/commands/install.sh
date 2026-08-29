#!/usr/bin/env bash

set -euo pipefail

source "$COMPONENTS_DIR"/input_info.sh
source "$COMPONENTS_DIR"/print_message.sh
source "$HELPERS_DIR"/docker.sh
source "$HELPERS_DIR"/version.sh
source "$TASKS_DIR"/admin_bootstrap.sh

# Cache/session backend service name. Magento 2.4.9+ ships Valkey (a drop-in Redis
# replacement) and some environments name the compose service "valkey" instead of
# "redis". Detect the real service so setup:install connects to the right host.
cache_host="redis"
if [ -n "${DOCKER_COMPOSE:-}" ] && $DOCKER_COMPOSE config --services 2>/dev/null | grep -qx "valkey"; then
    cache_host="valkey"
fi

command_arguments="--db-host=db \
    --backend-frontname=admin \
    --use-rewrites=1 \
    --db-name=magento \
    --db-user=magento \
    --db-password=magento \
    --session-save=redis \
    --session-save-redis-host=$cache_host \
    --session-save-redis-db=0 \
    --session-save-redis-disable-locking=1 \
    --cache-backend=redis \
    --cache-backend-redis-server=$cache_host \
    --cache-backend-redis-db=1 \
    --page-cache=redis \
    --page-cache-redis-server=$cache_host \
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

    if [ -z "${EQUIVALENT_VERSION:-""}" ]; then
        export EQUIVALENT_VERSION=${MAGENTO_VERSION%-*}
    fi

    # Default configuration
    
    if  [[ $MAGENTO_VERSION != 2.3.* ]]; then
        command_arguments="$command_arguments \
            --elasticsearch-host=search \
            --elasticsearch-port=9200 \
            --elasticsearch-username=admin \
            --elasticsearch-password=admin"
    fi

    # OpenSearch is required for Magento >= 2.4.6
    if version_gte "${EQUIVALENT_VERSION}" "2.4.6"; then
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
    # Remove existing env.php file
    if [ -f "$MAGENTO_DIR/app/etc/env.php" ]; then
        rm -rf "$MAGENTO_DIR/app/etc/env.php"
    fi

    # If config.php file exists, create a backup and remote it
    if [ -f "$MAGENTO_DIR/app/etc/config.php" ]; then
        mv "$MAGENTO_DIR/app/etc/config.php" "$MAGENTO_DIR/app/etc/_config.php"
    fi

    #
    # A generated password is used for the install and never written down.
    #
    # data/config.json lives in the tool's directory, is shared by every project and records what
    # is answered: a password written there becomes the next project's default and sits in plain
    # text on disk. A password that *is* set there is respected — that is somebody's decision.
    #
    local configured_password
    configured_password=$(jq -r '."admin-password" // ""' "$DATA_DIR/config.json")

    if [ -n "$configured_password" ]; then
        HM_ADMIN_PASSWORD="$configured_password"
        HM_ADMIN_PASSWORD_GENERATED=false
    else
        hm_generate_admin_password
        HM_ADMIN_PASSWORD_GENERATED=true
    fi

    config=$(jq -r --arg password "$HM_ADMIN_PASSWORD" \
        '. + {"admin-password": $password} | to_entries | map("--" + .key + "=" + .value) | join(" ")' \
        "$DATA_DIR/config.json")

    admin_user=$(jq -r '."admin-user" // "admin"' "$DATA_DIR/config.json")

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
    get_domain "${DOMAIN:-}"
    command_arguments="$command_arguments --base-url=https://$DOMAIN/ --base-url-secure=https://$DOMAIN/"
}

#
# Get arguments for setup-install command
#
get_argument_command() {
    argument=$(jq -r '.["'$1'"]' "$DATA_DIR/config.json")
  
    custom_question "Define $1" "$argument"
    if [[ $REPLY != '' ]]; then
        argument=$REPLY
    fi

    result=$(cat < "$DATA_DIR/config.json" | jq --arg ARGUMENT "$argument" '. | ."'"$1"'"=$ARGUMENT')

    echo "$result" > "$DATA_DIR/config.json"
}

#
# Get config and run command
#
get_config() {
    if [[ ! -f "$DATA_DIR/config.json" ]]; then
        echo "{}" > "$DATA_DIR/config.json"

         conf=$(cat "$DATA_DIR"/config.json | jq '{
            "language": "es_ES",
            "currency": "EUR",
            "timezone": "Europe/Madrid",
            "admin-firstname": "hiberus",
            "admin-lastname": "hiberus",
            "admin-email": "noreply@hiberus.com",
            "admin-user": "hiberus",
            "admin-password": ""
        }')
        echo $conf | jq '.' > "$DATA_DIR"/config.json
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
}

#
# The second factor, and everything needed to log in the first time
#
finish_admin_bootstrap() {
    local user="${admin_user:-admin}"

    if hm_two_factor_enabled; then
        if ! hm_register_second_factor "$user"; then
            print_warning "Could not register the second factor for '$user'\n"
        fi
    else
        print_info "Two factor authentication is disabled in this project, so none was set up.\n"
    fi

    hm_print_admin_summary "$user" "$HM_ADMIN_PASSWORD"
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
    finish_admin_bootstrap
}

#
# `--use-default` is the same thing as `-u`. The long form is what the command documents, and
# what somebody types when they are not looking at the letter
#
install_args=()
for argument in "$@"; do
    case "$argument" in
        --use-default) install_args[${#install_args[@]}]="-u" ;;
        *)             install_args[${#install_args[@]}]="$argument" ;;
    esac
done

set -- ${install_args[@]+"${install_args[@]}"}

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
            exit "$HM_EXIT_USAGE"
        ;;
    esac
done

init
