#!/usr/bin/env bash
# Local certificate authority
source "$HELPERS_DIR"/doctor.sh

if ! command -v mkcert >/dev/null 2>&1; then
    doctor_warning "mkcert is not installed, so HTTPS certificates cannot be issued" \
        "$COMMAND_BIN_NAME ssl"
    exit 0
fi

caroot=$(mkcert -CAROOT 2>/dev/null || true)

if [ -z "$caroot" ] || [ ! -f "$caroot/rootCA.pem" ]; then
    doctor_warning "mkcert is installed but its local authority is missing" "mkcert -install"
    exit 0
fi

doctor_ok "mkcert is installed and its local authority is in place"
