#!/usr/bin/env bash
#
# The machine's own name resolution: /etc/hosts.
#
# A hosts line carries one address and everything after it is an alias, so the single
# "0.0.0.0 ::1 <domain>" line written until 1.5.0 resolves the domain to 0.0.0.0 only. Chrome
# and curl hide that, because the kernel sends a connection to 0.0.0.0 to the loopback
# interface; Firefox refuses the address outright and the failure surfaces as a CORS error with
# a null status, which is a very long way from /etc/hosts.
#
# So: one entry per address family, and any legacy line repaired on the way, which is what
# heals an existing installation on its next `hm setup`.
#
# Deciding and rewriting live here, apart from the command, because the command needs the
# system password and nothing that needs a password can be exercised by a test.
#

#
# hm_hosts_legacy_pattern <domain> — the malformed entry this tool used to write
#
# It matches the line with or without the marker after the domain, because both were written.
#
hm_hosts_legacy_pattern() {
    printf '^0[.]0[.]0[.]0[[:space:]]+::1[[:space:]]+%s([[:space:]]|$)' "${1//./\\.}"
}

#
# hm_hosts_domain_pattern <domain> — the domain as a name on a hosts line
#
# The name has to be preceded by whitespace, and its dots are dots: `grep "$DOMAIN"` matched any
# line the name appeared anywhere in, so a project called shop.test was believed to resolve
# because myshop.test did.
#
hm_hosts_domain_pattern() {
    printf '[[:space:]]%s([[:space:]]|$)' "${1//./\\.}"
}

#
# hm_hosts_marked_pattern <domain> <marker> — an entry for a domain that this tool added
#
hm_hosts_marked_pattern() {
    printf '[[:space:]]%s[[:space:]].*%s' "${1//./\\.}" "$2"
}

#
# hm_hosts_has_legacy <file> <domain> — is the malformed entry in there
#
# Asked before anything a resolver says, because the hosts file answers first: a domain with this
# line in it resolves to 0.0.0.0 whatever else could have resolved it.
#
hm_hosts_has_legacy() {
    grep -qE "$(hm_hosts_legacy_pattern "$2")" "$1" 2>/dev/null
}

#
# hm_hosts_has_domain <file> <domain> — does the file resolve the domain, however it got there
#
hm_hosts_has_domain() {
    grep -qE "$(hm_hosts_domain_pattern "$2")" "$1" 2>/dev/null
}

#
# hm_hosts_needs_repair <file> <domain> — whether the file has to be rewritten
#
# Asked on its own, because rewriting this file costs a password prompt: a domain that already
# resolves per family is left exactly as it is, prompt included.
#
hm_hosts_needs_repair() {
    hm_hosts_has_legacy "$1" "$2" || ! hm_hosts_has_domain "$1" "$2"
}

#
# hm_hosts_entries <domain> [marker] — the lines this tool writes for a domain
#
hm_hosts_entries() {
    local marker=""

    if [ -n "${2:-}" ]; then
        marker=" $2"
    fi

    printf '127.0.0.1 %s%s\n::1 %s%s\n' "$1" "$marker" "$1" "$marker"
}

#
# hm_hosts_repaired <file> <domain> [marker] — what the file should contain for the domain
#
# The whole file, with the legacy entry for this domain removed and one entry per address family
# appended when the domain does not resolve without it. Appending only then is what keeps a file
# somebody already repaired by hand from gaining a second copy of the pair.
#
# Entries of other domains are not this function's business, whatever shape they are in.
#
hm_hosts_repaired() {
    local kept
    kept=$(grep -vE "$(hm_hosts_legacy_pattern "$2")" "$1" 2>/dev/null) || kept=""

    if [ -n "$kept" ]; then
        printf '%s\n' "$kept"
    fi

    if ! printf '%s\n' "$kept" | grep -qE "$(hm_hosts_domain_pattern "$2")"; then
        hm_hosts_entries "$2" "${3:-}"
    fi
}
