#!/usr/bin/env bash
#
# Snapshots against every database image the tool can configure.
#
# Projects run anything from MariaDB 10.2 to 12.3, and the two names for every tool changed along
# the way: `mysqldump` became `mariadb-dump`, `mysql` became `mariadb`. A snapshot that only works
# on the newest image is worse than none, because it fails on the projects that have been running
# longest.
#
# By default this runs against the images already on the machine, so a normal run stays fast. Set
# HM_TEST_DB_MATRIX=1 to pull and check every version in requirements.json.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

LAB=$(mktemp -d)
CONTAINER=""
cleanup() {
    [ -n "$CONTAINER" ] && docker rm -f "$CONTAINER" >/dev/null 2>&1
    rm -rf "$LAB"
}
trap cleanup EXIT

if ! docker info >/dev/null 2>&1; then
    echo "  - skipped: docker is not available"
    echo "RESULT 0 0"
    exit 0
fi

# The same resolution the command uses, and the reason it works across the rename
DUMP='dump=$(command -v mariadb-dump || command -v mysqldump)'
CLIENT='client=$(command -v mariadb || command -v mysql)'

# A value with a colon is a full image reference; a bare one is a tag under hiberusmagento.
# (Written as a function because bash 3.2 mis-parses a `case` inside a command substitution.)
qualify_image() {
    case "$1" in
        *:*) printf '%s\n' "$1" ;;
        *)   printf 'hiberusmagento/mariadb:%s\n' "$1" ;;
    esac
}

images=""
while IFS= read -r value; do
    images="$images$(qualify_image "$value")
"
done <<< "$(jq -r '[.[] | .mariadb] | unique[]' "$DATA_DIR/requirements.json")"
images=$(printf '%s' "$images" | sed '/^$/d' | sort -u)

client_in() {
    docker exec "$CONTAINER" sh -c "$CLIENT"'; "$client" -uroot -ppassword '"$1" 2>/dev/null
}

for image in $images; do
    if ! docker image inspect "$image" >/dev/null 2>&1; then
        if [ -z "${HM_TEST_DB_MATRIX:-}" ]; then
            echo "  - skipped $image (set HM_TEST_DB_MATRIX=1 to pull it)"
            continue
        fi
        docker pull "$image" >/dev/null 2>&1 || {
            echo "  - skipped $image (could not be pulled)"
            continue
        }
    fi

    CONTAINER="hm-matrix-$$"
    docker rm -f "$CONTAINER" >/dev/null 2>&1
    docker run -d --name "$CONTAINER" \
        -e MYSQL_ROOT_PASSWORD=password -e MYSQL_DATABASE=magento \
        "$image" >/dev/null 2>&1

    waited=0
    until client_in '-e "SELECT 1"' >/dev/null 2>&1 || [ "$waited" -gt 90 ]; do
        sleep 2
        waited=$((waited + 2))
    done

    if ! client_in '-e "SELECT 1"' >/dev/null 2>&1; then
        echo "  - skipped $image (never became reachable)"
        docker rm -f "$CONTAINER" >/dev/null 2>&1
        CONTAINER=""
        continue
    fi

    # What a Magento database actually contains: tables, a trigger and a routine. A single
    # statement body needs no DELIMITER, which the client would not understand through -e.
    client_in 'magento -e "
        CREATE TABLE orders (id INT PRIMARY KEY, total INT);
        INSERT INTO orders VALUES (1, 100);
        CREATE TRIGGER t_orders BEFORE INSERT ON orders FOR EACH ROW SET NEW.total = NEW.total;
        CREATE PROCEDURE p_noop() SELECT 1;"' >/dev/null

    # Exactly the flags `hm db snapshot` uses
    docker exec "$CONTAINER" sh -c "$DUMP"'; "$dump" --single-transaction --quick --no-tablespaces \
        --routines --triggers --events -uroot -ppassword magento' > "$LAB/dump.sql" 2>"$LAB/err"

    test_case "$image accepts the snapshot options"
    assert_empty "$(grep -i 'unknown option\|unknown variable' "$LAB/err" || true)"

    test_case "$image produces a usable dump"
    [ -s "$LAB/dump.sql" ] && r=produced || r=empty
    assert_equals "produced" "$r"

    # Change it, then restore the way the command does
    client_in 'magento -e "UPDATE orders SET total = 999; CREATE TABLE afterwards (id INT);"' >/dev/null
    # No backticks: this string reaches `sh -c`, where they would be command substitution and
    # would quietly turn the statement into nonsense instead of dropping anything.
    client_in '-e "DROP DATABASE IF EXISTS magento; CREATE DATABASE magento;"' >/dev/null
    docker exec -i "$CONTAINER" sh -c "$CLIENT"'; "$client" -uroot -ppassword magento' \
        < "$LAB/dump.sql" 2>/dev/null

    test_case "$image restores the data"
    assert_equals "100" "$(client_in 'magento -N -B -e "SELECT total FROM orders WHERE id = 1"' | tr -d '\r')"

    test_case "$image restores routines and triggers, not just tables"
    routines=$(client_in 'magento -N -B -e "SELECT COUNT(*) FROM information_schema.routines WHERE routine_schema = \"magento\""' | tr -d '\r')
    triggers=$(client_in 'magento -N -B -e "SELECT COUNT(*) FROM information_schema.triggers WHERE trigger_schema = \"magento\""' | tr -d '\r')
    assert_equals "1 1" "${routines:-0} ${triggers:-0}"

    test_case "$image leaves nothing behind from after the snapshot"
    assert_empty "$(client_in 'magento -N -B -e "SHOW TABLES LIKE \"afterwards\""' | tr -d '\r')"

    docker rm -f "$CONTAINER" >/dev/null 2>&1
    CONTAINER=""
done

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
