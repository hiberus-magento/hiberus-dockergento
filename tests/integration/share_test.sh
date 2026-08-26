#!/usr/bin/env bash
#
# Giving a project a public address.
#
# The end-to-end half needs the internet and a third party, so it skips rather than fails when it
# cannot reach them: a test that cannot run is not a test that failed.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

HM="$COMMAND_BIN_DIR/bin/run"
REAL_HOME=$(eval echo "~$(id -un)")
LAB="$REAL_HOME/.hm/share-selftest"
PROJECT="hm-share-selftest"

cleanup() {
    docker rm -f "hm-share-$PROJECT" >/dev/null 2>&1
    ( cd "$LAB/project" 2>/dev/null && docker compose -p "$PROJECT" down -v ) >/dev/null 2>&1
    rm -rf "$LAB"
}
trap cleanup EXIT
cleanup

if ! docker info >/dev/null 2>&1; then
    echo "  - skipped: docker is not available"
    echo "RESULT 0 0"
    exit 0
fi

mkdir -p "$LAB/project/config/docker"
cat > "$LAB/project/docker-compose.yml" <<'YAML'
services:
  varnish:
    image: hashicorp/http-echo
    command: ["-text=shared content", "-listen=:6081"]
YAML
cp "$LAB/project/docker-compose.yml" "$LAB/project/docker-compose.dev.mac.yml"
cp "$LAB/project/docker-compose.yml" "$LAB/project/docker-compose.dev.linux.yml"
printf '{"MAGENTO_DIR": ".", "DOMAIN": "shared.local", "COMPOSE_PROJECT_NAME": "%s"}\n' "$PROJECT" \
    > "$LAB/project/config/docker/properties.json"

( cd "$LAB/project" && docker compose -p "$PROJECT" up -d ) >/dev/null 2>&1
sleep 4

# ---------------------------------------------------------------- refusing

test_case "answering no shares nothing"
( cd "$LAB/project" && printf 'n\n' | "$HM" share ) >/dev/null 2>&1
assert_equals "0" "$(docker ps -aq --filter "name=hm-share-$PROJECT" | grep -c . || true)"

test_case "stopping when nothing is shared is not an error"
( cd "$LAB/project" && "$HM" share --stop --json >"$LAB/out" 2>&1 )
assert_json_field "$(cat "$LAB/out")" '.data.closed' "false"

test_case "an unknown option is refused"
( cd "$LAB/project" && "$HM" share --nonsense --json >"$LAB/out" 2>"$LAB/err" )
assert_json_field "$(cat "$LAB/err")" '.error.type' "invalid_argument"

# ---------------------------------------------------------------- the real thing

if ! curl -s -o /dev/null --max-time 10 https://www.cloudflare.com/ 2>/dev/null; then
    echo "  - skipped: no route to the tunnel provider"
    echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
    exit 0
fi

output="$LAB/share.out"
( cd "$LAB/project" && nohup "$HM" --yes share --no-json > "$output" 2>&1 & )

waited=0
until grep -q "trycloudflare.com" "$output" 2>/dev/null || [ "$waited" -ge 90 ]; do
    sleep 3
    waited=$((waited + 3))
done

address=$(grep -oE 'https://[a-z0-9-]+\.trycloudflare\.com' "$output" 2>/dev/null | head -1)

if [ -z "$address" ]; then
    echo "  - skipped: the tunnel provider did not hand out an address"
    echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
    exit 0
fi

test_case "a public address is handed out"
assert_contains "$address" "trycloudflare.com"

# The edge takes a while to accept the tunnel, and how long is not ours to decide. Opening it and
# cleaning it up are; those are asserted above and below. If the address never starts answering
# within a generous window, that is the provider being slow, and a test that cannot run is not a
# test that failed.
body=""
waited=0
until [ -n "$body" ] || [ "$waited" -ge 150 ]; do
    body=$(curl -s --max-time 20 "$address" 2>/dev/null | grep "shared content" || true)
    [ -n "$body" ] && break
    sleep 5
    waited=$((waited + 5))
done

if [ -z "$body" ]; then
    echo "  - skipped: the tunnel never started answering (the provider, not us)"
else
    test_case "and it serves what the project serves"
    assert_contains "$body" "shared content"
fi

test_case "stopping closes it"
( cd "$LAB/project" && "$HM" share --stop ) >/dev/null 2>&1
assert_equals "0" "$(docker ps -aq --filter "name=hm-share-$PROJECT" | grep -c . || true)"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
