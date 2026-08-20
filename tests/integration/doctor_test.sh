#!/usr/bin/env bash
#
# hm doctor: scopes, severities, exit codes and robustness.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

HM="$COMMAND_BIN_DIR/bin/run"
DOCTOR_DIR="$COMMAND_BIN_DIR/console/tasks/doctor"
WORKDIR=$(mktemp -d)

# Temporary checks used to exercise the runner's own robustness
SLOW_CHECK="$DOCTOR_DIR/99-selftestslow.sh"
BROKEN_CHECK="$DOCTOR_DIR/99-selftestbroken.sh"

cleanup() {
    rm -f "$SLOW_CHECK" "$BROKEN_CHECK"
    rm -rf "$WORKDIR"
}
trap cleanup EXIT

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

test_case "doctor runs outside a project"
run_in "$WORKDIR" doctor --json
assert_json "$STDOUT"

test_case "outside a project only global checks report"
project_scoped=$(printf '%s' "$STDOUT" | jq -r '[.data.checks[] | select(.scope == "project")] | length')
assert_equals "0" "$project_scoped"

test_case "the diagnosis reports a summary"
assert_json_field "$STDOUT" '.data.summary | has("errors")' "true"

test_case "every check reports the documented fields"
missing=$(printf '%s' "$STDOUT" | jq -r '
    [.data.checks[] | select((has("id") and has("scope") and has("severity") and
                              has("message") and has("action")) | not)] | length')
assert_equals "0" "$missing"

test_case "every severity is one of the three allowed"
unexpected=$(printf '%s' "$STDOUT" | jq -r '
    [.data.checks[] | select([.severity] | inside(["ok","warning","error"]) | not)] | length')
assert_equals "0" "$unexpected"

test_case "--only runs a single check"
run_in "$WORKDIR" doctor --only=docker-daemon --json
assert_json_field "$STDOUT" '.data.checks | length' "1"

test_case "--only picks the requested check"
assert_json_field "$STDOUT" '.data.checks[0].id' "docker-daemon"

test_case "an unknown option is rejected"
run_in "$WORKDIR" doctor --nope
assert_equals "2" "$STATUS"

test_case "a check that hangs is reported without blocking the rest"
cat > "$SLOW_CHECK" <<'CHECK'
#!/usr/bin/env bash
source "$HELPERS_DIR"/doctor.sh
sleep 30
doctor_ok "never reached"
CHECK
run_in "$WORKDIR" doctor --json
assert_json_field "$STDOUT" '[.data.checks[] | select(.id == "selftestslow")] | length' "1"

test_case "a hanging check is a warning, not a failure"
assert_json_field "$STDOUT" '.data.checks[] | select(.id == "selftestslow") | .severity' "warning"

test_case "the rest of the diagnosis still ran"
assert_json_field "$STDOUT" '[.data.checks[] | select(.id == "docker-daemon")] | length' "1"

test_case "a hanging check does not fail the command"
assert_equals "0" "$STATUS"
rm -f "$SLOW_CHECK"

test_case "a check that crashes is reported as a warning"
cat > "$BROKEN_CHECK" <<'CHECK'
#!/usr/bin/env bash
exit 3
CHECK
run_in "$WORKDIR" doctor --json
assert_json_field "$STDOUT" '.data.checks[] | select(.id == "selftestbroken") | .severity' "warning"

test_case "a crashing check does not stop the others"
assert_json_field "$STDOUT" '[.data.checks[] | select(.id == "certificates")] | length' "1"
rm -f "$BROKEN_CHECK"

test_case "warnings alone keep the exit code at zero"
run_in "$WORKDIR" doctor --only=disk-usage --json
assert_equals "0" "$STATUS"

test_case "the whole diagnosis stays under ten seconds"
start=$(date +%s)
run_in "$WORKDIR" doctor --json
elapsed=$(( $(date +%s) - start ))
[ "$elapsed" -lt 10 ] && r=fast || r="$elapsed seconds"
assert_equals "fast" "$r"

# ------------------------------------------------ a project with a port conflict

PROJECT="$WORKDIR/project"
mkdir -p "$PROJECT/config/docker"

sed -e 's/{YML_VERSION}//' \
    -e 's|<php_version>|hiberusmagento/php:8.5-bookworm|' \
    -e 's|<nginx_version>|hiberusmagento/nginx:1.30|' \
    -e 's|<mariadb_version>|mariadb:12.3|' \
    -e 's|<search_version>|hiberusmagento/search:3-opensearch|' \
    -e 's|<varnish_version>|hiberusmagento/varnish:7.1|' \
    -e 's|<redis_version>|valkey/valkey:9-alpine|' \
    -e 's|<mailhog_version>|hiberusmagento/mailhog:1|' \
    -e 's|<rabbitmq_version>|hiberusmagento/rabbitmq:4.3|' \
    -e 's|<hitch_version>|hiberusmagento/hitch:1.7|' \
    -e 's|<composer_version>|2.9|' \
    "$COMMAND_BIN_DIR/docker-compose/docker-compose.template.yml" > "$PROJECT/docker-compose.yml"
sed -e 's/{YML_VERSION}//' -e 's|{MAGENTO_DIR}|.|g' -e 's|# {FILES_IN_GIT}||' \
    "$COMMAND_BIN_DIR/docker-compose/docker-compose.dev.mac.template.yml" > "$PROJECT/docker-compose.dev.mac.yml"
sed -e 's/{YML_VERSION}//' -e 's|{MAGENTO_DIR}|.|g' \
    "$COMMAND_BIN_DIR/docker-compose/docker-compose.dev.linux.template.yml" > "$PROJECT/docker-compose.dev.linux.yml"

cat > "$PROJECT/config/docker/properties.json" <<'JSON'
{"MAGENTO_DIR": ".", "COMPOSE_PROJECT_NAME": "hm-doctor-selftest", "DOMAIN": "doctor-selftest.local"}
JSON

test_case "inside a project the project checks run too"
run_in "$PROJECT" doctor --json
project_scoped=$(printf '%s' "$STDOUT" | jq -r '[.data.checks[] | select(.scope == "project")] | length')
[ "$project_scoped" -gt 0 ] && r=yes || r=no
assert_equals "yes" "$r"

# Regression: the expensive labels are only exported for commands that create containers,
# so a check reading HM_MAGENTO straight from the environment misread "not computed yet" as
# "this project has no Magento".
test_case "a project with Magento is not reported as lacking it"
cat > "$PROJECT/composer.lock" <<'JSON'
{"packages":[{"name":"magento/product-community-edition","version":"2.4.9"}]}
JSON
run_in "$PROJECT" doctor --json
message=$(printf '%s' "$STDOUT" | jq -r '.data.checks[] | select(.id == "magento") | .message')
assert_not_contains "$message" "has no Magento package"

test_case "and the version it reports is the real one"
assert_contains "$message" "2.4.9"
rm -f "$PROJECT/composer.lock"

test_case "a project without Magento reports it, without failing the diagnosis"
assert_json_field "$STDOUT" '.data.checks[] | select(.id == "magento") | .severity' "warning"

test_case "a missing host entry is a warning with an action"
assert_json_field "$STDOUT" '.data.checks[] | select(.id == "hosts") | .action | length > 0' "true"

test_case "the compose configuration of the synthetic project is valid"
assert_json_field "$STDOUT" '.data.checks[] | select(.id == "compose-config") | .severity' "ok"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
