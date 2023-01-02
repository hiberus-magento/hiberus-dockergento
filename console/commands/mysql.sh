#!/usr/bin/env bash
set -euo pipefail
docker_config_dir="config/docker"
MYSQL_CONTAINER=$(docker ps -qf "name=db")
MYSQL_QUERY=${*:1}

#
# Execute query in mysql container
#
function query() {
    echo -e "$1" | docker exec -i $MYSQL_CONTAINER bash -c "mysql -u\"root\" -p\"\$MYSQL_ROOT_PASSWORD\" \"\$MYSQL_DATABASE\""
}

#
# Set Domain in properties project file if DOMAIN variable is not setted
#
function setCurrentDomain() {
    # Check if magento database is initialized
    exist_magento_db=$(query "SHOW TABLES LIKE 'core_config_data';")

    if [[ -n $exist_magento_db && -z ${DOMAIN+x} ]]; then
        # Get current domain
        url=$(query "SELECT value FROM core_config_data WHERE path='web/secure/base_url';")

        # Save domian
        DOMAIN=$(awk -F/ '{print $3}' <<< $url)

         # Add domain in properties project file
        echo "  DOMAIN=\"$DOMAIN\"" >> $docker_config_dir/properties
    fi
}

#
# Execute all neccesary commands to prepare local environment 
#
function setSettingsForDovelop() {
    source "$COMPONENTS_DIR"/print_message.sh

    # Get domain if not exists
    if [[ -z ${DOMAIN+x} ]]; then
        source "$COMPONENTS_DIR"/input_info.sh
        get_domain
    fi

    command_keys=$(jq -r 'keys[]' < "$DATA_DIR"/local_settings.json)
    for key in $command_keys; do
        # Replace '_' to ':' to get magento format command
        magento_command=$(echo $key | sed 's/_/:/')
        # Get all options to configurate
        command_data=$(jq -r '."'"$key"'"[] | 
            select(.value == "URL").value = "https://'$DOMAIN'/" |
            select(.value == "DOMAIN").value = "'$DOMAIN'" |
            "'$magento_command'=" + .path + "=" + .value' < "$DATA_DIR"/local_settings.json)
        # Replace '=' to ' ' and execute each command
        for data in $command_data; do
            final_command=$(echo "$data" | sed 's/=/ /g')
            print_processing "bin/magento $final_command"
            "$COMMANDS_DIR"/magento.sh $final_command
        done
    done
}

if [ -z "$MYSQL_CONTAINER" ]; then
    print_error "Error: DB container is not running\n"
    exit 1
fi

if [ ! -t 0 ]; then
    setCurrentDomain
    MYSQL_QUERY=$(cat)
    MYSQL_QUERY=$(echo "$MYSQL_QUERY" | sed 's/DEFINER=[^*]*\*/\*/g')
fi

if [ ! -z "$MYSQL_QUERY" ]; then
    query "$MYSQL_QUERY"
    if [ ! -t 0 ]; then
        setSettingsForDovelop
    fi
else
    docker exec -it $MYSQL_CONTAINER bash -c "mysql -u\"root\" -p\"\$MYSQL_ROOT_PASSWORD\" \"\$MYSQL_DATABASE\""
fi