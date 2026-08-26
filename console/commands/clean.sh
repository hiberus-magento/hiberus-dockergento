#!/usr/bin/env bash
set -uo pipefail

source "$COMPONENTS_DIR"/print_message.sh
source "$COMPONENTS_DIR"/print_json.sh
source "$COMPONENTS_DIR"/input_info.sh
source "$HELPERS_DIR"/exit_codes.sh
source "$HELPERS_DIR"/docker.sh

#
# Collect what abandoned environments left behind.
#
# The whole design of this command is about what it is *not* allowed to touch. `docker system
# prune` already exists and is the wrong tool: it cannot tell our leftovers from somebody's
# hand-written stack, or a dead project from a stopped one.
#
# Two facts shape it, both checked rather than assumed:
#
#   - Volumes carry no hm.* labels, only Compose's. So a volume can only be attributed through the
#     containers of its project. Where those are gone, it cannot be attributed at all.
#   - Working out volume sizes takes about 25 seconds on a machine with a hundred of them, so that
#     only happens on the path that is about to delete something.
#
# Looking is the default. Deleting is --force. That way round on purpose: a --dry-run you have to
# remember to type protects the people who were already being careful.
#

remove=false

for argument in "$@"; do
    case "$argument" in
        --force) remove=true ;;
        *)
            hm_fail "$HM_EXIT_USAGE" "invalid_argument" "Unknown option: $argument" \
                "$COMMAND_BIN_NAME clean [--force]"
            ;;
    esac
done

# `--force` is also the global flag, so it arrives either way
[ "${HM_FORCE:-}" == "1" ] && remove=true

hm_load_container_table

#
# An environment is collectable when both are true: we made it, and its directory is gone.
#
# A stopped project whose directory is still there is not rubbish, it is a stopped project. That
# distinction is the reason this command exists.
#
collectable=""
unattributable=""

while IFS='|' read -r name root; do
    [ -z "$name" ] && continue

    if [ -z "$root" ]; then
        unattributable="${unattributable}${name}\tno recorded directory\n"
        continue
    fi

    if [ -d "$root" ]; then
        continue
    fi

    collectable="${collectable}${name}\t${root}\n"
done <<< "$(hm_container_table |
    awk -F'|' '$5 != "" { print $5 "|" $6 }' | sort -u)"

# Environments without hm.* labels: known only by their phpfpm service, with no root to check
while IFS='|' read -r name; do
    [ -z "$name" ] && continue
    unattributable="${unattributable}${name}\tno hm labels\n"
done <<< "$(hm_container_table |
    awk -F'|' '$5 == "" && $4 == "phpfpm" { print $3 }' | sort -u)"

#
# Volumes are attributed through their project's containers. A volume whose project has no
# containers left could belong to anything, so it is listed and left alone.
#
attributable_projects=$(printf "$collectable" | awk -F'\t' 'NF { print $1 }')
known_projects=$(hm_container_table | awk -F'|' '{ print $3 }' | sort -u)

collectable_volumes=""
orphan_volumes=""

while IFS= read -r volume; do
    [ -z "$volume" ] && continue
    project=$(docker volume inspect "$volume" \
        --format '{{index .Labels "com.docker.compose.project"}}' 2>/dev/null)
    [ -z "$project" ] && continue

    if printf '%s\n' "$attributable_projects" | grep -qx "$project"; then
        collectable_volumes="${collectable_volumes}${volume}\n"
    elif ! printf '%s\n' "$known_projects" | grep -qx "$project"; then
        orphan_volumes="${orphan_volumes}${volume}\n"
    fi
done <<< "$(docker volume ls -q 2>/dev/null)"

count_lines() {
    [ -z "$1" ] && { printf '0'; return 0; }
    printf "$1" | sed '/^$/d' | grep -c . | tr -d ' '
}

environments=$(count_lines "$collectable")
volumes=$(count_lines "$collectable_volumes")
strays=$(count_lines "$orphan_volumes")
unknown=$(count_lines "$unattributable")

# ------------------------------------------------------------------ report

if is_json_output; then
    json_success "clean" "$(jq -n \
        --arg collectable "$collectable" \
        --arg volumes "$collectable_volumes" \
        --arg strays "$orphan_volumes" \
        --arg unknown "$unattributable" \
        --argjson removed "$($remove && echo true || echo false)" \
        '{
            removed: $removed,
            environments: ($collectable | split("\n") | map(select(length > 0) | split("\t") |
                {name: .[0], root: .[1]})),
            volumes: ($volumes | split("\n") | map(select(length > 0))),
            unattributable: {
                volumes: ($strays | split("\n") | map(select(length > 0))),
                environments: ($unknown | split("\n") | map(select(length > 0) | split("\t") |
                    {name: .[0], reason: .[1]}))
            }
        }')"
    $remove || exit 0
fi

if ! is_json_output; then
    printf '\n'

    if [ "$environments" -eq 0 ]; then
        print_info "Nothing to collect: every environment on this machine still has its directory.\n"
    else
        print_heading "Environments whose directory is gone\n\n"
        printf "$collectable" | while IFS=$'\t' read -r name root; do
            [ -n "$name" ] && printf '  %-28s was at %s\n' "$name" "$root"
        done
        printf '\n  %s container group(s), %s volume(s)\n' "$environments" "$volumes"
    fi

    if [ "$strays" -gt 0 ] || [ "$unknown" -gt 0 ]; then
        printf '\n'
        print_warning "Cannot be attributed, so they are left alone\n\n"
        print_default "  Volumes carry no hm labels, so a project with no containers left could\n"
        print_default "  belong to anything. These are yours to judge:\n\n"

        printf "$orphan_volumes" | while IFS= read -r volume; do
            [ -n "$volume" ] && printf '  %s\n' "$volume"
        done
        printf "$unattributable" | while IFS=$'\t' read -r name reason; do
            [ -n "$name" ] && printf '  %-28s %s\n' "$name" "$reason"
        done
    fi

    printf '\n'
fi

if [ "$environments" -eq 0 ]; then
    exit 0
fi

if ! $remove; then
    if ! is_json_output; then
        print_default "  Nothing was deleted. To collect them:\n"
        print_code "  $COMMAND_BIN_NAME clean --force\n\n"
    fi
    exit 0
fi

# ------------------------------------------------------------------ remove

if ! is_non_interactive; then
    print_info "Working out how much space this frees...\n"

    freed=$(docker system df -v --format '{{json .Volumes}}' 2>/dev/null |
        jq -r --arg names "$collectable_volumes" '
            ($names | split("\n") | map(select(length > 0))) as $wanted
            | map(select(.Name as $n | $wanted | index($n)) | .Size) | join(" ")' 2>/dev/null)

    printf '\n'
    print_warning "This deletes $environments environment(s) and $volumes volume(s).\n"
    [ -n "$freed" ] && print_warning "Volume sizes: $freed\n"
    print_warning "Their database snapshots are not touched.\n\n"

    confirm "Delete them? [y/N]: "

    case "$REPLY" in
        Y | y) ;;
        *)
            print_info "Nothing was deleted.\n"
            exit 0
            ;;
    esac
fi

printf "$collectable" | while IFS=$'\t' read -r name root; do
    [ -z "$name" ] && continue
    containers=$(docker ps -aq --filter "label=hm.project=$name" 2>/dev/null)
    [ -n "$containers" ] && docker rm -f $containers >/dev/null 2>&1
    print_info "Removed containers of $name\n"
done

printf "$collectable_volumes" | while IFS= read -r volume; do
    [ -z "$volume" ] && continue
    docker volume rm "$volume" >/dev/null 2>&1 && print_info "Removed volume $volume\n"
done

print_info "Done.\n"
