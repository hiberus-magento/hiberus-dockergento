#!/usr/bin/env bash
#
# The contract every diagnostic check follows.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"
source "$HELPERS_DIR/doctor.sh"

fields() {
    printf '%s' "$1" | awk -F'\037' "{ print \$$2 }"
}

test_case "a result carries id, scope, severity, message and action"
export HM_DOCTOR_ID="ports" HM_DOCTOR_SCOPE="global"
line=$(doctor_error "Port 80 is taken" "hm stop")
assert_equals "5" "$(printf '%s' "$line" | awk -F'\037' '{ print NF }')"

test_case "the id comes from the environment"
assert_equals "ports" "$(fields "$line" 1)"

test_case "the scope comes from the environment"
assert_equals "global" "$(fields "$line" 2)"

test_case "the severity is the one requested"
assert_equals "error" "$(fields "$line" 3)"

test_case "the message survives intact"
assert_equals "Port 80 is taken" "$(fields "$line" 4)"

test_case "the action survives intact"
assert_equals "hm stop" "$(fields "$line" 5)"

test_case "an empty action still leaves five fields"
line=$(doctor_ok "All good")
assert_equals "5" "$(printf '%s' "$line" | awk -F'\037' '{ print NF }')"

test_case "a warning is reported as a warning"
assert_equals "warning" "$(fields "$(doctor_warning "Careful")" 3)"

test_case "messages with spaces and punctuation are not split"
line=$(doctor_error "Ports 80, 443 are taken by 'other'" "cd there; run hm stop")
assert_equals "Ports 80, 443 are taken by 'other'" "$(fields "$line" 4)"

test_case "the action with a semicolon is not split either"
assert_equals "cd there; run hm stop" "$(fields "$line" 5)"

test_case "a check knows when it runs inside a project"
export HM_DOCTOR_IN_PROJECT=true
doctor_in_project && r=yes || r=no
assert_equals "yes" "$r"

test_case "and when it does not"
export HM_DOCTOR_IN_PROJECT=false
doctor_in_project && r=yes || r=no
assert_equals "no" "$r"

test_case "a project check exits quietly outside a project"
out=$( ( export HM_DOCTOR_IN_PROJECT=false
         source "$HELPERS_DIR/doctor.sh"
         doctor_requires_project
         doctor_ok "should not be printed" ) 2>/dev/null )
assert_empty "$out"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
