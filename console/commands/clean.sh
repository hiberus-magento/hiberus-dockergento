#!/usr/bin/env bash
set -uo pipefail

source "$COMPONENTS_DIR"/print_message.sh
source "$COMPONENTS_DIR"/print_json.sh
source "$COMPONENTS_DIR"/input_info.sh
source "$COMPONENTS_DIR"/progress.sh
source "$HELPERS_DIR"/exit_codes.sh
source "$HELPERS_DIR"/docker.sh
source "$TASKS_DIR"/worktree_env.sh

#
# Collect what abandoned environments left behind.
#
# The whole design of this command is about what it is *not* allowed to touch. `docker system
# prune` already exists and is the wrong tool: it cannot tell our leftovers from somebody's
# hand-written stack, or a dead project from a stopped one.
#
# Two facts shape it, both checked rather than assumed:
#
#   - Data volumes carry no hm.* labels, only Compose's. So a volume can only be attributed through the
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
    awk -F'|' '$5 != "" { print $5 "|" $6 }' | LC_ALL=C sort -u)"

# Environments without hm.* labels: known only by their phpfpm service, with no root to check
while IFS='|' read -r name; do
    [ -z "$name" ] && continue
    unattributable="${unattributable}${name}\tno hm labels\n"
done <<< "$(hm_container_table |
    awk -F'|' '$5 == "" && $4 == "phpfpm" { print $3 }' | LC_ALL=C sort -u)"

#
# Volumes are attributed through their project's containers. A volume whose project has no
# containers left could belong to anything, so it is listed and left alone.
#
attributable_projects=$(printf "$collectable" | awk -F'\t' 'NF { print $1 }')
known_projects=$(hm_container_table | awk -F'|' '{ print $3 }' | LC_ALL=C sort -u)

collectable_volumes=""
orphan_volumes=""

while IFS= read -r volume; do
    [ -z "$volume" ] && continue
    project=$(docker volume inspect "$volume" \
        --format '{{index .Labels "com.docker.compose.project"}}' 2>/dev/null)

    #
    # A frozen data directory (`hm db freeze`) belongs to no compose project, so the rule above
    # cannot see it — and they are the largest volumes the tool makes. It carries the project
    # that made it and where that project lived, which is the same question asked of everything
    # else here: is the directory still there?
    #
    if [ -z "$project" ]; then
        template_root=$(docker volume inspect "$volume" \
            --format '{{index .Labels "hm.template"}}|{{index .Labels "hm.root"}}' 2>/dev/null)

        case "$template_root" in
            "" | "|"*) continue ;;
        esac

        [ -d "${template_root#*|}" ] || collectable_volumes="${collectable_volumes}${volume}\n"
        continue
    fi

    if printf '%s\n' "$attributable_projects" | grep -qx "$project"; then
        collectable_volumes="${collectable_volumes}${volume}\n"
    elif ! printf '%s\n' "$known_projects" | grep -qx "$project"; then
        orphan_volumes="${orphan_volumes}${volume}\n"
    fi
done <<< "$(docker volume ls -q 2>/dev/null)"

#
# Branch environments whose worktree is gone.
#
# The containers and volumes of one are collected by the rules above already — they carry
# `hm.root`, and that directory no longer exists. What is left over is the registration in
# ~/.hm/worktrees, which nothing else deletes: `hm worktree remove` is the tidy path and it needs
# the directory to still be there. Somebody who removes a worktree with git, or deletes the folder,
# leaves an entry that makes `worktree list` say "missing" for ever and refuses the name if they
# ever want it back.
#
orphan_worktrees=""

for registry in "$HM_WORKTREE_HOME"/*; do
    [ -d "$registry" ] || continue
    parent=$(basename "$registry")

    while IFS= read -r name; do
        [ -z "$name" ] && continue
        hm_worktree_load "$parent" "$name" || continue
        [ -d "$WORKTREE_PATH" ] && continue

        orphan_worktrees="${orphan_worktrees}${parent}/${name}\t${WORKTREE_PROJECT}\t${WORKTREE_PATH}\n"
    done <<< "$(hm_worktree_names "$parent")"
done

#
# Entries this tool put in /etc/hosts whose environment no longer exists.
#
# Listed and never removed here. It needs sudo, it is a file other things depend on, and a
# command that quietly rewrites it in the middle of an unrelated cleanup is one nobody trusts
# twice. What it can do is find them — which nothing could before the entries were marked.
#
orphan_hosts=""

if [ -r /etc/hosts ]; then
    known_domains=$(hm_container_table | awk -F'|' '{ print $3 }' | LC_ALL=C sort -u)

    while IFS= read -r domain; do
        [ -z "$domain" ] && continue

        # A domain belongs to a live environment when some project on the machine is named after
        # its first label — which is how this tool builds them
        printf '%s\n' "$known_domains" | grep -qx "${domain%%.*}" && continue

        orphan_hosts="${orphan_hosts}${domain}\n"
    done <<< "$(grep "added by $COMMAND_BIN_NAME" /etc/hosts 2>/dev/null |
        awk '{ for (i = 1; i <= NF; i++) if ($i ~ /\./) { print $i; break } }')"
fi

count_lines() {
    [ -z "$1" ] && { printf '0'; return 0; }
    printf "$1" | sed '/^$/d' | grep -c . | tr -d ' '
}

environments=$(count_lines "$collectable")
volumes=$(count_lines "$collectable_volumes")
strays=$(count_lines "$orphan_volumes")
unknown=$(count_lines "$unattributable")
worktrees=$(count_lines "$orphan_worktrees")
hosts=$(count_lines "$orphan_hosts")

# ------------------------------------------------------------------ report

#
# The accumulators above are built with `\n` and `\t` inside double quotes, which are two
# characters and not one. The text report prints them through `printf`, which interprets them; jq
# does not, so `split("\n")` found nothing to split and every list arrived as a single entry with
# the whole blob inside it and a null reason.
#
# Interpreted here and not by reassigning the variables: `$(...)` eats the trailing newline, and
# the deletion below reads them with `while read`, which drops the last line without one.
#
if is_json_output; then
    json_success "clean" "$(jq -n \
        --arg collectable "$(printf '%b' "$collectable")" \
        --arg volumes "$(printf '%b' "$collectable_volumes")" \
        --arg strays "$(printf '%b' "$orphan_volumes")" \
        --arg unknown "$(printf '%b' "$unattributable")" \
        --arg worktrees "$(printf '%b' "$orphan_worktrees")" \
        --arg hosts "$(printf '%b' "$orphan_hosts")" \
        --argjson removed "$($remove && echo true || echo false)" \
        '{
            removed: $removed,
            environments: ($collectable | split("\n") | map(select(length > 0) | split("\t") |
                {name: .[0], root: .[1]})),
            volumes: ($volumes | split("\n") | map(select(length > 0))),
            worktrees: ($worktrees | split("\n") | map(select(length > 0) | split("\t") |
                {name: .[0], project: .[1], path: .[2]})),
            hosts: ($hosts | split("\n") | map(select(length > 0))),
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

    if [ "$worktrees" -gt 0 ]; then
        printf '\n'
        print_heading "Branch environments whose worktree is gone\n\n"
        printf "$orphan_worktrees" | while IFS=$'\t' read -r name project path; do
            [ -n "$name" ] && printf '  %-28s was at %s\n' "$name" "$path"
        done
    fi

    if [ "$hosts" -gt 0 ]; then
        printf '\n'
        print_heading "Entries in /etc/hosts with no environment left\n\n"
        printf "$orphan_hosts" | while IFS= read -r domain; do
            [ -n "$domain" ] && printf '  %-32s %s set-host --remove %s\n' \
                "$domain" "$COMMAND_BIN_NAME" "$domain"
        done
        printf '\n  Not removed from here: that file needs a password and other things depend on it.\n'
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

if [ "$environments" -eq 0 ] && [ "$worktrees" -eq 0 ]; then
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
    hm_start "Working out how much space this frees..."

    freed=$(docker system df -v --format '{{json .Volumes}}' 2>/dev/null |
        jq -r --arg names "$collectable_volumes" '
            ($names | split("\n") | map(select(length > 0))) as $wanted
            | map(select(.Name as $n | $wanted | index($n)) | .Size) | join(" ")' 2>/dev/null)

    hm_stop 0

    printf '\n'
    print_warning "This deletes $environments environment(s), $volumes volume(s) and $worktrees branch environment(s).\n"
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

#
# The worktree's own containers and volumes are deleted by name, not by asking Compose: the
# directory that held its configuration is exactly what is missing.
#
printf "$orphan_worktrees" | while IFS=$'\t' read -r name project path; do
    [ -z "$name" ] && continue

    containers=$(docker ps -aq --filter "label=com.docker.compose.project=$project" 2>/dev/null)
    [ -n "$containers" ] && docker rm -f $containers >/dev/null 2>&1

    for volume in $(docker volume ls -q 2>/dev/null | grep "^${project}_" || true); do
        docker volume rm "$volume" >/dev/null 2>&1
    done

    hm_worktree_forget "${name%%/*}" "${name#*/}"
    print_info "Removed the branch environment $name\n"
done

printf "$collectable_volumes" | while IFS= read -r volume; do
    [ -z "$volume" ] && continue
    docker volume rm "$volume" >/dev/null 2>&1 && print_info "Removed volume $volume\n"
done

print_info "Done.\n"
