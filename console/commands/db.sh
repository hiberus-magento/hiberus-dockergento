#!/usr/bin/env bash
set -uo pipefail

source "$COMPONENTS_DIR"/print_message.sh
source "$COMPONENTS_DIR"/print_json.sh
source "$COMPONENTS_DIR"/input_info.sh
source "$HELPERS_DIR"/exit_codes.sh
source "$HELPERS_DIR"/docker.sh

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
    print_default "  $COMMAND_BIN_NAME db remove <name>\n\n"
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

# ------------------------------------------------------------------ router

subcommand="${1:-}"
[ "$#" -gt 0 ] && shift

case "$subcommand" in
    snapshot) do_snapshot "$@" ;;
    list)     do_list "$@" ;;
    restore)  do_restore "$@" ;;
    remove)   do_remove "$@" ;;
    "" | --help | -h)
        usage
        ;;
    *)
        hm_fail "$HM_EXIT_USAGE" "unknown_subcommand" \
            "'$subcommand' is not something $COMMAND_BIN_NAME db does" \
            "$COMMAND_BIN_NAME db snapshot | list | restore | remove"
        ;;
esac
