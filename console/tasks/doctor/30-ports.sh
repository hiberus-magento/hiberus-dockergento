#!/usr/bin/env bash
# Ports the stack needs, and who is holding them
#
# The most frequent reason an environment refuses to start is another project already
# listening on 80, 443 or 3306, and until now nothing said so.
source "$HELPERS_DIR"/doctor.sh

# The list comes from the compose configuration, never duplicated here
if doctor_in_project; then
    ports=$($DOCKER_COMPOSE config --format json 2>/dev/null |
        jq -r '[.services // {} | .[].ports // [] | .[].published // empty] | unique | .[]' 2>/dev/null)
else
    ports=$(grep -oE '^ *- [0-9]+:[0-9]+' "$COMMAND_BIN_DIR/docker-compose/docker-compose.template.yml" 2>/dev/null |
        sed 's/.*- \([0-9]*\):.*/\1/' | sort -un)
fi

if [ -z "$ports" ]; then
    doctor_warning "Could not work out which ports this environment needs"
    exit 0
fi

listeners=""
if command -v lsof >/dev/null 2>&1; then
    listeners=$(lsof -nP -iTCP -sTCP:LISTEN 2>/dev/null || true)
elif command -v ss >/dev/null 2>&1; then
    listeners=$(ss -ltn 2>/dev/null || true)
else
    doctor_warning "No tool available to inspect listening ports" \
        "Install lsof to enable this check"
    exit 0
fi

# One query for every published port on the machine, instead of one per port
published=$(docker ps --format '{{.Ports}}|{{.Label "com.docker.compose.project"}}' 2>/dev/null || true)

problems=0
mine=""
conflicts=""
host_ports=""

for port in $ports; do
    if ! printf '%s\n' "$listeners" | grep -qE "[:.]$port( |$|->)"; then
        continue
    fi

    owner=$(printf '%s\n' "$published" |
        awk -F'|' -v port="$port" '$1 ~ (":" port "->") { print $2; exit }')

    if [ -n "$owner" ] && [ "$owner" == "${COMPOSE_PROJECT_NAME:-}" ]; then
        continue
    fi

    if [ -n "$owner" ]; then
        # Group by owner: one line naming the culprit and every port it holds beats one
        # line per port saying the same thing seven times
        conflicts=$(printf '%s\n%s %s' "$conflicts" "$owner" "$port")
    else
        host_ports="$host_ports $port"
    fi
done

if [ -n "$conflicts" ]; then
    while read -r owner owned_ports; do
        [ -z "$owner" ] && continue

        if doctor_in_project; then
            problems=$((problems + 1))
            doctor_error "Ports $owned_ports are taken by the '$owner' environment" \
                "cd into that project and run '$COMMAND_BIN_NAME stop'"
        else
            mine="$mine $owner"
        fi
    done <<< "$(printf '%s\n' "$conflicts" | sed '/^$/d' |
        awk '{ owner = $1; ports[owner] = ports[owner] (ports[owner] ? ", " : "") $2 }
             END { for (o in ports) print o, ports[o] }')"
fi

if [ -n "$host_ports" ]; then
    process=$(printf '%s\n' "$listeners" |
        grep -E "[:.]$(printf '%s' "$host_ports" | awk '{ print $1 }')( |$|->)" | awk 'NR==1 { print $1 }')

    if doctor_in_project; then
        problems=$((problems + 1))
        doctor_error "Ports$host_ports are taken by ${process:-processes on the host}" \
            "Stop whatever is listening on those ports"
    else
        doctor_warning "Ports$host_ports are taken by ${process:-processes on the host}" \
            "They will clash with any environment that needs them"
    fi
fi

if [ -n "$mine" ]; then
    doctor_ok "Ports held by running environments:$mine"
fi

if [ "$problems" -eq 0 ] && [ -z "$mine" ] && [ -z "$host_ports" ]; then
    doctor_ok "Every required port is free"
fi
