#!/usr/bin/env bash
# Platform specific conditions
source "$HELPERS_DIR"/doctor.sh

if [ "${MACHINE:-}" == "linux" ]; then
    if id -nG 2>/dev/null | tr ' ' '\n' | grep -qx docker; then
        doctor_ok "User belongs to the docker group"
    else
        doctor_error "User does not belong to the docker group" \
            "sudo usermod -aG docker \$USER  # then log out and back in"
    fi
    exit 0
fi

available=$(df -g / 2>/dev/null | awk 'NR==2 { print $4 }')

if [ -n "$available" ] && [ "$available" -lt 10 ]; then
    doctor_warning "Only ${available}GB free on the startup disk" \
        "Free up disk space: Docker Desktop fails in confusing ways when it runs out"
else
    doctor_ok "Disk space available: ${available:-unknown}GB"
fi
