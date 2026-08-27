#!/usr/bin/env bash
#
# How a template is addressed and named.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

COMPOSE_PROJECT_NAME="shop"
source "$TASKS_DIR/db_template.sh"

# ---------------------------------------------------------------- addressing
#
# A bare name means this project's; the qualified form is what makes a template usable from the
# environment that needs it, which is never the one that made it.

test_case "a bare name belongs to the current project"
hm_template_parse "base"
assert_equals "shop" "$TEMPLATE_PROJECT"
assert_equals "base" "$TEMPLATE_NAME"

test_case "a qualified name names its project"
hm_template_parse "other/base"
assert_equals "other" "$TEMPLATE_PROJECT"
assert_equals "base" "$TEMPLATE_NAME"

test_case "only the first slash separates"
hm_template_parse "other/with/slashes"
assert_equals "other" "$TEMPLATE_PROJECT"
assert_equals "with/slashes" "$TEMPLATE_NAME"

test_case "the address round-trips"
hm_template_parse "other/base"
assert_equals "other/base" "$(hm_template_address "$TEMPLATE_PROJECT" "$TEMPLATE_NAME")"

# ---------------------------------------------------------------- volume names
#
# The prefix is what `docker volume ls --filter` and the tests find them by, and what tells a
# person reading `docker volume ls` that the tool made it.

test_case "the volume is prefixed and identifies both halves"
assert_equals "hm-template-shop-base" "$(hm_template_volume "shop" "base")"

test_case "two projects with the same template name do not collide"
assert_equals "hm-template-other-base" "$(hm_template_volume "other" "base")"

# ---------------------------------------------------------------- sizes
#
# Recorded at freeze time as a label, because a volume's size on macOS lives inside a virtual
# machine and asking for it later would mean starting a container per template.

test_case "bytes are reported in the unit a person reads"
assert_equals "512B" "$(hm_template_human_size 512)"
assert_equals "1.0KB" "$(hm_template_human_size 1024)"
assert_equals "245.0MB" "$(hm_template_human_size $((245 * 1024 * 1024)))"
assert_equals "1.5GB" "$(hm_template_human_size $((1536 * 1024 * 1024)))"

test_case "an unmeasured volume does not print an empty size"
assert_equals "0B" "$(hm_template_human_size 0)"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
