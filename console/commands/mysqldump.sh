#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=/dev/null
source "$COMPONENTS_DIR"/print_message.sh

if [ -z "$@" ]; then
  print_warning "Please, specify a path for saving the database dump file.\nUsage: $COMMAND_BIN_NAME mysqldump <path>\n"
  exit
fi

docker-compose exec db bash -c 'mysqldump --skip-triggers -uroot -p"$MYSQL_ROOT_PASSWORD" "$MYSQL_DATABASE"' > "$@"