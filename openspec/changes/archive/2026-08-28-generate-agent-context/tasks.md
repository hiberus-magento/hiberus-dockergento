# Tasks

## The declaration

- [x] `data/ai-exclusions.json`: path, reason, and whether it is sensitive or just noise
- [x] `hm permissions` emits a `deny` rule per excluded path

## The command

- [x] `console/commands/ai-context.sh` with `--json`
- [x] Generate the block from `hm describe --json`, with no secrets in it
- [x] Replace only what is between the markers; append when there are none
- [x] Create `CLAUDE.md` when absent, never modify one that exists
- [x] Merge the server entry into `.mcp.json`

## Staleness

- [x] Record the tool version and a hash of the facts in the block
- [x] `console/tasks/doctor/96-agent-context.sh`

## Verification

- [x] Unit tests: the block markers, the hash, the exclusion list
- [x] Integration test: generate, regenerate, preserve surrounding content, no secrets
- [x] `docs/ai-context.md`, changelog, backlog (AI-05 and AI-06)
