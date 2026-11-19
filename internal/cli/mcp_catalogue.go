package cli

//
// The tool catalogue, as the protocol wants it: a name, a sentence a model can choose by, and a
// schema for the arguments. Copied from console/tasks/mcp_tools.sh, key order and all, because
// tools/list answers with it as it is written.
//

// mcpReadTools are offered always.
const mcpReadTools = `
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
`

// mcpWriteTools are offered only when the server was started with --write.
//
// Absent rather than refused: a tool that exists and says no is worse than no tool, because the
// model sees it, plans around it, reads the refusal and then reaches for a shell — which is the
// permission these exist to make unnecessary. An agent that has to flush a cache is given
// `hm magento` today, and that runs anything Magento can do; four typed tools are a smaller
// permission than that shell, not a larger one.
const mcpWriteTools = `
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
`

// mcpCatalogue is the whole catalogue: the read-only tools, plus the write tools when they are
// enabled.
func mcpCatalogue(write bool) jqValue {
	catalogue, _ := parseJQ([]byte(mcpReadTools))

	if write {
		more, _ := parseJQ([]byte(mcpWriteTools))
		catalogue.items = append(catalogue.items, more.items...)
	}

	return catalogue
}

// mcpToolNames is every name in the catalogue, in its order.
func mcpToolNames(write bool) []string {
	var names []string
	for _, tool := range mcpCatalogue(write).items {
		name, _ := tool.field("name")
		names = append(names, name.text)
	}

	return names
}
