#!/usr/bin/env bash
set -euo pipefail

MYSQL_QUERY=${*:1}
if [ ! -z "$MYSQL_QUERY" ]; then
  MYSQL_QUERY="\"$MYSQL_QUERY\""
fi

docker-compose exec db bash -c "mysql -u\"\$MYSQL_USER\" -p\"\$MYSQL_PASSWORD\" \"\$MYSQL_DATABASE\" $MYSQL_QUERY"