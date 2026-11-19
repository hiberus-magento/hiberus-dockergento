#!/usr/bin/env bash
#
# What an agent is told never to read: data/ai-exclusions.json, which ai-context writes into
# AGENTS.md and permissions turns into deny rules. The file itself is what is checked here.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

EXCLUSIONS="$HM_TEST_PROJECT_ROOT/data/ai-exclusions.json"

test_case "every exclusion states a path and a reason"
assert_equals "0" "$(jq '[.exclusions[] | select((.path | length) == 0 or (.reason | length) == 0)] | length' "$EXCLUSIONS")"

test_case "the ones that matter are in the list"
for path in "app/etc/env.php" "var/log" "pub/media/customer" "vendor" "generated" "var/cache"; do
    assert_equals "1" "$(jq --arg p "$path" '[.exclusions[] | select(.path == $p)] | length' "$EXCLUSIONS")"
done

test_case "secrets and customer data are marked as sensitive, noise is not"
assert_equals "true" "$(jq -r '.exclusions[] | select(.path == "app/etc/env.php") | .sensitive' "$EXCLUSIONS")"
assert_equals "false" "$(jq -r '.exclusions[] | select(.path == "vendor") | .sensitive' "$EXCLUSIONS")"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
