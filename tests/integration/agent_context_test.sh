#!/usr/bin/env bash
#
# The project context an agent reads, generated from the resolved configuration.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

HM="$COMMAND_BIN_DIR/bin/run"
LAB=$(cd "$(mktemp -d)" && pwd -P)
PROJECT="hm-context-selftest"
trap 'rm -rf "$LAB"' EXIT

if ! docker info >/dev/null 2>&1; then
    echo "  - skipped: docker is not available"
    echo "RESULT 0 0"
    exit 0
fi

DIR="$LAB/$PROJECT"
mkdir -p "$DIR/config/docker" "$DIR/app/etc"
cat > "$DIR/docker-compose.yml" <<'YAML'
services:
  phpfpm:
    image: hiberusmagento/php:8.2
  nginx:
    image: hiberusmagento/nginx:1.28
  db:
    image: mariadb:10.6
    environment:
      MYSQL_ROOT_PASSWORD: unapasswordquenodebeaparecer
      MYSQL_DATABASE: magento
YAML
cp "$DIR/docker-compose.yml" "$DIR/docker-compose.dev.mac.yml"
cp "$DIR/docker-compose.yml" "$DIR/docker-compose.dev.linux.yml"
printf '{"MAGENTO_DIR": ".", "DOMAIN": "context.test", "COMPOSE_PROJECT_NAME": "%s"}\n' "$PROJECT" \
    > "$DIR/config/docker/properties.json"

# A custom admin front name, which is the fact an agent gets wrong most often
cat > "$DIR/app/etc/env.php" <<'PHP'
<?php
return [
    'backend' => ['frontName' => 'panel-secreto'],
    'crypt' => ['key' => 'no-debe-aparecer-en-el-contexto'],
    'MAGE_MODE' => 'developer',
];
PHP

run() { ( cd "$DIR" && "$HM" "$@" >"$LAB/out" 2>"$LAB/err" ); STATUS=$?; STDOUT=$(cat "$LAB/out"); STDERR=$(cat "$LAB/err"); return 0; }

# ---------------------------------------------------------------- generating

run ai-context
assert_equals "0" "$STATUS" "the context is generated"

test_case "AGENTS.md is written with the generated block"
assert_equals "0" "$([ -f "$DIR/AGENTS.md" ] && echo 0 || echo 1)"
assert_contains "$(cat "$DIR/AGENTS.md")" "hm:begin"
assert_contains "$(cat "$DIR/AGENTS.md")" "hm:end"

test_case "it carries the resolved facts, not the configured ones"
assert_contains "$(cat "$DIR/AGENTS.md")" "panel-secreto"
assert_contains "$(cat "$DIR/AGENTS.md")" "$PROJECT"
assert_contains "$(cat "$DIR/AGENTS.md")" "developer"

test_case "and no secrets"
assert_not_contains "$(cat "$DIR/AGENTS.md")" "unapasswordquenodebeaparecer"
assert_not_contains "$(cat "$DIR/AGENTS.md")" "no-debe-aparecer-en-el-contexto"

test_case "it says what not to read"
assert_contains "$(cat "$DIR/AGENTS.md")" "app/etc/env.php"
assert_contains "$(cat "$DIR/AGENTS.md")" "pub/media/customer"

test_case "and what not to run"
assert_contains "$(cat "$DIR/AGENTS.md")" "down -v"

test_case "CLAUDE.md is created pointing at it"
assert_equals "@AGENTS.md" "$(cat "$DIR/CLAUDE.md")"

test_case "the MCP server is wired up"
assert_equals "mcp" "$(jq -r '.mcpServers.hm.args[0]' "$DIR/.mcp.json")"
assert_equals "$DIR" "$(jq -r '.mcpServers.hm.cwd' "$DIR/.mcp.json")"

# ---------------------------------------------------------------- regenerating

printf '\n## Mis notas\n\nEsto lo escribí yo.\n' >> "$DIR/AGENTS.md"
printf 'Y esto también, antes del bloque.\n' | cat - "$DIR/AGENTS.md" > "$DIR/tmp" && mv "$DIR/tmp" "$DIR/AGENTS.md"

run ai-context
test_case "regenerating leaves everything outside the block alone"
assert_contains "$(cat "$DIR/AGENTS.md")" "Esto lo escribí yo."
assert_contains "$(cat "$DIR/AGENTS.md")" "Y esto también, antes del bloque."

test_case "and does not duplicate the block"
assert_equals "1" "$(grep -c 'hm:begin' "$DIR/AGENTS.md")"

test_case "an existing CLAUDE.md is never modified"
printf 'Mis instrucciones propias.\n' > "$DIR/CLAUDE.md"
run ai-context
assert_equals "Mis instrucciones propias." "$(cat "$DIR/CLAUDE.md")"
assert_contains "$STDOUT" "@AGENTS.md"

test_case "an existing .mcp.json keeps its other servers"
jq '.mcpServers.otro = {"command": "otro"}' "$DIR/.mcp.json" > "$DIR/tmp" && mv "$DIR/tmp" "$DIR/.mcp.json"
run ai-context
assert_equals "otro" "$(jq -r '.mcpServers.otro.command' "$DIR/.mcp.json")"
assert_equals "mcp" "$(jq -r '.mcpServers.hm.args[0]' "$DIR/.mcp.json")"

# ---------------------------------------------------------------- staleness

run doctor --json
test_case "the doctor sees the context as current"
assert_equals "ok" "$(printf '%s' "$STDOUT" | jq -r '.data.checks[] | select(.message | test("agent context")) | .severity' | head -1)"

sed -i.bak "s|'panel-secreto'|'otro-panel'|" "$DIR/app/etc/env.php"
run doctor --json
test_case "and sees it go stale when the project changes"
assert_equals "error" "$(printf '%s' "$STDOUT" | jq -r '.data.checks[] | select(.message | test("agent context")) | .severity' | head -1)"

run ai-context
run doctor --json
test_case "regenerating settles it"
assert_equals "ok" "$(printf '%s' "$STDOUT" | jq -r '.data.checks[] | select(.message | test("agent context")) | .severity' | head -1)"

# ---------------------------------------------------------------- permissions

run permissions --json
test_case "the excluded paths are denied outright"
assert_contains "$(printf '%s' "$STDOUT" | jq -r '.permissions.deny | join(" ")')" "app/etc/env.php"
assert_contains "$(printf '%s' "$STDOUT" | jq -r '.permissions.deny | join(" ")')" "var/log"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
