#!/usr/bin/env bash
set -euo pipefail

if [ "$#" == 0 ];then
  exit 0
fi

YML_FILE="${MAGENTO_DIR}/docker-compose.yml"
PAHT_TO_MYSQL_KEYS="services_db_environment_MYSQL"
LOOK_TASK="${TASKS_DIR}/look_at_yml.sh"

# Get credentials
USER=$(${TASKS_DIR}/look_at_yml.sh "${YML_FILE}" "${PAHT_TO_MYSQL_KEYS}_USER")
PASSWORD=$(${LOOK_TASK} "${YML_FILE}" "${PAHT_TO_MYSQL_KEYS}_PASSWORD")
DATABASE=$(${LOOK_TASK} "${YML_FILE}" "${PAHT_TO_MYSQL_KEYS}_DATABASE")

COMMAND_MYSQL="exec db mysql -u${USER} -p${PASSWORD} ${DATABASE}" 

${COMMANDS_DIR}/docker-compose.sh $COMMAND_MYSQL "$@"