#!/usr/bin/env bash

#
# The tools the MCP server offers, and the one piece of judgement in them.
#
# Everything else here is a wrapper over a command that already answers in JSON, so there is one
# implementation of "what is this project" and it is the one with tests. The exception is the
# database tool, which takes a string from a model and gives it to a database: that one needs a
# rule, and the rule is written here so it can be tested without a protocol around it.
#

HM_MCP_ROW_CAP="${HM_MCP_ROW_CAP:-200}"
HM_MCP_LOG_CAP="${HM_MCP_LOG_CAP:-500}"

#
# Is this a statement that only reads?
#
# A whitelist, and one that errs towards refusing. A model that cannot run its query gets a
# sentence explaining why; a model that can run `SELECT ... INTO OUTFILE` gets a file written on
# the host as the database user.
#
# Assigns HM_MCP_QUERY_REFUSAL when it says no.
#
hm_mcp_query_is_read_only() {
    local sql="$1" stripped

    HM_MCP_QUERY_REFUSAL=""

    # Comments first: everything after them is invisible to the checks below and visible to the
    # database, which is the whole trick
    stripped=$(printf '%s' "$sql" |
        sed -e 's|/\*[^*]*\*/| |g' -e 's/--[[:space:]].*$//' -e 's/#.*$//' |
        tr '\n\t' '  ')

    # Leading and trailing space, and a single trailing semicolon, are ordinary
    stripped="${stripped#"${stripped%%[![:space:]]*}"}"
    stripped="${stripped%"${stripped##*[![:space:]]}"}"
    stripped="${stripped%;}"
    stripped="${stripped%"${stripped##*[![:space:]]}"}"

    if [ -z "$stripped" ]; then
        HM_MCP_QUERY_REFUSAL="There is no statement to run"
        return 1
    fi

    case "$stripped" in
        *";"*)
            HM_MCP_QUERY_REFUSAL="One statement at a time: this contains more than one"
            return 1
            ;;
    esac

    local upper
    upper=$(printf '%s' "$stripped" | tr '[:lower:]' '[:upper:]')

    case "$upper" in
        *"INTO OUTFILE"* | *"INTO DUMPFILE"* | *"LOAD_FILE"*)
            HM_MCP_QUERY_REFUSAL="Refused: that writes or reads a file as the database user"
            return 1
            ;;
    esac

    case "$upper" in
        SELECT[[:space:]]* | SHOW[[:space:]]* | DESCRIBE[[:space:]]* | DESC[[:space:]]* | EXPLAIN[[:space:]]* | "SELECT("*)
            return 0
            ;;
    esac

    HM_MCP_QUERY_REFUSAL="Only SELECT, SHOW, DESCRIBE and EXPLAIN can be run here"
    return 1
}

#
# The tool catalogue, as the protocol wants it: a name, a sentence a model can choose by, and a
# schema for the arguments.
#
hm_mcp_tool_definitions() {
    cat <<'JSON'
[
  {
    "name": "describe_project",
    "description": "What the Magento project in the current directory is: PHP, database and search versions, the services and whether they are running, its URLs and its deploy mode.",
    "inputSchema": { "type": "object", "properties": {}, "additionalProperties": false },
    "annotations": { "readOnlyHint": true }
  },
  {
    "name": "list_environments",
    "description": "Every Dockergento environment on this machine, with the directory it belongs to, its branch and its state. Use it to find out what else is running.",
    "inputSchema": { "type": "object", "properties": {}, "additionalProperties": false },
    "annotations": { "readOnlyHint": true }
  },
  {
    "name": "check_environment",
    "description": "Run the diagnostics (hm doctor) and return each check with its result. Use it when something does not work and you do not know why yet.",
    "inputSchema": { "type": "object", "properties": {}, "additionalProperties": false },
    "annotations": { "readOnlyHint": true }
  },
  {
    "name": "service_logs",
    "description": "The last lines of one service's log. Services are named as in the compose file: phpfpm, nginx, db, search, redis, varnish.",
    "inputSchema": {
      "type": "object",
      "properties": {
        "service": { "type": "string", "description": "The service whose log to read" },
        "lines": { "type": "integer", "description": "How many lines from the end (default 100)" }
      },
      "required": ["service"],
      "additionalProperties": false
    },
    "annotations": { "readOnlyHint": true }
  },
  {
    "name": "database_query",
    "description": "Run one read-only SQL statement against this project's database and return the rows. Only SELECT, SHOW, DESCRIBE and EXPLAIN are accepted; anything that writes is refused.",
    "inputSchema": {
      "type": "object",
      "properties": {
        "sql": { "type": "string", "description": "A single SELECT, SHOW, DESCRIBE or EXPLAIN statement" }
      },
      "required": ["sql"],
      "additionalProperties": false
    },
    "annotations": { "readOnlyHint": true }
  }
]
JSON
}

#
# The write tools, offered only when the server was started with --write.
#
# Absent rather than refused: a tool that exists and says no is worse than no tool, because the
# model sees it, plans around it, reads the refusal and then reaches for a shell — which is the
# permission these exist to make unnecessary.
#
# Which is also the argument for the boundary. An agent that has to flush a cache is given
# `hm magento` today, and that runs anything Magento can do, `setup:upgrade` included. Four typed
# tools are a smaller permission than that shell, not a larger one.
#
hm_mcp_write_tool_definitions() {
    cat <<'JSON'
[
  {
    "name": "cache_flush",
    "description": "Flush the whole Magento cache. Use after a change that a page does not seem to have picked up.",
    "inputSchema": { "type": "object", "properties": {}, "additionalProperties": false },
    "annotations": { "readOnlyHint": false, "destructiveHint": false, "idempotentHint": true }
  },
  {
    "name": "cache_clean",
    "description": "Clean Magento cache types. With no argument, all of them; otherwise the types named, for example config or layout.",
    "inputSchema": {
      "type": "object",
      "properties": {
        "types": {
          "type": "array",
          "items": { "type": "string" },
          "description": "Cache types to clean, for example [\"config\", \"layout\"]"
        }
      },
      "additionalProperties": false
    },
    "annotations": { "readOnlyHint": false, "destructiveHint": false, "idempotentHint": true }
  },
  {
    "name": "reindex",
    "description": "Rebuild Magento indexes. With no argument, all of them; otherwise the one named. Use when the catalogue or search results do not match the data.",
    "inputSchema": {
      "type": "object",
      "properties": {
        "indexer": { "type": "string", "description": "One indexer, for example catalog_product_price" }
      },
      "additionalProperties": false
    },
    "annotations": { "readOnlyHint": false, "destructiveHint": false, "idempotentHint": true }
  },
  {
    "name": "config_set",
    "description": "Set a Magento configuration value at default scope, as bin/magento config:set does.",
    "inputSchema": {
      "type": "object",
      "properties": {
        "path": { "type": "string", "description": "Configuration path, for example web/secure/use_in_frontend" },
        "value": { "type": "string", "description": "The value to set" }
      },
      "required": ["path", "value"],
      "additionalProperties": false
    },
    "annotations": { "readOnlyHint": false, "destructiveHint": false, "idempotentHint": true }
  }
]
JSON
}

#
# The whole catalogue: read-only, plus the write tools when they are enabled.
#
hm_mcp_catalogue() {
    if [ "${1:-false}" == "true" ]; then
        jq -c -s 'add' <(hm_mcp_tool_definitions) <(hm_mcp_write_tool_definitions)
    else
        hm_mcp_tool_definitions | jq -c .
    fi
}

#
# Is this a Magento configuration path?
#
# Not a boundary against a hostile model, and not pretending to be one: it is what stops a
# plausible mistake from being passed on as a command argument.
#
hm_mcp_is_config_path() {
    case "$1" in
        "" | */ | /*) return 1 ;;
        */*) ;;
        *) return 1 ;;
    esac

    printf '%s' "$1" | grep -qE '^[a-z0-9_]+(/[a-z0-9_]+)+$'
}

hm_mcp_tool_names() {
    hm_mcp_catalogue "${1:-false}" | jq -r '.[].name'
}
