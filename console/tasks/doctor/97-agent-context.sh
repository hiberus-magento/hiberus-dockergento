#!/usr/bin/env bash
# Does the generated agent context still describe this project?
source "$HELPERS_DIR"/doctor.sh
source "$TASKS_DIR"/collect_project_info.sh
source "$TASKS_DIR"/agent_context.sh
doctor_requires_project

agents_file="${HM_ROOT:-$PWD}/AGENTS.md"

#
# Not every project wants one, so its absence is a suggestion and not a failure. Its being wrong
# is another matter: an agent obeys what it reads, and a context that says PHP 7.4 in a project
# that moved to 8.2 is worse than no context at all.
#
if [ ! -f "$agents_file" ] || ! grep -qF "$HM_CONTEXT_BEGIN" "$agents_file"; then
    doctor_ok "No generated agent context (optional: $COMMAND_BIN_NAME ai-context)"
    return 0 2>/dev/null || exit 0
fi

recorded=$(grep -m1 -o 'hm:fingerprint [a-f0-9]*' "$agents_file" | awk '{print $2}')
current=$(hm_context_fingerprint "$(collect_project_info false)")

if [ -z "$recorded" ]; then
    doctor_warning "The agent context has no fingerprint and cannot be checked" \
        "$COMMAND_BIN_NAME ai-context"
elif [ "$recorded" == "$current" ]; then
    doctor_ok "The agent context matches this project"
else
    doctor_error "The agent context describes a different configuration than this project" \
        "$COMMAND_BIN_NAME ai-context"
fi
