#!/usr/bin/env bash
#
# What `hm setup` was told to do.
#
# The options it documents were not the options it took: the parser was a getopts string that
# understands short forms only, so `--dump=dump.sql` was an unknown option and the command asked
# the question it had been given the answer to.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"
source "$COMPONENTS_DIR/print_message.sh"
source "$HELPERS_DIR/exit_codes.sh"
source "$TASKS_DIR/setup_options.sh"

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT
printf -- '-- un volcado\n' > "$WORK/dump.sql"

parse() { hm_setup_parse_options "$@" 2>"$WORK/err"; }

# ---------------------------------------------------------------- the long forms

test_case "every documented option is taken"
parse --domain=shop.test --project-name=shop --root-directory=./src --force --use-default
assert_equals "shop.test" "$SETUP_DOMAIN"
assert_equals "shop" "$SETUP_PROJECT_NAME"
assert_equals "./src" "$SETUP_ROOT"
assert_equals "true" "$SETUP_FORCE"
assert_equals "true" "$SETUP_USE_DEFAULT"

test_case "a value after a space reads the same as one after an equals"
parse --domain shop.test
assert_equals "shop.test" "$SETUP_DOMAIN"

test_case "the short forms still work"
parse -d shop.test -p shop -r ./src -f -u -i
assert_equals "shop.test" "$SETUP_DOMAIN"
assert_equals "shop" "$SETUP_PROJECT_NAME"
assert_equals "true" "$SETUP_INSTALL"

test_case "and nothing given means nothing decided"
parse
assert_equals "" "$SETUP_DOMAIN"
assert_equals "false" "$SETUP_INSTALL"
assert_equals "" "$SETUP_DUMP"

# ---------------------------------------------------------------- the Warden names
#
# Aliases are usually a smell, and these are not: half the department has used Warden, and a tool
# that refuses the word somebody typed in order to be tidy is being tidy at their expense.

test_case "--clean-install means --install"
parse --clean-install
assert_equals "true" "$SETUP_INSTALL"

test_case "--db-dump means --dump"
parse --db-dump="$WORK/dump.sql"
assert_equals "$WORK/dump.sql" "$SETUP_DUMP"

test_case "in both spellings"
parse --db-dump "$WORK/dump.sql"
assert_equals "$WORK/dump.sql" "$SETUP_DUMP"
parse -D "$WORK/dump.sql"
assert_equals "$WORK/dump.sql" "$SETUP_DUMP"

# ---------------------------------------------------------------- a dump that is not there
#
# It used to be a warning, after which the command carried on and asked interactively — so an
# automated bootstrap with a wrong path hung instead of failing.

test_case "a dump that does not exist stops the command"
( parse --dump=/no/such/file.sql ) && r=aceptado || r=rechazado
assert_equals "rechazado" "$r"

test_case "and says which file it was"
assert_contains "$(cat "$WORK/err")" "/no/such/file.sql"

test_case "a home-relative path is expanded rather than refused"
cp "$WORK/dump.sql" "$HOME/hm-setup-test-dump.sql"
parse --dump="~/hm-setup-test-dump.sql"
assert_equals "$HOME/hm-setup-test-dump.sql" "$SETUP_DUMP"
rm -f "$HOME/hm-setup-test-dump.sql"

# ---------------------------------------------------------------- refusals

test_case "an option nobody declared is a usage error"
( parse --nonsense ) && r=aceptado || r=rechazado
assert_equals "rechazado" "$r"
assert_contains "$(cat "$WORK/err")" "--nonsense"

test_case "and so is an argument that is not an option"
( parse loquesea ) && r=aceptado || r=rechazado
assert_equals "rechazado" "$r"

# ---------------------------------------------------------------- the mail catcher

test_case "the mail catcher is part of the same parse"
parse --mail=mailpit
assert_equals "mailpit" "$SETUP_MAIL"

# ---------------------------------------------------------------- what the command declares

test_case "everything the parser takes is declared where options are declared"
declared=$(jq -r '.setup.opts[] | (if .name.short != "" then "-" + .name.short + "\n" else "" end) + (if .name.long != "" then "--" + (.name.long | split("=")[0]) else "" end)' "$DATA_DIR/command_descriptions.json" | grep .)
for option in --domain --dump --install --project-name --root-directory --force --use-default --clean-install --db-dump --mail; do
    assert_contains "$declared" "$option"
done

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
