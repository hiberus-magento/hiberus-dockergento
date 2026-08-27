# Tasks

## The transport

- [x] `console/commands/mcp.sh`: read a line, dispatch, answer a line
- [x] `initialize`, `notifications/initialized`, `ping`, `tools/list`, `tools/call`
- [x] JSON-RPC errors for unknown methods and malformed lines, without exiting
- [x] Keep stdout clean: every wrapped command captured, stderr redirected

## The tools

- [x] `describe_project`, `list_environments`, `check_environment`
- [x] `service_logs` with a line cap and a service that must exist
- [x] `database_query`: whitelist, single statement, no file writes, row cap
- [x] Error results the model can read, rather than protocol errors

## Wiring

- [x] `hm mcp --config` prints the client entry and writes nothing
- [x] Register the command with its safety classification and exempt it from project validation

## Verification

- [x] Unit tests for the statement whitelist
- [x] Integration test: a real session over a pipe — initialize, list, call, error cases
- [x] `docs/mcp.md`, changelog and backlog
