#!/usr/bin/env bash
set -euo pipefail

MYSQL_QUERY=${*:1}
if [ ! -z "$MYSQL_QUERY" ]; then
  MYSQL_QUERY="\"$MYSQL_QUERY\""
fi

docker-compose exec -T db bash -c "mysql -u\"root\" -p\"\$MYSQL_ROOT_PASSWORD\" \"\$MYSQL_DATABASE\" $MYSQL_QUERY"