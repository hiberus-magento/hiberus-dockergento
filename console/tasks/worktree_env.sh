#!/usr/bin/env bash

#
# Branch environments: a git worktree with a Docker environment of its own.
#
# The registration lives in ~/.hm/worktrees/<project>/, not in the checkout. A worktree shares
# the repository's tracked files, and config/docker/properties.json is one of them: writing the
# worktree's project name there would put it in somebody's commit and rename the main
# environment at the same time.
#
# That file is also the switch. A worktree with no registration keeps the guardrails of WT-01 —
# it is still the case that repoints the main environment's mounts and destroys its database.
#

HM_WORKTREE_HOME="${HM_WORKTREE_DIR:-$HOME/.hm/worktrees}"

#
# The services each profile keeps. Empty means "everything the project has".
#
# A branch environment that also runs Varnish, TLS termination, a mail catcher and a message
# queue costs more than the branch is worth; one without a search engine fails on the first
# reindex, which is not a surprise to leave in an environment meant for unattended work.
#
hm_worktree_profile_keeps() {
    case "$1" in
        lite)  printf 'phpfpm' ;;
        agent) printf 'phpfpm nginx db search redis' ;;
        full)  printf '' ;;
        *)     return 1 ;;
    esac
}

hm_worktree_profiles() {
    printf 'lite agent full'
}

#
# A branch name is not a host name, a volume name or a file name. This makes it all three.
#
hm_worktree_slug() {
    local slug
    slug=$(printf '%s' "$1" | tr '[:upper:]' '[:lower:]' | tr -c 'a-z0-9-' '-')

    # No leading, trailing or repeated dashes: they are legal in a host name only in the middle
    while [ "$slug" != "${slug//--/-}" ]; do slug="${slug//--/-}"; done
    slug="${slug#-}"
    slug="${slug%-}"

    printf '%s' "$slug"
}

hm_worktree_home() {
    printf '%s/%s' "$HM_WORKTREE_HOME" "$1"
}

hm_worktree_record() {
    printf '%s/%s/%s.json' "$HM_WORKTREE_HOME" "$1" "$2"
}

hm_worktree_overlay_file() {
    printf '%s/%s/%s.yml' "$HM_WORKTREE_HOME" "$1" "$2"
}

hm_worktree_is_registered() {
    [ -f "$(hm_worktree_record "$1" "$2")" ]
}

#
# Assigns WORKTREE_PATH, WORKTREE_BRANCH, WORKTREE_PROFILE, WORKTREE_DOMAIN, WORKTREE_PROJECT
# and WORKTREE_CREATED in the current shell, for the reason the rest of the resolvers do: read
# through `$(...)` only one of the six could come back.
#
hm_worktree_load() {
    local record
    record=$(hm_worktree_record "$1" "$2")

    [ -f "$record" ] || return 1

    local line
    line=$(jq -r '[.path, .branch, .profile, .domain, .project, .created] | @tsv' < "$record" 2>/dev/null) || return 1

    IFS=$'\t' read -r WORKTREE_PATH WORKTREE_BRANCH WORKTREE_PROFILE \
        WORKTREE_DOMAIN WORKTREE_PROJECT WORKTREE_CREATED <<< "$line"

    [ -n "$WORKTREE_PATH" ]
}

hm_worktree_names() {
    local home file base
    home=$(hm_worktree_home "$1")

    [ -d "$home" ] || return 0

    for file in "$home"/*.json; do
        [ -f "$file" ] || continue
        base="${file##*/}"
        printf '%s\n' "${base%.json}"
    done
}

hm_worktree_save() {
    local project="$1" name="$2" path="$3" branch="$4" profile="$5" domain="$6" child="$7"

    mkdir -p "$(hm_worktree_home "$project")"

    jq -n --arg path "$path" --arg branch "$branch" --arg profile "$profile" \
        --arg domain "$domain" --arg project "$child" --arg parent "$project" \
        --arg created "$(date "+%Y-%m-%d %H:%M")" '$ARGS.named' \
        > "$(hm_worktree_record "$project" "$name")"
}

hm_worktree_forget() {
    rm -f "$(hm_worktree_record "$1" "$2")" "$(hm_worktree_overlay_file "$1" "$2")"
    rmdir "$(hm_worktree_home "$1")" 2>/dev/null || true
}

#
# The service that answers on the branch's address.
#
# With the full stack that is Varnish, as in the main environment. Without it, nginx: a profile
# that drops the page cache should still answer, and pointing the router at a service the
# profile removed is a 404 nobody would understand.
#
hm_worktree_web_service() {
    case "$1" in
        full) printf 'varnish 6081' ;;
        *)    printf 'nginx 8080' ;;
    esac
}

#
# The overlay that turns the project's configuration into this branch's environment.
#
# It does the whole job on its own — the profile, the ports and the routing — because the
# project's own proxy overlay must NOT be loaded here: it claims the main environment's Host
# rule, and two containers claiming one rule is a router that answers with either.
#
hm_worktree_write_overlay() {
    local file="$1" profile="$2" domain="$3" router="$4" services="$5" network="$6"

    local keeps web port
    keeps=$(hm_worktree_profile_keeps "$profile") || return 1
    read -r web port <<< "$(hm_worktree_web_service "$profile")"

    # A profile that keeps no web service answers on no address: `lite` is code without HTTP,
    # and a router pointing at a service that was removed is a 404 with an explanation nobody
    # has
    if [ -n "$keeps" ] && ! printf '%s\n' $keeps | grep -qx "$web"; then
        web=""
    fi

    mkdir -p "$(dirname "$file")"

    #
    # Every service is written exactly once. A repeated key in YAML is not a merge: the last one
    # wins and the earlier block disappears without a word.
    #
    {
        printf '# Generated by %s for the branch environment %s. Edits are lost.\n' \
            "${COMMAND_BIN_NAME:-hm}" "$router"
        printf 'services:\n'

        local service
        for service in $services; do
            if [ -n "$keeps" ] && ! printf '%s\n' $keeps | grep -qx "$service"; then
                printf '  %s: !reset null\n' "$service"
                continue
            fi

            if [ "$service" == "$web" ]; then
                printf '  %s:\n    ports: !reset []\n' "$service"
                [ -n "$keeps" ] && printf '    depends_on: !reset []\n'
                printf '    networks:\n      - default\n      - %s\n' "$network"
                printf '    labels:\n'
                printf '      traefik.enable: "true"\n'
                printf '      traefik.docker.network: "%s"\n' "$network"
                printf '      traefik.http.routers.%s.rule: "Host(`%s`)"\n' "$router" "$domain"
                printf '      traefik.http.routers.%s.entrypoints: "websecure"\n' "$router"
                printf '      traefik.http.routers.%s.tls: "true"\n' "$router"
                printf '      traefik.http.services.%s.loadbalancer.server.port: "%s"\n' "$router" "$port"
                continue
            fi

            printf '  %s:\n    ports: !reset []\n' "$service"
            [ -n "$keeps" ] && printf '    depends_on: !reset []\n'
        done

        printf 'networks:\n  %s:\n    external: true\n' "$network"
    } > "$file"
}
