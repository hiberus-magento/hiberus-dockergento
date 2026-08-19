#!/usr/bin/env bash
# Certificate for this project's domain
source "$HELPERS_DIR"/doctor.sh
doctor_requires_project

if [ -z "${DOMAIN:-}" ]; then
    doctor_warning "This project has no domain configured" "$COMMAND_BIN_NAME set-host <domain>"
    exit 0
fi

if [ ! -f "ssl.pem" ]; then
    doctor_warning "No certificate found for $DOMAIN" "$COMMAND_BIN_NAME ssl $DOMAIN"
    exit 0
fi

if ! command -v openssl >/dev/null 2>&1; then
    doctor_ok "Certificate present for $DOMAIN"
    exit 0
fi

if openssl x509 -checkend 604800 -noout -in ssl.pem >/dev/null 2>&1; then
    doctor_ok "Certificate for $DOMAIN is valid"
else
    doctor_warning "The certificate for $DOMAIN expires within a week" \
        "$COMMAND_BIN_NAME ssl $DOMAIN"
fi
