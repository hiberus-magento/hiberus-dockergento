#!/usr/bin/env bash
set -euo pipefail

MYSQL_CONTAINER=$(docker ps -qf "name=db")
MYSQL_QUERY=${*:1}

if [ -z "$MYSQL_CONTAINER" ]; then
    print_error "Error: DB container is not running\n"
    exit 1
fi

if [ ! -t 0 ]; then
  MYSQL_QUERY=$(cat)
  MYSQL_QUERY=$(echo "$MYSQL_QUERY" | sed 's/DEFINER=[^*]*\*/\*/g')
fi

if [ ! -z "$MYSQL_QUERY" ]; then
  echo -e "$MYSQL_QUERY" | docker exec -i $MYSQL_CONTAINER bash -c "mysql -u\"root\" -p\"\$MYSQL_ROOT_PASSWORD\" \"\$MYSQL_DATABASE\""
else
  docker exec -it $MYSQL_CONTAINER bash -c "mysql -u\"root\" -p\"\$MYSQL_ROOT_PASSWORD\" \"\$MYSQL_DATABASE\""
fi


