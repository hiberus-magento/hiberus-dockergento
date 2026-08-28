#!/usr/bin/env bash
#
# The one piece of judgement in the MCP server: what counts as a statement that only reads.
#
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib/assert.sh"
source "$TASKS_DIR/mcp_tools.sh"

accepts() {
    hm_mcp_query_is_read_only "$1" && printf 'accepted' || printf 'refused'
}

# ---------------------------------------------------------------- what reads

test_case "a select"
assert_equals "accepted" "$(accepts "SELECT value FROM core_config_data LIMIT 5")"

test_case "case does not matter"
assert_equals "accepted" "$(accepts "select 1 from dual")"

test_case "leading space and a trailing semicolon are ordinary"
assert_equals "accepted" "$(accepts "   SELECT 1;  ")"

test_case "show, describe and explain read too"
assert_equals "accepted" "$(accepts "SHOW TABLES")"
assert_equals "accepted" "$(accepts "DESCRIBE sales_order")"
assert_equals "accepted" "$(accepts "EXPLAIN SELECT 1 FROM dual")"

# ---------------------------------------------------------------- what does not

test_case "anything that writes is refused"
assert_equals "refused" "$(accepts "UPDATE core_config_data SET value = 1")"
assert_equals "refused" "$(accepts "DELETE FROM sales_order")"
assert_equals "refused" "$(accepts "DROP TABLE sales_order")"
assert_equals "refused" "$(accepts "INSERT INTO admin_user VALUES (1)")"
assert_equals "refused" "$(accepts "TRUNCATE sales_order")"

test_case "and the refusal says what is allowed"
hm_mcp_query_is_read_only "DROP TABLE x"
assert_contains "$HM_MCP_QUERY_REFUSAL" "SELECT"

test_case "an empty statement is not a statement"
assert_equals "refused" "$(accepts "")"
assert_equals "refused" "$(accepts "   ")"

# ---------------------------------------------------------------- what hides
#
# A model does not have to be hostile to produce these: a query built by concatenation arrives
# looking exactly like an attempt at one.

test_case "a second statement is refused"
assert_equals "refused" "$(accepts "SELECT 1; DROP TABLE sales_order")"

test_case "a statement hidden behind a comment is refused"
assert_equals "refused" "$(accepts "/* SELECT */ DROP TABLE sales_order")"

test_case "a comment cannot turn a write into a read"
assert_equals "refused" "$(accepts "-- SELECT 1
DELETE FROM sales_order")"

test_case "writing a file through the database is refused"
assert_equals "refused" "$(accepts "SELECT 1 INTO OUTFILE '/tmp/x'")"
assert_equals "refused" "$(accepts "SELECT 1 into dumpfile '/tmp/x'")"

test_case "and reading one is too"
assert_equals "refused" "$(accepts "SELECT LOAD_FILE('/etc/passwd')")"

test_case "the refusal names the reason"
hm_mcp_query_is_read_only "SELECT 1 INTO OUTFILE '/tmp/x'"
assert_contains "$HM_MCP_QUERY_REFUSAL" "file"

# ---------------------------------------------------------------- the catalogue

test_case "every tool has a name, a description and a schema"
assert_equals "5" "$(hm_mcp_tool_definitions | jq -r 'length')"
assert_equals "5" "$(hm_mcp_tool_definitions | jq -r '[.[] | select(.name and .description and .inputSchema)] | length')"

test_case "the read-only tools say so"
assert_equals "5" "$(hm_mcp_tool_definitions | jq -r '[.[] | select(.annotations.readOnlyHint == true)] | length')"

# ---------------------------------------------------------------- the write half
#
# Absent rather than refused: a tool that exists and says no is worse than no tool, because the
# model sees it, plans around it, reads the refusal and reaches for a shell instead.

test_case "without --write the catalogue is the read-only one"
assert_equals "5" "$(hm_mcp_catalogue false | jq -r 'length')"
assert_equals "" "$(hm_mcp_catalogue false | jq -r '.[].name' | grep -E 'cache|reindex|config_set' || true)"

test_case "with it, four more"
assert_equals "9" "$(hm_mcp_catalogue true | jq -r 'length')"
assert_equals "cache_clean cache_flush config_set reindex" \
    "$(hm_mcp_catalogue true | jq -r '.[] | select(.annotations.readOnlyHint == false) | .name' | sort | tr '\n' ' ' | sed 's/ $//')"

test_case "none of them claims to destroy anything, and all are repeatable"
assert_equals "4" "$(hm_mcp_catalogue true | jq -r '[.[] | select(.annotations.readOnlyHint == false) | select(.annotations.destructiveHint == false and .annotations.idempotentHint == true)] | length')"

#
# These are not slow versions of the tools above: they are the operations whose failure costs an
# afternoon, and they belong to a person at a terminal.
#
test_case "the operations that need a person are not offered at all"
for forbidden in setup upgrade composer compile install import down; do
    assert_equals "" "$(hm_mcp_catalogue true | jq -r '.[].name' | grep -- "$forbidden" || true)"
done

# ---------------------------------------------------------------- configuration paths

test_case "a configuration path looks like a configuration path"
hm_mcp_is_config_path "web/secure/use_in_frontend" && r=si || r=no
assert_equals "si" "$r"
hm_mcp_is_config_path "dev/js/merge_files" && r=si || r=no
assert_equals "si" "$r"

test_case "and anything else is not one"
for bad in "" "web" "/web/secure" "web/secure/" "web secure use" "web/secure;rm -rf /" "WEB/SECURE/X" "web/secure/\$(id)"; do
    hm_mcp_is_config_path "$bad" && r=aceptado || r=rechazado
    assert_equals "rechazado" "$r"
done

test_case "tool names are what the protocol allows"
assert_equals "" "$(hm_mcp_tool_definitions | jq -r '.[].name' | grep -vE '^[a-zA-Z0-9_-]{1,64}$' || true)"

echo "RESULT $HM_TESTS_RUN $HM_TESTS_FAILED"
