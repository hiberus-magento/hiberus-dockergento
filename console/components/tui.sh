#!/usr/bin/env bash

#
# Terminal control primitives.
#
# Raw VT100 sequences, no tput: tput costs 10-15ms per invocation and buys no portability
# over sequences every terminal of the last thirty years understands. No ncurses, no
# dialog, no gum.
#
# Written for Bash 3.2, which is what macOS ships: no associative arrays, no `read -N`,
# no `wait -n`.
#
# Every emitting function is a no-op when stdout is not a terminal, so the same code is
# harmless in a pipe, in a script or in an agent's hands.
#
# Sequences used, documented so that nobody replaces them with `clear` in a year's time:
#
#   \e[?25l / \e[?25h     hide / show the cursor
#   \e[?1049h / \e[?1049l enter / leave the alternate screen, which is what makes the user's
#                         scrollback survive: leaving restores exactly what was there
#   \e[<row>;<col>H       move the cursor
#   \e[s / \e[u           save / restore the cursor position
#   \e[2K                 erase the current line
#   \e[2J                 erase the screen
#   \e[J                  erase from the cursor down
#   \e[H                  cursor to the origin
#   \e[?2026h / \e[?2026l  hold the frame while it is written, then present it
#

TUI_ROWS="${TUI_ROWS:-24}"
TUI_COLS="${TUI_COLS:-80}"
TUI_IN_ALTERNATE_SCREEN="${TUI_IN_ALTERNATE_SCREEN:-false}"
TUI_CURSOR_HIDDEN="${TUI_CURSOR_HIDDEN:-false}"
TUI_PREVIOUS_TRAPS="${TUI_PREVIOUS_TRAPS:-}"

#
# Is there a terminal to draw on?
#
tui_available() {
    [ -t 1 ]
}

# ---------------------------------------------------------------------------- size

#
# Rows and columns out of an `stty size` output given as text. Pure: no terminal needed,
# which is what makes it testable.
#
tui_parse_size() {
    local raw="$1"
    local rows cols

    rows=$(printf '%s' "$raw" | awk '{print $1}')
    cols=$(printf '%s' "$raw" | awk '{print $2}')

    case "$rows" in
        '' | *[!0-9]*) rows="" ;;
    esac

    case "$cols" in
        '' | *[!0-9]*) cols="" ;;
    esac

    if [ -z "$rows" ] || [ -z "$cols" ] || [ "$rows" -eq 0 ] || [ "$cols" -eq 0 ]; then
        return 1
    fi

    printf '%s %s\n' "$rows" "$cols"
}

#
# Current terminal size, always usable: `stty size` first because it is POSIX and works on
# Bash 3.2, unlike `checkwinsize` which needs Bash 4; then the shell variables; then the
# size every terminal has assumed since 1978.
#
tui_size() {
    local parsed

    if parsed=$(tui_parse_size "$(stty size 2>/dev/null)" 2>/dev/null); then
        printf '%s\n' "$parsed"
        return 0
    fi

    if parsed=$(tui_parse_size "${LINES:-} ${COLUMNS:-}" 2>/dev/null); then
        printf '%s\n' "$parsed"
        return 0
    fi

    printf '24 80\n'
}

#
# Refresh TUI_ROWS and TUI_COLS
#
tui_update_size() {
    local size
    size=$(tui_size)
    TUI_ROWS=$(printf '%s' "$size" | awk '{print $1}')
    TUI_COLS=$(printf '%s' "$size" | awk '{print $2}')
}

#
# React to the window being resized. The handler only updates the size: drawing belongs to
# whoever owns the main loop.
#
#
# tui_watch_resize [command]
#
# The optional command runs after the size is updated, in the shell that installed the trap —
# which is how a resize can redraw straight away instead of waiting for the next keystroke.
#
tui_watch_resize() {
    if [ -n "${1:-}" ]; then
        trap "tui_update_size; $1" WINCH
    else
        trap 'tui_update_size' WINCH
    fi
}

# ---------------------------------------------------------------------- cursor

tui_hide_cursor() {
    tui_available || return 0
    printf '\033[?25l'
    TUI_CURSOR_HIDDEN=true
    tui_install_restore
}

tui_show_cursor() {
    tui_available || return 0
    printf '\033[?25h'
    TUI_CURSOR_HIDDEN=false
}

#
# tui_move <row> <col>
#
tui_move() {
    tui_available || return 0
    printf '\033[%s;%sH' "$1" "$2"
}

tui_save_cursor() {
    tui_available || return 0
    printf '\033[s'
}

tui_restore_cursor() {
    tui_available || return 0
    printf '\033[u'
}

tui_clear_line() {
    tui_available || return 0
    printf '\033[2K'
}

tui_clear_screen() {
    tui_available || return 0
    printf '\033[2J'
}

#
# Erase from the cursor to the end of the screen.
#
# This is what replaces erasing everything: a frame overwrites each line in place and then
# clears whatever is left below it, so there is never an instant with an empty screen. Erasing
# the whole screen first is what makes a full-screen program flicker.
#
tui_clear_below() {
    tui_available || return 0
    printf '\033[J'
}

tui_home() {
    tui_available || return 0
    printf '\033[H'
}

# ---------------------------------------------------------------- synchronized output

#
# Frame markers, DEC private mode 2026.
#
# Between begin and end the terminal is asked to hold what it has and present the frame in one
# go — the terminal equivalent of vsync, and the difference between a frame appearing and a
# frame being drawn in front of you. A terminal that does not know the mode ignores it, which
# is how DEC private modes work, so there is nothing to detect and no second code path.
#
tui_sync_begin() {
    tui_available || return 0
    printf '\033[?2026h'
}

tui_sync_end() {
    tui_available || return 0
    printf '\033[?2026l'
}

# ---------------------------------------------------------------- alternate screen

#
# Enter a screen of our own. Leaving it gives the user back exactly what they had,
# scrollback included, which is the whole reason not to use `clear`.
#
tui_enter_screen() {
    tui_available || return 0
    $TUI_IN_ALTERNATE_SCREEN && return 0

    tui_install_restore
    printf '\033[?1049h'
    TUI_IN_ALTERNATE_SCREEN=true
    tui_update_size
}

tui_leave_screen() {
    tui_available || return 0
    $TUI_IN_ALTERNATE_SCREEN || return 0

    printf '\033[?1049l'
    TUI_IN_ALTERNATE_SCREEN=false
}

#
# Hand the terminal over to another command and take it back.
#
# The dashboard shows the output of `hm start` this way instead of multiplexing it into a
# box: if the command fails, the user sees the whole error rather than a cropped version.
#
tui_suspend() {
    tui_available || return 0
    tui_show_cursor
    tui_leave_screen
    stty echo 2>/dev/null || true
}

tui_resume() {
    tui_available || return 0
    tui_enter_screen
    tui_hide_cursor
}

# -------------------------------------------------------------------- restoring

#
# Put the terminal back: main screen, cursor visible, echo on.
#
# Idempotent on purpose, because it can arrive twice: once from the program's own exit and
# once from the trap. A dashboard that dies leaving an invisible cursor burns trust for
# good, so this is the most important function in the file.
#
tui_restore() {
    if $TUI_IN_ALTERNATE_SCREEN; then
        printf '\033[?1049l' 2>/dev/null || true
        TUI_IN_ALTERNATE_SCREEN=false
    fi

    if $TUI_CURSOR_HIDDEN; then
        printf '\033[?25h' 2>/dev/null || true
        TUI_CURSOR_HIDDEN=false
    fi

    stty echo 2>/dev/null || true

    return 0
}

#
# Install the restore trap once, chaining whatever the program already had installed
# instead of replacing it.
#
tui_install_restore() {
    [ -n "$TUI_PREVIOUS_TRAPS" ] && return 0
    TUI_PREVIOUS_TRAPS="installed"

    local existing signal
    for signal in EXIT INT TERM; do
        existing=$(trap -p "$signal" | sed "s/^trap -- '//; s/' $signal\$//")

        if [ -n "$existing" ]; then
            trap "tui_restore; $existing" "$signal"
        else
            trap 'tui_restore' "$signal"
        fi
    done
}

# ---------------------------------------------------------------------- keyboard

#
# How long to wait for the rest of an escape sequence.
#
# Bash 3.2, which is what macOS ships, rejects fractional timeouts outright:
#   read: 0.01: invalid timeout specification
# so the shortest bounded wait there is a whole second. Arrow keys are instant either way,
# because their bytes are already buffered; only a lone Esc pays the wait.
#
# Takes the major version as an argument so it can be tested: BASH_VERSINFO is readonly and
# cannot be faked in a subshell.
tui_escape_timeout() {
    local major="${1:-${BASH_VERSINFO[0]:-3}}"

    if [ "$major" -ge 4 ]; then
        printf '0.05\n'
    else
        printf '1\n'
    fi
}

#
# Name of a key from the bytes read. Pure: takes the sequence as text, so it is testable
# without a terminal.
#
tui_key_name() {
    case "$1" in
        $'\033[A' | $'\033OA') printf 'up\n' ;;
        $'\033[B' | $'\033OB') printf 'down\n' ;;
        $'\033[C' | $'\033OC') printf 'right\n' ;;
        $'\033[D' | $'\033OD') printf 'left\n' ;;
        $'\033')               printf 'esc\n' ;;
        $'\n' | $'\r' | '')    printf 'enter\n' ;;
        $'\003')               printf 'ctrl-c\n' ;;
        $'\177' | $'\010')     printf 'backspace\n' ;;
        $'\t')                 printf 'tab\n' ;;
        *)                     printf '%s\n' "$1" ;;
    esac
}

#
# Read one keypress and return its name
#
#
# The next key, in TUI_KEY.
#
# Assigning rather than writing matters more than it looks: read inside `$( )` puts the wait
# inside a subshell, and a signal that arrives during that wait is handled by the subshell. A
# window resize would then update the size where nobody could see it, and the terminal would
# keep the old width until the next keystroke.
#
tui_read_key_into() {
    local key rest

    IFS= read -rsn1 key || return 1

    if [ "$key" == $'\033' ]; then
        IFS= read -rsn2 -t "$(tui_escape_timeout)" rest 2>/dev/null || rest=""
        key="$key$rest"
    fi

    TUI_KEY=$(tui_key_name "$key")
}

#
# The same, written out
#
tui_read_key() {
    tui_read_key_into || return 1
    printf '%s' "$TUI_KEY"
}
