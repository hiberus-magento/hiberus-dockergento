#!/usr/bin/env bash
#
# The generated block: how it is placed, replaced, and refused.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"

DATA_DIR="$COMMAND_BIN_DIR/data"
source "$TASKS_DIR/agent_context.sh"

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

BLOCK=$(printf '%s\ncontenido generado\n%s' "$HM_CONTEXT_BEGIN" "$HM_CONTEXT_END")

# ---------------------------------------------------------------- placing it

test_case "a file that does not exist is created with the block"
hm_context_write_block "$WORK/nuevo.md" "$BLOCK"
assert_contains "$(cat "$WORK/nuevo.md")" "contenido generado"

test_case "a file with no block gets one appended, keeping what was there"
printf '# Mis notas\n\nAlgo mío.\n' > "$WORK/existente.md"
hm_context_write_block "$WORK/existente.md" "$BLOCK"
assert_contains "$(cat "$WORK/existente.md")" "Algo mío."
assert_contains "$(cat "$WORK/existente.md")" "contenido generado"

# ---------------------------------------------------------------- replacing it

test_case "the block is replaced in place"
NUEVO=$(printf '%s\notro contenido\n%s' "$HM_CONTEXT_BEGIN" "$HM_CONTEXT_END")
hm_context_write_block "$WORK/existente.md" "$NUEVO"
assert_contains "$(cat "$WORK/existente.md")" "otro contenido"
assert_not_contains "$(cat "$WORK/existente.md")" "contenido generado"

test_case "and what surrounds it is untouched"
printf 'Y esto después.\n' >> "$WORK/existente.md"
hm_context_write_block "$WORK/existente.md" "$BLOCK"
assert_contains "$(cat "$WORK/existente.md")" "Algo mío."
assert_contains "$(cat "$WORK/existente.md")" "Y esto después."

test_case "replacing does not multiply the block"
hm_context_write_block "$WORK/existente.md" "$BLOCK"
hm_context_write_block "$WORK/existente.md" "$BLOCK"
assert_equals "1" "$(grep -cF "$HM_CONTEXT_BEGIN" "$WORK/existente.md")"

# ---------------------------------------------------------------- refusing
#
# An opening marker with no closing one means somebody edited it by hand. Rewriting from there
# to the end of the file would take their text with it.

test_case "an unclosed block is refused, and the file is left alone"
printf '# Notas\n%s\nmedio bloque\n\nTexto de después.\n' "$HM_CONTEXT_BEGIN" > "$WORK/roto.md"
hm_context_write_block "$WORK/roto.md" "$BLOCK" && r=escribió || r=rechazó
assert_equals "rechazó" "$r"
assert_contains "$(cat "$WORK/roto.md")" "Texto de después."

test_case "and no temporary file is left behind"
assert_equals "" "$(ls "$WORK"/*.hm-tmp 2>/dev/null || true)"

# ---------------------------------------------------------------- the mcp entry

test_case "the entry is written when there is no file"
hm_context_write_mcp "$WORK/.mcp.json" "/ruta/hm" "/ruta/proyecto"
assert_equals "mcp" "$(jq -r '.mcpServers.hm.args[0]' "$WORK/.mcp.json")"
assert_equals "/ruta/proyecto" "$(jq -r '.mcpServers.hm.cwd' "$WORK/.mcp.json")"

test_case "other servers in the file survive"
jq '.mcpServers.otro = {"command": "otro"}' "$WORK/.mcp.json" > "$WORK/tmp" && mv "$WORK/tmp" "$WORK/.mcp.json"
hm_context_write_mcp "$WORK/.mcp.json" "/ruta/hm" "/ruta/proyecto"
assert_equals "otro" "$(jq -r '.mcpServers.otro.command' "$WORK/.mcp.json")"
assert_equals "mcp" "$(jq -r '.mcpServers.hm.args[0]' "$WORK/.mcp.json")"

test_case "a file that is not JSON is replaced rather than corrupted"
printf 'esto no es json\n' > "$WORK/malo.json"
hm_context_write_mcp "$WORK/malo.json" "/ruta/hm" "/ruta/proyecto"
assert_equals "mcp" "$(jq -r '.mcpServers.hm.args[0]' "$WORK/malo.json")"

# ---------------------------------------------------------------- the fingerprint

INFO='{"project":{"domain":"a.test","urls":{"admin":"https://a.test/panel"}},"magento":{"version":"2.4.7"},"services":[{"name":"phpfpm","image":"php:8.2"}]}'
OTRO='{"project":{"domain":"a.test","urls":{"admin":"https://a.test/panel"}},"magento":{"version":"2.4.7"},"services":[{"name":"phpfpm","image":"php:7.4"}]}'

test_case "the same project gives the same fingerprint"
assert_equals "$(hm_context_fingerprint "$INFO")" "$(hm_context_fingerprint "$INFO")"

test_case "a changed image gives a different one"
assert_equals "distinto" \
    "$([ "$(hm_context_fingerprint "$INFO")" != "$(hm_context_fingerprint "$OTRO")" ] && echo distinto || echo igual)"

# ---------------------------------------------------------------- the exclusions

test_case "every exclusion states a path and a reason"
assert_equals "0" "$(jq '[.exclusions[] | select((.path | length) == 0 or (.reason | length) == 0)] | length' "$DATA_DIR/ai-exclusions.json")"

test_case "the ones that matter are in the list"
for path in "app/etc/env.php" "var/log" "pub/media/customer" "vendor" "generated" "var/cache"; do
    assert_equals "1" "$(jq --arg p "$path" '[.exclusions[] | select(.path == $p)] | length' "$DATA_DIR/ai-exclusions.json")"
done

test_case "secrets and customer data are marked as sensitive, noise is not"
assert_equals "true" "$(jq -r '.exclusions[] | select(.path == "app/etc/env.php") | .sensitive' "$DATA_DIR/ai-exclusions.json")"
assert_equals "false" "$(jq -r '.exclusions[] | select(.path == "vendor") | .sensitive' "$DATA_DIR/ai-exclusions.json")"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
