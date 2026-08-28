#!/usr/bin/env bash
#
# The anonymisation state of an environment, through the commands that change it.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

HM="$COMMAND_BIN_DIR/bin/run"
LAB=$(cd "$(mktemp -d)" && pwd -P)
PROJECT="hm-anon-selftest"
trap 'rm -rf "$LAB"' EXIT

if ! docker info >/dev/null 2>&1; then
    echo "  - skipped: docker is not available"
    echo "RESULT 0 0"
    exit 0
fi

export HM_STATE_DIR="$LAB/state"

DIR="$LAB/$PROJECT"
mkdir -p "$DIR/config/docker"
cat > "$DIR/docker-compose.yml" <<'YAML'
services:
  phpfpm:
    image: alpine:latest
  db:
    image: mariadb:10.6
    environment:
      MYSQL_ROOT_PASSWORD: password
      MYSQL_DATABASE: magento
YAML
cp "$DIR/docker-compose.yml" "$DIR/docker-compose.dev.mac.yml"
cp "$DIR/docker-compose.yml" "$DIR/docker-compose.dev.linux.yml"
printf '{"MAGENTO_DIR": ".", "DOMAIN": "anon.test", "COMPOSE_PROJECT_NAME": "%s"}\n' "$PROJECT" \
    > "$DIR/config/docker/properties.json"

run() { ( cd "$DIR" && "$HM" "$@" >"$LAB/out" 2>"$LAB/err" ); STATUS=$?; STDOUT=$(cat "$LAB/out"); STDERR=$(cat "$LAB/err"); return 0; }

# ---------------------------------------------------------------- what describe says

run describe --json
test_case "a project nobody anonymised says so"
assert_equals "unknown" "$(printf '%s' "$STDOUT" | jq -r '.data.data.anonymised')"

mkdir -p "$LAB/state"
printf '{"anonymised_at": "2026-08-28 09:00"}\n' > "$LAB/state/$PROJECT.json"

run describe --json
test_case "and one that was anonymised carries the date"
assert_equals "yes" "$(printf '%s' "$STDOUT" | jq -r '.data.data.anonymised')"
assert_equals "2026-08-28 09:00" "$(printf '%s' "$STDOUT" | jq -r '.data.data.anonymised_at')"

# ---------------------------------------------------------------- what the agent reads

run ai-context
test_case "the context says the data is anonymised"
assert_contains "$(cat "$DIR/AGENTS.md")" "Anonymised on 2026-08-28"

printf '{}\n' > "$LAB/state/$PROJECT.json"
run ai-context
test_case "and says the opposite in words when it is not"
assert_contains "$(cat "$DIR/AGENTS.md")" "has not been anonymised"
assert_contains "$(cat "$DIR/AGENTS.md")" "real personal data"

test_case "changing it makes the context stale, so it is regenerated"
printf '{"anonymised_at": "2026-08-28 09:00"}\n' > "$LAB/state/$PROJECT.json"
run doctor --json
assert_equals "error" \
    "$(printf '%s' "$STDOUT" | jq -r '.data.checks[] | select(.message | test("agent context")) | .severity' | head -1)"

# ---------------------------------------------------------------- what clears it

run mysql -i "$LAB/nada.sql"
test_case "an import of a file that does not exist changes nothing"
assert_equals "yes" "$(jq -r 'if .anonymised_at then "yes" else "no" end' "$LAB/state/$PROJECT.json")"

( cd "$DIR" && docker compose -p "$PROJECT" up -d ) >/dev/null 2>&1
waited=0
until ( cd "$DIR" && docker compose -p "$PROJECT" exec -T db mariadb -uroot -ppassword magento -e "SELECT 1" ) >/dev/null 2>&1 || [ "$waited" -gt 120 ]; do
    sleep 2; waited=$((waited + 2))
done

cleanup_containers() { ( cd "$DIR" && docker compose -p "$PROJECT" down -v --remove-orphans ) >/dev/null 2>&1; }
trap 'cleanup_containers; rm -rf "$LAB"' EXIT

printf 'CREATE TABLE prueba (id INT);\n' > "$LAB/dump.sql"
run mysql -i "$LAB/dump.sql"

test_case "importing a dump clears it, because nobody anonymised what it brought in"
assert_equals "no" "$(jq -r 'if .anonymised_at then "yes" else "no" end' "$LAB/state/$PROJECT.json")"

test_case "the doctor does not complain about a project that is not an agent's"
run doctor --json
assert_equals "ok" \
    "$(printf '%s' "$STDOUT" | jq -r '.data.checks[] | select(.message | test("anonymis")) | .severity' | head -1)"

test_case "and does complain when the environment is an agent's"
( cd "$DIR" && HM_PROFILE=agent "$HM" doctor --json >"$LAB/out" 2>"$LAB/err" )
assert_equals "error" \
    "$(jq -r '.data.checks[] | select(.message | test("anonymis")) | .severity' "$LAB/out" | head -1)"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
