#!/usr/bin/env bash
set -uo pipefail

source "$COMPONENTS_DIR"/print_message.sh
source "$COMPONENTS_DIR"/print_json.sh
source "$COMPONENTS_DIR"/input_info.sh
source "$HELPERS_DIR"/exit_codes.sh
source "$HELPERS_DIR"/docker.sh
source "$HELPERS_DIR"/domain_resolution.sh
source "$TASKS_DIR"/proxy.sh
source "$TASKS_DIR"/worktree_env.sh
source "$TASKS_DIR"/db_template.sh
source "$TASKS_DIR"/anonymisation.sh
source "$TASKS_DIR"/collect_project_info.sh

#
# An environment per branch.
#
# A git worktree is a second working directory of the same repository. Until now the tool could
# only defend the main environment from it (WT-01); this gives the worktree an environment of its
# own instead — its own containers, its own database, its own address — which is what people
# wanted the first time they tried.
#
# It is affordable because of two things that came before: the global proxy, so a second
# environment needs no second set of ports, and database templates, so its data is a file copy
# rather than an import.
#

HM="$COMMAND_BIN_DIR/bin/run"

usage() {
    print_info "An environment per branch\n\n"
    print_default "  $COMMAND_BIN_NAME worktree add <branch> [--profile=agent] [--path=<dir>] [--no-start] [--no-anonymise]\n"
    print_default "  $COMMAND_BIN_NAME worktree list\n"
    print_default "  $COMMAND_BIN_NAME worktree remove <name> [--force]\n\n"
    print_info "Profiles: "
    print_code "lite"
    print_default " (php) · "
    print_code "agent"
    print_default " (php, nginx, db, search, redis) · "
    print_code "full"
    print_default " (everything)\n\n"
}

#
# The parent project. From the main checkout it is simply this one; the command refuses to run
# from a worktree, so there is no second case.
#
parent_project() {
    printf '%s' "${HM_PARENT_PROJECT:-$COMPOSE_PROJECT_NAME}"
}

require_main_checkout() {
    if [ "${HM_IS_WORKTREE:-false}" == "true" ]; then
        hm_fail "$HM_EXIT_BLOCKED" "not_the_main_checkout" \
            "Branch environments are created from the main checkout" \
            "cd ${HM_MAIN_ROOT:-$HM_ROOT} && $COMMAND_BIN_NAME worktree add <branch>"
    fi
}

# ------------------------------------------------------------------ add

do_add() {
    local branch="" profile="agent" path="" start=true anonymise=true

    while [ "$#" -gt 0 ]; do
        case "$1" in
            --profile=*) profile="${1#--profile=}"; shift ;;
            --profile)   profile="${2:-}"; shift 2 ;;
            --path=*)    path="${1#--path=}"; shift ;;
            --path)      path="${2:-}"; shift 2 ;;
            --no-start)  start=false; shift ;;
            --no-anonymise | --no-anonymize) anonymise=false; shift ;;
            -*)
                hm_fail "$HM_EXIT_USAGE" "invalid_argument" "Unknown option: $1" \
                    "$COMMAND_BIN_NAME worktree add <branch> --profile=agent"
                ;;
            *)
                [ -n "$branch" ] && hm_fail "$HM_EXIT_USAGE" "too_many_arguments" \
                    "One branch at a time" "$COMMAND_BIN_NAME worktree add <branch>"
                branch="$1"; shift
                ;;
        esac
    done

    [ -z "$branch" ] && hm_fail "$HM_EXIT_USAGE" "missing_branch" \
        "Which branch should get an environment?" \
        "$COMMAND_BIN_NAME worktree add feature/checkout"

    if ! hm_worktree_profile_keeps "$profile" >/dev/null; then
        hm_fail "$HM_EXIT_USAGE" "unknown_profile" \
            "'$profile' is not a profile" \
            "One of: $(hm_worktree_profiles)"
    fi

    require_main_checkout
    is_docker_service_running

    #
    # Without the proxy every branch environment would publish its own ports, which is the
    # collision the proxy was built to end — with as many environments as branches this time.
    #
    if ! hm_project_uses_proxy; then
        hm_fail "$HM_EXIT_BLOCKED" "proxy_required" \
            "Branch environments are reached by name, which needs the global proxy" \
            "$COMMAND_BIN_NAME proxy up && $COMMAND_BIN_NAME setup -f"
    fi

    local project name slug
    project=$(parent_project)
    slug=$(hm_worktree_slug "$branch")

    [ -z "$slug" ] && hm_fail "$HM_EXIT_USAGE" "unusable_branch_name" \
        "'$branch' leaves nothing that can be used as a name" \
        "$COMMAND_BIN_NAME worktree add feature/checkout --path=<dir>"

    if [ -z "$path" ]; then
        path="$(dirname "$HM_ROOT")/$(basename "$HM_ROOT")-worktrees/$slug"
    fi

    name=$(hm_worktree_slug "$(basename "$path")")

    if hm_worktree_is_registered "$project" "$name"; then
        hm_worktree_load "$project" "$name"
        hm_fail "$HM_EXIT_USAGE" "already_registered" \
            "'$name' already has an environment at $WORKTREE_PATH" \
            "$COMMAND_BIN_NAME worktree list"
    fi

    if [ -e "$path" ]; then
        hm_fail "$HM_EXIT_USAGE" "path_in_use" \
            "$path already exists" \
            "$COMMAND_BIN_NAME worktree add $branch --path=<somewhere else>"
    fi

    local child domain
    child="$project-$name"
    domain="$name.${DOMAIN:-localhost}"

    # ------------------------------------------------------------ the checkout

    print_info "Creating the worktree...\n"
    mkdir -p "$(dirname "$path")"

    local git_output
    if git -C "$HM_ROOT" show-ref --verify --quiet "refs/heads/$branch"; then
        git_output=$(git -C "$HM_ROOT" worktree add "$path" "$branch" 2>&1)
    else
        git_output=$(git -C "$HM_ROOT" worktree add -b "$branch" "$path" 2>&1)
    fi

    if [ ! -d "$path" ]; then
        hm_fail "$HM_EXIT_BLOCKED" "worktree_failed" \
            "git could not create the worktree: $(printf '%s' "$git_output" | tail -1)" \
            "git -C $HM_ROOT worktree list"
    fi

    # ------------------------------------------------------------ the environment

    local services
    services=$($DOCKER_COMPOSE config --services 2>/dev/null | tr '\n' ' ')

    hm_worktree_write_overlay \
        "$(hm_worktree_overlay_file "$project" "$name")" \
        "$profile" "$domain" "$child" "$services" "$HM_PROXY_NETWORK"

    hm_worktree_save "$project" "$name" "$path" "$branch" "$profile" "$domain" "$child"

    # ------------------------------------------------------------ dependencies

    share_dependencies "$path" "$child"

    # ------------------------------------------------------------ the address

    if ! hm_domain_resolves_locally "$domain"; then
        print_warning_line "$domain does not resolve yet"
        print_default "  Add it with "
        print_code "$COMMAND_BIN_NAME set-host"
        print_default " from $path, or point *.${DOMAIN:-localhost} at 127.0.0.1\n"
    fi

    # ------------------------------------------------------------ the data

    clone_database "$path" "$project"

    if $start; then
        print_info "Starting $child...\n"
        ( cd "$path" && "$HM" start ) || true

        anonymise_agent_data "$path" "$profile" "$anonymise"
    fi

    if is_json_output; then
        json_success "worktree" "$(jq -n --arg name "$name" --arg branch "$branch" \
            --arg profile "$profile" --arg path "$path" --arg project "$child" \
            --arg url "https://$domain" '$ARGS.named')"
        return 0
    fi

    printf '\n'
    print_info "Branch environment "
    print_code "$name"
    print_info " is at "
    print_code "https://$domain"
    printf '\n'
    print_default "  Code:    $path\n"
    print_default "  Profile: $profile\n"
    print_default "  Run "
    print_code "$COMMAND_BIN_NAME"
    print_default " commands from that directory and they act on this environment.\n\n"
}

#
# Dependencies without installing them again.
#
# On Linux the code is bind mounted, so vendor/ and node_modules/ are links to the main
# checkout: instant, nothing duplicated, and both are git-ignored so nothing shows up as
# modified. On macOS the code lives in a named volume and there is nothing to link, so the
# volume is copied — seconds, and the space is what macOS charges for not bind mounting.
#
# Only those two are shared. generated/, var/ and pub/static are compiled per branch, and a
# class from another branch is the hardest kind of bug to see.
#
share_dependencies() {
    local path="$1" child="$2"

    if [ "${MACHINE:-}" == "mac" ]; then
        local source_volume="${COMPOSE_PROJECT_NAME}_workspace"
        local target_volume="${child}_workspace"

        if ! hm_template_exists "$source_volume"; then
            print_warning_line "The main environment has no code volume yet; this one starts empty"
            return 0
        fi

        print_info "Copying the code volume...\n"
        hm_template_copy "$source_volume" "$target_volume" "$(hm_template_project_image)" ||
            print_warning_line "The code volume could not be copied"
        return 0
    fi

    local directory
    for directory in vendor node_modules; do
        if [ -d "$HM_ROOT/$directory" ] && [ ! -e "$path/$directory" ]; then
            ln -s "$HM_ROOT/$directory" "$path/$directory"
        fi
    done
}

#
# The database, cloned from a template.
#
# With no template it is not invented: a branch environment sharing the main database would not
# be an isolated environment at all, and a `setup:upgrade` on the branch would land on everybody.
#
clone_database() {
    local path="$1" project="$2"

    if ! hm_template_exists "$(hm_template_volume "$project" "base")"; then
        print_warning_line "This project has no database template, so the environment starts empty"
        print_default "  Freeze one with "
        print_code "$COMMAND_BIN_NAME db freeze"
        print_default " and clone it with "
        print_code "$COMMAND_BIN_NAME db clone $project/base"
        printf '\n'
        return 0
    fi

    print_info "Cloning the database...\n"
    ( cd "$path" && "$HM" db clone "$project/base" --force ) >/dev/null 2>&1 ||
        print_warning_line "The database could not be cloned; the environment starts empty"
}

#
# An environment called `agent` is an environment an agent works in, and what an agent reads goes
# to a model, over a network, outside the company. A development database is a copy of production:
# real names, addresses, emails and orders. So this is the moment to anonymise — the data has just
# been cloned and nobody is waiting on it.
#
# A default and not a rule: reproducing a bug that only happens with one customer's order history
# is a real thing people do, and it is their data and their decision.
#
anonymise_agent_data() {
    local path="$1" profile="$2" wanted="$3"

    [ "$profile" == "agent" ] || return 0

    if [ "$wanted" != "true" ]; then
        print_warning_line "Not anonymised, as asked. This database holds whatever the original held."
        return 0
    fi

    print_info "Anonymising the branch environment's database...\n"

    if ( cd "$path" && HM_NON_INTERACTIVE=1 "$HM" masquerade ) >/dev/null 2>&1; then
        print_info "Anonymised.\n"
        return 0
    fi

    #
    # Reported, not swallowed: an environment that was supposed to be anonymised and is not is
    # exactly the situation this feature exists to prevent
    #
    print_warning_line "The database could not be anonymised"
    print_default "  It holds whatever the original held. Run "
    print_code "$COMMAND_BIN_NAME masquerade"
    print_default " from $path before letting an agent read it.\n"
}

# ------------------------------------------------------------------ list

do_list() {
    local project name rows="" state
    project=$(parent_project)

    hm_load_container_table

    while IFS= read -r name; do
        [ -z "$name" ] && continue
        hm_worktree_load "$project" "$name" || continue

        state="stopped"
        [ -n "$(hm_container_table | awk -F'|' -v p="$WORKTREE_PROJECT" \
            '$3 == p && $2 == "running" { print $1 }')" ] && state="running"

        [ -d "$WORKTREE_PATH" ] || state="missing"

        rows="${rows}${name}\t${WORKTREE_BRANCH}\t${WORKTREE_PROFILE}\t${state}\t${WORKTREE_DOMAIN}\t${WORKTREE_PATH}\n"
    done <<< "$(hm_worktree_names "$project")"

    if is_json_output; then
        json_success "worktree" "$(printf "$rows" | jq -R -s --arg project "$project" '
            {project: $project,
             worktrees: (split("\n") | map(select(length > 0) | split("\t") |
                {name: .[0], branch: .[1], profile: .[2], state: .[3],
                 url: ("https://" + .[4]), path: .[5]}))}')"
        return 0
    fi

    if [ -z "$rows" ]; then
        print_info "No branch environments for this project.\n"
        print_default "  $COMMAND_BIN_NAME worktree add <branch>\n"
        return 0
    fi

    printf '\n'
    print_heading "Branch environments of $project\n\n"
    printf "$rows" | while IFS=$'\t' read -r name branch profile state domain _; do
        printf '  %-18s %-24s %-7s %-9s %s\n' "$name" "$branch" "$profile" "$state" "https://$domain"
    done
    printf '\n'
}

# ------------------------------------------------------------------ remove

do_remove() {
    local name="" force=false
    [ "${HM_FORCE:-}" == "1" ] && force=true

    while [ "$#" -gt 0 ]; do
        case "$1" in
            -*) hm_fail "$HM_EXIT_USAGE" "invalid_argument" "Unknown option: $1" \
                    "$COMMAND_BIN_NAME worktree remove <name>" ;;
            *)  name="$1"; shift ;;
        esac
    done

    [ -z "$name" ] && hm_fail "$HM_EXIT_USAGE" "missing_name" \
        "Which branch environment should be removed?" "$COMMAND_BIN_NAME worktree list"

    require_main_checkout

    local project
    project=$(parent_project)

    if ! hm_worktree_load "$project" "$name"; then
        hm_fail "$HM_EXIT_USAGE" "unknown_worktree" \
            "'$name' is not a branch environment of this project" \
            "$COMMAND_BIN_NAME worktree list"
    fi

    #
    # The containers and the database can be rebuilt in seconds; uncommitted code cannot be
    # rebuilt at all, which is why git's own refusal is repeated here rather than worked around.
    #
    local dirty=""
    [ -d "$WORKTREE_PATH" ] && dirty=$(git -C "$WORKTREE_PATH" status --porcelain 2>/dev/null)

    if [ -n "$dirty" ] && ! $force; then
        hm_fail "$HM_EXIT_BLOCKED" "uncommitted_changes" \
            "$name has uncommitted changes" \
            "Commit them, or repeat with --force"
    fi

    if ! is_non_interactive && ! $force; then
        print_warning "This destroys the environment $WORKTREE_PROJECT and removes $WORKTREE_PATH.\n\n"
        read -rp "$(print_question "Type the name to confirm")" reply

        if [ "$reply" != "$name" ]; then
            print_info "Nothing was removed.\n"
            return 0
        fi
    fi

    print_info "Removing $name...\n"

    if [ -d "$WORKTREE_PATH" ]; then
        ( cd "$WORKTREE_PATH" && COMPOSE_PROJECT_NAME="$WORKTREE_PROJECT" \
            docker compose --project-directory "$WORKTREE_PATH" \
                -f "$WORKTREE_PATH/$DOCKER_COMPOSE_FILE" \
                -f "$WORKTREE_PATH/$DOCKER_COMPOSE_FILE_MACHINE" \
                -f "$(hm_worktree_overlay_file "$project" "$name")" \
                down -v --remove-orphans ) >/dev/null 2>&1
    fi

    if $force; then
        git -C "$HM_ROOT" worktree remove --force "$WORKTREE_PATH" >/dev/null 2>&1
    else
        git -C "$HM_ROOT" worktree remove "$WORKTREE_PATH" >/dev/null 2>&1
    fi

    # A worktree whose directory a person deleted by hand leaves a stale administrative entry,
    # and git refuses to reuse the name until it is pruned
    git -C "$HM_ROOT" worktree prune >/dev/null 2>&1

    hm_worktree_forget "$project" "$name"

    if is_json_output; then
        json_success "worktree" "$(jq -n --arg removed "$name" '$ARGS.named')"
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
    add)    do_add "$@" ;;
    list)   do_list "$@" ;;
    remove) do_remove "$@" ;;
    "" | --help | -h)
        usage
        ;;
    *)
        hm_fail "$HM_EXIT_USAGE" "unknown_subcommand" \
            "'$subcommand' is not something $COMMAND_BIN_NAME worktree does" \
            "$COMMAND_BIN_NAME worktree add | list | remove"
        ;;
esac
