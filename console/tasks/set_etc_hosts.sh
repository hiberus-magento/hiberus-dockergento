#!/usr/bin/env bash
set -euo pipefail

source "$COMPONENTS_DIR"/input_info.sh
source "$COMPONENTS_DIR"/print_message.sh
source "$HELPERS_DIR"/docker.sh

is_run_service "phpfpm"

#
# The self-routing entries point at Hitch, and a project routed through the global proxy has no
# Hitch: the proxy overlay deletes it, because Hitch was only there to give Varnish the HTTPS it
# does not have and the proxy terminates TLS itself.
#
# Demanding it anyway is what made `hm start` bring the whole environment up on Linux and then
# fail with "Service 'hitch' is not running" — every time, for every project on the proxy. There
# is nothing to point the entries at in that case, so they are skipped and said out loud.
#
HITCH_CONTAINER=$(hm_service_container hitch 2>/dev/null || true)

if [ -z "$HITCH_CONTAINER" ]; then
    print_processing "No TLS terminator in this project, so there are no self-routing entries to add"
else
    DOCKER_IP=$(docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "$HITCH_CONTAINER")

    # Read domains from database and include them into /etc/hosts file of php container
    for DOMAIN in `"$COMMANDS_DIR"/mysql.sh -q "SELECT DISTINCT value FROM core_config_data WHERE path like 'web/%/base_url'" 2> /dev/null`
    do
      if [[ "$DOMAIN" == *"://"* ]]; then
        DOMAIN=$(echo "$DOMAIN" | sed -e 's|^[^/]*//||' -e 's|/.*$||')
        $DOCKER_COMPOSE exec -uroot phpfpm bash -c "echo \"$DOCKER_IP $DOMAIN\" >> /etc/hosts"
      fi
    done
    $DOCKER_COMPOSE exec -uroot phpfpm bash -c "echo \"$DOCKER_IP localhost\" >> /etc/hosts"
fi

# Copy local certificates to php container
if [ -d "/usr/local/share/ca-certificates" ];
then
  docker cp /usr/local/share/ca-certificates "$($DOCKER_COMPOSE ps -q phpfpm)":/usr/local/share/
  $DOCKER_COMPOSE exec -uroot phpfpm update-ca-certificates > /dev/null 2> /dev/null
fi
