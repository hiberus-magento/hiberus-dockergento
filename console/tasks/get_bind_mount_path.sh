#!/usr/bin/env bash
set -euo pipefail

sanitize_path() {
    sanitized_path=$(echo "$1" | sed "s#/./#/#g")
    sanitized_path=${sanitized_path%/}
    sanitized_path=${sanitized_path%/.}
    echo "$sanitized_path"
}

PATH_TO_CHECK="$1"
BIND_PATH_NEEDLE="bind:${PATH_TO_CHECK}"

CONTAINER_ID=$($DOCKER_COMPOSE ps -q phpfpm)
if [[ ${CONTAINER_ID} != "" ]]; then
    MOUNTS=$(docker container inspect -f '{{ range .Mounts }}{{ .Type }}:{{ .Destination }} {{ end }}' "${CONTAINER_ID}")

    for MOUNT in ${MOUNTS}; do
        BIND_PATH_NEEDLE=$(sanitize_path "${BIND_PATH_NEEDLE}")
        MOUNT=$(sanitize_path "${MOUNT}")
        if [[ "${BIND_PATH_NEEDLE}" == "${MOUNT}" ]] ||
            # needle path inside bind path
            [[ "${BIND_PATH_NEEDLE}" == "${MOUNT}/"* ]] ||
            # needle path contains a bind path
            [[ "${MOUNT}" == "${BIND_PATH_NEEDLE}/"* ]]; then
            echo "${MOUNT#bind:}"
            exit 0
        fi
    done
fi

echo false
exit 0
