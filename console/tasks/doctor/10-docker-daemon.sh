#!/usr/bin/env bash
# Is the Docker daemon reachable?
source "$HELPERS_DIR"/doctor.sh

if docker info >/dev/null 2>&1; then
    doctor_ok "Docker daemon is running"
    exit 0
fi

doctor_error "Docker daemon is not running" "Start Docker and run the diagnosis again"
