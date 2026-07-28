#!/usr/bin/env bash
set -euo pipefail

source "$COMPONENTS_DIR"/print_message.sh

if [ -z "$@" ]; then
  print_warning "Please, specify a path for saving the database dump file.\nUsage: $COMMAND_BIN_NAME mysqldump <path>\n"
  exit
fi

# MariaDB 11+ renamed mysqldump -> mariadb-dump; resolve inside the container
# (fallback to mysqldump for older images).
$DOCKER_COMPOSE exec db bash -c 'dump=$(command -v mariadb-dump || command -v mysqldump); "$dump" --skip-triggers -uroot -p"$MYSQL_ROOT_PASSWORD" "$MYSQL_DATABASE"' > "$@"
