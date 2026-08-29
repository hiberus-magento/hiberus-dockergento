#!/usr/bin/env bash
#
# Frozen data directories, against a real MariaDB.
#
# The claim being tested is the one that makes the feature worth having: a database that exists
# in one project can be standing in another without an import, and the rows are there.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

HM="$COMMAND_BIN_DIR/bin/run"
LAB=$(cd "$(mktemp -d)" && pwd -P)
SOURCE="hm-template-selftest"
TARGET="hm-template-target"
OTHER="hm-template-otherversion"

#
# Removes the containers and volumes by name as well as through Compose. A run that is
# interrupted leaves the data volumes behind, and the next run then finds a target that already
# has data: the clone asks for confirmation, nobody answers, and it returns successfully having
# done nothing — which is a confusing way to fail a test about something else entirely.
#
cleanup() {
    local project container volume
    for project in "$SOURCE" "$TARGET" "$OTHER"; do
        ( cd "$LAB/$project" 2>/dev/null && docker compose -p "$project" down -v --remove-orphans ) >/dev/null 2>&1

        for container in $(docker ps -aq --filter "label=com.docker.compose.project=$project" 2>/dev/null); do
            docker rm -f "$container" >/dev/null 2>&1
        done

        for volume in $(docker volume ls -q 2>/dev/null | grep -E "^(${project}_|hm-template-${project}-)" || true); do
            docker volume rm -f "$volume" >/dev/null 2>&1
        done
    done
    rm -rf "$LAB"
}
trap cleanup EXIT

# Anything a previous interrupted run left behind, before this one starts believing it
cleanup_leftovers() {
    local project container volume
    for project in "$SOURCE" "$TARGET" "$OTHER"; do
        for container in $(docker ps -aq --filter "label=com.docker.compose.project=$project" 2>/dev/null); do
            docker rm -f "$container" >/dev/null 2>&1
        done

        for volume in $(docker volume ls -q 2>/dev/null | grep -E "^(${project}_|hm-template-${project}-)" || true); do
            docker volume rm -f "$volume" >/dev/null 2>&1
        done
    done
}

if ! docker info >/dev/null 2>&1; then
    echo "  - skipped: docker is not available"
    echo "RESULT 0 0"
    exit 0
fi

make_project() {
    local name="$1" image="$2" dir="$LAB/$1"
    mkdir -p "$dir/config/docker"
    cat > "$dir/docker-compose.yml" <<YAML
services:
  db:
    image: $image
    environment:
      MYSQL_ROOT_PASSWORD: password
      MYSQL_DATABASE: magento
      MYSQL_USER: magento
      MYSQL_PASSWORD: magento
    volumes:
      - dbdata:/var/lib/mysql
volumes:
  dbdata:
YAML
    cp "$dir/docker-compose.yml" "$dir/docker-compose.dev.mac.yml"
    cp "$dir/docker-compose.yml" "$dir/docker-compose.dev.linux.yml"
    printf '{"MAGENTO_DIR": ".", "DOMAIN": "tmpl.local", "COMPOSE_PROJECT_NAME": "%s"}\n' "$name" \
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

wait_for_db() {
    local waited=0
    until query "$1" "SELECT 1" >/dev/null 2>&1 || [ "$waited" -gt 120 ]; do
        sleep 2
        waited=$((waited + 2))
    done
    query "$1" "SELECT 1" >/dev/null 2>&1
}

# The anonymisation state is per project and lives outside the checkout; a test has no business
# writing in the developer's
export HM_STATE_DIR="$LAB/state"

cleanup_leftovers

make_project "$SOURCE" "mariadb:10.6"
( cd "$LAB/$SOURCE" && docker compose -p "$SOURCE" up -d ) >/dev/null 2>&1

if ! wait_for_db "$SOURCE"; then
    echo "  - skipped: the database never became reachable"
    echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
    exit 0
fi

query "$SOURCE" "CREATE TABLE proof (id INT PRIMARY KEY, note VARCHAR(64))" >/dev/null
query "$SOURCE" "INSERT INTO proof VALUES (1, 'frozen and cloned')" >/dev/null

# --------------------------------------------------------------------------- freeze

hm_in "$SOURCE" db freeze --name=base
assert_equals "0" "$STATUS" "freezing a running project succeeds"
assert_contains "$STDOUT" "$SOURCE/base" "it reports the full address of the template"

assert_equals "0" \
    "$(docker volume inspect "hm-template-$SOURCE-base" >/dev/null 2>&1; echo $?)" \
    "the template volume exists"

assert_equals "mariadb:10.6" \
    "$(docker volume inspect "hm-template-$SOURCE-base" --format '{{index .Labels "hm.db_image"}}')" \
    "the template records the image it was made with"

assert_equals "running" \
    "$( cd "$LAB/$SOURCE" && docker compose -p "$SOURCE" ps --format '{{.State}}' db )" \
    "the database is running again after the copy"

wait_for_db "$SOURCE"
assert_equals "frozen and cloned" "$(query "$SOURCE" "SELECT note FROM proof")" \
    "the source database still answers after being frozen"

hm_in "$SOURCE" db freeze --name=base
assert_equals "2" "$STATUS" "a second freeze under the same name is refused"
assert_contains "$STDERR" "already a template" "it says the name is taken"

hm_in "$SOURCE" db templates --json
assert_equals "0" "$STATUS" "templates can be listed"
assert_equals "$SOURCE/base" \
    "$(printf '%s' "$STDOUT" | jq -r --arg a "$SOURCE/base" \
        '.data.templates[] | select(.address == $a) | .address')" \
    "the template is listed under its address"

# --------------------------------------------------------------------------- clone

hm_in "$SOURCE" db clone base
assert_equals "6" "$STATUS" "cloning into a running environment is refused"
assert_contains "$STDERR" "running" "it says the environment is running"

make_project "$TARGET" "mariadb:10.6"

#
# Whatever that template holds, nobody anonymised it afterwards: the record has to expire when
# the data is replaced, or somebody relies on a reassuring "yes" from before an import
#
mkdir -p "$LAB/state"
printf '{"anonymised_at": "2026-01-01 00:00"}\n' > "$LAB/state/$TARGET.json"

hm_in "$TARGET" db clone "$SOURCE/base"
assert_equals "0" "$STATUS" "a template of another project can be cloned by address"

assert_equals "" "$(jq -r '.anonymised_at // ""' "$LAB/state/$TARGET.json")" \
    "and cloning clears the anonymisation record"

( cd "$LAB/$TARGET" && docker compose -p "$TARGET" up -d ) >/dev/null 2>&1

if wait_for_db "$TARGET"; then
    assert_equals "frozen and cloned" "$(query "$TARGET" "SELECT note FROM proof")" \
        "the cloned environment has the data of the template"
else
    assert_equals "reachable" "unreachable" "the cloned database starts"
fi

hm_in "$TARGET" db clone "$SOURCE/nothing-like-this"
assert_equals "2" "$STATUS" "an unknown template is a usage error"
assert_contains "$STDERR" "no template" "it says the template does not exist"

# --------------------------------------------------------------------------- versions

make_project "$OTHER" "mariadb:10.4"

hm_in "$OTHER" db clone "$SOURCE/base"
assert_equals "6" "$STATUS" "a template from another server version is refused"
assert_contains "$STDERR" "10.4" "it names the version this project runs"

# --------------------------------------------------------------------------- drop

hm_in "$SOURCE" db drop "$SOURCE/base"
assert_equals "0" "$STATUS" "an unconfirmed drop is not an error"
assert_equals "0" \
    "$(docker volume inspect "hm-template-$SOURCE-base" >/dev/null 2>&1; echo $?)" \
    "nothing is deleted without confirmation"

hm_in "$SOURCE" db drop "$SOURCE/base" --force
assert_equals "0" "$STATUS" "a template can be dropped"
assert_equals "1" \
    "$(docker volume inspect "hm-template-$SOURCE-base" >/dev/null 2>&1; echo $?)" \
    "the volume is gone after dropping it"

hm_in "$SOURCE" db drop "$SOURCE/base" --force
assert_equals "2" "$STATUS" "dropping what does not exist is a usage error"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
