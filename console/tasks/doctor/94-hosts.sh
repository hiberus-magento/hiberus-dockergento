#!/usr/bin/env bash
# Does the project domain resolve locally?
source "$HELPERS_DIR"/doctor.sh
doctor_requires_project

if [ -z "${DOMAIN:-}" ]; then
    exit 0
fi

if grep -q "[[:space:]]$DOMAIN\([[:space:]]\|$\)" /etc/hosts 2>/dev/null; then
    doctor_ok "$DOMAIN resolves locally"
else
    doctor_warning "$DOMAIN has no entry in /etc/hosts" \
        "$COMMAND_BIN_NAME set-host $DOMAIN --no-database"
fi
