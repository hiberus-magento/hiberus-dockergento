#!/usr/bin/env bash
set -uo pipefail

source "$COMPONENTS_DIR"/print_message.sh
source "$COMPONENTS_DIR"/print_json.sh
source "$HELPERS_DIR"/exit_codes.sh
source "$HELPERS_DIR"/docker.sh

#
# A temporary way in to something that cannot be routed by name.
#
# MySQL and AMQP carry no hostname —Traefik refuses them outright: `has HostSNI matcher, but no TLS
# on router`— so a project using the proxy publishes nothing and they are unreachable from the
# machine. This opens a door for as long as it is held open, and closes it on the way out.
#
# Runs in the foreground on purpose, like `ssh -L`: the lifetime of the tunnel is the lifetime of
# the command, and nothing is left behind to find later.
#

RELAY_IMAGE="alpine/socat"

close_only=false
arguments=()

for argument in "$@"; do
    case "$argument" in
        --close) close_only=true ;;
        *)       arguments[${#arguments[@]}]="$argument" ;;
    esac
done

set -- ${arguments[@]+"${arguments[@]}"}

service="${1:-}"
port="${2:-}"

#
# Close whatever this project left open.
#
# The trap below covers Ctrl-C, which is how a tunnel normally ends. It does not cover a terminal
# window being closed or the process being killed outright, and a relay left running keeps a port
# bound with nobody watching. So the name is deterministic, a stale one is cleared before opening
# a new one, and `--close` exists to say so explicitly.
#
close_open_tunnels() {
    local pattern="hm-tunnel-${COMPOSE_PROJECT_NAME}"
    [ -n "${1:-}" ] && pattern="$pattern-$1"

    local stale
    stale=$(docker ps -aq --filter "name=^${pattern}" 2>/dev/null)

    [ -z "$stale" ] && return 1
    docker rm -f $stale >/dev/null 2>&1
    return 0
}

if $close_only; then
    if close_open_tunnels "$service"; then
        is_json_output && json_success "tunnel" "$(jq -n '{closed: true}')" ||
            print_info "Tunnels closed.\n"
    else
        is_json_output && json_success "tunnel" "$(jq -n '{closed: false}')" ||
            print_info "No tunnels were open.\n"
    fi
    exit 0
fi

if [ -z "$service" ]; then
    hm_fail "$HM_EXIT_USAGE" "missing_service" \
        "Which service should be reachable?" \
        "$COMMAND_BIN_NAME tunnel db"
fi

available=$($DOCKER_COMPOSE config --services 2>/dev/null | sort)

if ! printf '%s\n' "$available" | grep -qx "$service"; then
    hm_fail "$HM_EXIT_SERVICE" "unknown_service" \
        "This project has no service called '$service'" \
        "$COMMAND_BIN_NAME tunnel $(printf '%s' "$available" | tr '\n' ' ')"
fi

is_run_service "$service"

#
# Which port inside the container. Taken from the image when it declares exactly one, asked for
# when it does not: guessing between 5672 and 15691 is how you end up debugging the wrong thing.
#
if [ -z "$port" ]; then
    container=$($DOCKER_COMPOSE ps -q "$service" | head -1)
    exposed=$(docker inspect "$container" \
        --format '{{range $p, $_ := .Config.ExposedPorts}}{{$p}} {{end}}' 2>/dev/null |
        tr ' ' '\n' | sed 's|/tcp||' | sed '/^$/d' | sort -n)
    count=$(printf '%s\n' "$exposed" | grep -c . || true)

    if [ "${count:-0}" -eq 1 ]; then
        port="$exposed"
    else
        hm_fail "$HM_EXIT_USAGE" "ambiguous_port" \
            "'$service' exposes $count ports, so the one to forward has to be said" \
            "$COMMAND_BIN_NAME tunnel $service $(printf '%s' "$exposed" | tr '\n' ' ')"
    fi
fi

# A free port chosen by the system, so several tunnels can be open at once
local_port=$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()' 2>/dev/null)

if [ -z "$local_port" ]; then
    hm_fail "$HM_EXIT_ERROR" "no_free_port" \
        "No free local port could be found" \
        "$COMMAND_BIN_NAME tunnel $service $port"
fi

network=$(docker inspect "$($DOCKER_COMPOSE ps -q "$service" | head -1)" \
    --format '{{range $n, $_ := .NetworkSettings.Networks}}{{$n}} {{end}}' 2>/dev/null |
    awk '{print $1}')

relay="hm-tunnel-${COMPOSE_PROJECT_NAME}-${service}"

# Anything left over from a window that was closed rather than interrupted
close_open_tunnels "$service" >/dev/null 2>&1 || true

close_tunnel() {
    docker rm -f "$relay" >/dev/null 2>&1
    printf '\n'
    print_info "Tunnel closed.\n"
}
trap close_tunnel EXIT INT TERM

if ! docker run -d --rm --name "$relay" --network "$network" \
    -p "127.0.0.1:$local_port:$port" "$RELAY_IMAGE" \
    "tcp-listen:$port,fork,reuseaddr" "tcp-connect:$service:$port" >/dev/null 2>&1; then
    hm_fail "$HM_EXIT_DOCKER" "tunnel_failed" \
        "The tunnel could not be opened" \
        "docker pull $RELAY_IMAGE"
fi

#
# Wait until it actually accepts a connection before saying it is there.
#
# `docker run -d` returns as soon as the container is created, and the port is published a moment
# later. Announcing an address that is not ready yet is a small lie, and the kind that turns into
# "it says it works but it does not".
#
waited=0
until python3 -c "
import socket, sys
s = socket.socket(); s.settimeout(1)
sys.exit(0 if s.connect_ex(('127.0.0.1', $local_port)) == 0 else 1)" 2>/dev/null ||
    [ "$waited" -ge 20 ]; do
    sleep 0.25
    waited=$((waited + 1))
done

if [ "$waited" -ge 20 ]; then
    hm_fail "$HM_EXIT_ERROR" "tunnel_not_ready" \
        "The tunnel was opened but nothing is answering on 127.0.0.1:$local_port" \
        "docker logs $relay"
fi

if is_json_output; then
    json_success "tunnel" "$(jq -n --arg service "$service" --arg host "127.0.0.1" \
        --argjson port "$local_port" --argjson remote "$port" \
        '{service: $service, host: $host, port: $port, container_port: $remote}')"
fi

if ! is_json_output; then
    printf '\n'
    print_info "$service is reachable at "
    print_code "127.0.0.1:$local_port"
    printf '\n\n'
    print_default "  Leave this running while you use it. Ctrl-C closes it.\n\n"
fi

# Hold it open for as long as the relay lives
docker wait "$relay" >/dev/null 2>&1
