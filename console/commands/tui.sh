#!/usr/bin/env bash
set -uo pipefail

source "$COMPONENTS_DIR"/print_message.sh
source "$COMPONENTS_DIR"/print_json.sh
source "$COMPONENTS_DIR"/tui.sh
source "$HELPERS_DIR"/exit_codes.sh
source "$TASKS_DIR"/tui_render.sh

#
# The fleet dashboard.
#
# It presents; the CLI decides. Data comes from `hm list`, `hm describe` and `hm doctor` in
# JSON, and actions are the CLI's own commands, so there is no logic duplicated here and the
# protections —worktree guardrails, exit codes— are inherited rather than bypassed.
#

HM="$COMMAND_BIN_DIR/bin/run"

VIEW="fleet"
SELECTED=0
FLEET_JSON=""
DOCTOR_JSON=""
DETAIL_JSON=""
LOADED_AT=""
MESSAGE="loading…"

#
# Drawing on something that is not a terminal would write control codes into a file
#
if ! tui_available; then
    hm_fail "$HM_EXIT_USAGE" "requires_terminal" \
        "The dashboard needs a terminal, and this output is not one" \
        "$COMMAND_BIN_NAME list --json"
fi

# ------------------------------------------------------------------ data

load_fleet() {
    MESSAGE="loading…"
    draw

    FLEET_JSON=$("$HM" list --json 2>/dev/null || echo '{}')
    DOCTOR_JSON=$("$HM" doctor --json 2>/dev/null || echo '{}')
    LOADED_AT=$(date "+%H:%M:%S")
    MESSAGE=""

    local count
    count=$(tui_fleet_count "$FLEET_JSON")

    if [ "${count:-0}" -eq 0 ]; then
        SELECTED=0
    elif [ "$SELECTED" -ge "${count:-0}" ]; then
        SELECTED=$((count - 1))
    fi
}

load_detail() {
    local root
    root=$(tui_fleet_field "$FLEET_JSON" "$SELECTED" "root")

    if [ -z "$root" ] || [ ! -d "$root" ]; then
        DETAIL_JSON=""
        MESSAGE="that environment's directory is gone"
        return 1
    fi

    MESSAGE="loading…"
    draw
    DETAIL_JSON=$( (cd "$root" && "$HM" describe --json 2>/dev/null) || echo '{}')
    MESSAGE=""
}

# ------------------------------------------------------------------ drawing

draw_header() {
    local title="$1"
    printf '%b%s%b' "${BOLD:-}" "$title" "${COLOR_RESET:-}"
    printf '\n\n'
}

draw_fleet() {
    draw_header "Dockergento — environments on this machine"

    local warnings
    warnings=$(tui_doctor_lines "$DOCTOR_JSON" "$((TUI_COLS - 2))")

    if [ -n "$warnings" ]; then
        printf '%s\n' "$warnings" | while IFS= read -r line; do
            case "$line" in
                ERROR*) printf '  %b%s%b\n' "${RED:-}" "$line" "${COLOR_RESET:-}" ;;
                *)      printf '  %b%s%b\n' "${YELLOW:-}" "$line" "${COLOR_RESET:-}" ;;
            esac
        done
        printf '\n'
    fi

    printf '  %b%s%b\n' "${BOLD:-}" "$(tui_fleet_header "$((TUI_COLS - 4))")" "${COLOR_RESET:-}"

    local count
    count=$(tui_fleet_count "$FLEET_JSON")

    # An empty fleet before the first read is not an empty fleet: saying "create one" while
    # the data is still being read sends the user to fix something that is not broken.
    if [ "${count:-0}" -eq 0 ]; then
        if [ -z "$LOADED_AT" ]; then
            printf '\n  Reading the environments on this machine…\n'
        else
            printf '\n  No environments found on this machine.\n'
            printf '  Create one with %bhm setup%b inside a Magento project.\n' "${GREEN:-}" "${COLOR_RESET:-}"
        fi
        return 0
    fi

    local index=0
    tui_fleet_rows "$FLEET_JSON" "$((TUI_COLS - 4))" | while IFS= read -r row; do
        if [ "$index" -eq "$SELECTED" ]; then
            printf '%b> %s%b\n' "${GREEN:-}" "$row" "${COLOR_RESET:-}"
        else
            printf '  %s\n' "$row"
        fi
        index=$((index + 1))
    done
}

draw_detail() {
    local name
    name=$(tui_fleet_field "$FLEET_JSON" "$SELECTED" "name")
    draw_header "$name"

    if [ -z "$DETAIL_JSON" ]; then
        printf '  Nothing to show.\n'
        return 0
    fi

    tui_detail_lines "$DETAIL_JSON" "$((TUI_COLS - 4))" | while IFS= read -r line; do
        printf '  %s\n' "$line"
    done
}

#
# The keys available right now, because a dashboard whose keys must be memorised from the
# documentation does not get used
#
draw_footer() {
    local keys

    # ASCII only: the arrows would be mojibake on a terminal without a UTF-8 locale, and the
    # footer is the one line that must always be readable. Short form on narrow terminals,
    # where the full list would be truncated into uselessness.
    if [ "$VIEW" == "fleet" ]; then
        if [ "$TUI_COLS" -ge 100 ]; then
            keys="j/k move   enter open   s start   x stop   r restart   l logs   o browser   g refresh   ? keys   q quit"
        else
            keys="j/k move   enter open   s/x/r start/stop/restart   ? keys   q quit"
        fi
    else
        if [ "$TUI_COLS" -ge 100 ]; then
            keys="esc back   s start   x stop   r restart   l logs   o browser   ? keys   q quit"
        else
            keys="esc back   s/x/r start/stop/restart   ? keys   q quit"
        fi
    fi

    tui_move "$TUI_ROWS" 1
    tui_clear_line

    local state="$MESSAGE"
    [ -z "$state" ] && [ -n "$LOADED_AT" ] && state="data from $LOADED_AT"

    printf '%b%s%b' "${BOLD:-}" "$(tui_truncate "$keys" "$((TUI_COLS - 2))")" "${COLOR_RESET:-}"

    if [ -n "$state" ]; then
        tui_move "$((TUI_ROWS - 1))" 1
        tui_clear_line
        printf '%s' "$(tui_truncate "$state" "$((TUI_COLS - 2))")"
    fi
}

draw() {
    tui_clear_screen
    tui_move 1 1

    if [ "$VIEW" == "fleet" ]; then
        draw_fleet
    else
        draw_detail
    fi

    draw_footer
}

# ------------------------------------------------------------------ actions

#
# Hand the terminal over, run the CLI command where the environment lives, and come back.
#
# Running it in the environment's directory is what makes the CLI's own protections apply,
# and letting it write normally means a failure shows its whole error instead of a cropped
# version inside a box.
#
run_action() {
    local root name
    root=$(tui_fleet_field "$FLEET_JSON" "$SELECTED" "root")
    name=$(tui_fleet_field "$FLEET_JSON" "$SELECTED" "name")

    if [ -z "$root" ] || [ ! -d "$root" ]; then
        MESSAGE="that environment's directory is gone"
        return 0
    fi

    tui_suspend

    printf '%b%s %s%b  in %s\n\n' "${BOLD:-}" "$COMMAND_BIN_NAME" "$*" "${COLOR_RESET:-}" "$name"
    ( cd "$root" && "$HM" "$@" )
    local status=$?

    printf '\n'
    if [ "$status" -eq 0 ]; then
        print_info "Done. Press any key to go back to the dashboard.\n"
    else
        print_error "Failed with exit code $status. Press any key to go back to the dashboard.\n"
    fi

    IFS= read -rsn1 _ </dev/tty || true

    tui_resume
    load_fleet
}

#
# The storefront in the default browser.
#
# The address is the one the CLI itself reports, so a project with no domain configured says
# so instead of opening something wrong.
#
open_in_browser() {
    local root url opener
    root=$(tui_fleet_field "$FLEET_JSON" "$SELECTED" "root")

    if [ -z "$root" ] || [ ! -d "$root" ]; then
        MESSAGE="that environment's directory is gone"
        return 0
    fi

    url=$( (cd "$root" && "$HM" describe --json 2>/dev/null) | jq -r '.project.urls.base // ""' )

    if [ -z "$url" ]; then
        MESSAGE="that environment has no domain configured"
        return 0
    fi

    opener=$(command -v open || command -v xdg-open) || {
        MESSAGE="no browser opener on this machine: $url"
        return 0
    }

    "$opener" "$url" >/dev/null 2>&1 || MESSAGE="could not open $url"
}

show_keys() {
    tui_suspend
    printf '%bKeys%b\n\n' "${BOLD:-}" "${COLOR_RESET:-}"
    printf '  %-12s %s\n' "up/down, k/j" "move through the list"
    printf '  %-12s %s\n' "enter" "open the selected environment"
    printf '  %-12s %s\n' "esc" "back to the fleet"
    printf '  %-12s %s\n' "s" "start the environment"
    printf '  %-12s %s\n' "x" "stop it"
    printf '  %-12s %s\n' "r" "restart it"
    printf '  %-12s %s\n' "l" "follow its logs"
    printf '  %-12s %s\n' "o" "open it in the browser"
    printf '  %-12s %s\n' "g" "refresh the data"
    printf '  %-12s %s\n' "q" "quit"
    printf '\nPress any key to go back.\n'
    IFS= read -rsn1 _ </dev/tty || true
    tui_resume
}

# ------------------------------------------------------------------ main loop

tui_enter_screen
tui_hide_cursor
tui_watch_resize
tui_update_size

load_fleet

while true; do
    draw

    key=$(tui_read_key) || key="q"
    count=$(tui_fleet_count "$FLEET_JSON")

    case "$key" in
        q | ctrl-c)
            break
            ;;
        up | k)
            [ "$SELECTED" -gt 0 ] && SELECTED=$((SELECTED - 1))
            ;;
        down | j)
            [ "$SELECTED" -lt $(( ${count:-1} - 1 )) ] && SELECTED=$((SELECTED + 1))
            ;;
        enter)
            if [ "$VIEW" == "fleet" ] && [ "${count:-0}" -gt 0 ]; then
                load_detail && VIEW="detail"
            fi
            ;;
        esc | h | backspace)
            VIEW="fleet"
            ;;
        g)
            load_fleet
            ;;
        s)
            [ "${count:-0}" -gt 0 ] && run_action start
            ;;
        x)
            [ "${count:-0}" -gt 0 ] && run_action stop
            ;;
        r)
            [ "${count:-0}" -gt 0 ] && run_action restart
            ;;
        l)
            # There is no `hm logs`: the CLI reaches Compose through `docker-compose`,
            # and going through it keeps the file selection and project name right.
            [ "${count:-0}" -gt 0 ] && run_action docker-compose logs -f --tail 100
            ;;
        o)
            [ "${count:-0}" -gt 0 ] && open_in_browser
            ;;
        "?")
            show_keys
            ;;
    esac
done

tui_show_cursor
tui_leave_screen
