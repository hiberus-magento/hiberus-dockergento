#!/usr/bin/env bash

#
# From composed lines to a frame, and from a frame to bytes.
#
# The dashboard has three stages and this file is the last two. Composing turns the JSON the
# CLI produces into lines already cut to the terminal width; painting turns those lines into
# one write. The border between them is the point of the whole thing: composing costs processes
# and happens when the data or the terminal size changes, painting costs nothing and happens on
# every keystroke.
#
# It lives here, apart from the command, so a frame can be built and measured without a
# terminal — which is how the budget below is enforced.
#

# State this file reads. The command owns it; the defaults are here so the file can be sourced
# on its own, in a test, without a dashboard around it.
VIEW="${VIEW:-fleet}"
SELECTED="${SELECTED:-0}"
FLEET_JSON="${FLEET_JSON:-}"
DOCTOR_JSON="${DOCTOR_JSON:-}"
DETAIL_JSON="${DETAIL_JSON:-}"
DETAIL_NAME="${DETAIL_NAME:-}"
LOADED_AT="${LOADED_AT:-}"
MESSAGE="${MESSAGE:-}"
FLEET_HEADER="${FLEET_HEADER:-}"
COMPOSED_COLS="${COMPOSED_COLS:-0}"
FIRST_VISIBLE="${FIRST_VISIBLE:-0}"
DETAIL_OFFSET="${DETAIL_OFFSET:-0}"
FRAME_N="${FRAME_N:-0}"
FLEET_ROWS=()
DOCTOR_ROWS=()
DETAIL_ROWS=()
FRAME=()

# ------------------------------------------------------------------ composing

#
# Any field of the selected environment
#
fleet_field() {
    tui_fleet_field "$FLEET_JSON" "$SELECTED" "$1"
}

#
# Read lines into an array without forking a subshell for each one.
#
# `while read` inside `$( )` would put the array in a subshell and lose it; a here-string keeps
# the loop in this shell. The empty guard is for the empty payload, where the here-string still
# produces one blank line.
#
read_into_fleet_rows() {
    local line
    FLEET_ROWS=()
    while IFS= read -r line; do
        [ -z "$line" ] && continue
        FLEET_ROWS[${#FLEET_ROWS[@]}]="$line"
    done <<< "$1"
}

read_into_doctor_rows() {
    local line
    DOCTOR_ROWS=()
    while IFS= read -r line; do
        [ -z "$line" ] && continue
        DOCTOR_ROWS[${#DOCTOR_ROWS[@]}]="$line"
    done <<< "$1"
}

read_into_detail_rows() {
    local line
    DETAIL_ROWS=()
    while IFS= read -r line; do
        DETAIL_ROWS[${#DETAIL_ROWS[@]}]="$line"
    done <<< "$1"
}

#
# From JSON to lines cut to the terminal width.
#
# This is where `jq` lives. It runs when data arrives and when the terminal is resized, never
# on a keystroke: the width is an input to composing, not to painting.
#
compose() {
    COMPOSED_COLS="$TUI_COLS"

    FLEET_HEADER=$(tui_fleet_header "$((TUI_COLS - 4))")
    read_into_fleet_rows "$(tui_fleet_rows "$FLEET_JSON" "$((TUI_COLS - 4))")"
    read_into_doctor_rows "$(tui_doctor_lines "$DOCTOR_JSON" "$((TUI_COLS - 2))")"

    if [ -n "$DETAIL_JSON" ]; then
        read_into_detail_rows "$(tui_detail_lines "$DETAIL_JSON" "$((TUI_COLS - 4))")"
    else
        DETAIL_ROWS=()
    fi
}

# ------------------------------------------------------------------ the frame

frame_add() {
    FRAME[$FRAME_N]="$1"
    FRAME_N=$((FRAME_N + 1))
}

#
# Keep the selection on screen.
#
# With the frame positioned by absolute row, anything past the bottom is simply not visible, so
# a long fleet needs a window rather than trusting that everything fits.
#
scroll_into_view() {
    local visible="$1"

    [ "$visible" -lt 1 ] && visible=1

    if [ "$SELECTED" -lt "$FIRST_VISIBLE" ]; then
        FIRST_VISIBLE="$SELECTED"
    elif [ "$SELECTED" -ge $((FIRST_VISIBLE + visible)) ]; then
        FIRST_VISIBLE=$((SELECTED - visible + 1))
    fi

    [ "$FIRST_VISIBLE" -lt 0 ] && FIRST_VISIBLE=0
}

build_fleet_frame() {
    local i

    frame_add "${BOLD:-}Dockergento — environments on this machine${COLOR_RESET:-}"
    frame_add ""

    for ((i = 0; i < ${#DOCTOR_ROWS[@]}; i++)); do
        case "${DOCTOR_ROWS[$i]}" in
            ERROR*) frame_add "  ${RED:-}${DOCTOR_ROWS[$i]}${COLOR_RESET:-}" ;;
            *)      frame_add "  ${YELLOW:-}${DOCTOR_ROWS[$i]}${COLOR_RESET:-}" ;;
        esac
    done

    [ "${#DOCTOR_ROWS[@]}" -gt 0 ] && frame_add ""

    frame_add "  ${BOLD:-}${FLEET_HEADER}${COLOR_RESET:-}"

    # An empty fleet before the first read is not an empty fleet: saying "create one" while the
    # data is still being read sends the user to fix something that is not broken.
    if [ "${#FLEET_ROWS[@]}" -eq 0 ]; then
        frame_add ""
        if [ -z "$LOADED_AT" ]; then
            frame_add "  Reading the environments on this machine…"
        else
            frame_add "  No environments found on this machine."
            frame_add "  Create one with ${GREEN:-}hm setup${COLOR_RESET:-} inside a Magento project."
        fi
        return 0
    fi

    # Two rows are reserved at the bottom for the status and the keys, and one more for the
    # line that says which part of the list is on screen — which only exists when the list does
    # not fit, and which otherwise pushed the frame one row past the bottom of the terminal.
    local room=$((TUI_ROWS - FRAME_N - 2))
    local visible="$room"

    [ "${#FLEET_ROWS[@]}" -gt "$room" ] && visible=$((room - 1))
    [ "$visible" -lt 1 ] && visible=1

    scroll_into_view "$visible"

    local last=$((FIRST_VISIBLE + visible))
    [ "$last" -gt "${#FLEET_ROWS[@]}" ] && last="${#FLEET_ROWS[@]}"

    for ((i = FIRST_VISIBLE; i < last; i++)); do
        if [ "$i" -eq "$SELECTED" ]; then
            frame_add "${GREEN:-}> ${FLEET_ROWS[$i]}${COLOR_RESET:-}"
        else
            frame_add "  ${FLEET_ROWS[$i]}"
        fi
    done

    if [ "$last" -lt "${#FLEET_ROWS[@]}" ] || [ "$FIRST_VISIBLE" -gt 0 ]; then
        frame_add "  ${WHITE:-}showing $((FIRST_VISIBLE + 1))-$last of ${#FLEET_ROWS[@]}${COLOR_RESET:-}"
    fi
}

build_detail_frame() {
    local i

    frame_add "${BOLD:-}${DETAIL_NAME}${COLOR_RESET:-}"
    frame_add ""

    if [ "${#DETAIL_ROWS[@]}" -eq 0 ]; then
        frame_add "  Nothing to show."
        return 0
    fi

    # A project with nine services and five URLs does not fit in 24 rows, and with the frame
    # positioned by absolute row what does not fit is simply not there. Rather than cutting it
    # off in silence, the view scrolls and says so.
    local room=$((TUI_ROWS - FRAME_N - 2))
    local visible="$room"

    [ "${#DETAIL_ROWS[@]}" -gt "$room" ] && visible=$((room - 1))
    [ "$visible" -lt 1 ] && visible=1

    local most=$(( ${#DETAIL_ROWS[@]} - visible ))
    [ "$most" -lt 0 ] && most=0
    [ "$DETAIL_OFFSET" -gt "$most" ] && DETAIL_OFFSET="$most"
    [ "$DETAIL_OFFSET" -lt 0 ] && DETAIL_OFFSET=0

    local last=$((DETAIL_OFFSET + visible))
    [ "$last" -gt "${#DETAIL_ROWS[@]}" ] && last="${#DETAIL_ROWS[@]}"

    for ((i = DETAIL_OFFSET; i < last; i++)); do
        frame_add "  ${DETAIL_ROWS[$i]}"
    done

    if [ "${#DETAIL_ROWS[@]}" -gt "$room" ]; then
        frame_add "  ${WHITE:-}showing $((DETAIL_OFFSET + 1))-$last of ${#DETAIL_ROWS[@]}   j/k to scroll${COLOR_RESET:-}"
    fi
}

#
# The keys available right now, because a dashboard whose keys must be memorised from the
# documentation does not get used.
#
# ASCII only: the arrows would be mojibake on a terminal without a UTF-8 locale, and the footer
# is the one line that must always be readable. Short form on narrow terminals, where the full
# list would be truncated into uselessness.
#
footer_keys() {
    local long short

    if [ "$VIEW" == "fleet" ]; then
        long="j/k move   enter open   s start   x stop   r restart   l logs   o browser   g refresh   ? keys   q quit"
        short="j/k move   enter open   s/x/r start/stop/restart   ? keys   q quit"
    else
        long="esc back   s start   x stop   r restart   l logs   o browser   ? keys   q quit"
        short="esc back   s/x/r start/stop/restart   ? keys   q quit"
    fi

    # Whichever fits, measured rather than guessed: a column threshold picked by eye chose the
    # long form at 100 columns, where it was then truncated — and what got cut off was `q quit`,
    # the one key nobody can afford to lose.
    if [ "${#long}" -le $((TUI_COLS - 2)) ]; then
        printf '%s' "$long"
    else
        printf '%s' "$short"
    fi
}

#
# Every line of the screen, in order, exactly TUI_ROWS of them.
#
# Filling the frame to the full height is what makes leftovers impossible: every cell is
# rewritten, so a shorter screen than the one before cannot leave anything behind.
#
build_frame() {
    FRAME=()
    FRAME_N=0

    if [ "$VIEW" == "fleet" ]; then
        build_fleet_frame
    else
        build_detail_frame
    fi

    # Whatever the view did, the two bottom rows belong to the status and the keys
    [ "$FRAME_N" -gt $((TUI_ROWS - 2)) ] && FRAME_N=$((TUI_ROWS - 2))

    while [ "$FRAME_N" -lt $((TUI_ROWS - 2)) ]; do
        frame_add ""
    done

    local state="$MESSAGE"
    [ -z "$state" ] && [ -n "$LOADED_AT" ] && state="data from $LOADED_AT"

    tui_cut "$state" "$((TUI_COLS - 2))"
    frame_add "$TUI_TEXT"

    tui_cut "$(footer_keys)" "$((TUI_COLS - 2))"
    frame_add "${BOLD:-}${TUI_TEXT}${COLOR_RESET:-}"
}

# ------------------------------------------------------------------ painting

#
# One frame, one write.
#
# Every line carries its own position, so the frame does not depend on where the cursor was and
# a single line can be repainted with the same code. Each line clears what was to its right as
# it is written, which is what removes the need to erase the screen first — and with it the
# flicker.
#
#
# The whole frame as one string, in TUI_BYTES.
#
# Kept apart from painting so it can be built and measured with no terminal in sight, which is
# how the frame budget is enforced.
#
frame_bytes() {
    local i
    TUI_BYTES=""
    for ((i = 0; i < FRAME_N; i++)); do
        TUI_BYTES="${TUI_BYTES}"$'\033['"$((i + 1))"$';1H'"${FRAME[$i]}"$'\033[K'
    done
}

paint() {
    if [ "$TUI_COLS" != "$COMPOSED_COLS" ]; then
        compose
    fi

    build_frame
    frame_bytes

    tui_sync_begin
    printf '%s' "$TUI_BYTES"
    tui_clear_below
    tui_sync_end
}

