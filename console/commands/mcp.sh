#!/usr/bin/env bash
set -uo pipefail

source "$COMPONENTS_DIR"/print_message.sh
source "$HELPERS_DIR"/exit_codes.sh
source "$TASKS_DIR"/mcp_tools.sh

#
# The Model Context Protocol, spoken over stdin and stdout.
#
# An agent's first minutes on a project go into finding out what it is working on, by running
# shell commands and reading output meant for people. These commands already answer in JSON; what
# a tool adds is that the model is *told* it exists, asks for it by name, and gets boundaries the
# shell does not have.
#
# That boundary is why the read-only half is a separate decision from the write half. `hm exec`
# and `hm mysql` are classified dangerous precisely because they run whatever they are given; a
# list of typed questions is the opposite shape.
#
# Everything printed on stdout here is a protocol message. Every command called has its stderr
# redirected and its stdin closed: one stray warning on stdout is a parse error in the client and
# a server that "just stopped working", and one command reading stdin would eat the conversation.
#

HM="$COMMAND_BIN_DIR/bin/run"
HM_MCP_VERSION="2025-06-18"
HM_MCP_WRITE=false
HM_MCP_KNOWN_VERSIONS='["2025-06-18","2025-03-26","2024-11-05"]'

# ------------------------------------------------------------------ the wire

send() {
    printf '%s\n' "$1"
}

result() {
    send "$(jq -cn --argjson id "$1" --argjson result "$2" \
        '{jsonrpc: "2.0", id: $id, result: $result}')"
}

fail() {
    send "$(jq -cn --argjson id "$1" --argjson code "$2" --arg message "$3" \
        '{jsonrpc: "2.0", id: $id, error: {code: $code, message: $message}}')"
}

#
# A tool that could not answer returns a result, not a protocol error: a model can read the
# sentence and try something else, where a JSON-RPC error is surfaced by most clients as a broken
# server and ends the conversation.
#
content() {
    jq -n --arg text "$1" --argjson error "${2:-false}" \
        '{content: [{type: "text", text: $text}], isError: $error}'
}

# ------------------------------------------------------------------ the tools

#
# The envelope of the output contract is unwrapped here: the model is given the data, and the
# `ok` / `schema_version` scaffolding stays between the commands.
#
from_command() {
    local output
    output=$("$@" --json 2>/dev/null </dev/null)

    if [ -z "$output" ] || ! printf '%s' "$output" | jq -e . >/dev/null 2>&1; then
        content "$* produced nothing readable. The environment may not be running: try check_environment." true
        return 0
    fi

    if [ "$(printf '%s' "$output" | jq -r '.ok // true')" == "false" ]; then
        content "$(printf '%s' "$output" | jq -r '.error.message + "\n" + (.error.hint // "")')" true
        return 0
    fi

    content "$(printf '%s' "$output" | jq -r '.data // .')"
}

tool_describe_project() {
    from_command "$HM" describe
}

tool_list_environments() {
    from_command "$HM" list
}

tool_check_environment() {
    from_command "$HM" doctor
}

tool_service_logs() {
    local service lines output
    service=$(printf '%s' "$1" | jq -r '.service // ""')
    lines=$(printf '%s' "$1" | jq -r '.lines // 100')

    if [ -z "$service" ]; then
        content "Which service? Try phpfpm, nginx, db, search or redis." true
        return 0
    fi

    case "$lines" in
        "" | *[!0-9]*) lines=100 ;;
    esac

    [ "$lines" -gt "$HM_MCP_LOG_CAP" ] && lines="$HM_MCP_LOG_CAP"

    #
    # The global flags go before the command name: `logs` and `mysql` pass what follows them
    # through to Compose and to the client, where `--no-json` is not a flag anybody knows
    output=$("$HM" --no-json logs --tail "$lines" "$service" 2>&1 </dev/null)

    if [ -z "$output" ]; then
        output="(the log is empty)"
    fi

    content "$output"
}

tool_database_query() {
    local sql output rows
    sql=$(printf '%s' "$1" | jq -r '.sql // ""')

    if ! hm_mcp_query_is_read_only "$sql"; then
        content "$HM_MCP_QUERY_REFUSAL" true
        return 0
    fi

    output=$("$HM" --no-json mysql -q "$sql" 2>&1 </dev/null)

    if [ -z "$output" ]; then
        content "(no rows)"
        return 0
    fi

    rows=$(printf '%s\n' "$output" | wc -l | tr -d ' ')

    if [ "$rows" -gt "$HM_MCP_ROW_CAP" ]; then
        output="$(printf '%s\n' "$output" | head -n "$HM_MCP_ROW_CAP")
... truncated at $HM_MCP_ROW_CAP rows. Narrow the query with a WHERE or a LIMIT."
    fi

    content "$output"
}

# ------------------------------------------------------------------ the write tools

#
# Each one is a single Magento command with its arguments checked. Together they are a smaller
# permission than the shell an agent is given today to do the same four things.
#
run_magento() {
    local output
    output=$("$HM" --no-json magento "$@" 2>&1 </dev/null)

    if [ -z "$output" ]; then
        output="(done)"
    fi

    content "$output"
}

tool_cache_flush() {
    run_magento cache:flush
}

tool_cache_clean() {
    local types
    types=$(printf '%s' "$1" | jq -r '(.types // []) | join(" ")')

    # Unquoted on purpose: the list is words, and each one is a cache type
    run_magento cache:clean $types
}

tool_reindex() {
    local indexer
    indexer=$(printf '%s' "$1" | jq -r '.indexer // ""')

    if [ -n "$indexer" ]; then
        run_magento indexer:reindex "$indexer"
    else
        run_magento indexer:reindex
    fi
}

tool_config_set() {
    local path value
    path=$(printf '%s' "$1" | jq -r '.path // ""')
    value=$(printf '%s' "$1" | jq -r '.value // ""')

    if ! hm_mcp_is_config_path "$path"; then
        content "'$path' is not a configuration path. They look like section/group/field, for example web/secure/use_in_frontend." true
        return 0
    fi

    run_magento config:set "$path" "$value"
}

call_tool() {
    local name="$1" arguments="$2"

    case "$name" in
        describe_project)  tool_describe_project;            return 0 ;;
        list_environments) tool_list_environments;           return 0 ;;
        check_environment) tool_check_environment;           return 0 ;;
        service_logs)      tool_service_logs "$arguments";   return 0 ;;
        database_query)    tool_database_query "$arguments"; return 0 ;;
    esac

    #
    # Only reachable when the server was started with --write. Without it these names are not in
    # the catalogue either, so a model never sees them and has nothing to plan around.
    #
    if $HM_MCP_WRITE; then
        case "$name" in
            cache_flush) tool_cache_flush;             return 0 ;;
            cache_clean) tool_cache_clean "$arguments"; return 0 ;;
            reindex)     tool_reindex "$arguments";     return 0 ;;
            config_set)  tool_config_set "$arguments";  return 0 ;;
        esac
    fi

    content "There is no tool called '$name'. The ones there are: $(hm_mcp_tool_names "$HM_MCP_WRITE" | tr '\n' ' ')" true
}

# ------------------------------------------------------------------ the session

handle() {
    local line="$1"
    local id method

    if ! printf '%s' "$line" | jq -e . >/dev/null 2>&1; then
        fail "null" "-32700" "Parse error: that line is not JSON"
        return 0
    fi

    id=$(printf '%s' "$line" | jq -c '.id // null')
    method=$(printf '%s' "$line" | jq -r '.method // ""')

    # A notification has no id and must never be answered
    if [ "$id" == "null" ] && [ "$method" != "" ]; then
        return 0
    fi

    case "$method" in
        initialize)
            local requested version
            requested=$(printf '%s' "$line" | jq -r '.params.protocolVersion // ""')
            version="$HM_MCP_VERSION"

            # Answer in the client's version when it is one we know, which is what keeps an
            # older client talking to a newer server
            if printf '%s' "$HM_MCP_KNOWN_VERSIONS" | jq -e --arg v "$requested" 'index($v)' >/dev/null 2>&1; then
                version="$requested"
            fi

            result "$id" "$(jq -cn --arg version "$version" --arg name "${COMMAND_BIN_NAME:-hm}" \
                --arg tool_version "${HM_VERSION:-unknown}" '{
                    protocolVersion: $version,
                    capabilities: {tools: {}},
                    serverInfo: {name: $name, version: $tool_version},
                    instructions: "Read-only tools for the Magento environment in the current directory. Nothing here changes the environment; use the hm command line for that."
                }')"
            ;;
        tools/list)
            result "$id" "$(jq -cn --argjson tools "$(hm_mcp_catalogue "$HM_MCP_WRITE")" '{tools: $tools}')"
            ;;
        tools/call)
            local name arguments
            name=$(printf '%s' "$line" | jq -r '.params.name // ""')
            arguments=$(printf '%s' "$line" | jq -c '.params.arguments // {}')
            result "$id" "$(call_tool "$name" "$arguments")"
            ;;
        ping)
            result "$id" '{}'
            ;;
        "")
            fail "$id" "-32600" "Invalid request: no method"
            ;;
        *)
            fail "$id" "-32601" "This server does not implement '$method'"
            ;;
    esac
}

# ------------------------------------------------------------------ wiring

print_config() {
    #
    # Printed, not written. An MCP client's configuration has other servers in it, and editing
    # somebody's file to save them a copy and paste is not a good trade.
    #
    local arguments='["mcp"]'
    $HM_MCP_WRITE && arguments='["mcp", "--write"]'

    jq -n --arg command "$COMMAND_BIN_DIR/bin/run" --arg directory "$PWD" \
        --argjson arguments "$arguments" '{
        mcpServers: {
            hm: {command: $command, args: $arguments, cwd: $directory}
        }
    }'
}

show_config=false

for argument in "$@"; do
    case "$argument" in
        --config) show_config=true ;;
        --write)  HM_MCP_WRITE=true ;;
        *)
            hm_fail "$HM_EXIT_USAGE" "invalid_argument" "Unknown option: $argument" \
                "$COMMAND_BIN_NAME mcp [--config] [--write]"
            ;;
    esac
done

if $show_config; then
    print_config
    exit 0
fi

while IFS= read -r line; do
    [ -z "$line" ] && continue
    handle "$line"
done
