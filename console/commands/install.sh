#!/usr/bin/env bash

# shellcheck source=/dev/null
source "$COMPONENTS_DIR"/input_info.sh
# shellcheck source=/dev/null
source "$COMPONENTS_DIR"/print_message.sh

yml_file="$MAGENTO_DIR/docker-compose.yml"
path_to_mysql_keys="services_db_environment_MYSQL"
look_at_task="$TASKS_DIR/look_at_yml.sh"

# Get credentials
user=$($look_at_task "$yml_file" "${path_to_mysql_keys}_USER")
password=$($look_at_task "$yml_file" "${path_to_mysql_keys}_PASSWORD")
database=$($look_at_task "$yml_file" "${path_to_mysql_keys}_DATABASE")

# Default configuration
command_arguments="--db-host=db \
--backend-frontname=admin \
--elasticsearch-host=search \
--use-rewrites=1 \
--elasticsearch-port=9200 \
--db-name=$database \
--dber=$user \
--db-password=$password \
--elasticsearch-username=admin \
--elasticsearch-password=admin"

#
# Run magento setup:install command
#
run_install_magento_command() {
    config=$(cat <"$DATA_DIR/config.json" | jq -r 'to_entries | map("--" + .key + "=" + .value ) | join(" ")') 
    $COMMANDS_DIR/magento.sh setup:install $command_arguments $config

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

    print_question "Define $1: "
    if [ null != "$argument" ]; then
        print_default "[ $argument ] "
    fi

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
    # Pendding to confirm
    # get_argument_command "search-engine"
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
