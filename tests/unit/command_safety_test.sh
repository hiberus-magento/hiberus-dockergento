#!/usr/bin/env bash
#
# The risk each command declares, and the internal lists that must agree with it.
#
# The tool already classified its commands in two hand-written lists, in two files. This is the
# third consumer of the same idea, and the point of these tests is that a fourth cannot appear
# quietly out of step: the declaration is the source of truth for people, the Bash lists are a fast
# copy, and a copy nobody checks is a copy that drifts.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"
source "$(dirname "${BASH_SOURCE[0]}")/../lib/router.sh"
source "$HELPERS_DIR/worktree.sh"
source "$TASKS_DIR/set_environment_labels.sh"

DESCRIPTIONS="$DATA_DIR/command_descriptions.json"

commands_named() {
    jq -r 'to_entries[] | select(.key | startswith("_") | not) | .key' "$DESCRIPTIONS"
}

safety_of() {
    jq -r --arg name "$1" '.[$name].safety // ""' "$DESCRIPTIONS"
}

# ---------------------------------------------------------------- every command declares one

test_case "every command declares a risk level"
undeclared=""
while IFS= read -r name; do
    [ -z "$(safety_of "$name")" ] && undeclared="$undeclared $name"
done <<< "$(commands_named)"
assert_empty "$undeclared"

test_case "and it is one of the three levels"
invalid=""
while IFS= read -r name; do
    case "$(safety_of "$name")" in
        safe | caution | dangerous) ;;
        *) invalid="$invalid $name" ;;
    esac
done <<< "$(commands_named)"
assert_empty "$invalid"

test_case "every command with a file has an entry"
missing=""
for file in "$COMMANDS_DIR"/*.sh; do
    name=$(basename "$file" .sh)
    [ "$(jq -r --arg n "$name" 'has($n)' "$DESCRIPTIONS")" == "true" ] || missing="$missing $name"
done
assert_empty "$missing"

test_case "and every entry has a file or a route"
routed_names="$(hm_routed_commands)"
orphaned=""
while IFS= read -r name; do
    [ -f "$COMMANDS_DIR/$name.sh" ] && continue
    grep -qx "$name" <<< "$routed_names" && continue
    orphaned="$orphaned $name"
done <<< "$(commands_named)"
assert_empty "$orphaned"

#
# The reverse direction, over the twelve retired by name and not "every routed command": twenty
# routed commands legitimately still have a file (`magento`, `start`, `mysql`, the partials `db`
# and `proxy`, the three delegating stubs), and a blanket rule would be false today.
#
test_case "and what the binary owns outright has no file to drift from"
RETIRED="copy-to-container copy-from-container varnish-on varnish-off purge npm
         n98-magerun test-unit test-integration mysqldump version set-host"

drift=""
routed="$(hm_routed_commands)"
for name in $RETIRED; do
    [ -f "$COMMANDS_DIR/$name.sh" ] && drift="$drift ${name}(file)"
    grep -qx "$name" <<< "$routed" || drift="$drift ${name}(not-routed)"
done
assert_empty "$drift"

# ---------------------------------------------------------------- the internal lists agree

# `hm_alters_environment` decides what a worktree refuses. Anything it refuses changes something,
# so calling it side-effect-free somewhere else would be a contradiction.
test_case "nothing that alters the environment is declared harmless"
contradictions=""
while IFS= read -r name; do
    hm_alters_environment "$name" || continue
    [ "$(safety_of "$name")" == "safe" ] && contradictions="$contradictions $name"
done <<< "$(commands_named)"
assert_empty "$contradictions"

test_case "nothing that creates containers is declared harmless either"
contradictions=""
while IFS= read -r name; do
    hm_creates_containers "$name" || continue
    [ "$(safety_of "$name")" == "safe" ] && contradictions="$contradictions $name"
done <<< "$(commands_named)"
assert_empty "$contradictions"

# ---------------------------------------------------------------- the classification is sane

test_case "the commands that destroy data ask for confirmation"
for name in down docker-stop-all clean; do
    test_case "  $name is dangerous"
    assert_equals "dangerous" "$(safety_of "$name")"
done

test_case "and so do the ones that run whatever they are given"
for name in exec bash mysql docker-compose; do
    test_case "  $name is dangerous"
    assert_equals "dangerous" "$(safety_of "$name")"
done

test_case "reading the environment never is"
for name in describe list doctor logs verify; do
    test_case "  $name is safe"
    assert_equals "safe" "$(safety_of "$name")"
done

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
