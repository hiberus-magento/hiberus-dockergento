#!/usr/bin/env bash
# Docker leftovers
#
# `docker system df` computes real sizes and took 18s on a developer machine with 152
# volumes, which is unusable in a diagnosis. Counting volumes costs 0.13s and catches the
# same problem: an environment graveyard nobody cleans up.
source "$HELPERS_DIR"/doctor.sh

volumes=$(docker volume ls -q 2>/dev/null | sed '/^$/d' | wc -l | tr -d ' ')
dangling=$(docker images -qf dangling=true 2>/dev/null | sed '/^$/d' | wc -l | tr -d ' ')

if [ -z "$volumes" ]; then
    doctor_warning "Could not read Docker volumes"
    exit 0
fi

if [ "$volumes" -ge 100 ]; then
    doctor_warning "$volumes Docker volumes and $dangling dangling images on this machine" \
        "$COMMAND_BIN_NAME list  # look for orphaned environments"
else
    doctor_ok "$volumes Docker volumes, $dangling dangling images"
fi
