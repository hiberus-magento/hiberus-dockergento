#!/usr/bin/env bash
#
# The commands that destroy work, and what now stands between them and the damage.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

HM="$COMMAND_BIN_DIR/bin/run"
LAB=$(cd "$(mktemp -d)" && pwd -P)
PROJECT="hm-lifecycle-selftest"

cleanup() {
    ( cd "$LAB/project" 2>/dev/null && docker compose -p "$PROJECT" down -v --remove-orphans ) >/dev/null 2>&1
    rm -rf "$LAB"
}
trap cleanup EXIT

if ! docker info >/dev/null 2>&1; then
    echo "  - skipped: docker is not available"
    echo "RESULT 0 0"
    exit 0
fi

export HM_SNAPSHOT_DIR="$LAB/snapshots"

mkdir -p "$LAB/project/config/docker"
cat > "$LAB/project/docker-compose.yml" <<'YAML'
services:
  db:
    image: mariadb:10.6
    environment:
      MYSQL_ROOT_PASSWORD: password
      MYSQL_DATABASE: magento
    volumes:
      - dbdata:/var/lib/mysql
  phpfpm:
    image: alpine:latest
    command: ["sleep", "600"]
volumes:
  dbdata:
YAML
cp "$LAB/project/docker-compose.yml" "$LAB/project/docker-compose.dev.mac.yml"
cp "$LAB/project/docker-compose.yml" "$LAB/project/docker-compose.dev.linux.yml"
printf '{"MAGENTO_DIR": ".", "DOMAIN": "life.local", "COMPOSE_PROJECT_NAME": "%s"}\n' "$PROJECT" \
    > "$LAB/project/config/docker/properties.json"

start_environment() {
    ( cd "$LAB/project" && docker compose -p "$PROJECT" up -d ) >/dev/null 2>&1
    local waited=0
    until ( cd "$LAB/project" && docker compose -p "$PROJECT" exec -T db \
                mariadb -uroot -ppassword magento -e "SELECT 1" ) >/dev/null 2>&1 ||
          [ "$waited" -gt 90 ]; do
        sleep 2
        waited=$((waited + 2))
    done
}

volume_exists() {
    docker volume inspect "${PROJECT}_dbdata" >/dev/null 2>&1
}

start_environment

if ! volume_exists; then
    echo "  - skipped: the environment never came up"
    echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
    exit 0
fi

# ---------------------------------------------------------------- down -v asks

test_case "cancelling leaves the volumes alone"
( cd "$LAB/project" && printf '3\n' | "$HM" down -v ) >/dev/null 2>&1
volume_exists && r=kept || r=destroyed
assert_equals "kept" "$r"

test_case "and the answer names the volumes it would delete"
output=$( cd "$LAB/project" && printf '3\n' | "$HM" down -v 2>&1 )
assert_contains "$output" "${PROJECT}_dbdata"

test_case "destroying without saving leaves no snapshot"
( cd "$LAB/project" && printf '2\n' | "$HM" down -v ) >/dev/null 2>&1
assert_empty "$(find "$HM_SNAPSHOT_DIR" -name '*.sql.gz' 2>/dev/null)"

test_case "and it really did destroy them"
volume_exists && r=kept || r=destroyed
assert_equals "destroyed" "$r"

# ---------------------------------------------------------------- saving first

start_environment

test_case "choosing to save leaves a snapshot behind"
( cd "$LAB/project" && printf '1\n' | "$HM" down -v ) >/dev/null 2>&1
snapshots=$(find "$HM_SNAPSHOT_DIR" -name '*.sql.gz' 2>/dev/null | grep -c . || true)
assert_equals "1" "$snapshots"

test_case "and destroys the environment afterwards"
volume_exists && r=kept || r=destroyed
assert_equals "destroyed" "$r"

test_case "the snapshot survived the destruction, which is the whole point"
hm_out=$( cd "$LAB/project" && "$HM" db list --json 2>/dev/null )
assert_equals "1" "$(printf '%s' "$hm_out" | jq '.data.snapshots | length')"

# ---------------------------------------------------------------- down without -v

start_environment

test_case "down without volumes does not ask at all"
output=$( cd "$LAB/project" && printf '' | "$HM" down 2>&1 )
assert_not_contains "$output" "What should happen"

test_case "and leaves the volumes where they were"
volume_exists && r=kept || r=destroyed
assert_equals "kept" "$r"

# ---------------------------------------------------------------- --yes

start_environment

test_case "with --yes it destroys without asking, as it always did"
( cd "$LAB/project" && "$HM" --yes down -v ) >/dev/null 2>&1
volume_exists && r=kept || r=destroyed
assert_equals "destroyed" "$r"

# ---------------------------------------------------------------- stop --snapshot

rm -f "$HM_SNAPSHOT_DIR/$PROJECT"/*.sql.gz 2>/dev/null
start_environment

test_case "stop on its own creates no snapshot"
( cd "$LAB/project" && "$HM" stop ) >/dev/null 2>&1
assert_empty "$(find "$HM_SNAPSHOT_DIR/$PROJECT" -name '*.sql.gz' 2>/dev/null)"

start_environment

test_case "stop --snapshot saves one first"
( cd "$LAB/project" && "$HM" stop --snapshot ) >/dev/null 2>&1
assert_equals "1" "$(find "$HM_SNAPSHOT_DIR/$PROJECT" -name '*.sql.gz' 2>/dev/null | grep -c . || true)"

test_case "and the environment ends up stopped"
running=$(docker ps -q --filter "label=com.docker.compose.project=$PROJECT" 2>/dev/null | grep -c . || true)
assert_equals "0" "$running"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
