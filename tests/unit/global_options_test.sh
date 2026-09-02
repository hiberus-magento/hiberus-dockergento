#!/usr/bin/env bash
#
# Global option parsing: what the router consumes and what it forwards to the command.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"
source "$HELPERS_DIR/global_options.sh"

WORKDIR_TMP=$(mktemp)

parse() {
    unset HM_OUTPUT_FORMAT HM_NON_INTERACTIVE
    HM_OUTPUT_FORMAT=""
    HM_NON_INTERACTIVE=""
    parse_global_options "$@"
}

test_case "--json before the command is consumed"
parse --json describe
assert_equals "describe" "${HM_ARGS[*]}"

test_case "--json before the command sets the format"
assert_equals "json" "$HM_OUTPUT_FORMAT"

test_case "--json after a normal command is still consumed"
parse describe --json
assert_equals "describe" "${HM_ARGS[*]}"

test_case "--no-json forces text"
parse describe --no-json
assert_equals "text" "$HM_OUTPUT_FORMAT"

test_case "--yes enables non-interactive mode"
parse setup --yes
assert_equals "1" "$HM_NON_INTERACTIVE"

test_case "--yes is not forwarded to the command"
assert_equals "setup" "${HM_ARGS[*]}"

test_case "command options are forwarded untouched"
parse setup -f -p myproject
assert_equals "setup -f -p myproject" "${HM_ARGS[*]}"

test_case "flags of a transparent command are never swallowed"
parse composer show --format=json --json
assert_equals "composer show --format=json --json" "${HM_ARGS[*]}"

test_case "a transparent command keeps the default format"
assert_empty "$HM_OUTPUT_FORMAT"

test_case "globals before a transparent command still apply"
parse --json exec ls
assert_equals "exec ls" "${HM_ARGS[*]}"

test_case "globals before a transparent command set the format"
assert_equals "json" "$HM_OUTPUT_FORMAT"

test_case "mysql query arguments survive intact"
parse mysql -q "SELECT 1 FROM x"
assert_equals "mysql -q SELECT 1 FROM x" "${HM_ARGS[*]}"

test_case "no arguments produces an empty list"
parse
assert_equals "0" "${#HM_ARGS[@]}"

test_case "exec is a transparent command"
is_transparent_command exec && r=yes || r=no
assert_equals "yes" "$r"

#
# `-d` takes no argument. Declaring otherwise meant `hm mysql -d -i dump.sql` — the form this tool
# documents — read `-i` as the argument of `-d`, stopped at the file name, imported nothing and
# exited 0. The other order was refused outright, so there was no way to ask for it at all.
#
test_case "the DEFINER option takes no argument of its own"
opciones=$(grep -o 'getopts "[^"]*"' "$COMMAND_BIN_DIR/console/commands/mysql.sh" | head -1)
assert_equals 'getopts ":i:q:da"' "$opciones"

test_case "and the documented order reaches the import"
leidas=$(bash -c 'while getopts ":i:q:da" o; do printf "%s=%s " "$o" "$OPTARG"; done' -- -d -i /tmp/x.sql)
assert_equals "d= i=/tmp/x.sql" "$(printf '%s' "$leidas" | sed 's/ $//')"

test_case "mysqldump is a transparent command"
is_transparent_command mysqldump && r=yes || r=no
assert_equals "yes" "$r"

test_case "describe is not a transparent command"
is_transparent_command describe && r=yes || r=no
assert_equals "no" "$r"

test_case "non-interactive mode also enables USE_DEFAULT_SETTINGS"
( unset USE_DEFAULT_SETTINGS
  export HM_NON_INTERACTIVE=1 HM_OUTPUT_FORMAT=""
  resolve_output_format
  echo "${USE_DEFAULT_SETTINGS:-unset}" ) > "$WORKDIR_TMP" 2>/dev/null
assert_equals "true" "$(cat "$WORKDIR_TMP")"

test_case "interactive mode leaves USE_DEFAULT_SETTINGS alone"
( unset USE_DEFAULT_SETTINGS
  export HM_NON_INTERACTIVE="" HM_OUTPUT_FORMAT="text"
  resolve_output_format
  echo "${USE_DEFAULT_SETTINGS:-unset}" ) > "$WORKDIR_TMP" 2>/dev/null
assert_equals "unset" "$(cat "$WORKDIR_TMP")"

test_case "without a terminal the default format is json"
( export HM_OUTPUT_FORMAT=""; resolve_output_format; echo "$HM_OUTPUT_FORMAT" ) > "$WORKDIR_TMP"
assert_equals "json" "$(cat "$WORKDIR_TMP")"

rm -f "$WORKDIR_TMP"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
