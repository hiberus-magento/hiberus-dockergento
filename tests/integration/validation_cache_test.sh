#!/usr/bin/env bash
#
# The validation cache must never hide a broken configuration.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

HM="$COMMAND_BIN_DIR/bin/run"
LAB=$(cd "$(mktemp -d)" && pwd -P)
export HM_CACHE_DIR="$LAB/cache"
trap 'rm -rf "$LAB"' EXIT

if ! docker info >/dev/null 2>&1; then
    echo "  - skipped: docker is not available"
    echo "RESULT 0 0"
    exit 0
fi

mkdir -p "$LAB/project/config/docker"
cd "$LAB/project" || exit 1

cat > docker-compose.yml <<'YAML'
services:
  phpfpm:
    image: alpine:latest
YAML
cp docker-compose.yml docker-compose.dev.mac.yml
cp docker-compose.yml docker-compose.dev.linux.yml
echo '{"MAGENTO_DIR": ".", "COMPOSE_PROJECT_NAME": "hm-cache-selftest", "DOMAIN": "cache.local"}' \
    > config/docker/properties.json

run() {
    ( cd "$LAB/project" && "$HM" "$@" >"$LAB/out" 2>"$LAB/err" )
    STATUS=$?
    return 0
}

test_case "a valid configuration is accepted"
run describe --json
assert_equals "0" "$STATUS"

test_case "the result is cached"
[ -n "$(ls -A "$HM_CACHE_DIR" 2>/dev/null)" ] && r=yes || r=no
assert_equals "yes" "$r"

test_case "the cached result is reused"
run describe --json
assert_equals "0" "$STATUS"

test_case "a configuration broken after a successful validation is still detected"
echo "this: is: not: valid: yaml:" >> docker-compose.yml
run describe --json
assert_equals "4" "$STATUS"

test_case "a command that does not need the project still works"
run list --json
assert_equals "0" "$STATUS"

test_case "and the broken configuration keeps being detected"
run describe --json
assert_equals "4" "$STATUS"

test_case "fixing it makes the command work again"
cat > docker-compose.yml <<'YAML'
services:
  phpfpm:
    image: alpine:latest
YAML
run describe --json
assert_equals "0" "$STATUS"

test_case "the cache leaves nothing inside the project"
[ -z "$(ls -A config/docker | grep -v properties.json)" ] && r=clean || r="$(ls -A config/docker)"
assert_equals "clean" "$r"

test_case "an unreadable cache entry is treated as absent"
find "$HM_CACHE_DIR" -type f -exec sh -c 'printf "garbage" > "$1"' _ {} \;
run describe --json
assert_equals "0" "$STATUS"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
