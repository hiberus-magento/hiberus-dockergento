#!/usr/bin/env bash

#
# One lock, for everything that writes state shared between projects.
#
# It is a directory and not `flock` because macOS has no `flock(1)`, and `mkdir` is atomic on
# every POSIX filesystem and needs nothing installed. The pid goes inside, so a lock can say who
# is holding it.
#
# Three properties matter more than the mechanism:
#
#   - A lock held by a process that no longer exists is broken, not waited on. An agent killed
#     mid-command would otherwise block every other agent for ever, and the person who finds the
#     directory a week later has no way of knowing what it is.
#   - Waiting has an end. A CLI that hangs is worse than one that fails.
#   - It is released on interrupt, not only on a clean exit.
#

HM_LOCK_DIR="${HM_LOCK_DIR:-$HOME/.hm/locks}"
HM_LOCK_TIMEOUT="${HM_LOCK_TIMEOUT:-10}"
HM_LOCK_STALE_AFTER="${HM_LOCK_STALE_AFTER:-120}"

HM_LOCKS_HELD=""

hm_lock_path() {
    printf '%s/%s.lock' "$HM_LOCK_DIR" "$1"
}

#
# Is the process that took this lock still alive? A lock with no pid, or with a pid nobody
# answers for, is nobody's.
#
hm_lock_is_stale() {
    local path="$1" pid

    pid=$(cat "$path/pid" 2>/dev/null)

    [ -z "$pid" ] && return 0
    kill -0 "$pid" 2>/dev/null && return 1

    return 0
}

hm_lock_release() {
    local path
    path=$(hm_lock_path "$1")

    rm -rf "$path" 2>/dev/null || true
    HM_LOCKS_HELD=$(printf '%s\n' "$HM_LOCKS_HELD" | grep -vx "$1" || true)
}

hm_lock_release_all() {
    local name
    for name in $HM_LOCKS_HELD; do
        rm -rf "$(hm_lock_path "$name")" 2>/dev/null || true
    done
    HM_LOCKS_HELD=""
}

#
# Take it, or say who has it.
#
hm_lock_acquire() {
    local name="$1"
    local timeout="${2:-$HM_LOCK_TIMEOUT}"
    local path waited=0

    path=$(hm_lock_path "$name")
    mkdir -p "$HM_LOCK_DIR"

    while :; do
        if mkdir "$path" 2>/dev/null; then
            printf '%s\n' "$$" > "$path/pid"
            printf '%s\n' "${HM_COMMAND:-unknown}" > "$path/command"

            HM_LOCKS_HELD="$HM_LOCKS_HELD $name"
            trap hm_lock_release_all EXIT INT TERM

            return 0
        fi

        if hm_lock_is_stale "$path"; then
            rm -rf "$path" 2>/dev/null || true
            continue
        fi

        [ "$waited" -ge "$timeout" ] && return 1

        sleep 1
        waited=$((waited + 1))
    done
}

#
# The whole point of the above: run something with the lock held, and give it back afterwards
# whatever happens.
#
hm_with_lock() {
    local name="$1"; shift
    local status=0

    if ! hm_lock_acquire "$name"; then
        local holder
        holder=$(cat "$(hm_lock_path "$name")/command" 2>/dev/null)

        hm_fail "$HM_EXIT_BLOCKED" "locked" \
            "Another ${holder:-command} is working on the same thing; this one waited ${HM_LOCK_TIMEOUT}s" \
            "Wait for it to finish and try again"
    fi

    "$@" || status=$?

    hm_lock_release "$name"

    return "$status"
}

#
# Write a file the way a file that others read has to be written: a temporary with a name nobody
# else can guess, and a rename. The rename is atomic within a filesystem, so a reader sees the old
# content or the new one and never half of either.
#
# It reads what to write from stdin, which is what makes it composable with the jq that produced
# it.
#
hm_write_atomically() {
    local target="$1"
    local temporary

    mkdir -p "$(dirname "$target")"

    temporary=$(mktemp "$target.XXXXXX") || return 1

    if ! cat > "$temporary"; then
        rm -f "$temporary"
        return 1
    fi

    if [ ! -s "$temporary" ]; then
        rm -f "$temporary"
        return 1
    fi

    mv "$temporary" "$target"
}
