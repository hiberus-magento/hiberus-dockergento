#!/bin/bash
set -euo pipefail

source "$COMPONENTS_DIR"/input_info.sh
source "$COMPONENTS_DIR"/print_message.sh
source "$HELPERS_DIR"/docker.sh

# Check php container
is_run_service "phpfpm"
# Check mysql container
is_run_service "db"

sshHost="ssh.eu-3.magento.cloud"
sshUser=""
sqlHost="database.internal"
sqlUser="mysql"
sqlDb="main"
sqlPassword=""

print_info "Database transfer assistant: \n"

for i in "$@"; do
    case $i in
    --ssh-host=*)
        sshHost="${i#*=}" && shift
        ;;
    --ssh-user=*)
        sshUser="${i#*=}" && shift
        ;;
    --sql-host=*)
        sqlHost="${i#*=}" && shift
        ;;
    --sql-user=*)
        sqlUser="${i#*=}" && shift
        ;;
    --sql-db=*)
        sqlDb="${i#*=}" && shift
        ;;
    --sql-password=*)
        sqlPassword="${i#*=}" && shift
        ;;
    -* | --* | *) ;;
    esac
done

# Request SSH credentials
read -rp "$(print_question "Do you need to use SSH tunneling? [Y/n]: ")" sshTunnel
if [ -z "$sshTunnel" ] || [ "$sshTunnel" == "Y" ] || [ "$sshTunnel" == "y" ]; then
    read -p "$(print_question "SSH Host" "$sshHost")" inputSshHost
    read -p "$(print_question "SSH User" "$sshUser")" inputSshUser
    sshHost=${inputSshHost:-${sshHost}}
    sshUser=${inputSshUser:-${sshUser}}
else
    sshHost=""
    sshUser=""
fi

# Request Database credentials
read -p "$(print_question "Database Host" "$sqlHost")" inputSqlHost
read -p "$(print_question "Database User" "$sqlUser")" inputSqlUser
read -p "$(print_question "Database DB Name" "{sqlDb")" inputSqlDb
read -p "$(print_question "Database Password" "$sqlPassword")" inputSqlPassword
sqlHost=${inputSqlHost:-${sqlHost}}
sqlUser=${inputSqlUser:-${sqlUser}}
sqlDb=${inputSqlDb:-${sqlDb}}
sqlPassword=${inputSqlPassword:-${sqlPassword}}hm 

# Prepare password
[ -z "$sqlPassword" ] && sqlPassword="" || sqlPassword="-p'$sqlPassword'"

# Request SSH credentials
read -rp "$(print_question "Do you want to exclude 'core_config_data' table? [Y/n]: ")" sqlExclude
if [ -z "$sqlExclude" ] || [ "$sqlExclude" == "Y" ] || [ "$sqlExclude" == "y" ]; then
    sqlExclude=1
else
    sqlExclude=0
fi

print_info "You are going to transfer database from [${sshHost}:${sqlHost}] to [LOCALHOST].\n"
read -rp "$(print_default "Press any key continue...")"

# Check required data
if [ -z "$sqlHost" ] || [ -z "$sqlUser" ] || [ -z "$sqlDb" ]; then
    print_error "Error: Please enter all required data\n"
    exit 1
fi

print_info "Creating database dump from origin server...\n"

# Create database dump from origin server (WITHOUT SSH TUNNEL)
if [ -z "$sshHost" ]; then

    # Create dump into mysql container
    docker-compose exec db bash -c "mysqldump -h'$sqlHost' -u'$sqlUser' $sqlPassword $sqlDb | sed -e 's/DEFINER[ ]*=[ ]*[^*]*\*/\*/' | gzip -9 > /tmp/db.sql.gz"

# Create database dump from origin server (WITH SSH TUNNEL)
else

    # Create dump
    ssh ${sshUser}@${sshHost} "mysqldump -h'$sqlHost' -u'$sqlUser' $sqlPassword $sqlDb | sed -e 's/DEFINER[ ]*=[ ]*[^*]*\*/\*/' | gzip -9 > /tmp/db.sql.gz"

    # Download dump
    scp ${sshUser}@${sshHost}:/tmp/db.sql.gz .

    # Copy dump into mysql container
    docker cp db.sql.gz "$(docker-compose ps -q db | awk '{print $1}')":/tmp/db.sql.gz

fi

print_info "Restoring database dump into localhost...\n"

# Restore dump
[ $sqlExclude -eq 1 ] && docker-compose exec db bash -c "mysqldump -u\$MYSQL_USER -p\$MYSQL_PASSWORD \$MYSQL_DATABASE core_config_data > /tmp/ccd.sql 2> /dev/null"
docker-compose exec db bash -c "zcat /tmp/db.sql.gz | mysql -f -u\$MYSQL_USER -p\$MYSQL_PASSWORD \$MYSQL_DATABASE"
[ $sqlExclude -eq 1 ] && docker-compose exec db bash -c "[ -f /tmp/ccd.sql ] && mysql -f -u\$MYSQL_USER -p\$MYSQL_PASSWORD \$MYSQL_DATABASE < /tmp/ccd.sql"

# Reindex Magento
read -p "$(print_question "Do you want to reindex Magento? [Y/n]: ")" reindexMagento
if [ -z "$reindexMagento" ] || [ "$reindexMagento" == 'Y' ] || [ "$reindexMagento" == 'y' ]; then
    docker-compose exec phpfpm bin/magento indexer:reindex
fi

# Clear Magento cache
read -p "$(print_question "Do you want to clear Magento cache? [Y/n]: ")" clearMagento
if [ -z "$clearMagento" ] || [ "$clearMagento" == 'Y' ] || [ "$clearMagento" == 'y' ]; then
    docker-compose exec phpfpm bin/magento cache:flush
fi

print_info " All done!\n"
