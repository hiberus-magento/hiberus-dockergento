#!/bin/bash
set -euo pipefail

source "$COMPONENTS_DIR"/input_info.sh
source "$COMPONENTS_DIR"/print_message.sh
source "$COMPONENTS_DIR"/masquerade.sh
source "$HELPERS_DIR"/docker.sh

# Check php container
is_run_service "phpfpm"
# Check mysql container
is_run_service "db"

ssh_host="ssh.eu-3.magento.cloud"
ssh_user=""
sql_host="database.internal"
sql_port="3306"
sql_user="mysql"
sql_db="main"
sql_password=""
anonymise="prompt"

print_info "Database transfer assistant: \n"

for i in "$@"; do
    case $i in
    --ssh-host=*)
        ssh_host="${i#*=}" && shift
        ;;
    --ssh-user=*)
        ssh_user="${i#*=}" && shift
        ;;
    --sql-host=*)
        sql_host="${i#*=}" && shift
        ;;
    --sql-port=*)
        sql_port="${i#*=}" && shift
        ;;
    --sql-user=*)
        sql_user="${i#*=}" && shift
        ;;
    --sql-db=*)
        sql_db="${i#*=}" && shift
        ;;
    --sql-password=*)
        sql_password="${i#*=}" && shift
        ;;
    --anonymise=*)
        anonymise="${i#*=}" && shift
        ;;
    -* | --* | *) ;;
    esac
done

# Request SSH credentials.
# In non-interactive mode every prompt is skipped: the answers already fall back to the
# values passed as options, so an empty answer is exactly the right behaviour.
input_ssh_host=""
input_ssh_user=""
input_sql_host=""
input_sql_port=""
input_sql_user=""
input_sql_db=""
input_sql_password=""
ssh_tunnel=""
sql_exclude=""

if ! is_non_interactive; then
    confirm "Do you need to use SSH tunneling? [Y/n]: "
    ssh_tunnel="$REPLY"
fi

if [ -z "$ssh_tunnel" ] || [ "$ssh_tunnel" == "Y" ] || [ "$ssh_tunnel" == "y" ]; then
    if ! is_non_interactive; then
        read -p "$(print_question "SSH Host" "$ssh_host")" input_ssh_host
        read -p "$(print_question "SSH User" "$ssh_user")" input_ssh_user
    fi
    ssh_host=${input_ssh_host:-${ssh_host}}
    ssh_user=${input_ssh_user:-${ssh_user}}
else
    ssh_host=""
    ssh_user=""
fi

# Request Database credentials
if ! is_non_interactive; then
    read -p "$(print_question "Database Host" "$sql_host")" input_sql_host
    read -p "$(print_question "Database Port" "$sql_port")" input_sql_port
    read -p "$(print_question "Database User" "$sql_user")" input_sql_user
    read -p "$(print_question "Database DB Name" "$sql_db")" input_sql_db
    # No echo: this is a client environment's database password and it would otherwise stay
    # on screen and in the terminal's scrollback. The explicit newline is needed because
    # without echo the user's Enter leaves none.
    read -rsp "$(print_question "Database Password" "$sql_password")" input_sql_password
    printf '\\n'
fi
sql_host=${input_sql_host:-${sql_host}}
sql_port=${input_sql_port:-${sql_port}}
sql_user=${input_sql_user:-${sql_user}}
sql_db=${input_sql_db:-${sql_db}}
sql_password=${input_sql_password:-${sql_password}} 

# Prepare password
[ -z "$sql_password" ] && sql_password="" || sql_password="-p'$sql_password'"

# Request SSH credentials
if ! is_non_interactive; then
    confirm "Do you want to exclude 'core_config_data' table? [Y/n]: "
    sql_exclude="$REPLY"
fi

if [ -z "$sql_exclude" ] || [ "$sql_exclude" == "Y" ] || [ "$sql_exclude" == "y" ]; then
    sql_exclude=1
else
    sql_exclude=0
fi

print_info "You are going to transfer database from [${ssh_host}:[${sql_host}:${sql_port}]] to [LOCALHOST].\n"
if ! is_non_interactive; then
    read -rp "$(print_default "Press any key continue...")"
fi

# Check required data
if [ -z "$sql_host" ] || [ -z "$sql_port" ] || [ -z "$sql_user" ] || [ -z "$sql_db" ]; then
    hm_fail "$HM_EXIT_USAGE" "input_required" \
        "Missing connection data for the remote database" \
        "$COMMAND_BIN_NAME transfer-db --help"
fi

print_info "Creating database dump from origin server...\n"

# Create database dump from origin server (WITHOUT SSH TUNNEL)
if [ -z "$ssh_host" ]; then

    # Create dump into mysql container.
    # --single-transaction: consistent, LOCK-FREE snapshot (safe on a live/high-traffic
    #   InnoDB DB — avoids the table locks that can take production down).
    # --quick: stream row by row (low memory on large tables).
    # --no-tablespaces: don't require the PROCESS privilege (common on managed/Cloud DBs).
    $DOCKER_COMPOSE exec db bash -c "dump=\$(command -v mariadb-dump || command -v mysqldump); \"\$dump\" --single-transaction --quick --no-tablespaces --skip-comments -h'$sql_host' -u'$sql_user' -P $sql_port $sql_password $sql_db | sed -e 's/DEFINER[ ]*=[ ]*[^*]*\*/\*/' | gzip -9 > /tmp/db.sql.gz"

# Create database dump from origin server (WITH SSH TUNNEL)
else

    ssh ${ssh_user}@${ssh_host} \
        "dump=\$(command -v mariadb-dump || command -v mysqldump); \"\$dump\" --single-transaction --quick --no-tablespaces --skip-comments -h'$sql_host' -u'$sql_user' -P $sql_port $sql_password $sql_db \
        | sed -e 's/DEFINER[ ]*=[ ]*[^*]*\*/\*/' \
        | gzip -9" \
        > db.sql.gz

    # Copy dump into mysql container
    docker cp db.sql.gz "$($DOCKER_COMPOSE ps -q db | awk '{print $1}')":/tmp/db.sql.gz

fi

print_info "Restoring database dump into localhost...\n"

# Restore dump
[ $sql_exclude -eq 1 ] && $DOCKER_COMPOSE exec db bash -c "dump=\$(command -v mariadb-dump || command -v mysqldump); \"\$dump\" -u\$MYSQL_USER -p\$MYSQL_PASSWORD \$MYSQL_DATABASE core_config_data admin_user tfa_user_config > /tmp/ccd.sql 2> /dev/null"
$DOCKER_COMPOSE exec db bash -c "client=\$(command -v mariadb || command -v mysql); zcat /tmp/db.sql.gz | sed '/sandbox mode/d' | \"\$client\" -f -u\$MYSQL_USER -p\$MYSQL_PASSWORD \$MYSQL_DATABASE"
[ $sql_exclude -eq 1 ] && $DOCKER_COMPOSE exec db bash -c "client=\$(command -v mariadb || command -v mysql); [ -f /tmp/ccd.sql ] && \"\$client\" -f -u\$MYSQL_USER -p\$MYSQL_PASSWORD \$MYSQL_DATABASE < /tmp/ccd.sql"

# Anonymise database
if [ "$anonymise" = "true" ]; then
    print_info "Anonymising database in localhost...\n"
    masquerade_run
elif [ "$anonymise" = "false" ]; then
    print_info "Skipping database anonymisation...\n"
else
    anonymise_prompt=""
if ! is_non_interactive; then
    read -p "$(print_question "Do you want to anonymise the database? [Y/n]: ")" anonymise_prompt
fi
    if [ -z "$anonymise_prompt" ] || [ "$anonymise_prompt" == "Y" ] || [ "$anonymise_prompt" == "y" ]; then
        print_info "Anonymising database in localhost...\n"
        masquerade_run
    else
        print_info "Skipping database anonymisation...\n"
    fi
fi

# Import shared configuration (app/etc/config.php) into the database.
# Prevents "config import" / stale-configuration errors after importing a DB from
# another environment. Safe/idempotent: prints "Nothing to import" when there is
# no shared config to apply.
config_import_magento=""
if ! is_non_interactive; then
    read -p "$(print_question "Do you want to import app config (app:config:import)? [Y/n]: ")" config_import_magento
fi
if [ -z "$config_import_magento" ] || [ "$config_import_magento" == 'Y' ] || [ "$config_import_magento" == 'y' ]; then
    $DOCKER_COMPOSE exec phpfpm bin/magento app:config:import
fi

# Reindex Magento
reindex_magento=""
if ! is_non_interactive; then
    read -p "$(print_question "Do you want to reindex Magento? [Y/n]: ")" reindex_magento
fi
if [ -z "$reindex_magento" ] || [ "$reindex_magento" == 'Y' ] || [ "$reindex_magento" == 'y' ]; then
    $DOCKER_COMPOSE exec phpfpm bin/magento indexer:reindex
fi

# Clear Magento cache
clear_magento=""
if ! is_non_interactive; then
    read -p "$(print_question "Do you want to clear Magento cache? [Y/n]: ")" clear_magento
fi
if [ -z "$clear_magento" ] || [ "$clear_magento" == 'Y' ] || [ "$clear_magento" == 'y' ]; then
    $DOCKER_COMPOSE exec phpfpm bin/magento cache:flush
fi

print_info " All done!\n"
