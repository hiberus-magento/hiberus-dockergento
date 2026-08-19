#!/usr/bin/env bash
# Is the compose configuration of this project valid?
source "$HELPERS_DIR"/doctor.sh
doctor_requires_project

error=$($DOCKER_COMPOSE config -q 2>&1 >/dev/null) && valid=true || valid=false

if $valid; then
    doctor_ok "Docker Compose configuration is valid"
    exit 0
fi

doctor_error "Docker Compose configuration is invalid: ${error:-unknown reason}" \
    "$COMMAND_BIN_NAME setup -f"
