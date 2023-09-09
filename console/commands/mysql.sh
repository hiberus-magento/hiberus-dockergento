#!/usr/bin/env bash
set -euo pipefail

source "$COMPONENTS_DIR"/print_message.sh
source "$TASKS_DIR"/set_magento_configs.sh
source "$HELPERS_DIR"/docker.sh

# Check if db service is running
is_run_service "db"

clean_definers=false

#
# Execute query in mysql container
#
query() {
    echo -e "$1" | docker exec -i $mysql_container bash -c "mysql -u\"root\" -p\"\$MYSQL_ROOT_PASSWORD\" \"\$MYSQL_DATABASE\""
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
    $DOCKER_COMPOSE exec db bash -c "mysql -u\"root\" -p\"\$MYSQL_ROOT_PASSWORD\" \"\$MYSQL_DATABASE\""
}

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
                get_usage "$(basename ${0%.sh})"
                exit 1
            ;;
        esac
    done
    mysql_execute
fi