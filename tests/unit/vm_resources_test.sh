#!/usr/bin/env bash
#
# Whether the containers have enough memory, and enough of the machine's.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"
source "$HELPERS_DIR/doctor.sh"

GB=$((1024 * 1024 * 1024))

# ---------------------------------------------------------------- the verdict
#
# On macOS the containers do not run on the laptop but in a virtual machine with whatever memory
# somebody gave it once, and the two numbers being different is the whole reason this exists: a
# machine with 48 GB whose Docker VM has 6 fits about six environments, and nothing said so.

test_case "a VM with a fair share of a large machine is fine"
assert_equals "fine" "$(hm_vm_memory_verdict $((16 * GB)) $((48 * GB)))"

test_case "and so is one on a machine with nothing to spare"
assert_equals "fine" "$(hm_vm_memory_verdict $((6 * GB)) $((8 * GB)))"

test_case "a sliver of a large machine is a setting somebody forgot"
assert_equals "cramped" "$(hm_vm_memory_verdict $((6 * GB)) $((48 * GB)))"

test_case "under four gigabytes nothing fits, whatever the machine has"
assert_equals "small" "$(hm_vm_memory_verdict $((3 * GB)) $((48 * GB)))"
assert_equals "small" "$(hm_vm_memory_verdict $((3 * GB)) $((4 * GB)))"

test_case "on Linux, where both numbers are the same, there is nothing to say"
assert_equals "fine" "$(hm_vm_memory_verdict $((32 * GB)) $((32 * GB)))"

test_case "a number nobody could read is not a verdict"
assert_equals "unknown" "$(hm_vm_memory_verdict 0 $((48 * GB)))"
assert_equals "unknown" "$(hm_vm_memory_verdict "" $((48 * GB)))"

# ---------------------------------------------------------------- the machine's memory

test_case "the machine's memory is read on this platform"
assert_equals "true" "$([ "$(hm_host_memory_bytes)" -gt $((1024 * 1024 * 1024)) ] && echo true || echo false)"

# ---------------------------------------------------------------- the check itself

test_case "the check knows how to raise it on each runtime"
assert_contains "$(cat "$TASKS_DIR/doctor/45-vm-resources.sh")" "colima start --memory"
assert_contains "$(cat "$TASKS_DIR/doctor/45-vm-resources.sh")" "Docker Desktop"

test_case "and says nothing about how on a runtime it does not know"
assert_contains "$(cat "$TASKS_DIR/doctor/45-vm-resources.sh")" 'action=""'

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
