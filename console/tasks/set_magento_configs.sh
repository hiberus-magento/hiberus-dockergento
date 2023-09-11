#!/usr/bin/env bash
set -euo pipefail

#
# Set Domain in properties project file if DOMAIN variable is not setted
#
set_current_domain() {
    # Check if magento database is initialized
    exist_magento_db=$(query "SHOW TABLES LIKE 'core_config_data';")

    if [[ -n $exist_magento_db && -z ${DOMAIN+x} ]]; then
        # Get current domain
        url=$(query "SELECT value FROM core_config_data WHERE path='web/secure/base_url';")

        # Save domian
        DOMAIN=$(awk -F/ '{print $3}' <<< $url)

         # Add domain in properties project file
        echo "  DOMAIN=\"$DOMAIN\"" >> $DOCKER_CONFIG_DIR/properties
    fi
}

#
# Execute all neccesary commands to prepare local environment 
#
set_settings_for_develop() {
    source "$COMPONENTS_DIR"/print_message.sh

    # Get domain if not exists
    if [[ -z ${DOMAIN+x} ]]; then
        source "$COMPONENTS_DIR"/input_info.sh
        get_domain
    fi

    if [[ ! -f "$DOCKER_CONFIG_DIR"/local_settings.json ]]; then
        cp "$DATA_DIR"/local_settings.json "$DOCKER_CONFIG_DIR"/local_settings.json
    fi

    option_keys=$(jq -r 'keys[]' < "$DOCKER_CONFIG_DIR"/local_settings.json)

    for key in $option_keys; do
        # Replace '_' to ':' to get magento format command
        magento_command=$(echo $key | sed 's/_/:/')

        # Get all options to configurate
        command_data=$(jq -r '."'$key'"[] | 
            select(.value == "URL").value = "https://'$DOMAIN'/" |
            select(.value == "DOMAIN").value = "'$DOMAIN'" |
            if .scope then "'$magento_command'%" + "--scope=" + .scope + "%--scope-code=" + .["scope-code"] + "%" + .path + "%" + .value else "'$magento_command'%" + .path + "%" + .value end' "$DOCKER_CONFIG_DIR"/local_settings.json)
        # Replace '=' to ' ' and execute each command
        for data in $command_data; do
            final_command=$(echo "$data" | sed 's/%/ /g')
            print_processing "bin/magento $final_command"
            "$COMMANDS_DIR"/magento.sh $final_command
        done
    done
}