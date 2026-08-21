#!/usr/bin/env bash
#
# The three missing verbs, against a synthetic project.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

HM="$COMMAND_BIN_DIR/bin/run"
LAB=$(cd "$(mktemp -d)" && pwd -P)
PROJECT="hm-verbs-selftest"

cleanup() {
    ( cd "$LAB" && docker compose -p "$PROJECT" down -v --remove-orphans >/dev/null 2>&1 )
    rm -rf "$LAB"
}
trap cleanup EXIT

run_in_lab() {
    ( cd "$LAB" && "$HM" "$@" >"$LAB/out" 2>"$LAB/err" )
    STATUS=$?
    STDOUT=$(cat "$LAB/out")
    STDERR=$(cat "$LAB/err")
    return 0
}

# ---------------------------------------------------------------- version, without a project

test_case "version answers outside a project"
( cd "$LAB" && "$HM" version --json >"$LAB/out" 2>"$LAB/err" )
assert_equals "0" "$?"

test_case "and reports the version of the tool"
assert_json_field "$(cat "$LAB/out")" '.ok' "true"

test_case "the version carries an exact reference"
version=$(jq -r '.data.version' < "$LAB/out")
[ -n "$version" ] && [ "$version" != "null" ] && r=named || r="$version"
assert_equals "named" "$r"

test_case "readable output names the tool and the version"
( cd "$LAB" && "$HM" version --no-json >"$LAB/out" 2>&1 )
assert_contains "$(cat "$LAB/out")" "$COMMAND_BIN_NAME"

test_case "and reports docker and compose"
assert_contains "$(cat "$LAB/out")" "compose"

test_case "an unknown option is a usage error"
run_in_lab version --nonsense
assert_equals "2" "$STATUS"

test_case "the flag still answers as it did"
( cd "$LAB" && "$HM" --version --json >"$LAB/out" 2>&1 )
assert_equals "false" "$(jq 'has("docker")' < "$LAB/out" | head -1)"

# ---------------------------------------------------------------- a project to work against

mkdir -p "$LAB/config/docker"
cd "$LAB" || exit 1

cat > docker-compose.yml <<'YAML'
services:
  phpfpm:
    image: alpine:latest
    command: ["sh", "-c", "echo hello-from-phpfpm; sleep 120"]
  db:
    image: alpine:latest
    command: ["sh", "-c", "echo hello-from-db; sleep 120"]
YAML
cp docker-compose.yml docker-compose.dev.mac.yml
cp docker-compose.yml docker-compose.dev.linux.yml
echo '{"MAGENTO_DIR": ".", "COMPOSE_PROJECT_NAME": "hm-verbs-selftest", "DOMAIN": "verbs.local"}' \
    > config/docker/properties.json

# ---------------------------------------------------------------- launch

test_case "launch reports the storefront address"
run_in_lab launch --json
assert_json_field "$STDOUT" '.data.url' "https://verbs.local/"

test_case "and does not open anything when asked for JSON"
assert_json_field "$STDOUT" '.data.opened' "false"

test_case "the admin panel is a destination of its own"
run_in_lab launch --admin --json
assert_json_field "$STDOUT" '.data.url' "https://verbs.local/admin"

# Magento generates a random front name on install unless told otherwise, and on a project that
# has one /admin is a 404. It lives in app/etc/env.php, which is readable with the environment
# stopped.
test_case "a custom admin front name is honoured"
mkdir -p "$LAB/app/etc"
cat > "$LAB/app/etc/env.php" <<'PHP'
<?php
return [
    'backend' => [
        'frontName' => 'admin_7f3k9x'
    ],
    'MAGE_MODE' => 'developer'
];
PHP
run_in_lab launch --admin --json
assert_json_field "$STDOUT" '.data.url' "https://verbs.local/admin_7f3k9x"

test_case "and describe reports the same address"
run_in_lab describe --json
assert_json_field "$STDOUT" '.data.project.urls.admin' "https://verbs.local/admin_7f3k9x"

test_case "the storefront is unaffected by it"
run_in_lab launch --json
assert_json_field "$STDOUT" '.data.url' "https://verbs.local/"

test_case "an env.php without a front name falls back to admin"
printf '<?php\nreturn ['"'"'MAGE_MODE'"'"' => '"'"'developer'"'"'];\n' > "$LAB/app/etc/env.php"
run_in_lab launch --admin --json
assert_json_field "$STDOUT" '.data.url' "https://verbs.local/admin"

rm -rf "$LAB/app"

test_case "two destinations are a usage error"
run_in_lab launch --admin --search --json
assert_equals "2" "$STATUS"

test_case "and the error says why"
assert_json_field "$STDERR" '.error.type' "conflicting_options"

test_case "an unknown option is refused too"
run_in_lab launch --nonsense --json
assert_equals "2" "$STATUS"

test_case "a service that publishes no port has no address"
run_in_lab launch --rabbitmq --json
assert_json_field "$STDERR" '.error.type' "no_address"

test_case "a project with no domain says so instead of opening something wrong"
echo '{"MAGENTO_DIR": ".", "COMPOSE_PROJECT_NAME": "hm-verbs-selftest"}' \
    > config/docker/properties.json
run_in_lab launch --json
assert_json_field "$STDERR" '.error.type' "no_domain"

test_case "and points at how to fix it"
assert_contains "$STDERR" "--domain"

echo '{"MAGENTO_DIR": ".", "COMPOSE_PROJECT_NAME": "hm-verbs-selftest", "DOMAIN": "verbs.local"}' \
    > config/docker/properties.json

# ---------------------------------------------------------------- logs

test_case "logs of a service the project does not have is refused"
run_in_lab logs nonexistent
assert_equals "5" "$STATUS"

test_case "and the refusal names the services that do exist"
assert_contains "$STDERR" "phpfpm"

test_case "an option that needs a value says so when it has none"
run_in_lab logs --tail
assert_equals "2" "$STATUS"

if ! docker info >/dev/null 2>&1; then
    echo "  - skipped: docker is not available for the rest of the log checks"
    echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
    exit 0
fi

docker compose -p "$PROJECT" up -d >/dev/null 2>&1

test_case "logs shows what the containers printed"
run_in_lab logs
assert_contains "$STDOUT" "hello-from-phpfpm"

test_case "and everything, when no service is named"
assert_contains "$STDOUT" "hello-from-db"

test_case "naming a service narrows it down"
run_in_lab logs db
assert_contains "$STDOUT" "hello-from-db"

test_case "to that service only"
assert_not_contains "$STDOUT" "hello-from-phpfpm"

test_case "the value of an option is not read as a service name"
run_in_lab logs --tail 1 db
assert_equals "0" "$STATUS"

test_case "and the option reaches Compose"
assert_equals "1" "$(printf '%s\n' "$STDOUT" | grep -c "hello-from-db")"

# `logs` is a transparent command: a flag before the command name is the CLI's, a flag after it
# belongs to the process being wrapped. That is the same rule as `mysql` and `composer`.
test_case "the output is data: asking for JSON does not wrap it"
run_in_lab --json logs db
assert_contains "$STDOUT" "hello-from-db"

test_case "so nothing of the JSON envelope appears in it"
assert_not_contains "$STDOUT" "schema_version"

test_case "and a flag after the command name is left for Compose"
run_in_lab logs --json db
assert_equals "1" "$STATUS"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
