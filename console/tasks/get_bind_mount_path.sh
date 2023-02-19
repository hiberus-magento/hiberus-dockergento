#!/usr/bin/env bash
set -euo pipefail

sanitize_path() {
    sanitized_path=$(echo "$1" | sed "s#/./#/#g")
    sanitized_path=${sanitized_path%/}
    sanitized_path=${sanitized_path%/.}
    echo "$sanitized_path"
}

bind_path_needle="bind:$1"
container_id=$($DOCKER_COMPOSE ps -q phpfpm)

if [[ $container_id != "" ]]; then
    mounts=$(docker container inspect -f '{{ range .Mounts }}{{ .Type }}:{{ .Destination }} {{ end }}' "$container_id")

    for mount in $mounts; do
        bind_path_needle=$(sanitize_path "$bind_path_needle")
        mount=$(sanitize_path "$mount")
        
        if [[ "$bind_path_needle" == "$mount" ]] ||
            # needle path inside bind path
            [[ "$bind_path_needle" == "$mount/"* ]] ||
            # needle path contains a bind path
            [[ "$mount" == "$bind_path_needle/"* ]]; then
            echo "${mount#bind:}"
            exit 0
        fi
    done
fi

echo false
exit 0
