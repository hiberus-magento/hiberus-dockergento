#!/usr/bin/env bash

#
# Shared contract for the diagnostic checks.
#
# Every check is a standalone script under console/tasks/doctor/ that prints one result
# line per finding:
#
#   <id><US><scope><US><severity><US><message><US><action>
#
# Running each check as its own process is what makes the runner robust: a check that
# crashes or hangs cannot take the rest of the diagnosis with it. The unit separator is
# used instead of a tab because `read` collapses consecutive tabs and would shift the
# fields of any result with an empty action.
#

HM_DOCTOR_SEPARATOR=$'\037'

#
# Emit a result: doctor_result <severity> <message> [action]
#
# The id and scope come from the environment, so a check never has to repeat them.
#
doctor_result() {
    printf '%s%s%s%s%s%s%s%s%s\n' \
        "${HM_DOCTOR_ID:-unknown}" "$HM_DOCTOR_SEPARATOR" \
        "${HM_DOCTOR_SCOPE:-global}" "$HM_DOCTOR_SEPARATOR" \
        "$1" "$HM_DOCTOR_SEPARATOR" \
        "$2" "$HM_DOCTOR_SEPARATOR" \
        "${3:-}"
}

doctor_ok()      { doctor_result "ok" "$1" "${2:-}"; }
doctor_warning() { doctor_result "warning" "$1" "${2:-}"; }
doctor_error()   { doctor_result "error" "$1" "${2:-}"; }

#
# True when the diagnosis is running inside a configured project
#
doctor_in_project() {
    [ "${HM_DOCTOR_IN_PROJECT:-false}" == "true" ]
}

#
# A project check exits quietly when there is no project to check
#
doctor_requires_project() {
    # Declaring the requirement is what makes a check project scoped: there is no second
    # place to keep that fact in sync
    HM_DOCTOR_SCOPE="project"

    doctor_in_project || exit 0
}

#
# The container table, loaded once by the runner and handed down
#
doctor_container_table() {
    printf '%s\n' "${HM_DOCTOR_CONTAINER_TABLE:-}"
}
