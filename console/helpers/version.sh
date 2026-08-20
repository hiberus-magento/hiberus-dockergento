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
    local docker_path token cached

    # Detecting v2 means running `docker compose version`, which costs ~190ms. It is asked
    # on every single invocation and the answer only changes when Docker itself is
    # reinstalled, so it is cached against the modification time of the docker binary.
    docker_path=$(command -v docker 2>/dev/null || true)

    if [ -n "$docker_path" ]; then
        source "${HELPERS_DIR}"/cache.sh
        token=$(hm_file_mtime "$docker_path")
        cached=$(hm_cache_read "compose-cmd" "$token" 2>/dev/null) && {
            echo "$cached"
            return 0
        }
    fi

    # Try Docker Compose v2 first (docker compose)
    if [ -n "$docker_path" ] && docker compose version &> /dev/null 2>&1; then
        hm_cache_write "compose-cmd" "$token" "docker compose"
        echo "docker compose"
        return 0
    fi

    # Fall back to Docker Compose v1 (docker-compose)
    if command -v docker-compose &> /dev/null; then
        [ -n "$docker_path" ] && hm_cache_write "compose-cmd" "$token" "docker-compose"
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
    # Memoised for the invocation: three different callers ask for it and each one used to
    # pay for its own subprocess
    if [ -n "${HM_COMPOSE_VERSION_CACHE:-}" ]; then
        echo "$HM_COMPOSE_VERSION_CACHE"
        return 0
    fi

    local compose_cmd docker_path token cached

    # Same reasoning as the command detection: the answer only changes when Docker is
    # reinstalled, so it is cached on disk against the modification time of the binary
    docker_path=$(command -v docker 2>/dev/null || true)

    if [ -n "$docker_path" ]; then
        source "${HELPERS_DIR}"/cache.sh
        token=$(hm_file_mtime "$docker_path")
        cached=$(hm_cache_read "compose-version" "$token" 2>/dev/null) && {
            HM_COMPOSE_VERSION_CACHE="$cached"
            echo "$HM_COMPOSE_VERSION_CACHE"
            return 0
        }
    fi

    compose_cmd=$(get_docker_compose_cmd)
    HM_COMPOSE_VERSION_CACHE=$($compose_cmd version --short 2>/dev/null || echo "2.0.0")

    if [ -n "$docker_path" ]; then
        hm_cache_write "compose-version" "$token" "$HM_COMPOSE_VERSION_CACHE"
    fi

    echo "$HM_COMPOSE_VERSION_CACHE"
}
