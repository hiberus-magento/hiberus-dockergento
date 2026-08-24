#!/usr/bin/env bash
#
# hm describe and hm list: what they answer and where they answer from.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

HM="$COMMAND_BIN_DIR/bin/run"
WORKDIR=$(mktemp -d)
trap 'rm -rf "$WORKDIR"' EXIT

if ! docker info >/dev/null 2>&1; then
    echo "  - skipped: docker is not available"
    echo "RESULT 0 0"
    exit 0
fi

run_in() {
    local dir="$1"
    shift
    ( cd "$dir" && "$HM" "$@" >"$WORKDIR/out" 2>"$WORKDIR/err" )
    STATUS=$?
    STDOUT=$(cat "$WORKDIR/out")
    STDERR=$(cat "$WORKDIR/err")
    return 0
}

# ---------------------------------------------------------------- hm list

test_case "list works outside any project"
run_in "$WORKDIR" list --json
assert_equals "0" "$STATUS"

test_case "list answers with valid JSON"
assert_json "$STDOUT"

test_case "list reports how many environments it found"
assert_json_field "$STDOUT" '.data.count | type' "number"

test_case "list is versioned like every other response"
assert_json_field "$STDOUT" '.schema_version' "1"

test_case "list names itself in the envelope"
assert_json_field "$STDOUT" '.command' "list"

test_case "every environment carries the documented keys"
missing=$(printf '%s' "$STDOUT" | jq -r '
    [.data.environments[]? | select(
        (has("name") and has("root") and has("worktree") and has("magento") and
         has("branch") and has("status") and has("containers") and
         has("has_metadata") and has("orphan")) | not)] | length')
assert_equals "0" "$missing"

test_case "list keeps stderr clean on success"
assert_empty "$STDERR"

test_case "list rejects unknown options"
run_in "$WORKDIR" list --nope
assert_equals "2" "$STATUS"

# ---------------------------------------------------------------- hm describe

test_case "describe outside a project fails with the project code"
run_in "$WORKDIR" describe --json
assert_equals "4" "$STATUS"

test_case "describe explains why it failed"
assert_json_field "$STDERR" '.error.type' "project_not_configured"

test_case "describe rejects unknown options"
run_in "$WORKDIR" describe --nope --json
assert_equals "4" "$STATUS"

# ------------------------------------------- a synthetic project with nothing running

SYNTHETIC="$WORKDIR/synthetic"
mkdir -p "$SYNTHETIC/config/docker"

sed -e 's/{YML_VERSION}//' \
    -e 's|<php_version>|hiberusmagento/php:8.5-bookworm|' \
    -e 's|<nginx_version>|hiberusmagento/nginx:1.30|' \
    -e 's|<mariadb_version>|mariadb:12.3|' \
    -e 's|<search_version>|hiberusmagento/search:3-opensearch|' \
    -e 's|<varnish_version>|hiberusmagento/varnish:7.1|' \
    -e 's|<redis_version>|valkey/valkey:9-alpine|' \
    -e 's|<mail_service>|mailhog|g' \
    -e 's|<mail_version>|hiberusmagento/mailhog:1|' \
    -e 's|<rabbitmq_version>|hiberusmagento/rabbitmq:4.3|' \
    -e 's|<hitch_version>|hiberusmagento/hitch:1.7|' \
    -e 's|<composer_version>|2.9|' \
    "$COMMAND_BIN_DIR/docker-compose/docker-compose.template.yml" > "$SYNTHETIC/docker-compose.yml"

sed -e 's/{YML_VERSION}//' -e 's|{MAGENTO_DIR}|.|g' -e 's|# {FILES_IN_GIT}||' \
    "$COMMAND_BIN_DIR/docker-compose/docker-compose.dev.mac.template.yml" > "$SYNTHETIC/docker-compose.dev.mac.yml"
sed -e 's/{YML_VERSION}//' -e 's|{MAGENTO_DIR}|.|g' \
    "$COMMAND_BIN_DIR/docker-compose/docker-compose.dev.linux.template.yml" > "$SYNTHETIC/docker-compose.dev.linux.yml"

cat > "$SYNTHETIC/config/docker/properties.json" <<'JSON'
{"MAGENTO_DIR": ".", "COMPOSE_PROJECT_NAME": "hm-describe-selftest", "DOMAIN": "selftest.local"}
JSON

test_case "describe answers with the environment stopped"
run_in "$SYNTHETIC" describe --json
assert_equals "0" "$STATUS"

test_case "a stopped environment is reported as stopped"
assert_json_field "$STDOUT" '.data.project.status' "stopped"

test_case "the domain is still reported with nothing running"
assert_json_field "$STDOUT" '.data.project.domain' "selftest.local"

test_case "the services are still listed with nothing running"
assert_json_field "$STDOUT" '.data.services | length' "9"

test_case "without composer.lock the Magento version is unknown"
assert_json_field "$STDOUT" '.data.magento.version' ""

test_case "xdebug is unknown while the container is down"
assert_json_field "$STDOUT" '.data.tooling.xdebug' "unknown"

# A real project is needed for the rest. Point HM_TEST_PROJECT at one to run them.
PROJECT_DIR="${HM_TEST_PROJECT:-}"

if [ -z "$PROJECT_DIR" ] || [ ! -f "$PROJECT_DIR/docker-compose.yml" ]; then
    echo "  - skipped: set HM_TEST_PROJECT to a Dockergento project to run the describe checks"
    echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
    exit 0
fi

test_case "describe answers inside a project"
run_in "$PROJECT_DIR" describe --json
assert_equals "0" "$STATUS"

test_case "describe answers with valid JSON"
assert_json "$STDOUT"

test_case "describe reports the project name"
assert_json_field "$STDOUT" '.data.project.name | length > 0' "true"

test_case "describe reports every service"
assert_json_field "$STDOUT" '.data.services | length > 0' "true"

test_case "describe reports the environment status"
status=$(printf '%s' "$STDOUT" | jq -r '.data.project.status')
case "$status" in running|stopped|partial) r=ok ;; *) r="$status" ;; esac
assert_equals "ok" "$r"

test_case "describe hides credentials by default"
assert_json_field "$STDOUT" '.data | has("credentials")' "false"

test_case "describe shows credentials only when asked"
run_in "$PROJECT_DIR" describe --json --with-secrets
assert_json_field "$STDOUT" '.data.credentials.database | has("password")' "true"

test_case "describe carries the documented blocks"
missing=$(printf '%s' "$STDOUT" | jq -r '
    [.data | select((has("project") and has("magento") and has("services") and
                     has("paths") and has("tooling")) | not)] | length')
assert_equals "0" "$missing"

test_case "the readable output leads with the URLs"
run_in "$PROJECT_DIR" describe --no-json
assert_contains "$STDOUT" "URLs"

test_case "the readable output is not JSON"
assert_not_contains "$STDOUT" '"schema_version"'

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
