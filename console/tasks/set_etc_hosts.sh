#!/usr/bin/env bash
set -euo pipefail

source "$COMPONENTS_DIR"/input_info.sh
source "$COMPONENTS_DIR"/print_message.sh

if [ -z "$(docker ps | grep phpfpm)" ]; then
    print_error "Set etc/hosts: Error: PHP is not running!\n"
    exit
fi

if [ -z "$(docker ps | grep hitch)" ]; then
    print_error "Set etc/hosts: Error: Hitch is not running!\n"
    exit
fi

# Get IP Address of hitch container
DOCKER_IP=`docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' $(docker ps -qf "name=hitch")`

# Read domains from database and include them into /etc/hosts file of php container
for DOMAIN in `"$COMMANDS_DIR"/mysql.sh "SELECT DISTINCT value FROM core_config_data WHERE path like 'web/%/base_url'" 2> /dev/null`
do
  if [[ "$DOMAIN" == *"://"* ]]; then
    DOMAIN=$(echo "$DOMAIN" | sed -e 's|^[^/]*//||' -e 's|/.*$||')
    docker-compose exec -uroot phpfpm bash -c "echo \"$DOCKER_IP $DOMAIN\" >> /etc/hosts"
  fi
done

# Copy local certificates to php container
if [ -d "/usr/local/share/ca-certificates" ];
then
  docker cp /usr/local/share/ca-certificates $(docker ps -qf "name=phpfpm"):/usr/local/share/
  docker-compose exec -uroot phpfpm update-ca-certificates > /dev/null 2> /dev/null
fi
