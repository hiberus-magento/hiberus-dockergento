#!/usr/bin/env bash
# Project properties
source "$HELPERS_DIR"/doctor.sh
doctor_requires_project

properties="$CUSTOM_PROPERTIES_DIR/properties.json"

if ! jq -e . "$properties" >/dev/null 2>&1; then
    doctor_error "$properties is not valid JSON" "$COMMAND_BIN_NAME setup"
    exit 0
fi

missing=""
for key in COMPOSE_PROJECT_NAME DOMAIN MAGENTO_DIR; do
    value=$(jq -r --arg k "$key" '.[$k] // ""' "$properties")
    [ -z "$value" ] && missing="$missing $key"
done

if [ -n "$missing" ]; then
    doctor_warning "Project properties are missing:$missing" "$COMMAND_BIN_NAME setup"
    exit 0
fi

if [ ! -d "${MAGENTO_DIR:-.}" ]; then
    doctor_error "The Magento directory '${MAGENTO_DIR}' does not exist" \
        "$COMMAND_BIN_NAME setup"
    exit 0
fi

doctor_ok "Project properties are complete"
