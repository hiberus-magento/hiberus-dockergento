#!/usr/bin/env bash
# Are the services of this project up?
source "$HELPERS_DIR"/doctor.sh
doctor_requires_project

total=$($DOCKER_COMPOSE config --format json 2>/dev/null | jq -r '.services // {} | length')
running=$(doctor_container_table |
    awk -F'|' -v project="${COMPOSE_PROJECT_NAME:-}" \
        '($5 == project || ($5 == "" && $3 == project)) && $2 == "running" { count++ }
         END { print count + 0 }')

if [ "${total:-0}" -eq 0 ]; then
    doctor_warning "No services are defined for this project" "$COMMAND_BIN_NAME setup -f"
elif [ "$running" -eq 0 ]; then
    doctor_warning "The environment is stopped ($total services defined)" \
        "$COMMAND_BIN_NAME start"
elif [ "$running" -lt "${total:-0}" ]; then
    doctor_warning "Only $running of $total services are running" "$COMMAND_BIN_NAME start"
else
    doctor_ok "All $total services are running"
fi
