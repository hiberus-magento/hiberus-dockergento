#!/usr/bin/env bash
set -uo pipefail

source "$COMPONENTS_DIR"/print_message.sh
source "$COMPONENTS_DIR"/print_json.sh
source "$COMPONENTS_DIR"/tui.sh
source "$HELPERS_DIR"/exit_codes.sh
source "$TASKS_DIR"/tui_render.sh
source "$TASKS_DIR"/tui_frame.sh

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

# The composed content and the frame live in console/tasks/tui_frame.sh, sourced above: three
# stages with a clear border between them, where load talks to the CLI, compose turns JSON into
# lines already cut to the width, and paint turns lines into bytes. Only the first two are
# allowed to cost anything, and they run when the data or the terminal size changes — which is
# what makes a keystroke feel immediate.

#
# The palette holds `\033` as two characters, because the rest of the CLI prints it with
# `printf '%b'`. A frame is emitted with `%s` —one write, and no chance of a project name
# containing a backslash being interpreted as an escape— so the escapes are resolved here,
# once, into the real bytes.
#
resolve_palette() {
    local name value
    for name in BOLD RED GREEN YELLOW BLUE CYAN WHITE COLOR_RESET; do
        eval "value=\"\${$name:-}\""
        [ -z "$value" ] && continue
        printf -v value '%b' "$value"
        eval "$name=\"\$value\""
    done
}

resolve_palette

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
    paint

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

    compose
}

load_detail() {
    local root
    root=$(fleet_field "root")

    if [ -z "$root" ] || [ ! -d "$root" ]; then
        DETAIL_JSON=""
        MESSAGE="that environment's directory is gone"
        return 1
    fi

    MESSAGE="loading…"
    paint
    DETAIL_JSON=$( (cd "$root" && "$HM" describe --json 2>/dev/null) || echo '{}')
    MESSAGE=""

    # The name is captured with the data, not read from the selection when the frame is built:
    # otherwise moving through the list behind an open detail would retitle it with one
    # environment's name over another environment's contents.
    DETAIL_NAME=$(fleet_field "name")
    DETAIL_OFFSET=0
    read_into_detail_rows "$(tui_detail_lines "$DETAIL_JSON" "$((TUI_COLS - 4))")"
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
    root=$(fleet_field "root")
    name=$(fleet_field "name")

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
    root=$(fleet_field "root")

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
# Redraw on resize without waiting for a key: the handler runs in this shell, so it sees the
# new size and can recompose to it
tui_watch_resize "paint"
tui_update_size

load_fleet

while true; do
    paint

    tui_read_key_into || TUI_KEY="q"
    key="$TUI_KEY"
    # The count comes from the composed rows, not from another `jq` on the payload: this runs
    # on every keystroke.
    count="${#FLEET_ROWS[@]}"

    case "$key" in
        q | ctrl-c)
            break
            ;;
        up | k)
            if [ "$VIEW" == "detail" ]; then
                [ "$DETAIL_OFFSET" -gt 0 ] && DETAIL_OFFSET=$((DETAIL_OFFSET - 1))
            else
                [ "$SELECTED" -gt 0 ] && SELECTED=$((SELECTED - 1))
            fi
            ;;
        down | j)
            if [ "$VIEW" == "detail" ]; then
                DETAIL_OFFSET=$((DETAIL_OFFSET + 1))
            else
                [ "$SELECTED" -lt $(( ${count:-1} - 1 )) ] && SELECTED=$((SELECTED + 1))
            fi
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
