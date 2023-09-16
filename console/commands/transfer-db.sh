#!/bin/bash
set -euo pipefail

source "$COMPONENTS_DIR"/input_info.sh
source "$COMPONENTS_DIR"/print_message.sh
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
    -* | --* | *) ;;
    esac
done

# Request SSH credentials
read -rp "$(print_question "Do you need to use SSH tunneling? [Y/n]: ")" ssh_tunnel
if [ -z "$ssh_tunnel" ] || [ "$ssh_tunnel" == "Y" ] || [ "$ssh_tunnel" == "y" ]; then
    read -p "$(print_question "SSH Host" "$ssh_host")" input_ssh_host
    read -p "$(print_question "SSH User" "$ssh_user")" input_ssh_user
    ssh_host=${input_ssh_host:-${ssh_host}}
    ssh_user=${input_ssh_user:-${ssh_user}}
else
    ssh_host=""
    ssh_user=""
fi

# Request Database credentials
read -p "$(print_question "Database Host" "$sql_host")" input_sql_host
read -p "$(print_question "Database Port" "$sql_port")" input_sql_port
read -p "$(print_question "Database User" "$sql_user")" input_sql_user
read -p "$(print_question "Database DB Name" "$sql_db")" input_sql_db
read -p "$(print_question "Database Password" "$sql_password")" input_sql_password
sql_host=${input_sql_host:-${sql_host}}
sql_port=${input_sql_port:-${sql_port}}
sql_user=${input_sql_user:-${sql_user}}
sql_db=${input_sql_db:-${sql_db}}
sql_password=${input_sql_password:-${sql_password}}hm 

# Prepare password
[ -z "$sql_password" ] && sql_password="" || sql_password="-p'$sql_password'"

# Request SSH credentials
read -rp "$(print_question "Do you want to exclude 'core_config_data' table? [Y/n]: ")" sql_exclude
if [ -z "$sql_exclude" ] || [ "$sql_exclude" == "Y" ] || [ "$sql_exclude" == "y" ]; then
    sql_exclude=1
else
    sql_exclude=0
fi

print_info "You are going to transfer database from [${ssh_host}:[${sql_host}:${sql_port}]] to [LOCALHOST].\n"
read -rp "$(print_default "Press any key continue...")"

# Check required data
if [ -z "$sql_host" ] || [ -z "$sql_port" ] || [ -z "$sql_user" ] || [ -z "$sql_db" ]; then
    print_error "Error: Please enter all required data\n"
    exit 1
fi

print_info "Creating database dump from origin server...\n"

# Create database dump from origin server (WITHOUT SSH TUNNEL)
if [ -z "$ssh_host" ]; then

    # Create dump into mysql container
    $DOCKER_COMPOSE exec db bash -c "mysqldump -h'$sql_host' -u'$sql_user' -P $sql_port $sql_password $sql_db | sed -e 's/DEFINER[ ]*=[ ]*[^*]*\*/\*/' | gzip -9 > /tmp/db.sql.gz"

# Create database dump from origin server (WITH SSH TUNNEL)
else

    # Create dump
    ssh ${ssh_user}@${ssh_host} "mysqldump -h'$sql_host' -u'$sql_user' -P $sql_port $sql_password $sql_db | sed -e 's/DEFINER[ ]*=[ ]*[^*]*\*/\*/' | gzip -9 > /tmp/db.sql.gz"

    # Download dump
    scp ${ssh_user}@${ssh_host}:/tmp/db.sql.gz .

    # Copy dump into mysql container
    docker cp db.sql.gz "$(get_container_id db)":/tmp/db.sql.gz

fi

print_info "Restoring database dump into localhost...\n"

# Restore dump
[ $sql_exclude -eq 1 ] && $DOCKER_COMPOSE exec db bash -c "mysqldump -u\$MYSQL_USER -p\$MYSQL_PASSWORD \$MYSQL_DATABASE core_config_data > /tmp/ccd.sql 2> /dev/null"
$DOCKER_COMPOSE exec db bash -c "zcat /tmp/db.sql.gz | mysql -f -u\$MYSQL_USER -p\$MYSQL_PASSWORD \$MYSQL_DATABASE"
[ $sql_exclude -eq 1 ] && $DOCKER_COMPOSE exec db bash -c "[ -f /tmp/ccd.sql ] && mysql -f -u\$MYSQL_USER -p\$MYSQL_PASSWORD \$MYSQL_DATABASE < /tmp/ccd.sql"

# Reindex Magento
read -p "$(print_question "Do you want to reindex Magento? [Y/n]: ")" reindex_magento
if [ -z "$reindex_magento" ] || [ "$reindex_magento" == 'Y' ] || [ "$reindex_magento" == 'y' ]; then
    $DOCKER_COMPOSE exec phpfpm bin/magento indexer:reindex
fi

# Clear Magento cache
read -p "$(print_question "Do you want to clear Magento cache? [Y/n]: ")" clear_magento
if [ -z "$clear_magento" ] || [ "$clear_magento" == 'Y' ] || [ "$clear_magento" == 'y' ]; then
    $DOCKER_COMPOSE exec phpfpm bin/magento cache:flush
fi

print_info " All done!\n"
