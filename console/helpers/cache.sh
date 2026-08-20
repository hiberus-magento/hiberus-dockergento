#!/usr/bin/env bash

#
# Tiny on-disk cache for values that cost a subprocess to obtain and change very rarely.
#
# Lives in $HOME, never inside a project: `config/docker/` is versioned in real projects,
# so a cache file there would show up in every `git status`, end up committed sooner or
# later, and force a .gitignore entry in every repository of the department.
#
# Each entry stores a validity token on the first line and the value on the second. The
# token is whatever proves the value is still good (a file's modification time, a version
# string): if it does not match, the entry is ignored and recomputed. A missing, unreadable
# or malformed entry is treated as absent, never as an error.
#

HM_CACHE_DIR="${HM_CACHE_DIR:-$HOME/.hm/cache}"

#
# Modification time of a file, portable across macOS and Linux
#
hm_file_mtime() {
    stat -f %m "$1" 2>/dev/null || stat -c %Y "$1" 2>/dev/null || echo "0"
}

#
# hm_cache_read <key> <token> — prints the value, returns non-zero on a miss
#
hm_cache_read() {
    local file="$HM_CACHE_DIR/$1"
    local stored_token stored_value

    [ -f "$file" ] || return 1

    { read -r stored_token && read -r stored_value; } < "$file" 2>/dev/null || return 1
    [ "$stored_token" == "$2" ] || return 1
    [ -n "$stored_value" ] || return 1

    printf '%s\n' "$stored_value"
}

#
# hm_cache_write <key> <token> <value> — best effort, never fails the caller
#
hm_cache_write() {
    mkdir -p "$HM_CACHE_DIR" 2>/dev/null || return 0
    printf '%s\n%s\n' "$2" "$3" > "$HM_CACHE_DIR/$1" 2>/dev/null || true
    return 0
}
