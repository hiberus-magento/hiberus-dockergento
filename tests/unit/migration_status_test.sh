#!/usr/bin/env bash
#
# That MIGRATION.md does not lie.
#
# It is the document somebody opens when they arrive, possibly weeks from now and possibly
# without having seen any of this: what it says about who owns which command has to be true, or
# it is worse than not having it.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

DOCUMENT="$COMMAND_BIN_DIR/MIGRATION.md"
DESCRIPTIONS="$COMMAND_BIN_DIR/data/command_descriptions.json"

commands() {
    jq -r 'to_entries[] | select(.key | startswith("_") | not) | .key' "$DESCRIPTIONS" | sort
}

# The rows of the command table, as `name<tab>implementation`
rows() {
    grep -E '^\| `[a-z0-9-]+` \|' "$DOCUMENT" |
        awk -F'|' '{ gsub(/[` ]/, "", $2); gsub(/ /, "", $5); print $2 "\t" $5 }'
}

# ---------------------------------------------------------------- the table is complete

# Compared on the name alone: a pattern carrying a literal tab and a `.*` is read differently by
# BSD and busybox `grep`, and the difference only showed up on Alpine
listed=$(rows | cut -f1 | sort)

test_case "every command is in the table"
missing=""
while IFS= read -r command; do
    printf '%s\n' "$listed" | grep -qx "$command" || missing="$missing $command"
done <<< "$(commands)"
assert_equals "" "$missing"

test_case "and the table invents none"
invented=""
while IFS=$'\t' read -r command _; do
    [ -z "$command" ] && continue
    commands | grep -qx "$command" || invented="$invented $command"
done <<< "$(rows)"
assert_equals "" "$invented"

test_case "each one says who owns it"
assert_equals "" "$(rows | awk -F'\t' '$2 != "shell" && $2 != "go" { print $1 }')"

# ---------------------------------------------------------------- and it is true
#
# A row that says Go when nothing in the Go tree answers for that command is the document
# drifting, which is exactly what it exists to prevent.

test_case "what is marked as Go exists in the Go tree"
wrong=""
while IFS=$'\t' read -r command owner; do
    [ "$owner" == "go" ] || continue
    grep -rq "\"$command\"" "$COMMAND_BIN_DIR/internal/cli/" 2>/dev/null || wrong="$wrong $command"
done <<< "$(rows)"
assert_equals "" "$wrong"

test_case "and the count at the top matches the table"
declared=$(grep -oE 'Comandos en Go \| [0-9]+ de [0-9]+' "$DOCUMENT" | awk '{print $5}')
counted=$(rows | awk -F'\t' '$2 == "go"' | grep -c . || true)
assert_equals "${counted:-0}" "${declared:-x}"

test_case "and so does the total"
total_declared=$(grep -oE 'Comandos en Go \| [0-9]+ de [0-9]+' "$DOCUMENT" | awk '{print $7}')
assert_equals "$(commands | grep -c .)" "$total_declared"

# ---------------------------------------------------------------- the way back in

test_case "it says how to build and how to test"
assert_contains "$(cat "$DOCUMENT")" "go build -o bin/hm ./cmd/hm"
assert_contains "$(cat "$DOCUMENT")" "go test ./..."
assert_contains "$(cat "$DOCUMENT")" "tests/run.sh"

test_case "and points at where the decisions are"
assert_contains "$(cat "$DOCUMENT")" "2.0-arquitectura.md"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
