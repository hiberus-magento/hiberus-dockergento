#!/usr/bin/env bash

#
# Presentation for the terminal dashboard.
#
# Pure on purpose: every function here takes its data as text and returns lines. No terminal,
# no Docker, no state. That is what makes the dashboard testable at all — the drawing itself
# can only be checked against a pseudo-terminal, but the layout can be checked like any other
# function.
#
# Cheap on purpose too. Composing is what the dashboard does when data arrives, and it used to
# cost 400 ms for ten environments: one `jq` plus five `awk` per column set, called twice per
# frame, and three subshells per row for the truncation. Nothing here forks any more except the
# single `jq` that reads the payload. The functions that write to stdout are thin wrappers over
# ones that assign to a variable, so callers in a loop never pay for a subshell.
#

#
# tui_cut <text> <width> — cut from the right, marking it. Leaves the result in TUI_TEXT
# instead of writing it, which is what lets a row be assembled without forking.
#
tui_cut() {
    local text="$1"
    local width="$2"

    if [ "${#text}" -le "$width" ]; then
        TUI_TEXT="$text"
    elif [ "$width" -le 1 ]; then
        TUI_TEXT="${text:0:$width}"
    else
        TUI_TEXT="${text:0:$((width - 1))}~"
    fi
}

#
# tui_truncate <text> <width> — the same, written out
#
tui_truncate() {
    tui_cut "$1" "$2"
    printf '%s\n' "$TUI_TEXT"
}

#
# tui_cut_path <path> <width> — cut from the left: the end of a path is what identifies it, the
# beginning is almost always the same /Users/someone/projects prefix. Result in TUI_TEXT.
#
tui_cut_path() {
    local path="$1"
    local width="$2"

    if [ "${#path}" -le "$width" ]; then
        TUI_TEXT="$path"
    elif [ "$width" -le 1 ]; then
        TUI_TEXT="${path:$(( ${#path} - width ))}"
    else
        TUI_TEXT="~${path:$(( ${#path} - width + 1 ))}"
    fi
}

#
# tui_truncate_path <path> <width> — the same, written out
#
tui_truncate_path() {
    tui_cut_path "$1" "$2"
    printf '%s\n' "$TUI_TEXT"
}

#
# Column widths for a given terminal width.
#
# The path gives way first: what you came to read is the name and the state, and the path is
# context. Below a certain width the path disappears entirely rather than becoming an
# unreadable stub.
#
# The widths are left in TUI_W_* as well as printed: callers that are about to build a row
# read the variables, and never pay the five `awk` it used to take to parse them back out.
tui_fleet_columns() {
    local width="${1:-80}"
    local name=22 status=9 services=5 branch=18 path

    path=$((width - name - status - services - branch - 6))

    if [ "$path" -lt 12 ]; then
        path=0
        branch=$((width - name - status - services - 5))

        if [ "$branch" -lt 8 ]; then
            branch=0
        fi
    fi

    TUI_W_NAME="$name"
    TUI_W_STATUS="$status"
    TUI_W_SERVICES="$services"
    TUI_W_BRANCH="$branch"
    TUI_W_PATH="$path"

    printf '%s %s %s %s %s\n' "$name" "$status" "$services" "$branch" "$path"
}

#
# One line per environment, out of the JSON that `hm list` produces.
#
# tui_fleet_rows <json> <width>
#
tui_fleet_rows() {
    local json="$1"
    local width="${2:-80}"
    local name_w status_w services_w branch_w path_w

    tui_fleet_columns "$width" >/dev/null
    name_w="$TUI_W_NAME"
    status_w="$TUI_W_STATUS"
    services_w="$TUI_W_SERVICES"
    branch_w="$TUI_W_BRANCH"
    path_w="$TUI_W_PATH"

    local rows name status services branch root worktree orphan metadata label line column
    rows=$(printf '%s' "$json" | jq -r '
        (.data.environments // .environments // [])
        | .[]
        | [.name, .status,
           ((.containers.running | tostring) + "/" + (.containers.total | tostring)),
           (.branch // ""), (.root // ""), (.worktree // ""),
           (.orphan | tostring), (.has_metadata | tostring)]
        | join("\u001f")' 2>/dev/null)

    while IFS=$'\037' read -r name status services branch root worktree orphan metadata; do
        [ -z "$name" ] && continue

        # The flags are appended *after* truncating the name, never truncated themselves:
        # being a worktree or an orphan is the part of that column worth keeping, and on a
        # long name it was exactly the part being cut off.
        local flags=""
        [ -n "$worktree" ] && flags=" [$worktree]"
        [ "$orphan" == "true" ] && flags="$flags !"

        local name_room=$((name_w - ${#flags}))
        [ "$name_room" -lt 4 ] && name_room=4

        tui_cut "$name" "$name_room"
        label="$TUI_TEXT$flags"

        tui_cut "$label" "$name_w"
        printf -v line '%-*s %-*s %-*s' \
            "$name_w" "$TUI_TEXT" \
            "$status_w" "$status" \
            "$services_w" "$services"

        if [ "$branch_w" -gt 0 ]; then
            tui_cut "${branch:--}" "$branch_w"
            printf -v column '%-*s' "$branch_w" "$TUI_TEXT"
            line="$line $column"
        fi

        if [ "$path_w" -gt 0 ]; then
            tui_cut_path "$root" "$path_w"
            line="$line $TUI_TEXT"
        fi

        printf '%s\n' "$line"
    done <<< "$rows"
}

#
# Header for the fleet table, aligned with the rows
#
tui_fleet_header() {
    local width="${1:-80}"
    local line column

    tui_fleet_columns "$width" >/dev/null

    printf -v line '%-*s %-*s %-*s' \
        "$TUI_W_NAME" "PROJECT" "$TUI_W_STATUS" "STATUS" "$TUI_W_SERVICES" "UP"

    if [ "$TUI_W_BRANCH" -gt 0 ]; then
        printf -v column '%-*s' "$TUI_W_BRANCH" "BRANCH"
        line="$line $column"
    fi

    [ "$TUI_W_PATH" -gt 0 ] && line="$line PATH"

    printf '%s\n' "$line"
}

#
# How many environments the payload carries
#
tui_fleet_count() {
    printf '%s' "$1" | jq -r '(.data.environments // .environments // []) | length' 2>/dev/null || echo 0
}

#
# Name of the environment at a position, and its root, for acting on it
#
tui_fleet_field() {
    printf '%s' "$1" | jq -r --argjson index "$2" --arg field "$3" \
        '(.data.environments // .environments // [])[$index][$field] // ""' 2>/dev/null
}

#
# Warnings and errors from the diagnosis, most severe first
#
tui_doctor_lines() {
    local json="$1"
    local width="${2:-80}"

    printf '%s' "$json" | jq -r '
        (.data.checks // .checks // [])
        | map(select(.severity != "ok"))
        | sort_by(if .severity == "error" then 0 else 1 end)
        | .[]
        | (.severity | ascii_upcase) + "  " + .id + "  " + .message' 2>/dev/null |
        while IFS= read -r line; do
            [ -z "$line" ] && continue
            tui_cut "$line" "$width"
            printf '%s\n' "$TUI_TEXT"
        done
}

#
# What defines one environment, out of the JSON that `hm describe` produces
#
tui_detail_lines() {
    local json="$1"
    local width="${2:-80}"

    printf '%s' "$json" | jq -r '
        (.data // .) as $d
        | ["Project    " + ($d.project.name // "")]
        + ["Status     " + ($d.project.status // "")]
        + ["Domain     " + (($d.project.domain // "") | if . == "" then "-" else . end)]
        + ["Magento    " + (($d.magento.version // "") | if . == "" then "unknown" else . end)
                         + (($d.magento.mode // "") | if . == "" then "" else "  (" + . + ")" end)]
        + ["Root       " + ($d.project.root // "")]
        + [""]
        + ["URLs"]
        + [($d.project.urls // {} | to_entries[] | select(.value != "") | "  " + ((.key + "         ")[0:10]) + .value)]
        + [""]
        + ["Services"]
        + [($d.services // [] | .[] | "  " + ((.name + "            ")[0:13]) + ((.state + "        ")[0:10]) + .image)]
        | .[]' 2>/dev/null |
        while IFS= read -r line; do
            tui_cut "$line" "$width"
            printf '%s\n' "$TUI_TEXT"
        done
}
