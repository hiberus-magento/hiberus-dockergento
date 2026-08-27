#!/usr/bin/env bash
set -uo pipefail

source "$COMPONENTS_DIR"/print_message.sh
source "$COMPONENTS_DIR"/print_json.sh
source "$HELPERS_DIR"/exit_codes.sh

#
# The permission configuration an agent should be given, derived from what each command does.
#
# Somebody has to decide what an agent may run without asking. Today that decision is taken by each
# person in their own configuration file, and no two lists look alike — on the machine this was
# written for, the answer was `Bash`: everything allowed, `hm down -v` included. Not carelessness;
# keeping a list of sixty commands up to date by hand is not something anybody does.
#
# So it is derived from the classification each command already declares, and printed. Writing into
# somebody's configuration file — which may have rules of their own, comments and an order that
# matters to them — to save a copy and paste is not a good trade.
#

strict=false

for argument in "$@"; do
    case "$argument" in
        --strict) strict=true ;;
        *)
            hm_fail "$HM_EXIT_USAGE" "invalid_argument" "Unknown option: $argument" \
                "$COMMAND_BIN_NAME permissions [--strict]"
            ;;
    esac
done

#
# Allowed by default: what changes nothing, and what changes things reversibly — an agent that has
# to ask before `hm start` is an agent whose questions nobody reads. `--strict` allows only the
# first, for one that merely diagnoses.
#
allowed_levels='["safe","caution"]'
$strict && allowed_levels='["safe"]'

configuration=$(jq -n \
    --argjson allowed "$allowed_levels" \
    --arg binary "$COMMAND_BIN_NAME" \
    --slurpfile commands "$DATA_DIR/command_descriptions.json" \
    '
    ($commands[0] | to_entries | map(select(.key | startswith("_") | not))) as $entries
    | {
        permissions: {
            allow: [$entries[] | select(.value.safety as $s | $allowed | index($s))
                    | "Bash(" + $binary + " " + .key + ":*)"] | sort,
            ask:   [$entries[] | select(.value.safety as $s | $allowed | index($s) | not)
                    | "Bash(" + $binary + " " + .key + ":*)"] | sort
        }
    }')

if is_json_output; then
    printf '%s\n' "$configuration"
    exit 0
fi

printf '\n'
print_heading "Permissions for an agent using $COMMAND_BIN_NAME\n\n"

allowed_count=$(printf '%s' "$configuration" | jq '.permissions.allow | length')
ask_count=$(printf '%s' "$configuration" | jq '.permissions.ask | length')

if $strict; then
    print_default "  Strict: only commands with no side effects are allowed.\n\n"
else
    print_default "  $allowed_count command(s) allowed, $ask_count asking for confirmation.\n\n"
fi

printf '%s\n' "$configuration"

printf '\n'
print_default "  Copy this into your agent's settings. Nothing was written for you: your\n"
print_default "  configuration file is yours, and may have rules that matter to you.\n\n"
