#!/usr/bin/env bash
set -euo pipefail

YML_FILE="${MAGENTO_DIR}/docker-compose.yml"
PAHT_TO_MYSQL_KEYS="services_db_environment_MYSQL"
LOOK_TASK="${TASKS_DIR}/look_at_yml.sh"

# Get project credentials
USER=$(${LOOK_TASK} "${YML_FILE}" "${PAHT_TO_MYSQL_KEYS}_USER")
PASSWORD=$(${LOOK_TASK} "${YML_FILE}" "${PAHT_TO_MYSQL_KEYS}_PASSWORD")
DATABASE=$(${LOOK_TASK} "${YML_FILE}" "${PAHT_TO_MYSQL_KEYS}_DATABASE")

${COMMAND_BIN_NAME} docker-compose exec -T db mysql -u"${USER}" -p"${PASSWORD}" "${DATABASE}" "$@"