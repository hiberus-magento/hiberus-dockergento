#!/usr/bin/env bash
set -euo pipefail

source "$COMPONENTS_DIR"/print_message.sh

mysql_container=$(docker ps -qf "name=db")
clean_definers=false

#
# Execute query in mysql container
#
query() {
    echo -e "$1" | docker exec -i $mysql_container bash -c "mysql -u\"root\" -p\"\$MYSQL_ROOT_PASSWORD\" \"\$MYSQL_DATABASE\""
}

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

#
# Mysql execute
#
mysql_execute() {
    # Import option
    if [[ -n ${import_database:=""} ]]; then
        set_current_domain
        # Check if there is to delete DEFINER and import database
        if $clean_definers ; then
            cleaned=${import_database/".sql"/"-cleaned.sql"}
            cat $import_database | sed 's/DEFINER=[^*]*\*/\*/g' > $cleaned
            docker exec -i $mysql_container bash -c "mysql -u\"root\" -p\"\$MYSQL_ROOT_PASSWORD\" \"\$MYSQL_DATABASE\"" < $cleaned
        else
            # Only import database  
            docker exec -i $mysql_container bash -c "mysql -u\"root\" -p\"\$MYSQL_ROOT_PASSWORD\" \"\$MYSQL_DATABASE\"" < $import_database
        fi
        set_settings_for_develop
        exit
    fi
    # Go into mysql container
    docker exec -i $mysql_container bash -c "mysql -u\"root\" -p\"\$MYSQL_ROOT_PASSWORD\" \"\$MYSQL_DATABASE\""
}

if [ -z "$mysql_container" ]; then
    print_error "Error: DB container is not running\n"
    exit 1
fi

# If stdin has content
if [ ! -t 0 ]; then
    docker exec -i $mysql_container bash -c "mysql -u\"root\" -p\"\$MYSQL_ROOT_PASSWORD\" \"\$MYSQL_DATABASE\""
else
    while getopts ":i:q:d" options; do
        case "$options" in
            i)
                # Import database
                import_database=${OPTARG/"~"/$HOME}
                if [[ ! -f $import_database ]]; then
                    print_warning "No such file: $OPTARG\n"
                    exit 0
                fi
            ;;
            q)
                # Query
                query "$OPTARG"
                exit
            ;;
            d)
                # Clean DEFINER
                clean_definers=true
            ;;
            ?)
                print_error "The command is not correct\n\n"
                print_info "Use this format\n"
                source "$HELPERS_DIR"/print_usage.sh
                get_usage "mysql"
                exit 1
            ;;
        esac
    done
    mysql_execute
fi