package cli

//
// The tool catalogue, as the protocol wants it: a name, a sentence a model can choose by, and a
// schema for the arguments. Copied from console/tasks/mcp_tools.sh, key order and all, because
// tools/list answers with it as it is written; since then every tool but list_environments takes
// an `environment`, the tools that answer with JSON declare an outputSchema, and the Magento loop
// tools were added.
//
// The output schemas name the top-level fields and require none, and leave the rest open: the
// server must answer with data that conforms, and the data is what describe, list and doctor
// print, which grows a field now and then. A schema that pinned every field would turn an added
// field into an answer the client rejects.
//

// mcpReadTools are offered always.
const mcpReadTools = `
[
  {
    "name": "describe_project",
    "description": "What the Magento project in the current directory is: PHP, database and search versions, the services and whether they are running, its URLs and its deploy mode.",
    "inputSchema": {
      "type": "object",
      "properties": {
        "environment": { "type": "string", "description": "A branch environment of this project, by the name hm worktree list gives it. Leave it out for the current directory." }
      },
      "additionalProperties": false
    },
    "outputSchema": {
      "type": "object",
      "properties": {
        "project": { "type": "object", "description": "Name, root, domain, status and URLs" },
        "magento": { "type": "object", "description": "Version and deploy mode" },
        "data": { "type": "object", "description": "Whether the database is anonymised, and since when" },
        "services": { "type": "array", "description": "Each service with its image, version and state" },
        "paths": { "type": "object", "description": "Magento directory, workdir, sync strategy and compose files" },
        "tooling": { "type": "object", "description": "Machine, hm, Compose and xdebug" }
      }
    },
    "annotations": { "readOnlyHint": true }
  },
  {
    "name": "list_environments",
    "description": "Every Dockergento environment on this machine, with the directory it belongs to, its branch and its state. Use it to find out what else is running.",
    "inputSchema": { "type": "object", "properties": {}, "additionalProperties": false },
    "outputSchema": {
      "type": "object",
      "properties": {
        "environments": { "type": "array", "description": "Each environment with its name, root, branch, containers and status" },
        "count": { "type": "integer" }
      }
    },
    "annotations": { "readOnlyHint": true }
  },
  {
    "name": "check_environment",
    "description": "Run the diagnostics (hm doctor) and return each check with its result. Use it when something does not work and you do not know why yet.",
    "inputSchema": {
      "type": "object",
      "properties": {
        "environment": { "type": "string", "description": "A branch environment of this project, by the name hm worktree list gives it. Leave it out for the current directory." }
      },
      "additionalProperties": false
    },
    "outputSchema": {
      "type": "object",
      "properties": {
        "checks": { "type": "array", "description": "Each finding with its id, scope, severity, message and action" },
        "summary": { "type": "object", "description": "How many checks, and how many are ok, warnings and errors" }
      }
    },
    "annotations": { "readOnlyHint": true }
  },
  {
    "name": "environment_status",
    "description": "How the environment is doing right now, in one call (hm status): each container's state and the last error its log reported, whether the URLs answer, xdebug on or off, the deploy mode and whether the database is anonymised. Use it after a change, to see whether something broke.",
    "inputSchema": {
      "type": "object",
      "properties": {
        "environment": { "type": "string", "description": "A branch environment of this project, by the name hm worktree list gives it. Leave it out for the current directory." }
      },
      "additionalProperties": false
    },
    "outputSchema": {
      "type": "object",
      "properties": {
        "project": { "type": "string" },
        "state": { "type": "string", "description": "running, partial or stopped" },
        "mode": { "type": "string", "description": "Magento's deploy mode" },
        "xdebug": { "type": "string", "description": "on, off or unknown" },
        "anonymised": { "type": "string", "description": "yes, or unknown: unknown is never safe" },
        "services": { "type": "array", "description": "Each service with its state, whether it runs and the last error line of its log" },
        "urls": { "type": "array", "description": "Each URL, whether it answers and with which HTTP code" }
      }
    },
    "annotations": { "readOnlyHint": true }
  },
  {
    "name": "service_logs",
    "description": "The last lines of one service's log. Services are named as in the compose file: phpfpm, nginx, db, search, redis, varnish.",
    "inputSchema": {
      "type": "object",
      "properties": {
        "service": { "type": "string", "description": "The service whose log to read" },
        "lines": { "type": "integer", "description": "How many lines from the end (default 100)" },
        "environment": { "type": "string", "description": "A branch environment of this project, by the name hm worktree list gives it. Leave it out for the current directory." }
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
        "sql": { "type": "string", "description": "A single SELECT, SHOW, DESCRIBE or EXPLAIN statement" },
        "environment": { "type": "string", "description": "A branch environment of this project, by the name hm worktree list gives it. Leave it out for the current directory." }
      },
      "required": ["sql"],
      "additionalProperties": false
    },
    "annotations": { "readOnlyHint": true }
  },
  {
    "name": "xdebug",
    "description": "Whether xdebug is on or off in the php container.",
    "inputSchema": {
      "type": "object",
      "properties": {
        "environment": { "type": "string", "description": "A branch environment of this project, by the name hm worktree list gives it. Leave it out for the current directory." }
      },
      "additionalProperties": false
    },
    "outputSchema": {
      "type": "object",
      "properties": { "xdebug": { "type": "string", "description": "on, off or unknown" } }
    },
    "annotations": { "readOnlyHint": true }
  },
  {
    "name": "after_change",
    "description": "Given the files you changed, what Magento needs before it shows them: which cache types to clean, whether di:compile or setup:upgrade is needed, and whether static files are involved. It only says; nothing is run.",
    "inputSchema": {
      "type": "object",
      "properties": {
        "paths": { "type": "array", "items": { "type": "string" }, "description": "The changed files, relative to the project, for example [\"app/code/Acme/Shop/etc/di.xml\"]" },
        "environment": { "type": "string", "description": "A branch environment of this project, by the name hm worktree list gives it. Leave it out for the current directory." }
      },
      "required": ["paths"],
      "additionalProperties": false
    },
    "outputSchema": {
      "type": "object",
      "properties": {
        "cache_types": { "type": "array", "items": { "type": "string" } },
        "di_compile": { "type": "boolean" },
        "setup_upgrade": { "type": "boolean" },
        "static": { "type": "boolean", "description": "pub/static and var/view_preprocessed hold an older copy" },
        "paths": { "type": "array", "description": "Each path with what it needs" },
        "commands": { "type": "array", "items": { "type": "string" }, "description": "The commands that would apply it, in order" }
      }
    },
    "annotations": { "readOnlyHint": true }
  },
  {
    "name": "magento_exceptions",
    "description": "The last lines of var/log/exception.log. Only when the database is anonymised, and only what was logged after it was: before that, the log is about real customers.",
    "inputSchema": {
      "type": "object",
      "properties": {
        "lines": { "type": "integer", "description": "How many lines from the end (default 50, at most 200)" },
        "environment": { "type": "string", "description": "A branch environment of this project, by the name hm worktree list gives it. Leave it out for the current directory." }
      },
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
    "name": "xdebug",
    "description": "Whether xdebug is on or off in the php container, or, with state, turn it on or off (hm debug-on, hm debug-off), which restarts the php container.",
    "inputSchema": {
      "type": "object",
      "properties": {
        "state": { "type": "string", "enum": ["on", "off"], "description": "Turn it on or off. Leave it out to only ask" },
        "environment": { "type": "string", "description": "A branch environment of this project, by the name hm worktree list gives it. Leave it out for the current directory." }
      },
      "additionalProperties": false
    },
    "annotations": { "readOnlyHint": false, "destructiveHint": false, "idempotentHint": true }
  },
  {
    "name": "after_change",
    "description": "Given the files you changed, what Magento needs before it shows them: which cache types to clean, whether di:compile or setup:upgrade is needed, and whether static files are involved. With apply, it cleans those caches and runs di:compile when needed; setup:upgrade only runs when setup_upgrade is true as well.",
    "inputSchema": {
      "type": "object",
      "properties": {
        "paths": { "type": "array", "items": { "type": "string" }, "description": "The changed files, relative to the project, for example [\"app/code/Acme/Shop/etc/di.xml\"]" },
        "apply": { "type": "boolean", "description": "Clean the caches and run di:compile when needed, instead of only saying so" },
        "setup_upgrade": { "type": "boolean", "description": "With apply, also run setup:upgrade when the files need it. It changes the database schema and data" },
        "environment": { "type": "string", "description": "A branch environment of this project, by the name hm worktree list gives it. Leave it out for the current directory." }
      },
      "required": ["paths"],
      "additionalProperties": false
    },
    "annotations": { "readOnlyHint": false, "destructiveHint": true, "idempotentHint": false }
  },
  {
    "name": "run_tests",
    "description": "Run the project's tests, unit or integration, with Pest when the project has vendor/bin/pest and PHPUnit otherwise, optionally only those under a path, and say whether they passed: how many ran, failed, errored and were skipped, and each failure with its message, read from the runner's JUnit report. When there is no report, the exit code and the last lines of the output. It runs the project's code, and the integration suite installs into and empties its own test database.",
    "inputSchema": {
      "type": "object",
      "properties": {
        "suite": { "type": "string", "enum": ["unit", "integration"], "description": "Which suite (default unit)" },
        "path": { "type": "string", "description": "A test file or directory, relative to the Magento root (unit) or to dev/tests/integration (integration), for example app/code/Acme/Shop/Test/Unit" },
        "environment": { "type": "string", "description": "A branch environment of this project, by the name hm worktree list gives it. Leave it out for the current directory." }
      },
      "additionalProperties": false
    },
    "outputSchema": {
      "type": "object",
      "properties": {
        "suite": { "type": "string" },
        "runner": { "type": "string", "enum": ["pest", "phpunit"], "description": "Which ran: pest when the project has vendor/bin/pest, phpunit otherwise" },
        "all_tests_passed": { "type": "boolean", "description": "With a report: whether every test passed, apart from the runner's exit code (a run can fail over a configuration warning with every test green)" },
        "passed": { "type": "boolean" },
        "exit_code": { "type": "integer" },
        "source": { "type": "string", "description": "junit when the counts come from the runner's report, exit code when there was none" },
        "tests": { "type": "integer" },
        "failures": { "type": "integer" },
        "errors": { "type": "integer" },
        "skipped": { "type": "integer" },
        "failed": { "type": "array", "description": "Each failing test with its name, kind and message, up to twenty" },
        "tail": { "type": "string", "description": "The last lines of the output, when the report does not say why" }
      }
    },
    "annotations": { "readOnlyHint": false, "destructiveHint": true, "idempotentHint": false, "openWorldHint": false }
  },
  {
    "name": "cache_flush",
    "description": "Flush the whole Magento cache. Use after a change that a page does not seem to have picked up.",
    "inputSchema": {
      "type": "object",
      "properties": {
        "environment": { "type": "string", "description": "A branch environment of this project, by the name hm worktree list gives it. Leave it out for the current directory." }
      },
      "additionalProperties": false
    },
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
        },
        "environment": { "type": "string", "description": "A branch environment of this project, by the name hm worktree list gives it. Leave it out for the current directory." }
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
        "indexer": { "type": "string", "description": "One indexer, for example catalog_product_price" },
        "environment": { "type": "string", "description": "A branch environment of this project, by the name hm worktree list gives it. Leave it out for the current directory." }
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
        "value": { "type": "string", "description": "The value to set" },
        "environment": { "type": "string", "description": "A branch environment of this project, by the name hm worktree list gives it. Leave it out for the current directory." }
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

	if !write {
		return catalogue
	}

	//
	// A write tool with the name of a read tool takes its place: the same question, which with
	// --write can also do something — xdebug turns it on, after_change applies what it says.
	// Keeping one name keeps one tool to choose by.
	//
	more, _ := parseJQ([]byte(mcpWriteTools))

	for _, tool := range more.items {
		name, _ := tool.field("name")
		replaced := false

		for at, existing := range catalogue.items {
			if called, _ := existing.field("name"); called.text == name.text {
				catalogue.items[at], replaced = tool, true
			}
		}

		if !replaced {
			catalogue.items = append(catalogue.items, tool)
		}
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
