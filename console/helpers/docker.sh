#!/usr/bin/env bash

source "${HELPERS_DIR}"/exit_codes.sh

#
# Check if docker is running
#
is_docker_service_running() {
    if [[ ! $(docker info >/dev/null 2>&1; echo $?) -eq 0 ]]; then
        hm_fail "$HM_EXIT_DOCKER" "docker_unavailable" \
            "Docker is not running" \
            "Start Docker and try again"
    fi
}

#
# Check if container of services is running
#
is_run_service() {
    is_docker_service_running
    local container_id service
    service="${1:-phpfpm}"
    container_id=$(hm_service_container "$service")

    if [ -z "$container_id" ]; then
        # Last resort for containers created outside of a compose project
        container_id=$(docker ps -qf name="$COMPOSE_PROJECT_NAME"-"$service" -qf name="$COMPOSE_PROJECT_NAME"_"$service")
    fi
    
    if [ -z "$container_id" ]; then
        hm_fail "$HM_EXIT_SERVICE" "service_not_running" \
            "Service '$service' is not running" \
            "$COMMAND_BIN_NAME start $service"
    fi
}

#
# Environment discovery.
#
# Containers are grouped by the hm.* labels stamped by the compose template. Environments
# created before those labels existed are still found through the standard Compose labels,
# using the phpfpm service as the Dockergento signature, and reported as having no
# metadata.
#

#
# Value of a label for an environment: hm_environment_label <project> <label>
#
hm_environment_label() {
    local project="$1"
    local label="$2"

    docker ps -a \
        --filter "label=hm.project=$project" \
        --format "{{.Label \"$label\"}}" 2>/dev/null | head -1
}

#
# Containers of an environment: hm_environment_containers <project> [--running]
#
hm_environment_containers() {
    local project="$1"
    local all="-a"

    if [ "${2:-}" == "--running" ]; then
        all=""
    fi

    local ids
    ids=$(docker ps $all -q --filter "label=hm.project=$project" 2>/dev/null)

    if [ -n "$ids" ]; then
        echo "$ids"
        return 0
    fi

    # Environment without hm.* labels
    docker ps $all -q --filter "label=com.docker.compose.project=$project" 2>/dev/null
}

#
# True when the environment carries hm.* metadata
#
hm_environment_has_metadata() {
    [ -n "$(docker ps -a -q --filter "label=hm.project=$1" 2>/dev/null)" ]
}

#
# All Dockergento environments on the machine, one name per line
#
hm_environments() {
    {
        docker ps -a --filter "label=hm.project" --format '{{.Label "hm.project"}}' 2>/dev/null

        # Fallback: a compose project with a phpfpm service and no hm.* labels
        docker ps -a \
            --filter "label=com.docker.compose.service=phpfpm" \
            --format '{{.Label "com.docker.compose.project"}}|{{.Label "hm.project"}}' 2>/dev/null |
            awk -F'|' '$2 == "" && $1 != "" { print $1 }'
    } | sort -u | sed '/^$/d'
}

#
# An environment is orphaned when the directory it was started from is gone
#
hm_environment_is_orphan() {
    local root
    root=$(hm_environment_label "$1" "hm.root")

    [ -n "$root" ] && [ ! -d "$root" ]
}

#
# Current branch of an environment, derived at read time from hm.root so that it is never
# stale: hm_environment_branch <project>
#
hm_environment_branch() {
    local root
    root=$(hm_environment_label "$1" "hm.root")

    if [ -z "$root" ] || [ ! -d "$root" ]; then
        return 0
    fi

    git -C "$root" rev-parse --abbrev-ref HEAD 2>/dev/null || true
}

#
# Container of a service inside a project, without matching container names by substring:
#   hm_service_container <service> [project]
#
hm_service_container() {
    local service="$1"
    local project="${2:-$COMPOSE_PROJECT_NAME}"
    local container_id

    container_id=$(docker ps -q \
        --filter "label=hm.project=$project" \
        --filter "label=com.docker.compose.service=$service" 2>/dev/null | head -1)

    if [ -n "$container_id" ]; then
        echo "$container_id"
        return 0
    fi

    # Environment without hm.* labels
    docker ps -q \
        --filter "label=com.docker.compose.project=$project" \
        --filter "label=com.docker.compose.service=$service" 2>/dev/null | head -1
}
