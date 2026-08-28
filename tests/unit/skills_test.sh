#!/usr/bin/env bash
#
# The skills describe this tool's commands, so this checks them against the tool.
#
# It exists because the previous set did not have it: `hm bash <command>` appeared about a
# hundred and fifty times across five published skills, and `bash.sh` understands only `-r` —
# every one of those lines opens an interactive shell and drops the command. `hm mysql -e`
# appeared fifty-three times and leaves through the usage-error branch. Nothing checked, so
# nothing said.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

SKILLS_DIR="$COMMAND_BIN_DIR/skills"
DESCRIPTIONS="$COMMAND_BIN_DIR/data/command_descriptions.json"

#
# Every `hm <command> [options]` written in a skill, one per line, with its skill.
#
# Only the command and the options that follow it are taken: arguments are values, and a
# whitelist that tried to know them would be a second copy of every command's parser.
#
invocations() {
    local file skill
    for file in "$SKILLS_DIR"/*/SKILL.md; do
        [ -f "$file" ] || continue
        skill=$(basename "$(dirname "$file")")

        grep -oE '\bhm [a-z][a-z0-9-]*([[:space:]]+-[-a-zA-Z0-9=<>]+)*' "$file" |
            sed "s|^|${skill}\t|"
    done
}

GLOBAL_OPTIONS=$(jq -r '._global.opts[] |
    (if .name.short != "" then "-" + .name.short + "\n" else "" end) +
    (if .name.long != "" then "--" + .name.long else "" end)' "$DESCRIPTIONS" | grep .)
GLOBAL_OPTIONS="$GLOBAL_OPTIONS
--help
-h"

command_options() {
    jq -r --arg command "$1" '(.[$command].opts // [])[] |
        (if .name.short != "" then "-" + .name.short + "\n" else "" end) +
        (if .name.long != "" then "--" + (.name.long | split("=")[0]) else "" end)' \
        "$DESCRIPTIONS" | grep . || true
}

# ---------------------------------------------------------------- the set

test_case "there is a skill per area of work"
for skill in environment database debugging agents; do
    assert_equals "0" "$([ -f "$SKILLS_DIR/dockergento-$skill/SKILL.md" ] && echo 0 || echo 1)"
done

test_case "each one declares a name and a description"
for file in "$SKILLS_DIR"/*/SKILL.md; do
    assert_contains "$(head -5 "$file")" "name:"
    assert_contains "$(head -5 "$file")" "description:"
done

test_case "the name matches the directory, which is what the type filter matches on"
for file in "$SKILLS_DIR"/*/SKILL.md; do
    assert_equals "$(basename "$(dirname "$file")")" \
        "$(grep -m1 '^name:' "$file" | sed 's/^name:[[:space:]]*//')"
done

test_case "they are short enough to be read to choose a command"
for file in "$SKILLS_DIR"/*/SKILL.md; do
    assert_equals "short" "$([ "$(wc -l < "$file")" -le 200 ] && echo short || echo "$(basename "$(dirname "$file")") is $(wc -l < "$file") lines")"
done

# ---------------------------------------------------------------- the commands

test_case "every command a skill mentions exists"
unknown=""
while IFS=$'\t' read -r skill invocation; do
    [ -z "$invocation" ] && continue
    set -- $invocation
    command_name="$2"
    [ -f "$COMMAND_BIN_DIR/console/commands/$command_name.sh" ] ||
        unknown="${unknown}${skill}: hm ${command_name}\n"
done <<< "$(invocations)"
assert_equals "" "$(printf "$unknown")"

test_case "and is declared in the command descriptions"
undeclared=""
while IFS=$'\t' read -r skill invocation; do
    [ -z "$invocation" ] && continue
    set -- $invocation
    command_name="$2"
    jq -e --arg c "$command_name" 'has($c)' "$DESCRIPTIONS" >/dev/null ||
        undeclared="${undeclared}${skill}: hm ${command_name}\n"
done <<< "$(invocations)"
assert_equals "" "$(printf "$undeclared")"

# ---------------------------------------------------------------- the options
#
# A whitelist on purpose: an option nobody declared fails here rather than being assumed
# harmless, which is the property that keeps this from happening again.

test_case "every option a skill uses is declared for that command"
bad=""
while IFS=$'\t' read -r skill invocation; do
    [ -z "$invocation" ] && continue
    set -- $invocation
    command_name="$2"
    shift 2

    [ "$#" -eq 0 ] && continue
    allowed="$GLOBAL_OPTIONS
$(command_options "$command_name")"

    for option in "$@"; do
        option="${option%%=*}"
        printf '%s\n' "$allowed" | grep -qx -- "$option" ||
            bad="${bad}${skill}: hm ${command_name} ${option}\n"
    done
done <<< "$(invocations)"
assert_equals "" "$(printf "$bad")"

# ---------------------------------------------------------------- the shell rule
#
# `console/commands/bash.sh` understands `-r` and nothing else: any other argument is dropped
# and an interactive shell opens, which from an agent means hanging or doing nothing.

test_case "hm bash is never given a command"
offenders=""
while IFS=$'\t' read -r skill invocation; do
    case "$invocation" in
        "hm bash" | "hm bash -r") continue ;;
        "hm bash"*) offenders="${offenders}${skill}: ${invocation}\n" ;;
    esac
done <<< "$(invocations)"
assert_equals "" "$(printf "$offenders")"

# ---------------------------------------------------------------- the services

test_case "every service a skill names is a service of the stack"
SERVICES=$(grep -oE '^  [a-z]+:' "$COMMAND_BIN_DIR/docker-compose/docker-compose.template.yml" |
    tr -d ' :' | sort -u)
wrong=""
for file in "$SKILLS_DIR"/*/SKILL.md; do
    skill=$(basename "$(dirname "$file")")
    for word in $(grep -oE '\bhm (logs|restart|tunnel) [a-z]+' "$file" | awk '{print $3}' | sort -u); do
        printf '%s\n' "$SERVICES" | grep -qx "$word" ||
            wrong="${wrong}${skill}: ${word}\n"
    done
done
assert_equals "" "$(printf "$wrong")"

test_case "and the two services people get wrong are named right"
for file in "$SKILLS_DIR"/*/SKILL.md; do
    assert_equals "" "$(grep -oE '\bhm (logs|restart|tunnel) (mysql|elasticsearch)\b' "$file" || true)"
done

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
