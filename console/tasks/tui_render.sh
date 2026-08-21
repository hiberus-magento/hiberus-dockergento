#!/usr/bin/env bash

#
# Presentation for the terminal dashboard.
#
# Pure on purpose: every function here takes its data as text and returns lines. No terminal,
# no Docker, no state. That is what makes the dashboard testable at all — the drawing itself
# can only be checked against a pseudo-terminal, but the layout can be checked like any other
# function.
#

#
# tui_truncate <text> <width> — cut from the right, marking it
#
tui_truncate() {
    local text="$1"
    local width="$2"

    if [ "${#text}" -le "$width" ]; then
        printf '%s\n' "$text"
        return 0
    fi

    if [ "$width" -le 1 ]; then
        printf '%s\n' "${text:0:$width}"
        return 0
    fi

    printf '%s~\n' "${text:0:$((width - 1))}"
}

#
# tui_truncate_path <path> <width> — cut from the left: the end of a path is what identifies
# it, the beginning is almost always the same /Users/someone/projects prefix
#
tui_truncate_path() {
    local path="$1"
    local width="$2"

    if [ "${#path}" -le "$width" ]; then
        printf '%s\n' "$path"
        return 0
    fi

    if [ "$width" -le 1 ]; then
        printf '%s\n' "${path:$(( ${#path} - width ))}"
        return 0
    fi

    printf '~%s\n' "${path:$(( ${#path} - width + 1 ))}"
}

#
# Column widths for a given terminal width.
#
# The path gives way first: what you came to read is the name and the state, and the path is
# context. Below a certain width the path disappears entirely rather than becoming an
# unreadable stub.
#
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
    local columns name_w status_w services_w branch_w path_w

    columns=$(tui_fleet_columns "$width")
    name_w=$(printf '%s' "$columns" | awk '{print $1}')
    status_w=$(printf '%s' "$columns" | awk '{print $2}')
    services_w=$(printf '%s' "$columns" | awk '{print $3}')
    branch_w=$(printf '%s' "$columns" | awk '{print $4}')
    path_w=$(printf '%s' "$columns" | awk '{print $5}')

    local rows name status services branch root worktree orphan metadata label line
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

        label="$(tui_truncate "$name" "$name_room")$flags"

        line=$(printf '%-*s %-*s %-*s' \
            "$name_w" "$(tui_truncate "$label" "$name_w")" \
            "$status_w" "$status" \
            "$services_w" "$services")

        if [ "$branch_w" -gt 0 ]; then
            line="$line $(printf '%-*s' "$branch_w" "$(tui_truncate "${branch:--}" "$branch_w")")"
        fi

        if [ "$path_w" -gt 0 ]; then
            line="$line $(tui_truncate_path "$root" "$path_w")"
        fi

        printf '%s\n' "$line"
    done <<< "$rows"
}

#
# Header for the fleet table, aligned with the rows
#
tui_fleet_header() {
    local width="${1:-80}"
    local columns name_w status_w services_w branch_w path_w line

    columns=$(tui_fleet_columns "$width")
    name_w=$(printf '%s' "$columns" | awk '{print $1}')
    status_w=$(printf '%s' "$columns" | awk '{print $2}')
    services_w=$(printf '%s' "$columns" | awk '{print $3}')
    branch_w=$(printf '%s' "$columns" | awk '{print $4}')
    path_w=$(printf '%s' "$columns" | awk '{print $5}')

    line=$(printf '%-*s %-*s %-*s' "$name_w" "PROJECT" "$status_w" "STATUS" "$services_w" "UP")

    [ "$branch_w" -gt 0 ] && line="$line $(printf '%-*s' "$branch_w" "BRANCH")"
    [ "$path_w" -gt 0 ] && line="$line PATH"

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
            tui_truncate "$line" "$width"
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
            tui_truncate "$line" "$width"
        done
}
