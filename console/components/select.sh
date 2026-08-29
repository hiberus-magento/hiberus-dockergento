#!/usr/bin/env bash

#
# A list you can move through.
#
# Bash's `select` is a numbered list with no default, no arrow keys, and a redraw on every
# mistake that scrolls the question off the screen. The primitives to do better have been here
# since the dashboard: raw key reads, arrow decoding and cursor movement.
#
# The logic that can be tested without a terminal is kept separate from the part that cannot:
# moving through a list and rendering it are two functions that take an index and return text.
#

source "$COMPONENTS_DIR"/tui.sh

#
# The next index, wrapping at both ends. Somebody at the last option pressing down means the
# first one, not a beep.
#
hm_select_move() {
    local index="$1" delta="$2" count="$3"

    [ "$count" -le 0 ] && { printf '0'; return 0; }

    index=$(( (index + delta) % count ))
    [ "$index" -lt 0 ] && index=$(( index + count ))

    printf '%d' "$index"
}

#
# The list as it appears, one option per line. The marker is what the eye follows, so it is the
# first thing on the line rather than a colour that a monochrome terminal would swallow.
#
hm_select_render() {
    local selected="$1"; shift
    local index=0 option

    for option in "$@"; do
        if [ "$index" -eq "$selected" ]; then
            printf '  ❯ %s%d) %s%s\n' "${BOLD:-}" "$((index + 1))" "$option" "${COLOR_RESET:-}"
        else
            printf '    %d) %s\n' "$((index + 1))" "$option"
        fi
        index=$((index + 1))
    done
}

#
# Can this terminal be drawn on?
#
# Both ends have to be a terminal — the keys are read from one and the list is drawn on the
# other — and `TERM` has to describe something that understands cursor movement. The dashboard's
# `tui_available` only asks about stdout, because it is never driven by a pipe; this is asked of
# a question that can be.
#
hm_select_can_draw() {
    [ -t 0 ] && [ -t 1 ] || return 1

    case "${TERM:-}" in
        "" | dumb) return 1 ;;
    esac

    return 0
}

hm_select_has_fzf() {
    [ -t 0 ] && [ -t 1 ] && command -v fzf >/dev/null 2>&1
}

#
# Somebody who installed fzf has opinions about picking from a list, and this tool has no business
# overriding them. Aborting it is unambiguous — unlike escape in our own selector — so it stops
# the command rather than guessing.
#
hm_select_with_fzf() {
    local question="$1"; shift

    REPLY=$(printf '%s\n' "$@" | fzf --height=~40% --reverse --no-multi \
        --prompt="$(printf '%s ' "$question")" 2>/dev/null)

    if [ -z "$REPLY" ]; then
        print_info "\nNothing was chosen.\n"
        exit 130
    fi
}

#
# The arrow selector.
#
# Escape does nothing on purpose. Callers read REPLY and act on it, so a cancel that returned an
# empty answer would have them carry on with nothing chosen — for `hm down -v` that is the wrong
# branch of a destructive question. Ctrl-C still does what Ctrl-C does everywhere.
#
hm_select_interactive() {
    local question="$1"; shift
    local options=("$@")
    local count="${#options[@]}"
    local selected=0

    print_question "$question\n"
    hm_select_render "$selected" "${options[@]}"

    tui_hide_cursor

    while :; do
        tui_read_key_into || TUI_KEY="enter"

        case "$TUI_KEY" in
            up | k)    selected=$(hm_select_move "$selected" -1 "$count") ;;
            down | j)  selected=$(hm_select_move "$selected" 1 "$count") ;;
            enter)     break ;;
            [1-9])
                if [ "$TUI_KEY" -le "$count" ]; then
                    selected=$((TUI_KEY - 1))
                    break
                fi
                ;;
            *) continue ;;
        esac

        # Up as many lines as the list is long, then rewrite it: nothing scrolls, so the question
        # stays where it was — which is what people were losing on every wrong keystroke
        printf '\033[%dA' "$count"
        hm_select_render "$selected" "${options[@]}"
    done

    tui_show_cursor

    REPLY="${options[$selected]}"
}
