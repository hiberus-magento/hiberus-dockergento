#!/usr/bin/env bash
#
# Named database copies, against a real MariaDB.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

HM="$COMMAND_BIN_DIR/bin/run"
LAB=$(cd "$(mktemp -d)" && pwd -P)
PROJECT="hm-snapshot-selftest"
OTHER="hm-snapshot-other"

cleanup() {
    for project in "$PROJECT" "$OTHER"; do
        ( cd "$LAB/$project" 2>/dev/null && docker compose -p "$project" down -v --remove-orphans ) >/dev/null 2>&1
    done
    rm -rf "$LAB"
}
trap cleanup EXIT

if ! docker info >/dev/null 2>&1; then
    echo "  - skipped: docker is not available"
    echo "RESULT 0 0"
    exit 0
fi

# Snapshots go to a throwaway root: a test run has no business leaving copies in the developer's
export HM_SNAPSHOT_DIR="$LAB/snapshots"

make_project() {
    local name="$1" dir="$LAB/$1"
    mkdir -p "$dir/config/docker"
    cat > "$dir/docker-compose.yml" <<'YAML'
services:
  db:
    image: mariadb:10.6
    environment:
      MYSQL_ROOT_PASSWORD: password
      MYSQL_DATABASE: magento
      MYSQL_USER: magento
      MYSQL_PASSWORD: magento
  phpfpm:
    image: alpine:latest
    command: ["sleep", "600"]
YAML
    cp "$dir/docker-compose.yml" "$dir/docker-compose.dev.mac.yml"
    cp "$dir/docker-compose.yml" "$dir/docker-compose.dev.linux.yml"
    printf '{"MAGENTO_DIR": ".", "DOMAIN": "snap.local", "COMPOSE_PROJECT_NAME": "%s"}\n' "$name" \
        > "$dir/config/docker/properties.json"
}

hm_in() {
    local project="$1"; shift
    ( cd "$LAB/$project" && "$HM" "$@" >"$LAB/out" 2>"$LAB/err" )
    STATUS=$?
    STDOUT=$(cat "$LAB/out")
    STDERR=$(cat "$LAB/err")
    return 0
}

query() {
    ( cd "$LAB/$1" && docker compose -p "$1" exec -T db \
        mariadb -uroot -ppassword magento -N -B -e "$2" 2>/dev/null )
}

make_project "$PROJECT"
( cd "$LAB/$PROJECT" && docker compose -p "$PROJECT" up -d ) >/dev/null 2>&1

waited=0
until query "$PROJECT" "SELECT 1" >/dev/null 2>&1 || [ "$waited" -gt 90 ]; do
    sleep 2
    waited=$((waited + 2))
done

if ! query "$PROJECT" "SELECT 1" >/dev/null 2>&1; then
    echo "  - skipped: the database never became reachable"
    echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
    exit 0
fi

query "$PROJECT" "CREATE TABLE orders (id INT PRIMARY KEY, total INT); INSERT INTO orders VALUES (1, 100);" >/dev/null

# ---------------------------------------------------------------- snapshot

test_case "a snapshot is taken"
hm_in "$PROJECT" db snapshot --name=before --json
assert_equals "0" "$STATUS"

test_case "and it reports what it saved"
assert_json_field "$STDOUT" '.data.name' "before"

test_case "the database is untouched by having been copied"
assert_equals "100" "$(query "$PROJECT" "SELECT total FROM orders WHERE id = 1")"

test_case "nothing was written inside the project"
assert_empty "$(find "$LAB/$PROJECT" -type f \( -name '*.sql' -o -name '*.sql.gz' -o -name '*.gz' \) 2>/dev/null)"

test_case "taking it again under the same name is refused"
hm_in "$PROJECT" db snapshot --name=before --json
assert_json_field "$STDERR" '.error.type' "snapshot_exists"

test_case "unless told to overwrite"
hm_in "$PROJECT" --force db snapshot --name=before --json
assert_equals "0" "$STATUS"

test_case "a snapshot with no name is named after the date"
hm_in "$PROJECT" db snapshot --json
name=$(printf '%s' "$STDOUT" | jq -r '.data.name')
case "$name" in
    20*-*) r=dated ;;
    *)     r="$name" ;;
esac
assert_equals "dated" "$r"

test_case "an unusable name is refused"
hm_in "$PROJECT" db snapshot --name="../escape" --json
assert_json_field "$STDERR" '.error.type' "invalid_name"

# ---------------------------------------------------------------- list

test_case "the snapshots are listed"
hm_in "$PROJECT" db list --json
assert_contains "$(printf '%s' "$STDOUT" | jq -r '.data.snapshots[].name')" "before"

test_case "with their size"
assert_equals "true" "$(printf '%s' "$STDOUT" | jq '.data.snapshots | all(.size != "" and .size != null)')"

test_case "and when they were taken"
assert_equals "true" "$(printf '%s' "$STDOUT" | jq '.data.snapshots | all(.taken_at | test("^20"))')"

test_case "another project's snapshots are not listed here"
make_project "$OTHER"
mkdir -p "$HM_SNAPSHOT_DIR/$OTHER"
touch "$HM_SNAPSHOT_DIR/$OTHER/foreign.sql.gz"
hm_in "$PROJECT" db list --json
assert_not_contains "$(printf '%s' "$STDOUT" | jq -r '.data.snapshots[].name')" "foreign"

# ---------------------------------------------------------------- restore

query "$PROJECT" "UPDATE orders SET total = 999 WHERE id = 1; CREATE TABLE afterwards (id INT);" >/dev/null

test_case "the database really did change"
assert_equals "999" "$(query "$PROJECT" "SELECT total FROM orders WHERE id = 1")"

test_case "restoring without confirming changes nothing"
( cd "$LAB/$PROJECT" && printf 'no\n' | "$HM" db restore before >/dev/null 2>&1 )
assert_equals "999" "$(query "$PROJECT" "SELECT total FROM orders WHERE id = 1")"

test_case "restoring brings the data back"
hm_in "$PROJECT" --yes db restore before
assert_equals "100" "$(query "$PROJECT" "SELECT total FROM orders WHERE id = 1")"

test_case "and removes what was created after the snapshot"
assert_empty "$(query "$PROJECT" "SHOW TABLES LIKE 'afterwards'")"

test_case "restoring a snapshot that does not exist is refused"
hm_in "$PROJECT" --yes db restore nonexistent --json
assert_json_field "$STDERR" '.error.type' "unknown_snapshot"

# ---------------------------------------------------------------- survival

test_case "snapshots survive the environment being destroyed"
( cd "$LAB/$PROJECT" && "$HM" down -v ) >/dev/null 2>&1
hm_in "$PROJECT" db list --json
assert_contains "$(printf '%s' "$STDOUT" | jq -r '.data.snapshots[].name')" "before"

# ---------------------------------------------------------------- remove

test_case "a snapshot can be removed"
hm_in "$PROJECT" db remove before --json
assert_equals "0" "$STATUS"

test_case "and stops being listed"
hm_in "$PROJECT" db list --json
assert_not_contains "$(printf '%s' "$STDOUT" | jq -r '.data.snapshots[].name')" "before"

test_case "removing one that does not exist is refused"
hm_in "$PROJECT" db remove before --json
assert_json_field "$STDERR" '.error.type' "unknown_snapshot"

# ---------------------------------------------------------------- clear

# Clearing needs no database — it deletes files — and by this point the environment has been
# destroyed on purpose by the survival check above.
seed_snapshots() {
    mkdir -p "$HM_SNAPSHOT_DIR/$PROJECT"
    for name in "$@"; do
        printf 'a snapshot\n' | gzip > "$HM_SNAPSHOT_DIR/$PROJECT/$name.sql.gz"
    done
}

test_case "clearing asks before deleting anything"
rm -f "$HM_SNAPSHOT_DIR/$PROJECT"/*.sql.gz 2>/dev/null
seed_snapshots one two
( cd "$LAB/$PROJECT" && printf 'no\n' | "$HM" db clear >/dev/null 2>&1 )
hm_in "$PROJECT" db list --json
assert_equals "2" "$(printf '%s' "$STDOUT" | jq '.data.snapshots | length')"

test_case "confirming clears this project's snapshots"
( cd "$LAB/$PROJECT" && printf '%s\n' "$PROJECT" | "$HM" db clear >/dev/null 2>&1 )
hm_in "$PROJECT" db list --json
assert_equals "0" "$(printf '%s' "$STDOUT" | jq '.data.snapshots | length')"

test_case "and leaves another project's alone"
[ -f "$HM_SNAPSHOT_DIR/$OTHER/foreign.sql.gz" ] && r=kept || r=deleted
assert_equals "kept" "$r"

test_case "clearing everything asks too"
seed_snapshots one
( cd "$LAB/$PROJECT" && printf 'no\n' | "$HM" db clear --all >/dev/null 2>&1 )
[ -f "$HM_SNAPSHOT_DIR/$OTHER/foreign.sql.gz" ] && r=kept || r=deleted
assert_equals "kept" "$r"

test_case "and confirming it reaches every project"
( cd "$LAB/$PROJECT" && printf 'all\n' | "$HM" db clear --all >/dev/null 2>&1 )
assert_empty "$(find "$HM_SNAPSHOT_DIR" -name '*.sql.gz' 2>/dev/null)"

test_case "clearing nothing says so instead of failing"
hm_in "$PROJECT" db clear --json
assert_equals "0" "$(printf '%s' "$STDOUT" | jq -r '.data.removed')"

test_case "an unknown option to clear is a usage error"
hm_in "$PROJECT" db clear --everything --json
assert_json_field "$STDERR" '.error.type' "invalid_argument"

test_case "an unknown subcommand is a usage error"
hm_in "$PROJECT" db explode --json
assert_json_field "$STDERR" '.error.type' "unknown_subcommand"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
