#!/usr/bin/env bash
#
# Checking code with whatever the project has.
#
# Built against a real PHP container rather than a mocked one: the point of the command is that it
# runs the project's own tools with the project's own PHP, and a mock would prove none of that.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

HM="$COMMAND_BIN_DIR/bin/run"
REAL_HOME=$(eval echo "~$(id -un)")
LAB="$REAL_HOME/.hm/verify-selftest"
PROJECT="hm-verify-selftest"

cleanup() {
    ( cd "$LAB" 2>/dev/null && docker compose -p "$PROJECT" down -v ) >/dev/null 2>&1
    rm -rf "$LAB"
}
trap cleanup EXIT
cleanup

if ! docker info >/dev/null 2>&1; then
    echo "  - skipped: docker is not available"
    echo "RESULT 0 0"
    exit 0
fi

mkdir -p "$LAB/config/docker" "$LAB/app/code/Vendor/Module" "$LAB/vendor/bin"

cat > "$LAB/docker-compose.yml" <<'YAML'
services:
  phpfpm:
    image: php:8.3-cli
    command: ["sleep", "900"]
    working_dir: /var/www/html
    volumes:
      - .:/var/www/html
YAML
cp "$LAB/docker-compose.yml" "$LAB/docker-compose.dev.mac.yml"
cp "$LAB/docker-compose.yml" "$LAB/docker-compose.dev.linux.yml"
printf '{"MAGENTO_DIR": ".", "DOMAIN": "verify.local", "COMPOSE_PROJECT_NAME": "%s"}\n' "$PROJECT" \
    > "$LAB/config/docker/properties.json"

printf '<?php\nclass Good { public function run(): void {} }\n' > "$LAB/app/code/Vendor/Module/Good.php"

( cd "$LAB" && docker compose -p "$PROJECT" up -d ) >/dev/null 2>&1
sleep 5

run_verify() {
    ( cd "$LAB" && "$HM" verify "$@" >"$LAB/out" 2>"$LAB/err" )
    STATUS=$?
    STDOUT=$(cat "$LAB/out")
    return 0
}

check_status() {
    printf '%s' "$STDOUT" | jq -r --arg name "$1" '.data.checks[] | select(.name == $name) | .status'
}

# ---------------------------------------------------------------- a project with nothing

test_case "a project with no tools does not fail"
run_verify --json
assert_equals "0" "$STATUS"

test_case "and says the tools are missing rather than broken"
assert_equals "skipped" "$(check_status "static-analysis")"

test_case "the coding standard too"
assert_equals "skipped" "$(check_status "coding-standard")"

test_case "the slow ones are skipped unless asked for"
assert_equals "skipped" "$(check_status "unit-tests")"

test_case "syntax is checked even with nothing installed"
assert_equals "ok" "$(check_status "syntax")"

# ---------------------------------------------------------------- broken syntax

test_case "a file that does not parse is caught"
printf '<?php\nclass Broken { public function run(: void {} }\n' > "$LAB/app/code/Vendor/Module/Broken.php"
run_verify --json
assert_equals "failed" "$(check_status "syntax")"

test_case "and the command fails, so it can be chained"
assert_equals "1" "$STATUS"

test_case "the offending file is named"
assert_contains "$(printf '%s' "$STDOUT" | jq -r '.data.checks[] | select(.name=="syntax") | .output')" "Broken.php"

rm -f "$LAB/app/code/Vendor/Module/Broken.php"

# ---------------------------------------------------------------- a tool that is installed

# A stand-in for the real thing: what is being checked here is that an installed tool is run and
# its verdict respected, not that PHPStan works.
install_fake() {
    printf '#!/bin/sh\nexit %s\n' "$2" > "$LAB/vendor/bin/$1"
    chmod +x "$LAB/vendor/bin/$1"
}

test_case "an installed tool that passes is reported as passing"
install_fake phpstan 0
run_verify --json
assert_equals "ok" "$(check_status "static-analysis")"

test_case "and the command succeeds"
assert_equals "0" "$STATUS"

test_case "an installed tool that finds problems fails the verification"
install_fake phpstan 1
run_verify --json
assert_equals "failed" "$(check_status "static-analysis")"

test_case "and the command fails"
assert_equals "1" "$STATUS"

test_case "while the ones that are not installed stay skipped"
assert_equals "skipped" "$(check_status "coding-standard")"

rm -f "$LAB/vendor/bin/phpstan"

# ---------------------------------------------------------------- scope

test_case "without a base branch it verifies everything and says so"
run_verify --changed --json
assert_contains "$(printf '%s' "$STDOUT" | jq -r '.data.scope')" "everything"

test_case "readable output lists every check"
run_verify --no-json
assert_contains "$STDOUT" "syntax"

test_case "and marks the skipped ones with their reason"
assert_contains "$STDOUT" "not installed"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
