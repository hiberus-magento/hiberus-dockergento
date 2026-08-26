#!/usr/bin/env bash
#
# Whether a domain already points at this machine.
#
# This decides whether the tool asks for the system password and leaves a line in /etc/hosts, so
# the interesting cases are the ones where the answer is "no": a domain that resolves to somewhere
# on the internet must still be written, or working on it locally becomes impossible.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"
source "$HELPERS_DIR/domain_resolution.sh"

test_case "localhost resolves to this machine"
hm_domain_resolves_locally "localhost" && r=local || r=no
assert_equals "local" "$r"

test_case "a name that resolves nowhere does not count"
hm_domain_resolves_locally "nothing-resolves-to-this-name.invalid" && r=local || r=no
assert_equals "no" "$r"

test_case "an empty name does not count either"
hm_domain_resolves_locally "" && r=local || r=no
assert_equals "no" "$r"

# The case that matters: a real domain resolves, but not here. Writing it into /etc/hosts is
# exactly what somebody working on it locally wants.
test_case "a domain that resolves to the internet does not count as local"
if getent hosts example.com >/dev/null 2>&1 || dscacheutil -q host -a name example.com 2>/dev/null | grep -q ip_address; then
    hm_domain_resolves_locally "example.com" && r=local || r=no
    assert_equals "no" "$r"
else
    echo "  - skipped: no name resolution available"
fi

test_case "the source is reported as the hosts file when it is there"
if grep -qE '[[:space:]]localhost([[:space:]]|$)' /etc/hosts 2>/dev/null; then
    assert_equals "hosts" "$(hm_domain_resolution_source "localhost")"
else
    echo "  - skipped: localhost is not in /etc/hosts here"
fi

test_case "and as nothing when it does not resolve"
assert_equals "none" "$(hm_domain_resolution_source "nothing-resolves-to-this-name.invalid")"

test_case "deciding costs no Docker call"
noise=$(bash -c '
    source "'"$HELPERS_DIR"'/domain_resolution.sh"
    docker() { echo "docker was called"; }
    hm_domain_resolves_locally "localhost" >/dev/null
    hm_domain_resolution_source "localhost" >/dev/null' 2>&1)
assert_empty "$noise"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
