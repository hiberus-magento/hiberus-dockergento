#!/usr/bin/env bash
# Docker Compose present and recent enough
source "$HELPERS_DIR"/doctor.sh
source "$HELPERS_DIR"/version.sh

version=$(get_docker_compose_version 2>/dev/null || true)

if [ -z "$version" ]; then
    doctor_error "Docker Compose was not found" "Install Docker Compose v2"
    exit 0
fi

if version_gte "$version" "2.0.0"; then
    doctor_ok "Docker Compose $version"
else
    doctor_warning "Docker Compose $version is old and no longer tested" \
        "Upgrade to Docker Compose v2"
fi
