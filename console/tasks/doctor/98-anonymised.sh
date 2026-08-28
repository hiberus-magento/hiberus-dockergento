#!/usr/bin/env bash
# Does an environment an agent works in hold data nobody anonymised?
source "$HELPERS_DIR"/doctor.sh
source "$TASKS_DIR"/anonymisation.sh
doctor_requires_project

hm_anonymisation_state

#
# The question is only asked of the environments where it is compliance rather than tidiness: a
# branch environment on the agent profile, or one a tool has labelled as an agent's. Asking it of
# every project would produce a warning nobody reads on every project.
#
for_agent=false
[ "${HM_PROFILE:-}" == "agent" ] && for_agent=true
[ -n "${HM_AGENT:-}" ] && for_agent=true

if [ "$HM_ANONYMISED" == "yes" ]; then
    doctor_ok "The database was anonymised on $HM_ANONYMISED_AT"
    return 0 2>/dev/null || exit 0
fi

if $for_agent; then
    doctor_error "This environment is used by an agent and its database was never anonymised" \
        "$COMMAND_BIN_NAME masquerade"
else
    doctor_ok "Database anonymisation: unknown (not required here)"
fi
