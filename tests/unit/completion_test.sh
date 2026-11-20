#!/usr/bin/env bash
#
# The shell completion is generated from data/command_descriptions.json, the declaration the help
# reads. It used to glob one script per command, and once those were gone it offered a lone "*".
# The generator runs in a copy of the installation, so the suite never rewrites the committed file.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

copy=$(mktemp -d)
trap 'rm -rf "$copy"; hm_test_home_cleanup' EXIT

mkdir -p "$copy/console/helpers" "$copy/data"
cp "$HM_TEST_PROJECT_ROOT/generate_completion.sh" "$copy/"
cp "$HM_TEST_PROJECT_ROOT/console/helpers/array_manager.sh" "$copy/console/helpers/"
cp "$HM_TEST_PROJECT_ROOT/data/command_descriptions.json" "$copy/data/"

test_case "the generator runs"
output=$("$copy/generate_completion.sh" 2>&1)
assert_equals "0" "$?" "$output"

# What the file hands to `complete -W`, captured instead of registered
offered=$(bash -c 'complete() { [ "$1" == "-W" ] && printf "%s" "$2"; }; source "$1"' _ "$copy/console/hm-completion.bash")
words=" $(printf '%s' "$offered" | tr -s '[:space:]' ' ') "

test_case "every command in command_descriptions.json is offered"
missing=""
while IFS= read -r command_name; do
    case "$command_name" in
        copy-from-container | copy-to-container) [ "$(uname -s)" == "Darwin" ] || continue ;;
    esac
    [[ "$words" == *" $command_name "* ]] || missing="$missing $command_name"
done < <(jq -r 'keys[] | select(startswith("_") | not)' "$HM_TEST_PROJECT_ROOT/data/command_descriptions.json")
assert_empty "$missing" "missing from the completion"

test_case "no glob that matched nothing and no metadata key is offered"
assert_not_contains "$offered" "*"
assert_not_contains "$words" " _"
assert_not_contains "$words" ' \ '

# Only on a Mac: elsewhere the generator leaves out the copy-*-container commands
if [ "$(uname -s)" == "Darwin" ]; then
    test_case "the committed completion is the one the generator writes"
    assert_equals "$(cat "$copy/console/hm-completion.bash")" "$(cat "$HM_TEST_PROJECT_ROOT/console/hm-completion.bash")"
fi

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
