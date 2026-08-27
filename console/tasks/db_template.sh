#!/usr/bin/env bash

#
# Frozen copies of a database's data directory.
#
# `hm db snapshot` writes a dump: portable, small, and readable a year and two server versions
# later. This is the other half — a byte copy of the data directory in a Docker volume, which is
# not portable at all and is available again in seconds instead of the tens of minutes an import
# of the same data costs. One is for keeping, the other for standing environments up.
#
# The copy is made with the project's own database image. It is already on the machine, so
# nothing is pulled, and its GNU `cp -a` reproduces a data directory — ownership, sockets, sparse
# files — where busybox's would need arguing with.
#

HM_TEMPLATE_LABEL="hm.template"

#
# Assigns TEMPLATE_PROJECT and TEMPLATE_NAME from `<name>` or `<project>/<name>`.
#
# It assigns rather than prints for the reason the worktree resolver does: read through
# `$(...)`, the two halves would be split in a subshell and only one of them could come back.
#
hm_template_parse() {
    local address="$1"

    case "$address" in
        */*)
            TEMPLATE_PROJECT="${address%%/*}"
            TEMPLATE_NAME="${address#*/}"
            ;;
        *)
            TEMPLATE_PROJECT="$COMPOSE_PROJECT_NAME"
            TEMPLATE_NAME="$address"
            ;;
    esac
}

hm_template_address() {
    printf '%s/%s' "$1" "$2"
}

hm_template_volume() {
    printf 'hm-template-%s-%s' "$1" "$2"
}

hm_template_exists() {
    docker volume inspect "$1" >/dev/null 2>&1
}

#
# One line per template: project, name, size, image, created, volume
#
hm_template_rows() {
    docker volume ls --filter "label=$HM_TEMPLATE_LABEL" --format \
        '{{.Label "hm.project"}}	{{.Label "hm.template"}}	{{.Label "hm.size"}}	{{.Label "hm.db_image"}}	{{.Label "hm.created"}}	{{.Name}}' \
        2>/dev/null | sort
}

#
# The volume compose will mount as the database's data directory, and the image it will run.
#
# Both come from the resolved configuration rather than from a name built here: the project name
# can be overridden, the volume can be renamed in an overlay, and a guess that is right for most
# projects is the kind of thing that destroys the data of the rest.
#
hm_template_project_volume() {
    compose_config_json | jq -r '(.volumes // {}) | (.dbdata // {}) | .name // ""'
}

hm_template_project_image() {
    compose_config_json | jq -r '(.services // {}) | (.db // {}) | .image // ""'
}

#
# Bytes of a volume, measured from inside a container because on macOS the volume lives in a
# virtual machine and its mountpoint does not exist on the host filesystem.
#
hm_template_measure() {
    docker run --rm -v "$1":/from:ro --entrypoint /bin/bash "$2" -c \
        'du -s -B1 /from | cut -f1' 2>/dev/null
}

hm_template_human_size() {
    local bytes="${1:-0}"

    awk -v b="$bytes" 'BEGIN {
        split("B KB MB GB TB", unit, " ")
        i = 1
        while (b >= 1024 && i < 5) { b /= 1024; i++ }
        printf (i == 1 ? "%d%s" : "%.1f%s"), b, unit[i]
    }'
}

#
# Copy one data directory over another.
#
# The destination is emptied first: a data directory restored on top of another is neither of
# the two, and InnoDB will start on the mixture and only say so later.
#
hm_template_copy() {
    local source="$1" destination="$2" image="$3"

    docker run --rm -v "$source":/from:ro -v "$destination":/to --entrypoint /bin/bash "$image" -c '
        if [ -z "$(ls -A /from)" ]; then exit 3; fi
        find /to -mindepth 1 -delete 2>/dev/null || true
        cp -a /from/. /to/
    '
}

#
# Containers of any state holding a volume. A stopped container still holds it, which is why
# `docker volume rm` refuses, and saying so beforehand is better than relaying that error.
#
hm_template_users() {
    docker ps -a --filter "volume=$1" --format '{{.Names}}' 2>/dev/null
}
