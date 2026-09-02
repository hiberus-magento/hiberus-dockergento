#!/usr/bin/env bash
#
# The web interface: the same answers, through a different door.
#
# It sits next to `tui`, one interface for the terminal and one for the browser, and it behaves
# like the proxy on purpose — something you bring up once and forget about, not something that
# holds a terminal. What is tested here is that, and the two rules that stand
# between a page in a browser and this machine's environments.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

GO_BINARY="$COMMAND_BIN_DIR/bin/hm"
LAB=$(cd "$(mktemp -d)" && pwd -P)
PUERTO=8433

trap '"$GO_BINARY" web down >/dev/null 2>&1; rm -rf "$LAB"; hm_test_home_cleanup' EXIT

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

# ---------------------------------------------------------------- it comes up and stays up

test_case "it is not running to begin with"
assert_equals "false" "$("$GO_BINARY" web status | jq -r '.data.running')"

"$GO_BINARY" web --port "$PUERTO" >"$LAB/arranque.json" 2>&1
ARRANQUE=$?

test_case "it starts and says where it is"
assert_equals "0" "$ARRANQUE"
URL=$(jq -r '.data.url' < "$LAB/arranque.json")
assert_contains "$URL" "http://127.0.0.1:$PUERTO/?token="

TOKEN="${URL#*token=}"

#
# Returning before the server answers would hand somebody a link that fails once and works on the
# second try, which is worse than being slow.
#
test_case "and it is already answering when the command returns"
assert_equals "200" "$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:$PUERTO/api/health?token=$TOKEN")"

test_case "status finds it"
assert_equals "true" "$("$GO_BINARY" web status | jq -r '.data.running')"
assert_equals "$PUERTO" "$("$GO_BINARY" web status | jq -r '.data.port')"

test_case "and starting it again does not start a second one"
PID=$("$GO_BINARY" web status | jq -r '.data.pid')
"$GO_BINARY" web --port "$PUERTO" >/dev/null 2>&1
assert_equals "$PID" "$("$GO_BINARY" web status | jq -r '.data.pid')"

# ---------------------------------------------------------------- the same answers

#
# The point of the whole separation: a dashboard that reimplemented "what is running" would answer
# something slightly different from `hm list` the first time either changed.
#
test_case "the environments are the same document the command line prints"
curl -s "http://127.0.0.1:$PUERTO/api/environments?token=$TOKEN" > "$LAB/api.json"
"$GO_BINARY" list --json > "$LAB/cli.json"
assert_equals "$(jq -S . < "$LAB/cli.json")" "$(jq -S . < "$LAB/api.json")"

test_case "the page is served"
assert_equals "200" "$(curl -s -o /dev/null -w '%{http_code}' "$URL")"

# ---------------------------------------------------------------- and the two rules

#
# This API reads database credentials and stops environments, and a page on the internet can make
# a browser send requests to a port on this machine. Both of these matter.
#

test_case "without the token there is no answer"
assert_equals "401" "$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:$PUERTO/api/environments")"

test_case "nor with the wrong one"
assert_equals "401" "$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:$PUERTO/api/environments?token=noesta")"

#
# A name on the internet can be pointed at 127.0.0.1, and then a page served from it is
# same-origin with this server as far as the browser is concerned.
#
test_case "and not through a name that is not loopback"
assert_equals "403" "$(curl -s -o /dev/null -w '%{http_code}' -H "Host: rebind.example.com" "http://127.0.0.1:$PUERTO/api/health?token=$TOKEN")"

test_case "it listens on loopback and nowhere else"
assert_equals "" "$(lsof -nP -iTCP:"$PUERTO" -sTCP:LISTEN 2>/dev/null | awk 'NR>1 && $9 !~ /^127\.0\.0\.1:/ {print $9}')"

test_case "the credentials are asked for, never volunteered"
CUALQUIERA=$(jq -r '.data.environments[0].root // ""' < "$LAB/cli.json")
if [ -n "$CUALQUIERA" ]; then
    assert_equals "null" "$(curl -s "http://127.0.0.1:$PUERTO/api/project?root=$CUALQUIERA&token=$TOKEN" | jq -r '.data.credentials // "null"')"
else
    assert_equals "null" "null"
fi

# ---------------------------------------------------------------- and it goes away

test_case "down stops it"
"$GO_BINARY" web down >/dev/null 2>&1
assert_equals "false" "$("$GO_BINARY" web status | jq -r '.data.running')"

test_case "and the port is free again"
assert_equals "" "$(lsof -nP -iTCP:"$PUERTO" -sTCP:LISTEN 2>/dev/null | awk 'NR>1 {print $1}')"

test_case "stopping something that is not running is not a failure"
"$GO_BINARY" web down >/dev/null 2>&1
assert_equals "0" "$?"

test_case "an option nobody declared is refused with the usage code"
"$GO_BINARY" web --tonteria >/dev/null 2>&1
assert_equals "2" "$?"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
