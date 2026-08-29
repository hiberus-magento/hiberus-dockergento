#!/usr/bin/env bash

#
# Opening a GUI client on this project's database.
#
# Connecting one by hand means finding four things the tool already knows and nobody remembers,
# so people keep a saved connection per project and it goes stale the moment a port changes.
#
# Everything here is read from the resolved configuration, the way `hm describe --with-secrets`
# reads it: a project that renamed its database or moved its port gets the right connection
# without anybody updating a profile.
#

#
# Assigns HM_DB_HOST, HM_DB_PORT, HM_DB_USER, HM_DB_PASSWORD and HM_DB_NAME.
#
# Returns 1 when the database publishes no port, which is what the global proxy does to it:
# MySQL carries no hostname, Traefik cannot route it, so the overlay removes the published port
# altogether.
#
hm_db_connection() {
    local forced_port="${1:-}"
    local config
    config=$(compose_config_json)

    HM_DB_HOST="127.0.0.1"
    HM_DB_USER=$(printf '%s' "$config" | jq -r '(.services.db.environment.MYSQL_USER // "magento")')
    HM_DB_PASSWORD=$(printf '%s' "$config" | jq -r '(.services.db.environment.MYSQL_PASSWORD // "magento")')
    HM_DB_NAME=$(printf '%s' "$config" | jq -r '(.services.db.environment.MYSQL_DATABASE // "magento")')

    if [ -n "$forced_port" ]; then
        HM_DB_PORT="$forced_port"
        return 0
    fi

    HM_DB_PORT=$(printf '%s' "$config" |
        jq -r '[(.services.db.ports // [])[] | select((.target | tostring) == "3306") | .published][0] // ""')

    [ -n "$HM_DB_PORT" ]
}

hm_db_url() {
    printf 'mysql://%s:%s@%s:%s/%s' \
        "$HM_DB_USER" "$HM_DB_PASSWORD" "$HM_DB_HOST" "$HM_DB_PORT" "$HM_DB_NAME"
}

#
# The command that opens each client.
#
# TablePlus and Sequel Ace both take a `mysql://` URL; DBeaver takes a connection descriptor on
# its command line. On Linux the URL is handed to the desktop, which is what `xdg-open` is for.
#
hm_db_client_command() {
    local client="$1" url="$2"

    case "$client:${MACHINE:-}" in
        tableplus:mac) printf 'open -a TablePlus %s' "$url" ;;
        sequelace:mac) printf 'open -a "Sequel Ace" %s' "$url" ;;
        dbeaver:mac)   printf 'open -a DBeaver %s' "$url" ;;
        dbeaver:*)     printf 'dbeaver %s' "$url" ;;
        *)             printf 'xdg-open %s' "$url" ;;
    esac
}

hm_db_client_name() {
    case "$1" in
        tableplus) printf 'TablePlus' ;;
        sequelace) printf 'Sequel Ace' ;;
        dbeaver)   printf 'DBeaver' ;;
        *)         printf '%s' "$1" ;;
    esac
}

#
# The whole command, shared by the three launchers: they differ in one word.
#
hm_db_client_run() {
    local client="$1"; shift
    local print_only=false forced_port=""
    local name
    name=$(hm_db_client_name "$client")

    while [ "$#" -gt 0 ]; do
        case "$1" in
            --print)  print_only=true; shift ;;
            --port=*) forced_port="${1#--port=}"; shift ;;
            --port)   forced_port="${2:-}"; shift 2 ;;
            *)
                hm_fail "$HM_EXIT_USAGE" "invalid_argument" "Unknown option: $1" \
                    "$COMMAND_BIN_NAME $client [--print] [--port=<port>]"
                ;;
        esac
    done

    is_run_service "db"

    if ! hm_db_connection "$forced_port"; then
        hm_fail "$HM_EXIT_BLOCKED" "no_published_port" \
            "This project publishes no database port, so nothing on this machine can reach it" \
            "$COMMAND_BIN_NAME tunnel db, and then $COMMAND_BIN_NAME $client --port=<the port it prints>"
    fi

    local url
    url=$(hm_db_url)

    if $print_only || is_json_output; then
        if is_json_output; then
            json_success "$client" "$(jq -n --arg url "$url" --arg host "$HM_DB_HOST" \
                --arg port "$HM_DB_PORT" --arg user "$HM_DB_USER" --arg database "$HM_DB_NAME" \
                '{url: $url, host: $host, port: $port, user: $user, database: $database}')"
        else
            printf '%s\n' "$url"
        fi
        return 0
    fi

    if eval "$(hm_db_client_command "$client" "$url")" >/dev/null 2>&1; then
        print_info "Opened $name on "
        print_code "$COMPOSE_PROJECT_NAME"
        printf '\n'
        return 0
    fi

    #
    # What the person needed was the connection. A command that says "not installed" and nothing
    # else has wasted the trip.
    #
    print_warning_line "$name could not be opened; it may not be installed"
    print_default "  The connection:\n\n    "
    print_code "$url"
    printf '\n\n'
    return 0
}
