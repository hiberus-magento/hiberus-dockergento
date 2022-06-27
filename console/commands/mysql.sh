#!/usr/bin/env bash
set -euo pipefail

yml_file="$MAGENTO_DIR/docker-compose.yml"
path_to_mysql_keys="services_db_environment_MYSQL"
look_at_task="$TASKS_DIR/look_at_yml.sh"

# Get credentials
user=$($look_at_task "$yml_file" "${path_to_mysql_keys}_USER")
password=$($look_at_task "$yml_file" "${path_to_mysql_keys}_PASSWORD")
database=$($look_at_task "$yml_file" "${path_to_mysql_keys}_DATABASE")

$COMMAND_BIN_NAME docker-compose exec -T db mysql -u"$user" -p"$password" "$database" "$@"