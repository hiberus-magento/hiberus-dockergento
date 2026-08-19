#!/usr/bin/env bash
set -euo pipefail

source "$COMPONENTS_DIR"/print_message.sh
source "$TASKS_DIR"/set_magento_configs.sh
source "$HELPERS_DIR"/docker.sh
# Resolve the db container within the current compose project. `docker ps -f name=db`
# matches by substring across ALL projects on the host, so with more than one
# environment up it could return several ids (breaking `docker exec`) or target
# another project's database.
mysql_container=$($DOCKER_COMPOSE ps -q db)

# Check if db service is running
is_run_service "db"

# MariaDB 11+ removed the legacy `mysql` client name; use `mariadb` when present
# and fall back to `mysql` for older images (10.2-10.4). Resolved inside the
# container, where $MYSQL_ROOT_PASSWORD / $MYSQL_DATABASE are expanded.
db_client='client=$(command -v mariadb || command -v mysql); "$client" -u"root" -p"$MYSQL_ROOT_PASSWORD" "$MYSQL_DATABASE"'

#
# Execute query in mysql container
#
query() {
    docker exec -e QUERY="$1" $mysql_container bash -c "$db_client"' -e "$QUERY"'
}

#
# Anonymise database
#
anonymise() {
    source "$COMPONENTS_DIR"/masquerade.sh
    print_info "Anonymising database in localhost...\n"
    masquerade_run
}

#
# Mysql execute
#
mysql_execute() {
    # Import option
    if [[ -n ${import_database:=""} ]]; then
        set_current_domain
        # Check if DEFINER has to be deleted and import database
        if ${clean_definers-false} ; then
            cleaned=${import_database/".sql"/"-cleaned.sql"}
            cat $import_database | sed 's/DEFINER=[^*]*\*/\*/g' > $cleaned
            print_info "Importing database from file with cleaned up definers ...\n"
            docker exec -i $mysql_container bash -c "$db_client" < $cleaned
        else
            # Only import database  
            print_info "Importing database from file ...\n"
            docker exec -i $mysql_container bash -c "$db_client" < $import_database
        fi

        if ${anonymisation-false} ; then
            anonymise
        fi
        set_settings_for_develop
        exit
    fi
    # Go into mysql container
    $DOCKER_COMPOSE exec db bash -c "$db_client"
}

# Only treat a non-tty stdin as a dump to import when no options were given
# (e.g. `hm mysql < dump.sql`). Non-interactive callers (CI, agents) invoking
# `hm mysql -q ...` never have a tty either, so gating solely on `[ ! -t 0 ]`
# made -q/-i unreachable for them.
if [[ $# -eq 0 ]] && [ ! -t 0 ]; then
    print_info "Importing database from stdin ...\n"
    docker exec -i $mysql_container bash -c "$db_client"
else
    while getopts ":i:q:d:a" options; do
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
            a)
                # Anonymisation
                anonymisation=true
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
    mysql_execute
fi