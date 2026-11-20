#!/usr/bin/env bash
#
# The risk each command declares, and the internal lists that must agree with it.
#
# The declaration is the source of truth for people, and the router is what answers: the point of
# these tests is that the two cannot drift apart quietly. The shell lists this suite also compared
# against (hm_alters_environment, hm_creates_containers) went with the shell tree; each command's
# refusals are its handler's now, and tested there.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"
source "$(dirname "${BASH_SOURCE[0]}")/../lib/router.sh"

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

test_case "every entry is a command the binary routes"
routed_names="$(hm_routed_commands)"
orphaned=""
while IFS= read -r name; do
    grep -qx "$name" <<< "$routed_names" || orphaned="$orphaned $name"
done <<< "$(commands_named)"
assert_empty "$orphaned"

test_case "and every command the binary routes has an entry"
undeclared=""
while IFS= read -r name; do
    [ "$(jq -r --arg n "$name" 'has($n)' "$DESCRIPTIONS")" == "true" ] || undeclared="$undeclared $name"
done <<< "$routed_names"
assert_empty "$undeclared"

# ---------------------------------------------------------------- no contradiction with what they do

# The commands that change the environment (what a worktree without an environment of its own
# refuses) or create containers: the lists hm_alters_environment and hm_creates_containers held
# until the shell tree went. Each handler enforces its own refusal now, but the declaration must
# still not call any of them harmless.
ALTERS_ENVIRONMENT="start stop restart rebuild down setup install create-project docker-stop-all"
CREATES_CONTAINERS="start restart rebuild setup install create-project docker-compose"

test_case "nothing that alters the environment is declared harmless"
contradictions=""
for name in $ALTERS_ENVIRONMENT; do
    [ "$(safety_of "$name")" == "safe" ] && contradictions="$contradictions $name"
done
assert_empty "$contradictions"

test_case "nothing that creates containers is declared harmless either"
contradictions=""
for name in $CREATES_CONTAINERS; do
    [ "$(safety_of "$name")" == "safe" ] && contradictions="$contradictions $name"
done
assert_empty "$contradictions"

test_case "and every one of them is a declared command"
missing=""
for name in $ALTERS_ENVIRONMENT $CREATES_CONTAINERS; do
    [ "$(jq -r --arg n "$name" 'has($n)' "$DESCRIPTIONS")" == "true" ] || missing="$missing $name"
done
assert_empty "$missing"

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
