#!/usr/bin/env bash

#
# Does this domain already point at this machine?
#
# `hm set-host` asks for the system password once per project to add a line to /etc/hosts, and the
# lines are never removed: twenty-three of them on the machine this was written on, several from
# projects that no longer exist.
#
# None of that is needed when something already resolves the domain — a wildcard resolver for the
# TLD, whether it is ours or one the developer already had. So: ask about the result, not about who
# produces it.
#

#
# hm_domain_resolves_locally <domain>
#
# True when the name resolves to a loopback address. Resolving to something else is not enough:
# a domain that answers with a real internet address belongs to somebody, and pointing it at this
# machine in /etc/hosts is exactly what is wanted then.
#
hm_domain_resolves_locally() {
    local domain="$1"
    local addresses=""

    [ -z "$domain" ] && return 1

    if command -v dscacheutil >/dev/null 2>&1; then
        addresses=$(dscacheutil -q host -a name "$domain" 2>/dev/null |
            awk '/^ip_address:|^ipv6_address:/ { print $2 }')
    fi

    if [ -z "$addresses" ] && command -v getent >/dev/null 2>&1; then
        addresses=$(getent hosts "$domain" 2>/dev/null | awk '{ print $1 }')
    fi

    [ -z "$addresses" ] && return 1

    local address
    while IFS= read -r address; do
        case "$address" in
            127.*|::1|0:0:0:0:0:0:0:1) ;;
            "") ;;
            *) return 1 ;;
        esac
    done <<< "$addresses"

    return 0
}

#
# How it resolves, for the diagnosis: hosts file, dns, or not at all
#
hm_domain_resolution_source() {
    local domain="$1"

    if grep -qE "[[:space:]]$domain([[:space:]]|$)" /etc/hosts 2>/dev/null; then
        printf 'hosts'
        return 0
    fi

    if hm_domain_resolves_locally "$domain"; then
        printf 'dns'
        return 0
    fi

    printf 'none'
}
