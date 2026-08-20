#!/usr/bin/env bash
#
# What `hm --help` shows and how it degrades.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

HM="$COMMAND_BIN_DIR/bin/run"
LAB=$(cd "$(mktemp -d)" && pwd -P)

# A command with no declared group, removed on the way out
UNGROUPED="$COMMAND_BIN_DIR/console/commands/selftestungrouped.sh"

cleanup() {
    rm -f "$UNGROUPED"
    rm -rf "$LAB"
}
trap cleanup EXIT

help_output() {
    ( cd "$LAB" && env "$@" "$HM" --help 2>/dev/null )
}

OUTPUT=$(help_output)

test_case "the usage line comes first"
assert_contains "$(printf '%s' "$OUTPUT" | head -1)" "Usage:"

test_case "commands are grouped"
assert_contains "$OUTPUT" "Environment"

test_case "every group declared in the data file has a heading"
missing=""
while IFS= read -r title; do
    [ -z "$title" ] && continue
    [ "$title" == "Other" ] && continue
    case "$OUTPUT" in
        *"$title"*) ;;
        *) missing="$missing $title" ;;
    esac
done <<< "$(jq -r '._groups[].title' < "$DATA_DIR/command_descriptions.json")"
assert_empty "$missing" "these groups have no heading:"

test_case "the groups appear in the declared order"
declared=$(jq -r '._groups[] | select(.id != "other") | .title' < "$DATA_DIR/command_descriptions.json" | tr '\n' ' ')
shown=""
while IFS= read -r title; do
    case "$OUTPUT" in
        *"$title"*) shown="$shown$title " ;;
    esac
done <<< "$(jq -r '._groups[] | select(.id != "other") | .title' < "$DATA_DIR/command_descriptions.json")"
assert_equals "$declared" "$shown"

test_case "no command is missing from the help"
missing=""
for script in "$COMMAND_BIN_DIR"/console/commands/*.sh; do
    name=$(basename "$script" .sh)
    mac_only=$(jq -r --arg n "$name" '.[$n].mac // false' < "$DATA_DIR/command_descriptions.json")
    [ "$mac_only" == "true" ] && [ "$(uname -s)" != "Darwin" ] && continue
    case "$OUTPUT" in
        *"$name"*) ;;
        *) missing="$missing $name" ;;
    esac
done
assert_empty "$missing" "these commands are not listed:"

test_case "examples are shown"
assert_contains "$OUTPUT" "Examples"

test_case "the global options are shown"
assert_contains "$OUTPUT" "--no-color"

test_case "the footer points at the per-command help"
assert_contains "$OUTPUT" "--help"

# ------------------------------------------------------------------ ungrouped

test_case "a command with no declared group still appears"
printf '#!/usr/bin/env bash\nexit 0\n' > "$UNGROUPED"
chmod +x "$UNGROUPED"
OUTPUT=$(help_output)
assert_contains "$OUTPUT" "selftestungrouped"

test_case "and it appears under the catch-all group"
after_other=$(printf '%s' "$OUTPUT" | sed -n '/^Other$/,$p')
assert_contains "$after_other" "selftestungrouped"
rm -f "$UNGROUPED"

# ------------------------------------------------------------------ the logo

test_case "no logo when the output is piped"
OUTPUT=$(help_output)
assert_not_contains "$OUTPUT" "Docker environments for Magento 2"

test_case "no block characters when the locale is not UTF-8"
OUTPUT=$(help_output LC_ALL=C LANG=C LC_CTYPE=C)
assert_not_contains "$OUTPUT" "█"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
