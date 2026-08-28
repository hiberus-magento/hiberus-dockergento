#!/usr/bin/env bash
#
# A real MCP session over a pipe, against a real database.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

HM="$COMMAND_BIN_DIR/bin/run"
LAB=$(cd "$(mktemp -d)" && pwd -P)
PROJECT="hm-mcp-selftest"

cleanup() {
    ( cd "$LAB/$PROJECT" 2>/dev/null && docker compose -p "$PROJECT" down -v --remove-orphans ) >/dev/null 2>&1
    rm -rf "$LAB"
}
trap cleanup EXIT

if ! docker info >/dev/null 2>&1; then
    echo "  - skipped: docker is not available"
    echo "RESULT 0 0"
    exit 0
fi

mkdir -p "$LAB/$PROJECT/config/docker"
cat > "$LAB/$PROJECT/docker-compose.yml" <<'YAML'
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
    command: ["sh", "-c", "echo hola desde phpfpm; sleep 600"]
YAML
cp "$LAB/$PROJECT/docker-compose.yml" "$LAB/$PROJECT/docker-compose.dev.mac.yml"
cp "$LAB/$PROJECT/docker-compose.yml" "$LAB/$PROJECT/docker-compose.dev.linux.yml"
printf '{"MAGENTO_DIR": ".", "DOMAIN": "mcp.local", "COMPOSE_PROJECT_NAME": "%s"}\n' "$PROJECT" \
    > "$LAB/$PROJECT/config/docker/properties.json"

#
# One session per call: the server is a pipe, so a request and its answer are a whole
# conversation. Real clients keep it open, which is the same thing with more lines.
#
session() {
    ( cd "$LAB/$PROJECT" && printf '%s\n' "$@" | "$HM" mcp 2>/dev/null )
}

INIT='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{}}}'

# ---------------------------------------------------------------- the protocol

OUT=$(session "$INIT")
test_case "the server initialises"
assert_equals "2025-06-18" "$(printf '%s' "$OUT" | jq -r '.result.protocolVersion')"
assert_equals "true" "$(printf '%s' "$OUT" | jq -r '.result.capabilities | has("tools")')"
assert_contains "$(printf '%s' "$OUT" | jq -r '.result.serverInfo.name')" "hm"

test_case "an older client is answered in its own version"
assert_equals "2024-11-05" "$(session '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}' |
    jq -r '.result.protocolVersion')"

test_case "a version nobody has heard of gets ours"
assert_equals "2025-06-18" "$(session '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"1999-01-01"}}' |
    jq -r '.result.protocolVersion')"

test_case "a notification is not answered"
assert_equals "1" "$(session '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
    '{"jsonrpc":"2.0","id":9,"method":"ping"}' | grep -c . )"

test_case "the tools are listed with their schemas"
OUT=$(session '{"jsonrpc":"2.0","id":2,"method":"tools/list"}')
assert_equals "5" "$(printf '%s' "$OUT" | jq -r '.result.tools | length')"
assert_contains "$(printf '%s' "$OUT" | jq -r '.result.tools[].name' | tr '\n' ' ')" "database_query"
assert_equals "object" "$(printf '%s' "$OUT" | jq -r '.result.tools[0].inputSchema.type')"

test_case "an unimplemented method is an error, not a crash"
assert_equals "-32601" "$(session '{"jsonrpc":"2.0","id":3,"method":"resources/list"}' | jq -r '.error.code')"

test_case "a line that is not JSON is answered and the server keeps going"
OUT=$(session 'esto no es json' '{"jsonrpc":"2.0","id":4,"method":"ping"}')
assert_equals "-32700" "$(printf '%s' "$OUT" | head -1 | jq -r '.error.code')"
assert_equals "4" "$(printf '%s' "$OUT" | tail -1 | jq -r '.id')"

# ---------------------------------------------------------------- the tools

call() {
    session "$(jq -cn --arg name "$1" --argjson arguments "$2" \
        '{jsonrpc: "2.0", id: 7, method: "tools/call", params: {name: $name, arguments: $arguments}}')"
}

test_case "a tool that does not exist says which ones do"
OUT=$(call "delete_everything" '{}')
assert_equals "true" "$(printf '%s' "$OUT" | jq -r '.result.isError')"
assert_contains "$(printf '%s' "$OUT" | jq -r '.result.content[0].text')" "describe_project"

test_case "a write is refused before it is run"
OUT=$(call "database_query" '{"sql": "DROP TABLE core_config_data"}')
assert_equals "true" "$(printf '%s' "$OUT" | jq -r '.result.isError')"
assert_contains "$(printf '%s' "$OUT" | jq -r '.result.content[0].text')" "SELECT"

test_case "and so is a second statement"
assert_equals "true" "$(call "database_query" '{"sql": "SELECT 1; DROP TABLE x"}' | jq -r '.result.isError')"

( cd "$LAB/$PROJECT" && docker compose -p "$PROJECT" up -d ) >/dev/null 2>&1

waited=0
until ( cd "$LAB/$PROJECT" && docker compose -p "$PROJECT" exec -T db \
        mariadb -uroot -ppassword magento -e "SELECT 1" ) >/dev/null 2>&1 || [ "$waited" -gt 120 ]; do
    sleep 2
    waited=$((waited + 2))
done

test_case "the project can be described"
OUT=$(call "describe_project" '{}')
assert_equals "false" "$(printf '%s' "$OUT" | jq -r '.result.isError')"
assert_contains "$(printf '%s' "$OUT" | jq -r '.result.content[0].text')" "$PROJECT"

test_case "a select comes back with its rows"
OUT=$(call "database_query" '{"sql": "SELECT 42 AS respuesta"}')
assert_equals "false" "$(printf '%s' "$OUT" | jq -r '.result.isError')"
assert_contains "$(printf '%s' "$OUT" | jq -r '.result.content[0].text')" "42"

test_case "a service's log can be read"
OUT=$(call "service_logs" '{"service": "phpfpm", "lines": 20}')
assert_equals "false" "$(printf '%s' "$OUT" | jq -r '.result.isError')"
assert_contains "$(printf '%s' "$OUT" | jq -r '.result.content[0].text')" "hola desde phpfpm"

test_case "a service that does not exist says which ones do"
OUT=$(call "service_logs" '{"service": "no-existe"}')
assert_contains "$(printf '%s' "$OUT" | jq -r '.result.content[0].text')" "phpfpm"

test_case "the environments of the machine can be listed"
assert_equals "false" "$(call "list_environments" '{}' | jq -r '.result.isError')"

# ---------------------------------------------------------------- the write half

write_session() {
    ( cd "$LAB/$PROJECT" && printf '%s\n' "$@" | "$HM" mcp --write 2>/dev/null )
}

test_case "a read-only server does not list the write tools"
assert_equals "5" "$(session '{"jsonrpc":"2.0","id":2,"method":"tools/list"}' | jq -r '.result.tools | length')"

test_case "and calling one is an unknown tool, not a refusal to plan around"
OUT=$(call "cache_flush" '{}')
assert_equals "true" "$(printf '%s' "$OUT" | jq -r '.result.isError')"
assert_contains "$(printf '%s' "$OUT" | jq -r '.result.content[0].text')" "no tool called"

test_case "with --write there are four more"
OUT=$(write_session '{"jsonrpc":"2.0","id":2,"method":"tools/list"}')
assert_equals "9" "$(printf '%s' "$OUT" | jq -r '.result.tools | length')"
assert_contains "$(printf '%s' "$OUT" | jq -r '.result.tools[].name' | tr '\n' ' ')" "config_set"

test_case "and they declare that they are not read-only"
assert_equals "4" "$(printf '%s' "$OUT" | jq -r '[.result.tools[] | select(.annotations.readOnlyHint == false)] | length')"

test_case "a configuration path that is not one is refused without running anything"
OUT=$(write_session "$(jq -cn '{jsonrpc: "2.0", id: 7, method: "tools/call",
    params: {name: "config_set", arguments: {path: "no es una ruta", value: "1"}}}')")
assert_equals "true" "$(printf '%s' "$OUT" | jq -r '.result.isError')"
assert_contains "$(printf '%s' "$OUT" | jq -r '.result.content[0].text')" "section/group/field"

test_case "the configuration entry carries the flag"
OUT=$( cd "$LAB/$PROJECT" && "$HM" mcp --config --write 2>/dev/null )
assert_equals "mcp --write" "$(printf '%s' "$OUT" | jq -r '.mcpServers.hm.args | join(" ")')"

# ---------------------------------------------------------------- wiring

test_case "the client configuration is printed, not written"
OUT=$( cd "$LAB/$PROJECT" && "$HM" mcp --config 2>/dev/null )
assert_equals "mcp" "$(printf '%s' "$OUT" | jq -r '.mcpServers.hm.args[0]')"
assert_equals "$LAB/$PROJECT" "$(printf '%s' "$OUT" | jq -r '.mcpServers.hm.cwd')"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
