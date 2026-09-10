#!/usr/bin/env bash
#
# What the entries in /etc/hosts should look like, and when they have to be rewritten.
#
# This was verified by hand against copies of a real /etc/hosts, which is how the malformed
# entry survived a release: the decision and the rewrite lived inside a command that needs the
# system password, so nothing could exercise them. They are a helper now, and this is that
# helper being asked what a hosts file can be in.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

source "$HELPERS_DIR/hosts.sh"

LAB=$(cd "$(mktemp -d)" && pwd -P)
trap 'rm -rf "$LAB"' EXIT

# hosts_file <line...> — a throwaway hosts file with the given lines
hosts_file() {
    local file="$LAB/hosts.$RANDOM"
    printf '%s\n' "$@" > "$file"
    printf '%s' "$file"
}

# needs_repair <file> <domain> — "yes" or "no", so a decision can be asserted as a value
needs_repair() {
    if hm_hosts_needs_repair "$1" "$2"; then
        printf 'yes'
    else
        printf 'no'
    fi
}

#
# A hosts line carries one address and everything after it is an alias, so the single
# "0.0.0.0 ::1 <domain>" line written until 1.5.0 resolves the domain to 0.0.0.0 only, an
# address Firefox refuses to connect to. Finding one is a reason to rewrite the file.
#
test_case "a legacy entry for the domain is a reason to rewrite"
file=$(hosts_file '127.0.0.1 localhost' '0.0.0.0 ::1 shop.test')
assert_equals "yes" "$(needs_repair "$file" "shop.test")"

test_case "the legacy entry is replaced by one entry per address family"
assert_equals "127.0.0.1 localhost
127.0.0.1 shop.test
::1 shop.test" "$(hm_hosts_repaired "$file" "shop.test")"

#
# The password prompt is the cost of touching this file, so it is only paid when something
# actually has to change.
#
test_case "a domain that already resolves per family needs nothing"
file=$(hosts_file '127.0.0.1 localhost' '127.0.0.1 shop.test' '::1 shop.test')
assert_equals "no" "$(needs_repair "$file" "shop.test")"

test_case "and its file comes back untouched"
assert_equals "127.0.0.1 localhost
127.0.0.1 shop.test
::1 shop.test" "$(hm_hosts_repaired "$file" "shop.test")"

test_case "a missing domain is a reason to rewrite"
file=$(hosts_file '127.0.0.1 localhost')
assert_equals "yes" "$(needs_repair "$file" "shop.test")"

test_case "and it is appended once per address family"
assert_equals "127.0.0.1 localhost
127.0.0.1 shop.test
::1 shop.test" "$(hm_hosts_repaired "$file" "shop.test")"

#
# Everything else in the file belongs to somebody else.
#
test_case "the legacy entry of another domain is left alone"
file=$(hosts_file '127.0.0.1 localhost' '0.0.0.0 ::1 otra.test')
assert_equals "127.0.0.1 localhost
0.0.0.0 ::1 otra.test
127.0.0.1 shop.test
::1 shop.test" "$(hm_hosts_repaired "$file" "shop.test")"

#
# A file somebody already repaired by hand still has the legacy line in it, and appending the
# pair a second time is how a hosts file grows entries nobody can account for.
#
test_case "a half repaired file loses the legacy entry without gaining duplicates"
file=$(hosts_file '0.0.0.0 ::1 shop.test' '127.0.0.1 shop.test' '::1 shop.test')
assert_equals "yes" "$(needs_repair "$file" "shop.test")"

test_case "and the pair it already had is not duplicated"
assert_equals "127.0.0.1 shop.test
::1 shop.test" "$(hm_hosts_repaired "$file" "shop.test")"

#
# `grep "$DOMAIN"` matched any line the name appeared in, so a project called shop.test was
# believed to resolve because myshop.test did.
#
test_case "a domain another entry ends with does not count as resolved"
file=$(hosts_file '127.0.0.1 localhost' '127.0.0.1 myshop.test')
assert_equals "yes" "$(needs_repair "$file" "shop.test")"

test_case "and the entry it was confused with survives"
assert_contains "$(hm_hosts_repaired "$file" "shop.test")" "127.0.0.1 myshop.test"

#
# The domain is a regular expression away from being a wildcard: unescaped, shop.test matches
# shopxtest, which is somebody else's project.
#
test_case "the dots in a domain are not wildcards"
file=$(hosts_file '127.0.0.1 localhost' '127.0.0.1 shopxtest' '::1 shopxtest')
assert_equals "yes" "$(needs_repair "$file" "shop.test")"

printf '\n%s tests, %s failed\n' "$HM_TESTS_RUN" "$HM_TESTS_FAILED"
[ "$HM_TESTS_FAILED" -eq 0 ]
