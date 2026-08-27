#!/usr/bin/env bash
#
# The permission configuration handed to an agent.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

HM="$COMMAND_BIN_DIR/bin/run"
LAB=$(mktemp -d)
trap 'rm -rf "$LAB"' EXIT

generate() {
    ( cd "$LAB" && "$HM" permissions "$@" --json 2>/dev/null )
}

config=$(generate)

test_case "the configuration is valid JSON with the expected shape"
assert_equals "true" "$(printf '%s' "$config" | jq 'has("permissions") and (.permissions | has("allow") and has("ask"))')"

test_case "reading the environment is allowed without asking"
assert_contains "$(printf '%s' "$config" | jq -r '.permissions.allow[]')" "Bash(hm describe:*)"

test_case "so is running Magento commands, or the agent cannot work"
assert_contains "$(printf '%s' "$config" | jq -r '.permissions.allow[]')" "Bash(hm magento:*)"

test_case "destroying the environment asks first"
assert_contains "$(printf '%s' "$config" | jq -r '.permissions.ask[]')" "Bash(hm down:*)"

test_case "and so does anything that runs whatever it is given"
asked=$(printf '%s' "$config" | jq -r '.permissions.ask[]')
missing=""
for command in exec bash mysql docker-compose; do
    case "$asked" in
        *"Bash(hm $command:*)"*) ;;
        *) missing="$missing $command" ;;
    esac
done
assert_empty "$missing"

test_case "no command appears in both lists"
overlap=$(printf '%s' "$config" | jq -r '[.permissions.allow[]] - ([.permissions.allow[]] - [.permissions.ask[]]) | length')
assert_equals "0" "$overlap"

test_case "every command is in one list or the other"
listed=$(printf '%s' "$config" | jq '(.permissions.allow | length) + (.permissions.ask | length)')
declared=$(jq '[to_entries[] | select(.key | startswith("_") | not)] | length' "$DATA_DIR/command_descriptions.json")
assert_equals "$declared" "$listed"

# ---------------------------------------------------------------- strict

strict=$(generate --strict)

test_case "strict allows only what has no side effects"
assert_not_contains "$(printf '%s' "$strict" | jq -r '.permissions.allow[]')" "Bash(hm start:*)"

test_case "and still allows reading"
assert_contains "$(printf '%s' "$strict" | jq -r '.permissions.allow[]')" "Bash(hm describe:*)"

test_case "so it allows fewer commands than the default"
default_count=$(printf '%s' "$config" | jq '.permissions.allow | length')
strict_count=$(printf '%s' "$strict" | jq '.permissions.allow | length')
[ "$strict_count" -lt "$default_count" ] && r=fewer || r="$strict_count vs $default_count"
assert_equals "fewer" "$r"

# ---------------------------------------------------------------- it writes nothing

test_case "nothing is written to any settings file"
mkdir -p "$LAB/.claude"
printf '{"permissions": {"allow": ["mine"]}}\n' > "$LAB/.claude/settings.json"
before=$(cat "$LAB/.claude/settings.json")
generate >/dev/null
assert_equals "$before" "$(cat "$LAB/.claude/settings.json")"

test_case "an unknown option is refused"
( cd "$LAB" && "$HM" permissions --nonsense --json >"$LAB/out" 2>"$LAB/err" )
assert_json_field "$(cat "$LAB/err")" '.error.type' "invalid_argument"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
