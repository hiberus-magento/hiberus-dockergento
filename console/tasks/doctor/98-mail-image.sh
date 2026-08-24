#!/usr/bin/env bash
# Can the configured mail catcher's image be obtained?
source "$HELPERS_DIR"/doctor.sh
doctor_requires_project

#
# The Mailpit image is published to the registry by hand, outside the tool's release. Until
# somebody has pushed it, choosing Mailpit leaves the project pointing at something Docker cannot
# pull — and the natural failure for that is a half-created environment, at `up` time, with
# Docker's own wording. Saying it here turns it into a sentence that explains itself.
#
image=$($DOCKER_COMPOSE config --format json 2>/dev/null |
    jq -r '(.services // {}) | (.mailpit // .mailhog // {}) | .image // ""')

if [ -z "$image" ]; then
    doctor_warning "This project defines no mail catcher" "$COMMAND_BIN_NAME setup -f"
    return 0 2>/dev/null || exit 0
fi

if docker image inspect "$image" >/dev/null 2>&1; then
    doctor_ok "The mail catcher image is available locally ($image)"
elif docker manifest inspect "$image" >/dev/null 2>&1; then
    doctor_ok "The mail catcher image can be pulled ($image)"
else
    doctor_error "The mail catcher image cannot be obtained: $image" \
        "That image is published manually; ask whoever maintains the registry, or set MAIL_SERVICE back to mailhog"
fi
