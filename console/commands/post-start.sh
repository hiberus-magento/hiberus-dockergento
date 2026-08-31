#!/usr/bin/env bash
set -euo pipefail

source "$COMPONENTS_DIR"/print_message.sh

#
# What has to happen after an environment comes up, on the platform that needs it.
#
# It is a command and not a block inside `start` because `start` is not the only thing that
# brings an environment up any more: the Go implementation does the Compose part and hands this
# back, so there is one copy of these steps rather than one per caller.
#
# It stays in shell for a concrete reason. Matching the ids could be ported today; writing the
# project's domains into the container reads them out of the database through `hm mysql`, which is
# not ported, so porting this would mean porting that first.
#
if [ "${MACHINE:-}" != "linux" ]; then
    exit 0
fi

print_processing "Waiting for everything to spin up..."
sleep 5
print_processing "Fixing permissions"
"$TASKS_DIR"/fix_linux_permissions.sh
print_processing "Permissions fix finished"
print_processing "Configuring self-routing domains..."
"$TASKS_DIR"/set_etc_hosts.sh
