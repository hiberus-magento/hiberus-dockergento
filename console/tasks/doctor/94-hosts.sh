#!/usr/bin/env bash
# Does the project domain resolve locally?
source "$HELPERS_DIR"/doctor.sh
source "$HELPERS_DIR"/hosts.sh
doctor_requires_project

if [ -z "${DOMAIN:-}" ]; then
    exit 0
fi

#
# A domain can have an entry and still be unreachable. Until 1.5.0 the entry was a single
# "0.0.0.0 ::1 <domain>" line, which resolves the domain to 0.0.0.0 only, and Firefox refuses to
# connect to that address. Matching the name alone said OK on it, so the browser was the only
# thing that ever complained, and what it complained about was CORS.
#
if grep -qE "$(hm_hosts_legacy_pattern "$DOMAIN")" /etc/hosts 2>/dev/null; then
    doctor_warning "$DOMAIN resolves to 0.0.0.0, which Firefox refuses to connect to" \
        "$COMMAND_BIN_NAME set-host $DOMAIN --no-database"
elif grep -qE "$(hm_hosts_domain_pattern "$DOMAIN")" /etc/hosts 2>/dev/null; then
    doctor_ok "$DOMAIN resolves locally"
else
    doctor_warning "$DOMAIN has no entry in /etc/hosts" \
        "$COMMAND_BIN_NAME set-host $DOMAIN --no-database"
fi
