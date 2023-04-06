#!/usr/bin/env bash
set -euo pipefail

source "$COMPONENTS_DIR"/input_info.sh
source "$COMPONENTS_DIR"/print_message.sh

DOMAINS=()

# Read domains from database
# TODO: adapct to -q option
for DOMAIN in `"$COMMANDS_DIR"/mysql.sh -q "SELECT DISTINCT value FROM core_config_data WHERE path like 'web/%/base_url'" 2> /dev/null`
do
  if [[ "$DOMAIN" == *"://"* ]]; then
    DOMAINS+=($(echo "$DOMAIN" | sed -e 's|^[^/]*//||' -e 's|/.*$||'))
  fi
done

echo ${DOMAINS[*]}