#!/usr/bin/env bash
#
# End-to-end behaviour of the router: formats, exit codes and stream separation.
# Runs from a temporary directory that is deliberately not a Dockergento project.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

HM="$COMMAND_BIN_DIR/bin/run"
WORKDIR=$(mktemp -d)
OUT="$WORKDIR/stdout"
ERR="$WORKDIR/stderr"
trap 'rm -rf "$WORKDIR"' EXIT

# run <args...> — captures stdout, stderr and the exit code in separate files
run() {
    ( cd "$WORKDIR" && "$HM" "$@" >"$OUT" 2>"$ERR" )
    STATUS=$?
    STDOUT=$(cat "$OUT")
    STDERR=$(cat "$ERR")
    return 0
}

# run_binary <args...> — the same, asking bin/hm
run_binary() {
    ( cd "$WORKDIR" && "$COMMAND_BIN_DIR/bin/hm" "$@" >"$OUT" 2>"$ERR" )
    STATUS=$?
    STDOUT=$(cat "$OUT")
    STDERR=$(cat "$ERR")
    return 0
}

test_case "an unknown command exits with the usage code"
run noexiste
assert_equals "2" "$STATUS"

test_case "an unknown command reports the error as JSON when piped"
assert_json "$STDERR"

test_case "an unknown command reports its type"
assert_json_field "$STDERR" '.error.type' "command_not_found"

test_case "an unknown command keeps stdout clean"
assert_empty "$STDOUT"

test_case "--no-json falls back to readable text"
run --no-json noexiste
assert_not_contains "$STDERR" '"schema_version"'

test_case "a readable error still mentions the command"
assert_contains "$STDERR" "noexiste"

test_case "a readable error still exits with the usage code"
assert_equals "2" "$STATUS"

# A command that needs a project, asked of the binary that answers it: bin/run only says the
# command moved, which is what the retired-command contract below checks
test_case "outside a project the exit code says so"
run_binary magento
assert_equals "4" "$STATUS"

test_case "outside a project the error is machine readable"
assert_json_field "$STDERR" '.error.type' "project_not_configured"

test_case "outside a project stdout stays empty"
assert_empty "$STDOUT"

test_case "the error names the command that failed"
assert_json_field "$STDERR" '.command' "magento"

test_case "--help still works"
run --help
assert_equals "0" "$STATUS"

test_case "--help prints something"
assert_contains "$STDOUT$STDERR" "hm"

test_case "the global flag is not passed on to the command"
run --json noexiste
assert_json_field "$STDERR" '.command' "noexiste"

# ---------------------------------------------------------------- a command that moved into the binary
#
# `set-host` stood in for the twelve retired twins in unit 2: it was the one whose shell file went
# in that same unit, so it was the only one that could turn this RED into GREEN without waiting on
# a later unit. `version` is added here, in unit 4, now that `console/commands/version.sh` is
# actually gone.

test_case "a command that moved into the binary says so, not that it was never found"
run set-host
assert_equals "2" "$STATUS"
assert_json_field "$STDERR" '.error.type' "command_moved_to_binary"

test_case "and the readable hint names the binary command"
run --no-json set-host
assert_contains "$STDERR" "hm set-host"

test_case "and so does version, once its own shell twin is gone"
run version
assert_equals "2" "$STATUS"
assert_json_field "$STDERR" '.error.type' "command_moved_to_binary"

test_case "and the readable hint for version names the binary command too"
run --no-json version
assert_contains "$STDERR" "hm version"

test_case "and so does describe, once its own shell twin is gone"
run describe
assert_equals "2" "$STATUS"
assert_json_field "$STDERR" '.error.type' "command_moved_to_binary"

test_case "and the readable hint for describe names the binary command too"
run --no-json describe
assert_contains "$STDERR" "hm describe"

test_case "and so does debug-on, once its own shell twin is gone"
run debug-on
assert_equals "2" "$STATUS"
assert_json_field "$STDERR" '.error.type' "command_moved_to_binary"

test_case "and the readable hint for debug-on names the binary command too"
run --no-json debug-on
assert_contains "$STDERR" "hm debug-on"

test_case "and so does debug-off, once its own shell twin is gone"
run debug-off
assert_equals "2" "$STATUS"
assert_json_field "$STDERR" '.error.type' "command_moved_to_binary"

test_case "and the readable hint for debug-off names the binary command too"
run --no-json debug-off
assert_contains "$STDERR" "hm debug-off"

test_case "and so does bash, once its own shell twin is gone"
run bash
assert_equals "2" "$STATUS"
assert_json_field "$STDERR" '.error.type' "command_moved_to_binary"

test_case "and the readable hint for bash names the binary command too"
run --no-json bash
assert_contains "$STDERR" "hm bash"

test_case "and so does magento, once its own shell twin is gone"
run magento
assert_equals "2" "$STATUS"
assert_json_field "$STDERR" '.error.type' "command_moved_to_binary"

test_case "and the readable hint for magento names the binary command too"
run --no-json magento
assert_contains "$STDERR" "hm magento"

test_case "and so does docker-compose, once its own shell twin is gone"
run docker-compose
assert_equals "2" "$STATUS"
assert_json_field "$STDERR" '.error.type' "command_moved_to_binary"

test_case "and the readable hint for docker-compose names the binary command too"
run --no-json docker-compose
assert_contains "$STDERR" "hm docker-compose"

test_case "and so does restart, once its own shell twin is gone"
run restart
assert_equals "2" "$STATUS"
assert_json_field "$STDERR" '.error.type' "command_moved_to_binary"

test_case "and the readable hint for restart names the binary command too"
run --no-json restart
assert_contains "$STDERR" "hm restart"

test_case "and so does docker-stop-all, once its own shell twin is gone"
run docker-stop-all
assert_equals "2" "$STATUS"
assert_json_field "$STDERR" '.error.type' "command_moved_to_binary"

test_case "and the readable hint for docker-stop-all names the binary command too"
run --no-json docker-stop-all
assert_contains "$STDERR" "hm docker-stop-all"

test_case "and so does masquerade, once its own shell twin is gone"
run masquerade
assert_equals "2" "$STATUS"
assert_json_field "$STDERR" '.error.type' "command_moved_to_binary"

test_case "and the readable hint for masquerade names the binary command too"
run --no-json masquerade
assert_contains "$STDERR" "hm masquerade"

test_case "and so does web, once its own shell twin is gone"
run web
assert_equals "2" "$STATUS"
assert_json_field "$STDERR" '.error.type' "command_moved_to_binary"

test_case "and the readable hint for web names the binary command too"
run --no-json web
assert_contains "$STDERR" "hm web"

test_case "and so does worktree, once its own shell twin is gone"
run worktree list
assert_equals "2" "$STATUS"
assert_json_field "$STDERR" '.error.type' "command_moved_to_binary"

test_case "and the readable hint for worktree names the binary command too"
run --no-json worktree list
assert_contains "$STDERR" "hm worktree"

test_case "and so does stop, once its own shell twin is gone"
run stop
assert_equals "2" "$STATUS"
assert_json_field "$STDERR" '.error.type' "command_moved_to_binary"

test_case "and the readable hint for stop names the binary command too"
run --no-json stop
assert_contains "$STDERR" "hm stop"

test_case "and so does start, once its own shell twin is gone"
run start
assert_equals "2" "$STATUS"
assert_json_field "$STDERR" '.error.type' "command_moved_to_binary"

test_case "and the readable hint for start names the binary command too"
run --no-json start
assert_contains "$STDERR" "hm start"

test_case "and so does down, once its own shell twin is gone"
run down
assert_equals "2" "$STATUS"
assert_json_field "$STDERR" '.error.type' "command_moved_to_binary"

test_case "and the readable hint for down names the binary command too"
run --no-json down
assert_contains "$STDERR" "hm down"

test_case "and so does mysql, once its own shell twin is gone"
run mysql -q "SELECT 1"
assert_equals "2" "$STATUS"
assert_json_field "$STDERR" '.error.type' "command_moved_to_binary"

test_case "and the readable hint for mysql names the binary command too"
run --no-json mysql -q "SELECT 1"
assert_contains "$STDERR" "hm mysql"

test_case "and so does setup, once its own shell twin is gone"
run setup
assert_equals "2" "$STATUS"
assert_json_field "$STDERR" '.error.type' "command_moved_to_binary"

test_case "and the readable hint for setup names the binary command too"
run --no-json setup
assert_contains "$STDERR" "hm setup"

test_case "and so does doctor, once its own shell twin is gone"
run doctor
assert_equals "2" "$STATUS"
assert_json_field "$STDERR" '.error.type' "command_moved_to_binary"

test_case "and the readable hint for doctor names the binary command too"
run --no-json doctor
assert_contains "$STDERR" "hm doctor"

test_case "and so does clean, once its own shell twin is gone"
run clean
assert_equals "2" "$STATUS"
assert_json_field "$STDERR" '.error.type' "command_moved_to_binary"

test_case "and the readable hint for clean names the binary command too"
run --no-json clean
assert_contains "$STDERR" "hm clean"

test_case "and so does create-project, once it is the binary's"
run create-project
assert_equals "2" "$STATUS"
assert_json_field "$STDERR" '.error.type' "command_moved_to_binary"

for command in composer exec cloud cloud-login dbeaver sequelace tableplus config-env post-start rebuild grunt update ssl transfer-media launch permissions ai-context proxy share tunnel compatibility transfer-db switch verify; do
    test_case "and so does $command, once its own shell twin is gone"
    run "$command" --version
    assert_equals "2" "$STATUS"
    assert_json_field "$STDERR" '.error.type' "command_moved_to_binary"
done

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
