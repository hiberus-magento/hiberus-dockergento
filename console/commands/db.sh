#!/usr/bin/env bash
set -uo pipefail

source "$COMPONENTS_DIR"/print_message.sh
source "$COMPONENTS_DIR"/print_json.sh
source "$COMPONENTS_DIR"/input_info.sh
source "$HELPERS_DIR"/exit_codes.sh
source "$HELPERS_DIR"/docker.sh
source "$TASKS_DIR"/collect_project_info.sh
source "$TASKS_DIR"/db_template.sh
source "$TASKS_DIR"/anonymisation.sh

#
# Named copies of this project's database.
#
# `hm mysqldump` and `hm mysql -i` already move a database in and out of a file. What was missing
# was management: a place for the copies to live, a name to call them by, and a list to see them
# in. Without that, saving before a risky operation depends on somebody thinking of it, which is
# why it does not happen.
#
# The copies live outside the project, next to the cache, for two reasons that matter more than
# tidiness: config/docker is committed, so a copy there would end up in somebody's commit; and a
# copy stored inside the environment would not survive `down -v`, which is exactly the moment it
# is needed.
#

SNAPSHOT_ROOT="${HM_SNAPSHOT_DIR:-$HOME/.hm/snapshots}/${COMPOSE_PROJECT_NAME}"

#
# The database client and dumper, resolved inside the container: MariaDB 11 dropped the legacy
# names and 10.x does not have the new ones
#
DB_DUMP='dump=$(command -v mariadb-dump || command -v mysqldump)'
DB_CLIENT='client=$(command -v mariadb || command -v mysql)'

usage() {
    print_info "Named copies of this project's database\n\n"
    print_default "  $COMMAND_BIN_NAME db snapshot [--name=<name>] [--force]\n"
    print_default "  $COMMAND_BIN_NAME db list\n"
    print_default "  $COMMAND_BIN_NAME db restore <name>\n"
    print_default "  $COMMAND_BIN_NAME db remove <name>\n"
    print_default "  $COMMAND_BIN_NAME db clear [--all]\n\n"
    print_info "Frozen copies of the data directory, to stand environments up in seconds\n\n"
    print_default "  $COMMAND_BIN_NAME db freeze [--name=<name>] [--force]\n"
    print_default "  $COMMAND_BIN_NAME db templates\n"
    print_default "  $COMMAND_BIN_NAME db clone [<project>/]<name> [--force]\n"
    print_default "  $COMMAND_BIN_NAME db drop [<project>/]<name>\n\n"
}

#
# A name that can be a file name, and nothing else
#
validate_name() {
    case "$1" in
        "" | *[!A-Za-z0-9._-]* | -* | .*)
            hm_fail "$HM_EXIT_USAGE" "invalid_name" \
                "'$1' cannot be used as a snapshot name" \
                "Letters, digits, dots, dashes and underscores, not starting with a dot or a dash"
            ;;
    esac
}

snapshot_path() {
    printf '%s/%s.sql.gz' "$SNAPSHOT_ROOT" "$1"
}

existing_names() {
    [ -d "$SNAPSHOT_ROOT" ] || return 0
    for file in "$SNAPSHOT_ROOT"/*.sql.gz; do
        [ -f "$file" ] || continue
        local base="${file##*/}"
        printf '%s\n' "${base%.sql.gz}"
    done
}

# ------------------------------------------------------------------ snapshot

do_snapshot() {
    local name=""

    # --force is a global option, parsed by the router before the command sees it. Declaring a
    # second one here would mean two flags with the same name and different owners.
    local force=false
    [ "${HM_FORCE:-}" == "1" ] && force=true

    while [ "$#" -gt 0 ]; do
        case "$1" in
            --name=*) name="${1#--name=}"; shift ;;
            --name)   name="${2:-}"; shift 2 ;;
            *)
                hm_fail "$HM_EXIT_USAGE" "invalid_argument" "Unknown option: $1" \
                    "$COMMAND_BIN_NAME db snapshot --name=before-upgrade"
                ;;
        esac
    done

    [ -z "$name" ] && name=$(date "+%Y-%m-%d-%H%M%S")
    validate_name "$name"

    local target
    target=$(snapshot_path "$name")

    if [ -f "$target" ] && ! $force; then
        hm_fail "$HM_EXIT_USAGE" "snapshot_exists" \
            "This project already has a snapshot called '$name'" \
            "$COMMAND_BIN_NAME db snapshot --name=$name --force"
    fi

    is_run_service "db"
    mkdir -p "$SNAPSHOT_ROOT"

    local magento_version="${HM_MAGENTO:-unknown}"
    local taken_at
    taken_at=$(date "+%Y-%m-%d %H:%M:%S")

    print_info "Saving the database as '$name'...\n"

    #
    # --single-transaction takes the copy from a consistent snapshot without locking the tables,
    # so the project keeps working while it runs. Routines, triggers and events are included: a
    # copy that restores a Magento without them is not a copy of that Magento.
    #
    {
        printf -- '-- hm snapshot: %s\n-- taken: %s\n-- magento: %s\n' \
            "$name" "$taken_at" "$magento_version"

        $DOCKER_COMPOSE exec -T db bash -c \
            "$DB_DUMP"'; "$dump" --single-transaction --quick --no-tablespaces \
                --routines --triggers --events \
                -u"root" -p"$MYSQL_ROOT_PASSWORD" "$MYSQL_DATABASE"'
    } | gzip -6 > "$target.partial"

    # Renamed only once it is complete: an interrupted dump must not look like a usable snapshot
    mv "$target.partial" "$target"

    local size
    size=$(du -h "$target" | awk '{print $1}')

    if is_json_output; then
        json_success "db" "$(jq -n --arg name "$name" --arg path "$target" --arg size "$size" \
            --arg taken_at "$taken_at" '$ARGS.named')"
        return 0
    fi

    print_info "Saved "
    print_code "$name"
    print_info " ($size)\n"
}

# ------------------------------------------------------------------ list

do_list() {
    local rows="" file base size taken

    if [ -d "$SNAPSHOT_ROOT" ]; then
        for file in "$SNAPSHOT_ROOT"/*.sql.gz; do
            [ -f "$file" ] || continue
            base="${file##*/}"
            base="${base%.sql.gz}"
            size=$(du -h "$file" | awk '{print $1}')
            taken=$(date -r "$file" "+%Y-%m-%d %H:%M" 2>/dev/null ||
                    stat -c '%y' "$file" 2>/dev/null | cut -c1-16)
            rows="${rows}${base}\t${taken}\t${size}\n"
        done
    fi

    if is_json_output; then
        json_success "db" "$(printf "$rows" | jq -R -s --arg project "$COMPOSE_PROJECT_NAME" '
            {project: $project,
             snapshots: (split("\n") | map(select(length > 0) | split("\t") |
                {name: .[0], taken_at: .[1], size: .[2]}))}')"
        return 0
    fi

    if [ -z "$rows" ]; then
        print_info "No snapshots for this project yet.\n"
        print_default "  $COMMAND_BIN_NAME db snapshot --name=before-upgrade\n"
        return 0
    fi

    printf '\n'
    print_heading "Snapshots of $COMPOSE_PROJECT_NAME\n\n"
    printf "$rows" | while IFS=$'\t' read -r base taken size; do
        printf '  %-28s %-18s %s\n' "$base" "$taken" "$size"
    done
    printf '\n'
}

# ------------------------------------------------------------------ restore

do_restore() {
    local name="${1:-}"

    [ -z "$name" ] && hm_fail "$HM_EXIT_USAGE" "missing_name" \
        "Which snapshot should be restored?" "$COMMAND_BIN_NAME db list"

    validate_name "$name"

    local source_file
    source_file=$(snapshot_path "$name")

    if [ ! -f "$source_file" ]; then
        hm_fail "$HM_EXIT_USAGE" "unknown_snapshot" \
            "This project has no snapshot called '$name'" \
            "$COMMAND_BIN_NAME db list"
    fi

    is_run_service "db"

    #
    # The only destructive thing this command does, and it does not come back. Confirming means
    # typing the project's name rather than a letter: a blind `y` is a reflex, typing the name
    # means the sentence was read.
    #
    if ! is_non_interactive; then
        print_warning "This replaces the database of '$COMPOSE_PROJECT_NAME' with '$name'.\n"
        print_warning "Everything in it since that snapshot will be lost.\n\n"
        read -rp "$(print_question "Type the project name to confirm")" reply

        if [ "$reply" != "$COMPOSE_PROJECT_NAME" ]; then
            print_info "Nothing was restored.\n"
            return 0
        fi
    fi

    print_info "Restoring '$name'...\n"

    #
    # Emptied first. Restoring over a database that kept living would leave whatever was created
    # afterwards in place, and the result would not be the snapshot but a mixture of the two.
    #
    $DOCKER_COMPOSE exec -T db bash -c \
        "$DB_CLIENT"'; "$client" -u"root" -p"$MYSQL_ROOT_PASSWORD" \
            -e "DROP DATABASE IF EXISTS \`$MYSQL_DATABASE\`; CREATE DATABASE \`$MYSQL_DATABASE\`;"'

    gunzip -c "$source_file" | $DOCKER_COMPOSE exec -T db bash -c \
        "$DB_CLIENT"'; "$client" -u"root" -p"$MYSQL_ROOT_PASSWORD" "$MYSQL_DATABASE"'

    # Whatever that snapshot holds, nobody anonymised it after the fact
    hm_anonymisation_clear

    if is_json_output; then
        json_success "db" "$(jq -n --arg name "$name" --arg project "$COMPOSE_PROJECT_NAME" \
            '{restored: $name, project: $project}')"
        return 0
    fi

    print_info "Restored. Flush the cache with "
    print_code "$COMMAND_BIN_NAME magento cache:flush"
    printf '\n'
}

# ------------------------------------------------------------------ remove

do_remove() {
    local name="${1:-}"

    [ -z "$name" ] && hm_fail "$HM_EXIT_USAGE" "missing_name" \
        "Which snapshot should be removed?" "$COMMAND_BIN_NAME db list"

    validate_name "$name"

    local target
    target=$(snapshot_path "$name")

    if [ ! -f "$target" ]; then
        hm_fail "$HM_EXIT_USAGE" "unknown_snapshot" \
            "This project has no snapshot called '$name'" \
            "$COMMAND_BIN_NAME db list"
    fi

    rm -f "$target"

    if is_json_output; then
        json_success "db" "$(jq -n --arg name "$name" '{removed: $name}')"
        return 0
    fi

    print_info "Removed "
    print_code "$name"
    printf '\n'
}

# ------------------------------------------------------------------ clear

#
# Every snapshot of this project, or of all of them.
#
# `remove` deletes one by name; this is for reclaiming the space, which is the other reason to
# delete. Both ask, and the bulk one asks harder: it is the only command here that can destroy
# copies belonging to projects you are not standing in.
#
do_clear() {
    local scope="project"

    while [ "$#" -gt 0 ]; do
        case "$1" in
            --all) scope="all"; shift ;;
            *)
                hm_fail "$HM_EXIT_USAGE" "invalid_argument" "Unknown option: $1" \
                    "$COMMAND_BIN_NAME db clear [--all]"
                ;;
        esac
    done

    local root="${HM_SNAPSHOT_DIR:-$HOME/.hm/snapshots}"
    local targets="" confirmation="$COMPOSE_PROJECT_NAME" subject="$COMPOSE_PROJECT_NAME"

    if [ "$scope" == "all" ]; then
        confirmation="all"
        subject="every project on this machine"
        [ -d "$root" ] && targets=$(find "$root" -type f -name '*.sql.gz' 2>/dev/null | sort)
    else
        [ -d "$SNAPSHOT_ROOT" ] && targets=$(find "$SNAPSHOT_ROOT" -type f -name '*.sql.gz' 2>/dev/null | sort)
    fi

    local count=0 total="0B"
    if [ -n "$targets" ]; then
        count=$(printf '%s\n' "$targets" | grep -c .)
        total=$(printf '%s\n' "$targets" | tr '\n' '\0' | xargs -0 du -ch 2>/dev/null |
            tail -1 | awk '{print $1}')
    fi

    if [ "$count" -eq 0 ]; then
        if is_json_output; then
            json_success "db" "$(jq -n --argjson removed 0 '{removed: $removed, freed: "0B"}')"
            return 0
        fi
        print_info "There are no snapshots to clear.\n"
        return 0
    fi

    if ! is_non_interactive; then
        printf '\n'
        print_warning "This deletes $count snapshot(s) of $subject, freeing $total.\n"
        print_warning "There is no undo, and they are the only copies.\n\n"

        # What is being destroyed is named in the answer, so a reflex cannot do it
        printf '%s\n' "$targets" | while IFS= read -r file; do
            [ -n "$file" ] && printf '  %s\n' "${file#$root/}"
        done
        printf '\n'

        read -rp "$(print_question "Type '$confirmation' to confirm")" reply

        if [ "$reply" != "$confirmation" ]; then
            print_info "Nothing was deleted.\n"
            return 0
        fi
    fi

    printf '%s\n' "$targets" | while IFS= read -r file; do
        [ -n "$file" ] && rm -f "$file"
    done

    # Leave no empty project directories behind
    [ -d "$root" ] && find "$root" -type d -empty -delete 2>/dev/null

    if is_json_output; then
        json_success "db" "$(jq -n --argjson removed "$count" --arg freed "$total" \
            --arg scope "$scope" '$ARGS.named')"
        return 0
    fi

    print_info "Deleted $count snapshot(s), freeing $total.\n"
}

# ------------------------------------------------------------------ freeze

#
# Save the data directory as a template.
#
# The server is stopped while it copies. A running InnoDB keeps pages in memory that are not in
# the files yet, so a copy taken underneath it is a crash to be recovered from rather than a
# copy — and "usually recovers" is not a property a backup may have.
#
do_freeze() {
    local name="base"
    local force=false
    [ "${HM_FORCE:-}" == "1" ] && force=true

    while [ "$#" -gt 0 ]; do
        case "$1" in
            --name=*) name="${1#--name=}"; shift ;;
            --name)   name="${2:-}"; shift 2 ;;
            *)
                hm_fail "$HM_EXIT_USAGE" "invalid_argument" "Unknown option: $1" \
                    "$COMMAND_BIN_NAME db freeze --name=base"
                ;;
        esac
    done

    validate_name "$name"
    is_docker_service_running

    local volume image template
    volume=$(hm_template_project_volume)
    image=$(hm_template_project_image)
    template=$(hm_template_volume "$COMPOSE_PROJECT_NAME" "$name")

    if [ -z "$volume" ] || [ -z "$image" ]; then
        hm_fail "$HM_EXIT_PROJECT" "no_database_service" \
            "This project's configuration defines no database service" \
            "$COMMAND_BIN_NAME setup -f"
    fi

    if ! hm_template_exists "$volume"; then
        hm_fail "$HM_EXIT_BLOCKED" "no_data" \
            "This project has no data directory yet, so there is nothing to freeze" \
            "$COMMAND_BIN_NAME start"
    fi

    if hm_template_exists "$template" && ! $force; then
        hm_fail "$HM_EXIT_USAGE" "template_exists" \
            "There is already a template called '$(hm_template_address "$COMPOSE_PROJECT_NAME" "$name")'" \
            "$COMMAND_BIN_NAME db freeze --name=$name --force"
    fi

    local bytes size
    bytes=$(hm_template_measure "$volume" "$image")
    size=$(hm_template_human_size "${bytes:-0}")

    #
    # Only the database goes down, and only for the copy. Whatever happens next, it comes back
    # up: leaving somebody's environment half stopped because a disk filled up would be worse
    # than the failure itself.
    #
    local was_running=false
    if [ -n "$($DOCKER_COMPOSE ps -q --status running db 2>/dev/null)" ]; then
        was_running=true
        print_info "The database will be unavailable while it copies ($size)...\n"
        $DOCKER_COMPOSE stop db >/dev/null 2>&1
    else
        print_info "Copying the data directory ($size)...\n"
    fi

    $force && hm_template_exists "$template" && docker volume rm "$template" >/dev/null 2>&1

    docker volume create \
        --label "hm.template=$name" \
        --label "hm.project=$COMPOSE_PROJECT_NAME" \
        --label "hm.root=${HM_ROOT:-$PWD}" \
        --label "hm.db_image=$image" \
        --label "hm.created=$(date "+%Y-%m-%d %H:%M")" \
        --label "hm.bytes=${bytes:-0}" \
        --label "hm.size=$size" \
        "$template" >/dev/null

    local status=0
    hm_template_copy "$volume" "$template" "$image" || status=$?

    $was_running && $DOCKER_COMPOSE start db >/dev/null 2>&1

    if [ "$status" -ne 0 ]; then
        docker volume rm "$template" >/dev/null 2>&1
        hm_fail "$HM_EXIT_BLOCKED" "freeze_failed" \
            "The data directory could not be copied" \
            "$COMMAND_BIN_NAME doctor"
    fi

    local address
    address=$(hm_template_address "$COMPOSE_PROJECT_NAME" "$name")

    if is_json_output; then
        json_success "db" "$(jq -n --arg template "$address" --arg volume "$template" \
            --arg image "$image" --arg size "$size" --argjson bytes "${bytes:-0}" '$ARGS.named')"
        return 0
    fi

    print_info "Frozen as "
    print_code "$address"
    print_info " ($size). Build an environment from it with "
    print_code "$COMMAND_BIN_NAME db clone $address"
    printf '\n'
}

# ------------------------------------------------------------------ templates

do_templates() {
    local rows
    rows=$(hm_template_rows)

    if is_json_output; then
        json_success "db" "$(printf '%s\n' "$rows" | jq -R -s '
            {templates: (split("\n") | map(select(length > 0) | split("\t") |
                {project: .[0], name: .[1], address: (.[0] + "/" + .[1]),
                 size: .[2], db_image: .[3], created: .[4], volume: .[5]}))}')"
        return 0
    fi

    if [ -z "$rows" ]; then
        print_info "No templates on this machine yet.\n"
        print_default "  $COMMAND_BIN_NAME db freeze --name=base\n"
        return 0
    fi

    printf '\n'
    print_heading "Database templates\n\n"
    printf '%s\n' "$rows" | while IFS=$'\t' read -r project name size image created _; do
        [ -z "$name" ] && continue
        printf '  %-28s %-9s %-28s %s\n' "$project/$name" "$size" "$image" "$created"
    done
    printf '\n'
}

# ------------------------------------------------------------------ clone

#
# Build this project's data directory from a template.
#
# Nothing here talks to a database server: it replaces the files underneath one. That is what
# makes it seconds instead of an import, and it is also why the environment has to be down.
#
do_clone() {
    local address="${1:-base}"
    local force=false
    [ "${HM_FORCE:-}" == "1" ] && force=true

    case "$address" in
        -*) hm_fail "$HM_EXIT_USAGE" "invalid_argument" "Unknown option: $address" \
                "$COMMAND_BIN_NAME db clone base" ;;
    esac

    hm_template_parse "$address"
    validate_name "$TEMPLATE_NAME"
    is_docker_service_running

    local template
    template=$(hm_template_volume "$TEMPLATE_PROJECT" "$TEMPLATE_NAME")

    if ! hm_template_exists "$template"; then
        local known
        known=$(hm_template_rows | awk -F'\t' '{printf "  %s/%s\n", $1, $2}')
        [ -z "$known" ] && known="  (none yet: $COMMAND_BIN_NAME db freeze)"
        hm_fail "$HM_EXIT_USAGE" "unknown_template" \
            "There is no template called '$(hm_template_address "$TEMPLATE_PROJECT" "$TEMPLATE_NAME")'" \
            "$(printf 'Templates on this machine:\n%s' "$known")"
    fi

    local running
    running=$($DOCKER_COMPOSE ps -q --status running 2>/dev/null)

    if [ -n "$running" ]; then
        hm_fail "$HM_EXIT_BLOCKED" "environment_running" \
            "The database's files cannot be replaced while '$COMPOSE_PROJECT_NAME' is running" \
            "$COMMAND_BIN_NAME stop"
    fi

    local volume image template_image
    volume=$(hm_template_project_volume)
    image=$(hm_template_project_image)
    template_image=$(docker volume inspect "$template" --format '{{index .Labels "hm.db_image"}}' 2>/dev/null)

    if [ -z "$volume" ] || [ -z "$image" ]; then
        hm_fail "$HM_EXIT_PROJECT" "no_database_service" \
            "This project's configuration defines no database service" \
            "$COMMAND_BIN_NAME setup -f"
    fi

    #
    # A data directory is not portable across server versions: 10.6 files under 10.2 produce a
    # server that starts, complains, and loses data in ways that are found much later.
    #
    if [ -n "$template_image" ] && [ "$template_image" != "$image" ] && ! $force; then
        hm_fail "$HM_EXIT_BLOCKED" "image_mismatch" \
            "That template was made with $template_image and this project runs $image" \
            "Freeze a template from a project on the same image, or insist with --force"
    fi

    #
    # The confirmation is the project's name rather than a letter, like `restore`: a blind `y`
    # is a reflex, typing the name means the sentence was read.
    #
    local has_data=false
    if hm_template_exists "$volume"; then
        local bytes
        bytes=$(hm_template_measure "$volume" "$image")
        [ "${bytes:-0}" -gt 1048576 ] && has_data=true
    fi

    if $has_data && ! $force; then
        if is_non_interactive; then
            hm_fail "$HM_EXIT_BLOCKED" "would_replace_data" \
                "'$COMPOSE_PROJECT_NAME' already has a database and this would replace it" \
                "$COMMAND_BIN_NAME db clone $address --force"
        fi

        print_warning "This replaces the database of '$COMPOSE_PROJECT_NAME' with $address.\n"
        print_warning "Everything in it will be lost.\n\n"
        read -rp "$(print_question "Type the project name to confirm")" reply

        if [ "$reply" != "$COMPOSE_PROJECT_NAME" ]; then
            print_info "Nothing was cloned.\n"
            return 0
        fi
    fi

    print_info "Cloning $address...\n"

    hm_anonymisation_clear

    if ! hm_template_copy "$template" "$volume" "$image"; then
        hm_fail "$HM_EXIT_BLOCKED" "clone_failed" \
            "The template could not be copied into '$COMPOSE_PROJECT_NAME'" \
            "$COMMAND_BIN_NAME doctor"
    fi

    if is_json_output; then
        json_success "db" "$(jq -n --arg cloned "$address" --arg project "$COMPOSE_PROJECT_NAME" \
            --arg volume "$volume" '$ARGS.named')"
        return 0
    fi

    print_info "Cloned. Start the environment with "
    print_code "$COMMAND_BIN_NAME start"
    printf '\n'
}

# ------------------------------------------------------------------ drop

do_drop() {
    local address="${1:-}"

    [ -z "$address" ] && hm_fail "$HM_EXIT_USAGE" "missing_name" \
        "Which template should be dropped?" "$COMMAND_BIN_NAME db templates"

    hm_template_parse "$address"
    validate_name "$TEMPLATE_NAME"
    is_docker_service_running

    local template
    template=$(hm_template_volume "$TEMPLATE_PROJECT" "$TEMPLATE_NAME")

    if ! hm_template_exists "$template"; then
        hm_fail "$HM_EXIT_USAGE" "unknown_template" \
            "There is no template called '$(hm_template_address "$TEMPLATE_PROJECT" "$TEMPLATE_NAME")'" \
            "$COMMAND_BIN_NAME db templates"
    fi

    local users
    users=$(hm_template_users "$template")

    if [ -n "$users" ]; then
        hm_fail "$HM_EXIT_BLOCKED" "template_in_use" \
            "That template is attached to a container: $(printf '%s' "$users" | tr '\n' ' ')" \
            "Remove that container first, or clone the template instead of mounting it"
    fi

    local size
    size=$(docker volume inspect "$template" --format '{{index .Labels "hm.size"}}' 2>/dev/null)

    local full
    full=$(hm_template_address "$TEMPLATE_PROJECT" "$TEMPLATE_NAME")

    if ! is_non_interactive && [ "${HM_FORCE:-}" != "1" ]; then
        print_warning "This deletes the template $full ($size).\n\n"
        read -rp "$(print_question "Delete it? [y/N]")" reply

        case "$reply" in
            [yY] | [yY][eE][sS]) ;;
            *) print_info "Nothing was deleted.\n"; return 0 ;;
        esac
    fi

    docker volume rm "$template" >/dev/null

    if is_json_output; then
        json_success "db" "$(jq -n --arg dropped "$full" --arg freed "${size:-0}" '$ARGS.named')"
        return 0
    fi

    print_info "Dropped $full, freeing ${size:-nothing}.\n"
}

# ------------------------------------------------------------------ router

subcommand="${1:-}"
[ "$#" -gt 0 ] && shift

case "$subcommand" in
    snapshot)  do_snapshot "$@" ;;
    freeze)    do_freeze "$@" ;;
    templates) do_templates "$@" ;;
    clone)     do_clone "$@" ;;
    drop)      do_drop "$@" ;;
    clear)    do_clear "$@" ;;
    list)     do_list "$@" ;;
    restore)  do_restore "$@" ;;
    remove)   do_remove "$@" ;;
    "" | --help | -h)
        usage
        ;;
    *)
        hm_fail "$HM_EXIT_USAGE" "unknown_subcommand" \
            "'$subcommand' is not something $COMMAND_BIN_NAME db does" \
            "$COMMAND_BIN_NAME db snapshot | list | restore | remove | clear | freeze | templates | clone | drop"
        ;;
esac
