#!/usr/bin/env bash

YML_FILE="${MAGENTO_DIR}/docker-compose.yml"
PAHT_TO_MYSQL_KEYS="services_db_environment_MYSQL"
LOOK_TASK="${TASKS_DIR}/look_at_yml.sh"

# Get credentials
USER=$(${LOOK_TASK} "${YML_FILE}" "${PAHT_TO_MYSQL_KEYS}_USER")
PASSWORD=$(${LOOK_TASK} "${YML_FILE}" "${PAHT_TO_MYSQL_KEYS}_PASSWORD")
DATABASE=$(${LOOK_TASK} "${YML_FILE}" "${PAHT_TO_MYSQL_KEYS}_DATABASE")

# Default configuration
COMMAND_ARGUMENTS="--db-host=db \
--backend-frontname=admin \
--elasticsearch-host=elasticsearch \
--use-rewrites=1 \
--elasticsearch-port=9200 \
--db-name=${DATABASE} \
--db-user=${USER} \
--db-password=${PASSWORD} \
--elasticsearch-username=${USER} \
--elasticsearch-password=${PASSWORD}"

#
# Run magento setup:install command
#
run_install_magento_command() {
    CONFIG=$(cat <"${DATA_DIR}/config.json" | jq -r 'to_entries | map("--" + .key + "=" + .value ) | join(" ")') 
    "${COMMAND_BIN_NAME}" magento setup:install "$COMMAND_ARGUMENTS" "$CONFIG"
}

#
# Get base url
#
get_base_url() {
    # shellcheck source=/dev/null 
    source "${COMPONENTS_DIR}/input_info.sh"

    get_domain "$@"

    COMMAND_ARGUMENTS="$COMMAND_ARGUMENTS --base-url=https://${DOMAIN}/"
}

#
# Get arguments for setup-install command
#
get_argument_command() {
    ARGUMENT=$(cat <"${DATA_DIR}/config.json" | jq -r '."'"$1"'"')

    if [ null != "$ARGUMENT" ]; then
        printf "%bDefine %s: %b[ %s ] " "${BLUE}" "$1" "${COLOR_RESET}" "${ARGUMENT}"
    else
        printf "%bDefine %s: %b" "${BLUE}" "$1" "${COLOR_RESET}"
    fi

    read -r RESPONSE

    if [[ $RESPONSE != '' ]]; then
        ARGUMENT=$RESPONSE
    fi

    RESULT=$(cat <"${DATA_DIR}/config.json" | jq --arg ARGUMENT "$ARGUMENT" '. | ."'"$1"'"=$ARGUMENT')

    echo "${RESULT}" >"${DATA_DIR}/config.json"
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
