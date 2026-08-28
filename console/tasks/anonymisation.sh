#!/usr/bin/env bash

#
# Whether this project's database has been anonymised, and when.
#
# It is recorded outside the checkout, next to the snapshots and the worktrees, for the same
# reason: `config/docker/` is committed, and this is a fact about one machine's copy of the data,
# not about the project.
#
# What makes it worth recording is that it expires. Every command that replaces the contents of
# the database clears it, because a reassuring "yes" left over from before an import is worse than
# no record at all — somebody would rely on it.
#

HM_STATE_DIR="${HM_STATE_DIR:-$HOME/.hm/state}"

hm_state_file() {
    printf '%s/%s.json' "$HM_STATE_DIR" "${1:-$COMPOSE_PROJECT_NAME}"
}

#
# Assigns HM_ANONYMISED ("yes" or "unknown") and HM_ANONYMISED_AT.
#
# Three states, of which "unknown" is the honest one for a project nobody has touched, and it is
# never treated as safe.
#
hm_anonymisation_state() {
    local file
    file=$(hm_state_file "${1:-}")

    HM_ANONYMISED="unknown"
    HM_ANONYMISED_AT=""

    [ -f "$file" ] || return 0

    HM_ANONYMISED_AT=$(jq -r '.anonymised_at // ""' "$file" 2>/dev/null)
    [ -n "$HM_ANONYMISED_AT" ] && HM_ANONYMISED="yes"

    return 0
}

hm_anonymisation_record() {
    local file
    file=$(hm_state_file "${1:-}")

    mkdir -p "$(dirname "$file")"

    local existing='{}'
    [ -f "$file" ] && existing=$(jq -c . "$file" 2>/dev/null || echo '{}')

    printf '%s' "$existing" | jq --arg at "$(date '+%Y-%m-%d %H:%M')" \
        '.anonymised_at = $at' > "$file.tmp" && mv "$file.tmp" "$file"
}

#
# Called by everything that replaces the database. Whatever it brought in, nobody anonymised it.
#
hm_anonymisation_clear() {
    local file
    file=$(hm_state_file "${1:-}")

    [ -f "$file" ] || return 0

    jq 'del(.anonymised_at)' "$file" > "$file.tmp" 2>/dev/null && mv "$file.tmp" "$file"
    return 0
}
