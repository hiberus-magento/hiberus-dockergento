#!/usr/bin/env bash

#
# Check if version is greater than or equal to target (semver comparison)
# Usage: version_gte "2.4.6" "2.4.5" && echo "yes"
#
version_gte() {
    local version="$1"
    local target="$2"
    [ "$(printf '%s\n' "$target" "$version" | sort -V | head -n1)" == "$target" ]
}

#
# Detect which Docker Compose command is available
# Priority: 1) docker compose (v2), 2) docker-compose (v1)
# Returns: "docker compose" (v2) or "docker-compose" (v1)
#
get_docker_compose_cmd() {
    # Try Docker Compose v2 first (docker compose)
    if command -v docker &> /dev/null && docker compose version &> /dev/null 2>&1; then
        echo "docker compose"
        return 0
    fi

    # Fall back to Docker Compose v1 (docker-compose)
    if command -v docker-compose &> /dev/null; then
        echo "docker-compose"
        return 0
    fi

    # Default fallback
    echo "docker-compose"
}

#
# Get Docker Compose version (short format)
# Returns: version string (e.g., "2.25.0") or "2.0.0" as fallback
#
get_docker_compose_version() {
    local compose_cmd=$(get_docker_compose_cmd)
    $compose_cmd version --short 2>/dev/null || echo "2.0.0"
}
