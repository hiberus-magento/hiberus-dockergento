#!/usr/bin/env bash
#
# Whether the database has been anonymised, and what makes that answer expire.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

export HM_STATE_DIR="$WORK/state"
COMPOSE_PROJECT_NAME="tienda"
source "$TASKS_DIR/anonymisation.sh"

# ---------------------------------------------------------------- three states
#
# "Unknown" is the honest answer for a project nobody has touched, and it is never treated as
# safe: a reassuring "yes" left over from before an import is worse than no record at all.

test_case "a project nobody has anonymised is unknown"
hm_anonymisation_state
assert_equals "unknown" "$HM_ANONYMISED"
assert_equals "" "$HM_ANONYMISED_AT"

test_case "after anonymising it is recorded, with a date"
hm_anonymisation_record
hm_anonymisation_state
assert_equals "yes" "$HM_ANONYMISED"
assert_equals "true" "$([ -n "$HM_ANONYMISED_AT" ] && echo true || echo false)"

test_case "the record lives outside the checkout"
assert_equals "0" "$([ -f "$WORK/state/tienda.json" ] && echo 0 || echo 1)"

test_case "replacing the data clears it"
hm_anonymisation_clear
hm_anonymisation_state
assert_equals "unknown" "$HM_ANONYMISED"

test_case "clearing what was never recorded is not an error"
hm_anonymisation_clear && r=ok || r=falló
assert_equals "ok" "$r"

test_case "one project's state is not another's"
hm_anonymisation_record "otra-tienda"
hm_anonymisation_state "tienda"
assert_equals "unknown" "$HM_ANONYMISED"
hm_anonymisation_state "otra-tienda"
assert_equals "yes" "$HM_ANONYMISED"

test_case "a state file with other keys keeps them"
printf '{"algo": "mio"}\n' > "$WORK/state/tienda.json"
hm_anonymisation_record "tienda"
assert_equals "mio" "$(jq -r '.algo' "$WORK/state/tienda.json")"
hm_anonymisation_clear "tienda"
assert_equals "mio" "$(jq -r '.algo' "$WORK/state/tienda.json")"

# ---------------------------------------------------------------- the words in the context

DATA_DIR="$COMMAND_BIN_DIR/data"
source "$TASKS_DIR/agent_context.sh"

INFO='{"project":{"name":"tienda","status":"running","domain":"t.test","urls":{"base":"https://t.test/","admin":"https://t.test/panel"}},"magento":{"version":"2.4.7","mode":"developer"},"data":{"anonymised":"unknown"},"services":[{"name":"phpfpm","image":"php"}]}'

test_case "a database nobody anonymised is called what it is"
HM_ANONYMISED="unknown" HM_ANONYMISED_AT=""
assert_contains "$(hm_context_block "$INFO")" "has not been anonymised"
assert_contains "$(hm_context_block "$INFO")" "real personal data"

test_case "and one that was says so, with the date"
HM_ANONYMISED="yes" HM_ANONYMISED_AT="2026-08-28 10:00"
assert_contains "$(hm_context_block "$INFO")" "Anonymised on 2026-08-28"

test_case "anonymising changes the fingerprint, so the context is regenerated"
ANON='{"project":{"domain":"t.test","urls":{"admin":"https://t.test/panel"}},"magento":{"version":"2.4.7"},"data":{"anonymised":"yes"},"services":[{"name":"phpfpm","image":"php"}]}'
PLAIN='{"project":{"domain":"t.test","urls":{"admin":"https://t.test/panel"}},"magento":{"version":"2.4.7"},"data":{"anonymised":"unknown"},"services":[{"name":"phpfpm","image":"php"}]}'
assert_equals "distinto" \
    "$([ "$(hm_context_fingerprint "$ANON")" != "$(hm_context_fingerprint "$PLAIN")" ] && echo distinto || echo igual)"

# ---------------------------------------------------------------- the terminal bug
#
# `docker run -t -i` unconditionally meant `the input device is not a TTY` from CI, from a script
# and from an agent, so the command that anonymises has never been usable by anything but a person
# at a keyboard — which is the opposite of what it is for.

test_case "a terminal is only asked for when there is one"
assert_contains "$(cat "$COMMAND_BIN_DIR/console/components/masquerade.sh")" '[ -t 0 ] && [ -t 1 ]'
assert_equals "" "$(grep -E '^\s+-t -i --rm' "$COMMAND_BIN_DIR/console/components/masquerade.sh" || true)"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
