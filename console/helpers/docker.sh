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
# Every lookup reads from a single `docker ps` call cached for the invocation. Asking
# Docker once per question cost ~150ms per call and added up to seconds when describing a
# project or listing the machine, which is too slow for a command a TUI refreshes often.
#
# Columns: id, state, compose project, compose service, hm.project, hm.root, hm.worktree,
#          hm.magento, hm.profile, hm.version, hm.agent, compose working dir
#

HM_CONTAINER_TABLE_CACHE=""
HM_CONTAINER_TABLE_LOADED=false

#
# Load the table into the current shell. Command substitution runs in a subshell, so a
# cache filled inside `$(...)` is thrown away: it has to be primed by the caller before
# any lookup, or every question would hit Docker again.
#
hm_load_container_table() {
    HM_CONTAINER_TABLE_CACHE=$(docker ps -a --format \
        '{{.ID}}|{{.State}}|{{.Label "com.docker.compose.project"}}|{{.Label "com.docker.compose.service"}}|{{.Label "hm.project"}}|{{.Label "hm.root"}}|{{.Label "hm.worktree"}}|{{.Label "hm.magento"}}|{{.Label "hm.profile"}}|{{.Label "hm.version"}}|{{.Label "hm.agent"}}|{{.Label "com.docker.compose.project.working_dir"}}' \
        2>/dev/null || true)
    HM_CONTAINER_TABLE_LOADED=true
}

hm_container_table() {
    if ! $HM_CONTAINER_TABLE_LOADED; then
        HM_CONTAINER_TABLE_CACHE=$(docker ps -a --format \
            '{{.ID}}|{{.State}}|{{.Label "com.docker.compose.project"}}|{{.Label "com.docker.compose.service"}}|{{.Label "hm.project"}}|{{.Label "hm.root"}}|{{.Label "hm.worktree"}}|{{.Label "hm.magento"}}|{{.Label "hm.profile"}}|{{.Label "hm.version"}}|{{.Label "hm.agent"}}|{{.Label "com.docker.compose.project.working_dir"}}' \
            2>/dev/null || true)
        HM_CONTAINER_TABLE_LOADED=true
    fi

    printf '%s\n' "$HM_CONTAINER_TABLE_CACHE"
}

#
# Column holding a label
#
hm_label_column() {
    case "$1" in
        hm.project)  echo 5 ;;
        hm.root)     echo 6 ;;
        hm.worktree) echo 7 ;;
        hm.magento)  echo 8 ;;
        hm.profile)  echo 9 ;;
        hm.version)  echo 10 ;;
        hm.agent)    echo 11 ;;
        *)           echo 0 ;;
    esac
}

#
# Value of a label for an environment: hm_environment_label <project> <label>
#
hm_environment_label() {
    local column
    column=$(hm_label_column "$2")

    if [ "$column" == "0" ]; then
        return 0
    fi

    hm_container_table |
        awk -F'|' -v project="$1" -v col="$column" \
            '$5 == project && $col != "" { print $col; exit }'
}

#
# Containers of an environment: hm_environment_containers <project> [--running]
#
hm_environment_containers() {
    local only_running=""

    if [ "${2:-}" == "--running" ]; then
        only_running="running"
    fi

    hm_container_table |
        awk -F'|' -v project="$1" -v state="$only_running" \
            '($5 == project || ($5 == "" && $3 == project)) &&
             (state == "" || $2 == state) { print $1 }'
}

#
# True when the environment carries hm.* metadata
#
hm_environment_has_metadata() {
    [ -n "$(hm_container_table | awk -F'|' -v project="$1" '$5 == project { print $1; exit }')" ]
}

#
# All Dockergento environments on the machine, one name per line.
# Environments created before the hm.* labels are recognised by their phpfpm service.
#
hm_environments() {
    hm_container_table |
        awk -F'|' '
            $5 != "" { print $5; next }
            $4 == "phpfpm" && $3 != "" { print $3 }
        ' | sort -u | sed '/^$/d'
}

#
# Directory an environment was started from, falling back to the compose working dir
#
hm_environment_root() {
    hm_container_table |
        awk -F'|' -v project="$1" '
            $5 == project && $6 != "" { print $6; exit }
            $5 == "" && $3 == project && $12 != "" { print $12; exit }
        '
}

#
# An environment is orphaned when the directory it was started from is gone
#
hm_environment_is_orphan() {
    local root
    root=$(hm_environment_root "$1")

    [ -n "$root" ] && [ ! -d "$root" ]
}

#
# Current branch of an environment, derived at read time so that it is never stale:
#   hm_environment_branch <project> [root]
#
hm_environment_branch() {
    local root="${2:-}"

    if [ -z "$root" ]; then
        root=$(hm_environment_root "$1")
    fi

    if [ -z "$root" ] || [ ! -d "$root" ]; then
        return 0
    fi

    git -C "$root" rev-parse --abbrev-ref HEAD 2>/dev/null || true
}

#
# State of a container by id
#
hm_container_state() {
    hm_container_table | awk -F'|' -v id="$1" '$1 == id { print $2; exit }'
}

#
# Container of a service inside a project, without matching container names by substring:
#   hm_service_container <service> [project]
#
hm_service_container() {
    local service="$1"
    local project="${2:-${COMPOSE_PROJECT_NAME:-}}"

    hm_container_table |
        awk -F'|' -v project="$project" -v service="$service" \
            '($5 == project || ($5 == "" && $3 == project)) && $4 == service && $2 == "running" { print $1; exit }'
}

#
# Container of a service regardless of its state
#
hm_service_container_any() {
    local service="$1"
    local project="${2:-${COMPOSE_PROJECT_NAME:-}}"

    hm_container_table |
        awk -F'|' -v project="$project" -v service="$service" \
            '($5 == project || ($5 == "" && $3 == project)) && $4 == service { print $1; exit }'
}
