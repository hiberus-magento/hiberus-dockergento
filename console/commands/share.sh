#!/usr/bin/env bash
set -uo pipefail

source "$COMPONENTS_DIR"/print_message.sh
source "$COMPONENTS_DIR"/print_json.sh
source "$COMPONENTS_DIR"/input_info.sh
source "$HELPERS_DIR"/exit_codes.sh
source "$HELPERS_DIR"/docker.sh

#
# A public address for this project, for as long as you hold it open.
#
# Two things people actually need: showing progress to a client without deploying, and receiving
# real webhooks from payment gateways and marketplaces, which cannot reach a local environment.
#
# Cloudflare quick tunnels: no account, no credentials, nothing installed on the machine. The URL
# changes every time, which is right for a demo or for capturing webhooks and wrong for anything
# permanent — that would need a named tunnel over a domain of ours, and is not this.
#

TUNNEL_IMAGE="cloudflare/cloudflared:latest"
WEB_SERVICE="${HM_SHARE_SERVICE:-varnish}"
WEB_PORT="${HM_SHARE_PORT:-6081}"

stop_only=false
for argument in "$@"; do
    case "$argument" in
        --stop) stop_only=true ;;
        *)
            hm_fail "$HM_EXIT_USAGE" "invalid_argument" "Unknown option: $argument" \
                "$COMMAND_BIN_NAME share [--stop]"
            ;;
    esac
done

relay="hm-share-${COMPOSE_PROJECT_NAME}"

close_share() {
    docker rm -f "$relay" >/dev/null 2>&1
}

if $stop_only; then
    if [ -n "$(docker ps -aq --filter "name=^${relay}$" 2>/dev/null)" ]; then
        close_share
        is_json_output && json_success "share" "$(jq -n '{closed: true}')" ||
            print_info "The tunnel is closed.\n"
    else
        is_json_output && json_success "share" "$(jq -n '{closed: false}')" ||
            print_info "Nothing was being shared.\n"
    fi
    exit 0
fi

is_run_service "$WEB_SERVICE"

#
# The one confirmation in this tool that protects something outside the machine.
#
# It names what is being exposed rather than asking "are you sure?": while this is open, anyone
# with the address reaches the environment, admin panel and data included.
#
if ! is_non_interactive; then
    printf '\n'
    print_warning "This puts '$COMPOSE_PROJECT_NAME' on the public internet.\n"
    print_warning "Anyone with the address reaches it — the admin panel and the data in it.\n\n"
    confirm "Share it? [y/N]: "

    case "$REPLY" in
        Y | y) ;;
        *)
            print_info "Nothing was shared.\n"
            exit 0
            ;;
    esac
fi

# Anything a closed window left behind
close_share

network=$(docker inspect "$($DOCKER_COMPOSE ps -q "$WEB_SERVICE" | head -1)" \
    --format '{{range $n, $_ := .NetworkSettings.Networks}}{{$n}} {{end}}' 2>/dev/null | awk '{print $1}')

if [ -z "$network" ]; then
    hm_fail "$HM_EXIT_SERVICE" "no_network" \
        "The '$WEB_SERVICE' service is not on any network" \
        "$COMMAND_BIN_NAME start"
fi

print_info "Opening a tunnel...\n"

#
# The Host header is rewritten to the project's domain, or nginx has no vhost to match and answers
# with whatever its default is
#
if ! docker run -d --name "$relay" --network "$network" "$TUNNEL_IMAGE" \
    tunnel --no-autoupdate \
    --url "http://$WEB_SERVICE:$WEB_PORT" \
    --http-host-header "${DOMAIN:-localhost}" >/dev/null 2>&1; then
    hm_fail "$HM_EXIT_DOCKER" "tunnel_failed" \
        "The tunnel could not be started" \
        "docker pull $TUNNEL_IMAGE"
fi

trap 'close_share; printf "\n"; print_info "Sharing stopped.\n"' EXIT INT TERM

# The address appears in its output once the edge has accepted the tunnel
address=""
waited=0
while [ -z "$address" ] && [ "$waited" -lt 60 ]; do
    address=$(docker logs "$relay" 2>&1 | grep -oE 'https://[a-z0-9-]+\.trycloudflare\.com' | head -1)
    [ -n "$address" ] && break
    sleep 2
    waited=$((waited + 2))
done

if [ -z "$address" ]; then
    hm_fail "$HM_EXIT_ERROR" "no_address" \
        "The tunnel did not report a public address" \
        "docker logs $relay"
fi

if is_json_output; then
    json_success "share" "$(jq -n --arg url "$address" --arg project "$COMPOSE_PROJECT_NAME" \
        '{url: $url, project: $project}')"
fi

if ! is_json_output; then
    printf '\n'
    print_info "'$COMPOSE_PROJECT_NAME' is reachable at "
    print_link "$address\n"
    printf '\n'
    print_default "  The address changes every time, and it is gone when you stop.\n"
    print_default "  Magento builds its links from base_url, so they still point at your local\n"
    print_default "  domain. That matters for a demo, not for receiving webhooks.\n\n"
    print_default "  Leave this running while you use it. Ctrl-C stops sharing.\n\n"
fi

docker wait "$relay" >/dev/null 2>&1
