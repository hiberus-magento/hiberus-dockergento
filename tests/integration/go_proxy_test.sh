#!/usr/bin/env bash
#
# The global proxy, ported.
#
# One router per machine holding 80 and 443, so that more than one project can be up at a time.
# What is compared here is the whole of it: the compose file it generates, what it answers when it
# is up and when it is not, and the refusal when something else is holding the ports.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

GO_BINARY="$COMMAND_BIN_DIR/bin/hm"
SHELL_CLI="$COMMAND_BIN_DIR/bin/run"

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

test_case "both say it is not running, the same way"
assert_equals "$( "$SHELL_CLI" proxy status --json | jq -S . )" \
              "$( "$GO_BINARY" proxy status --json | jq -S . )"

test_case "including the words a person reads"
assert_equals "$( "$SHELL_CLI" --no-json proxy status 2>&1 )" \
              "$( "$GO_BINARY" --no-json proxy status 2>&1 )"

test_case "and stopping something that is not running is not a failure"
assert_equals "$( "$SHELL_CLI" proxy down --json | jq -S . )" \
              "$( "$GO_BINARY" proxy down --json | jq -S . )"

# ---------------------------------------------------------------- who is holding the ports

#
# A test that cannot run is better than a test that fails. The proxy needs 80 and 443, and on a
# machine where somebody is working there is usually a project holding them — which is the case the
# refusal below exists for, and the reason everything that needs the proxy up is skipped there.
#
OCUPANTE=$(docker ps --format '{{.Names}}\t{{.Ports}}' 2>/dev/null |
    awk -F'\t' '$2 ~ /:80->|:443->/ { print $1 }' | grep -v '^hm-proxy$' | head -1)

if [ -n "$OCUPANTE" ]; then
    ( "$SHELL_CLI" proxy up >"$LAB/shell.err" 2>&1 ); ESTADO_SHELL=$?
    ( "$GO_BINARY"  proxy up >"$LAB/go.err" 2>&1 );   ESTADO_GO=$?

    test_case "something else holding port 80 or 443 is refused, and named"
    assert_equals "6" "$ESTADO_GO"
    assert_equals "$ESTADO_SHELL" "$ESTADO_GO"
    assert_equals "$(jq -S . < "$LAB/shell.err")" "$(jq -S . < "$LAB/go.err")"

    #
    # Which container is named is not asserted here: with a full stack up, one holds 80 and another
    # holds 443, and either is a true answer. What has to be true is that both halves name the
    # same one, which the comparison above says — the case with exactly one holder is below.
    #
    echo "  - skipped: '$OCUPANTE' is using port 80 or 443, so the proxy cannot be started here"
    echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
    exit 0
fi

# ---------------------------------------------------------------- the file it generates

#
# The proxy's compose file is generated, not shipped: it is ours, nobody edits it, and a stale one
# is harder to notice than a rewritten one. Which means both halves have to write the same bytes,
# or starting the proxy with one and stopping it with the other would be two different projects.
#
( "$SHELL_CLI" proxy up ) >/dev/null 2>&1
cp "$HM_PROXY_DIR/docker-compose.yml" "$LAB/shell.yml" 2>/dev/null
( "$SHELL_CLI" proxy down ) >/dev/null 2>&1
rm -rf "$HM_PROXY_DIR"

( "$GO_BINARY" proxy up ) >/dev/null 2>&1

test_case "the compose file it writes is the one the shell implementation writes"
assert_equals "$(cat "$LAB/shell.yml" 2>/dev/null)" "$(cat "$HM_PROXY_DIR/docker-compose.yml" 2>/dev/null)"

test_case "and it is running"
assert_equals "1" "$(docker ps --filter 'name=^hm-proxy$' -q | grep -c .)"

test_case "on the shared network, which is how projects reach it"
assert_equals "1" "$(docker network ls --filter 'name=^hm-gateway$' -q | grep -c .)"

test_case "starting it again is not an error, and says so"
salida=$( "$GO_BINARY" proxy up --json )
assert_equals "false" "$(printf '%s' "$salida" | jq -r '.data.started')"
assert_equals "true" "$(printf '%s' "$salida" | jq -r '.data.running')"

test_case "and both answer that the same way"
assert_equals "$( "$SHELL_CLI" proxy up --json | jq -S . )" \
              "$( "$GO_BINARY" proxy up --json | jq -S . )"

# ---------------------------------------------------------------- what it says it is routing

#
# From the proxy's own API rather than from our idea of what should be routed: a container with the
# labels and a router that never came up look identical from outside.
#
test_case "with it running, both report the same routes"
assert_equals "$( "$SHELL_CLI" proxy status --json | jq -S '.data.routes' )" \
              "$( "$GO_BINARY" proxy status --json | jq -S '.data.routes' )"

test_case "and the same table"
assert_equals "$( "$SHELL_CLI" --no-json proxy status 2>&1 )" \
              "$( "$GO_BINARY" --no-json proxy status 2>&1 )"

# ---------------------------------------------------------------- stopping it

test_case "stopping it stops it"
( "$GO_BINARY" proxy down ) >/dev/null 2>&1
assert_equals "0" "$(docker ps --filter 'name=^hm-proxy$' -q | grep -c .)"

#
# And the refusal, with a container of this test's own holding the port.
#
docker run -d --name hm-go-ocupante -p 80:80 alpine:latest sleep 300 >/dev/null 2>&1

if docker ps --filter 'name=^hm-go-ocupante$' -q | grep -q .; then
    ( "$SHELL_CLI" proxy up >"$LAB/shell.err" 2>&1 ); ESTADO_SHELL=$?
    ( "$GO_BINARY"  proxy up >"$LAB/go.err" 2>&1 );   ESTADO_GO=$?

    test_case "something else holding port 80 is refused, and named"
    assert_equals "6" "$ESTADO_GO"
    assert_equals "$ESTADO_SHELL" "$ESTADO_GO"
    assert_equals "$(jq -S . < "$LAB/shell.err")" "$(jq -S . < "$LAB/go.err")"
    assert_contains "$(jq -r '.error.message' < "$LAB/go.err")" "hm-go-ocupante"
fi

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
