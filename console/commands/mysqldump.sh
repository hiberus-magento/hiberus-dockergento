#!/usr/bin/env bash
set -euo pipefail

database=$("$TASKS_DIR/look_at_yml.sh" "$MAGENTO_DIR/docker-compose.yml" "services_db_environment_MYSQL_DATABASE")

${COMMAND_BIN_NAME} docker-compose exec -T db mysqldump --skip-triggers -uroot -ppassword $database > "$@"
