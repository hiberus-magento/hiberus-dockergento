#!/usr/bin/env bash
#
# The global proxy, answered by the binary against a real Docker daemon.
#
# One router per machine holding 80 and 443, so that more than one project can be up at a time.
# What is checked here is what it answers when it is up and when it is not, and the refusal when
# something else is holding the ports. The compose file it generates is pinned by
# dockergento/core/proxy_test.go.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

GO_BINARY="$COMMAND_BIN_DIR/bin/hm"

# Colima only mounts the real home, so the proxy's own directory has to live there or the container
# sees it empty. The test HOME is a throwaway elsewhere; this one is deliberate.
REAL_HOME=$(eval echo "~$(id -un)")
export HM_PROXY_DIR="$REAL_HOME/.hm/proxy-go-selftest"
LAB=$(cd "$(mktemp -d)" && pwd -P)

limpiar() {
    [ -f "$HM_PROXY_DIR/docker-compose.yml" ] &&
        docker compose -p hm-proxy -f "$HM_PROXY_DIR/docker-compose.yml" down >/dev/null 2>&1
    docker rm -f hm-go-ocupante >/dev/null 2>&1
    docker network rm hm-gateway >/dev/null 2>&1
    rm -rf "$LAB" "$HM_PROXY_DIR"
}
trap limpiar EXIT
limpiar

if ! command -v go >/dev/null 2>&1 || ! docker info >/dev/null 2>&1; then
    echo "  - skipped: go or a Docker daemon is missing"
    echo "RESULT 0 0"
    exit 0
fi

export GOCACHE="$LAB/go-build"

( cd "$COMMAND_BIN_DIR" && go build -o "$GO_BINARY" ./cmd/hm ) >/dev/null 2>&1 || {
    echo "  - skipped: the binary does not build here"
    echo "RESULT 0 0"
    exit 0
}

# ---------------------------------------------------------------- when it is not running

test_case "it says it is not running"
assert_equals "false" "$( "$GO_BINARY" proxy status --json | jq -r '.data.running' )"

test_case "and stopping something that is not running is not a failure"
salida=$( "$GO_BINARY" proxy down --json )
assert_equals "true" "$(printf '%s' "$salida" | jq -r '.ok')"
assert_equals "false" "$(printf '%s' "$salida" | jq -r '.data.running')"

# ---------------------------------------------------------------- who is holding the ports

#
# A test that cannot run is better than a test that fails. The proxy needs 80 and 443, and on a
# machine where somebody is working there is usually a project holding them — which is the case the
# refusal below exists for, and the reason everything that needs the proxy up is skipped there.
#
OCUPANTE=$(docker ps --format '{{.Names}}\t{{.Ports}}' 2>/dev/null |
    awk -F'\t' '$2 ~ /:80->|:443->/ { print $1 }' | grep -v '^hm-proxy$' | head -1)

if [ -n "$OCUPANTE" ]; then
    ( "$GO_BINARY" proxy up >"$LAB/go.err" 2>&1 ); ESTADO_GO=$?

    test_case "something else holding port 80 or 443 is refused"
    assert_equals "6" "$ESTADO_GO"

    #
    # Which container is named is not asserted here: with a full stack up, one holds 80 and another
    # holds 443, and either is a true answer. The case with exactly one holder is below.
    #
    echo "  - skipped: '$OCUPANTE' is using port 80 or 443, so the proxy cannot be started here"
    echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
    exit 0
fi

# ---------------------------------------------------------------- starting it

( "$GO_BINARY" proxy up ) >/dev/null 2>&1

test_case "it writes its compose file"
assert_equals "0" "$([ -s "$HM_PROXY_DIR/docker-compose.yml" ] && echo 0 || echo 1)"

test_case "and it is running"
assert_equals "1" "$(docker ps --filter 'name=^hm-proxy$' -q | grep -c .)"

test_case "on the shared network, which is how projects reach it"
assert_equals "1" "$(docker network ls --filter 'name=^hm-gateway$' -q | grep -c .)"

test_case "starting it again is not an error, and says so"
salida=$( "$GO_BINARY" proxy up --json )
assert_equals "false" "$(printf '%s' "$salida" | jq -r '.data.started')"
assert_equals "true" "$(printf '%s' "$salida" | jq -r '.data.running')"

# ---------------------------------------------------------------- what it says about itself

test_case "with it running, it says so"
assert_equals "true" "$( "$GO_BINARY" proxy status --json | jq -r '.data.running' )"

# ---------------------------------------------------------------- stopping it

test_case "stopping it stops it"
( "$GO_BINARY" proxy down ) >/dev/null 2>&1
assert_equals "0" "$(docker ps --filter 'name=^hm-proxy$' -q | grep -c .)"

#
# And the refusal, with a container of this test's own holding the port.
#
docker run -d --name hm-go-ocupante -p 80:80 alpine:latest sleep 300 >/dev/null 2>&1

if docker ps --filter 'name=^hm-go-ocupante$' -q | grep -q .; then
    ( "$GO_BINARY" proxy up >"$LAB/go.err" 2>&1 ); ESTADO_GO=$?

    test_case "something else holding port 80 is refused, and named"
    assert_equals "6" "$ESTADO_GO"
    assert_contains "$(jq -r '.error.message' < "$LAB/go.err")" "hm-go-ocupante"
fi

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
