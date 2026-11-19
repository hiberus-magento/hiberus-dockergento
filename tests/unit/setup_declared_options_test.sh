#!/usr/bin/env bash
#
# What setup takes is declared where every command's options are declared, so the help and the
# completion offer it. The parser itself is the binary's, pinned in Go (core/setup_parse_test.go).
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

DATA_DIR="$HM_TEST_PROJECT_ROOT/data"

test_case "everything the parser takes is declared where options are declared"
declared=$(jq -r '.setup.opts[] | (if .name.short != "" then "-" + .name.short + "\n" else "" end) + (if .name.long != "" then "--" + (.name.long | split("=")[0]) else "" end)' "$DATA_DIR/command_descriptions.json" | grep .)
for option in --domain --dump --install --project-name --root-directory --force --use-default --clean-install --db-dump --mail; do
    assert_contains "$declared" "$option"
done

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
