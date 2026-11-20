# Hiberus Dockergento Constitution

<!--
Sync Impact Report - Constitution Amendment
===========================================
Version: 1.0.0 → 2.0.0 (MAJOR)
Date: 2026-11-20

Reason: hm 2.0 is a Go binary. `bin/run` and the `console/` tree were deleted once the binary
answered all 65 commands, the help, `--version` and project commands. Version 1.0.0 mandated Bash
and `bin/run`, which no longer exist, so its first, second and fourth principles were
fundamentally redefined (MAJOR per the Amendment Process).

Modified Principles:
  - I.   Bash Implementation Consistency → I. Go Implementation and Hexagonal Layers
  - II.  Command Router Architecture     → II. One Router, Thin Adapters
  - III. Docker Abstraction Priority     → III. Docker Abstraction Through Libraries
         (adds: Compose as a library, Docker API, the documented CLI exceptions)
  - IV.  Fail-Fast Error Handling        → IV. Fail-Fast Errors and Stable Contracts
         (`set -euo pipefail` replaced by typed refusals, exit codes, the JSON envelope and
         non-interactive mode)
  - V.   Platform-Specific Optimization  → kept, references moved to the Go code
  - VI.  Configuration Hierarchy Integrity → kept, references moved to the Go code
  - VII. Backward Compatibility          → kept, with the deliberate 2.0 breaks listed

Added Sections:
  - VIII. Test-First Through the Public Interface (NON-NEGOTIABLE)
  - IX.   Safety and Worktree Guardrails
  - Technical Constraints → Release and Installation
  - Quality Standards → Commit and Delivery Rules

Removed Sections:
  - Bash 4.0+ and oh-my-zsh as requirements; `console/helpers` usage rules

Template Sync Status:
  ✅ .specify/templates/plan-template.md - Constitution Check lists the hm 2.0 gates
  ✅ .specify/templates/tasks-template.md - tests are mandatory for behaviour changes in hm
  ✅ .specify/templates/spec-template.md - verified alignment, no change needed
  ✅ .specify/templates/checklist-template.md - verified alignment, no change needed
  ✅ .specify/templates/agent-file-template.md - verified alignment, no change needed
  ✅ CLAUDE.md, ARCHITECTURE.md, architecture/*.md - describe the Go architecture

Follow-up TODOs: None
-->

## Core Principles

### I. Go Implementation and Hexagonal Layers

The tool MUST be implemented in Go, as one binary (`cmd/hm`, built with
`go build -o bin/hm ./cmd/hm`), organised in hexagonal layers whose dependencies point inward:

- `dockergento/core`: domain types and pure rules (compose rendering, properties, refusals,
  naming). No I/O, no Docker, no filesystem.
- `dockergento/app`: use cases, written only against the interfaces in `dockergento/ports`.
- `dockergento/ports`: the interfaces a use case needs from the outside world.
- `dockergento/adapters/*`: the implementations of those ports (`composelib`, `dockerd`,
  `composecli`, `composecfg`, `fsprops`, `gitvcs`, `registry`, `selfrun`, `machine`, `toolinfo`,
  `traefik`, ...).
- `dockergento/dockergento.go`: the facade (`Engine`) that wires adapters into use cases.
- `dockergento/contract`: the shape of every answer (envelope, exit codes), shared by every door.
- `internal/cli` (terminal, TUI, MCP server) and `internal/api` (HTTP) are driving adapters: they
  import the facade, the domain and use-case types and the contract. A direct import of an adapter
  from them is an exception that needs a reason (today only `version_flag.go` uses `gitvcs` and
  `mcp.go` uses `selfrun`).

Shell survives only where it is not the tool: a project's own commands under
`config/hm/commands` (run by the binary), the installer, `generate_completion.sh`,
`build-publish.sh`, the Docker images and the shell test suites in `tests/`.

**Rationale**: One compiled language with typed errors replaced a Bash tree whose correctness
depended on conventions nothing enforced. The layers let the CLI, the TUI, HTTP and MCP share the
same use cases, and let the rules be tested without Docker.

**How to apply**: Put a rule in `core` when it can be a pure function, in `app` when it
orchestrates ports, and in an adapter only when it touches the outside world. Reject proposals to
reintroduce shell or another language for the tool's own behaviour.

### II. One Router, Thin Adapters

Every invocation MUST go through one router:

- `cmd/hm` → `internal/cli.Run`: global flags are taken out, then the switch in
  `internal/cli/run.go` picks the handler (`internal/cli/<command>.go`).
- A name the switch does not route is looked up as a project command
  (`config/hm/commands/<name>.sh`, `internal/cli/project_command.go`); otherwise it is
  `command_not_found` (exit 2). A tool command always wins over a project command.
- Every command is declared in `data/command_descriptions.json` (description, group, options,
  arguments, example). The help (`hm --help`, `hm <command> --help`) and the shell completion
  (`console/hm-completion.bash`, written by `generate_completion.sh`) are generated from it.
- Handlers only parse options, call the engine through the `commands` interface
  (`internal/cli/engine.go`) and render text or the JSON envelope. Behaviour lives in
  `dockergento/app`.
- The TUI, the HTTP API and the MCP server present; they call the same engine or the binary's own
  commands and never re-implement a decision.

**Rationale**: One router keeps the command surface discoverable and the help, completion and
validation consistent. Thin adapters keep each behaviour in one place for every door.

**How to apply**: To add a command: declare it in `data/command_descriptions.json`, add a `case`
in `run.go`, write the handler and its test against the fake engine in `internal/cli`, write the
use case in `dockergento/app` (with a method on the `commands` interface when the handler needs
the engine), add `docs/<command>.md`, and add its row to `MIGRATION.md`'s command table. Never add
a second entry point.

### III. Docker Abstraction Through Libraries

Commands MUST abstract Docker complexity from users:

- User-facing commands are Magento-focused (`hm magento cache:clean`); Docker and Compose are
  implementation details.
- Platform differences (macOS vs Linux) are transparent to users.
- Service versions are chosen automatically from the Magento version.
- Compose is used as a library (`adapters/composelib`, on `github.com/docker/compose/v2`) and the
  daemon through the Docker API (`adapters/dockerd`). The binary MUST NOT shell out to the
  `docker` or `docker compose` CLI, with two documented exceptions: `adapters/composecli`, the
  passthrough behind `hm docker-compose`, and the Compose version probe in `adapters/toolinfo`.
  A project command receives `DOCKER_COMPOSE`, a ready-made Compose command line, for its own use.

**Rationale**: The value of the tool is that developers think about Magento, not containers.
Talking to Compose and Docker as libraries gives typed errors, exit codes callers can branch on and
no dependency on the output format of another CLI.

**How to apply**: If a command requires users to understand Docker concepts, redesign it. A new
container operation is a port method implemented in `composelib` or `dockerd`, not a subprocess.

### IV. Fail-Fast Errors and Stable Contracts (NON-NEGOTIABLE)

Errors MUST stop the operation and say what to do, and every answer MUST follow the contract:

- A deliberate refusal is a typed `core.Refusal` (type, message, hint, exit code), never a bare
  string. Errors are never swallowed silently.
- Exit codes are a contract (`dockergento/contract`): `0` success, `1` error, `2` usage, `3`
  Docker unavailable, `4` not a project, `5` service not running, `6` blocked or refused on
  purpose; `130` when a question is aborted.
- Output: readable text on a terminal, the JSON envelope (`schema_version`, `command`, `ok`,
  `data`/`error`) when stdout is not a terminal; `--json`/`--no-json` force either. Errors go to
  stderr in both formats. Commands whose output is data (`mysqldump`, `exec`, `magento`, ...) are
  never wrapped.
- Non-interactive mode (`--yes` or `HM_NON_INTERACTIVE=1`) never waits for input: a question with
  a default takes it, one without fails with exit `2` naming the option to pass.
- Prerequisites are validated before acting: Docker reachable, a configured project, the services
  a command needs running.

**Rationale**: Scripts, CI jobs and AI agents branch on exit codes and parse the envelope. A tool
that manages databases and code cannot fail silently or hang on a question nobody can answer.

**How to apply**: Return a `core.Refusal` with an actionable hint for every expected failure.
Changing an exit code, an error type or an envelope field is a breaking change (Principle VII).

### V. Platform-Specific Optimization

Platform differences MUST be handled transparently but optimally:

- macOS: the code lives in a named volume with selected bind mounts for speed; Composer's vendor
  is synchronised between host and container; `copy-to-container`/`copy-from-container` exist.
- Linux: a direct bind mount; after start, the host's user and group ids and the self-routing
  hosts entries are applied (`dockergento/app/after_start.go`).
- Detection is automatic (`Platform()` and `Engine.ComposeFiles` in `dockergento/dockergento.go`).
- Platform-specific Compose configuration lives in overlay files (`docker-compose.dev.mac.yml`,
  `docker-compose.dev.linux.yml`).

**Rationale**: macOS and Linux have fundamentally different filesystem performance with Docker.

**How to apply**: When a feature touches volumes or the filesystem, consider both platforms; test
both or document the platform-specific behaviour.

### VI. Configuration Hierarchy Integrity

Configuration MUST follow a strict precedence:

1. Runtime arguments (highest priority)
2. Project configuration (`config/docker/properties.json`)
3. Global defaults (`data/properties.json`)

Additionally:

- Never override user-provided values; changes to configuration require explicit user action.
- A directory without `config/docker/properties.json` is not a project (exit 4).
- `COMPOSE_PROJECT_NAME`, when not configured, is derived from the directory the way Compose
  derives it (`core.DeriveProjectName`); a configured name always wins.
- Persistent state is written atomically: the worktree environment registry is SQLite
  (`adapters/registry`), and slots are handed out in one transaction.

**Rationale**: Users must be confident their customisations are never overwritten unexpectedly.

**How to apply**: Read properties through `adapters/fsprops` and the engine's resolver; never read
the JSON files ad hoc from a handler.

### VII. Backward Compatibility

Changes MUST stay backward compatible unless explicitly documented as breaking:

- Existing commands keep their behaviour, options, exit codes and JSON shapes; new options are
  additive.
- Configuration file formats stay compatible.
- Deprecations are announced with an alternative.

The deliberate breaks of 2.0, documented in `README.md` and `MIGRATION.md`: project commands no
longer receive `HELPERS_DIR`, `TASKS_DIR` or `COMPONENTS_DIR`; the old `config/docker/properties`
format is no longer converted on every invocation; `hm switch` and the installer refuse versions
before 2.0 (the shell tool).

**Rationale**: The tool manages development environments for whole teams. Stability is a feature.

**How to apply**: Before changing existing behaviour, check every caller and test that pins it.
Record breaking changes in the release notes and bump `schema_version` when an envelope changes
shape.

### VIII. Test-First Through the Public Interface (NON-NEGOTIABLE)

Every behaviour change MUST arrive with tests written first:

- RED before GREEN: a failing test for each rule, observed failing, then the implementation.
- Go tests go through the public interface of the layer: `core` with plain inputs, `app` with
  fake ports, `internal/cli` handlers through `Run` with the fake engine
  (`internal/cli/fake_engine_test.go`, swapped in through the `newEngine` variable in
  `internal/cli/engine.go`). Stable output that matters byte for byte is pinned with goldens under
  `testdata/` (for example the MCP catalogue and the agent-context digest).
- Unit tests MUST NOT contact a real Docker daemon. Broad runs set
  `DOCKER_HOST=unix:///nonexistent`, also for `go test ./...`, because `test/e2e` uses whatever
  daemon it finds.
- The shell suites in `tests/` drive the built `bin/hm` from the outside; `./tests/run.sh unit`
  runs the fast part (never `tests/unit` directly). Suites that need Docker never touch projects
  that are not their own.
- A port keeps the contract it replaces: same exit codes, same `--json`, same questions.

**Rationale**: The migration to Go was only safe because each command arrived with its tests and
its contract pinned. Tests through the public interface survive refactoring; tests against a real
daemon damage other people's environments.

**How to apply**: Write the test, see it fail, implement, refactor while green. For every existing
command that shares the code you change, add a test proving its previous behaviour holds.

### IX. Safety and Worktree Guardrails

Commands MUST protect the user's environments and data:

- From a git worktree that has no environment of its own, commands that act on the shared
  environment are refused with exit `6`; `--force` lifts the refusal for one invocation only.
- Destructive operations (`down -v`, `clean`, `db` removals, `docker-stop-all`, `masquerade`) ask
  for confirmation or require an explicit `--force`.
- The MCP server is read-only by default; its bounded write tools exist only with `--write`.
- Tests never run machine-wide commands with a reachable daemon, never write the real
  `/etc/hosts`, and never run `debug-on`/`debug-off`, `masquerade` or `docker-stop-all` against a
  real project.

**Rationale**: Several projects and branch environments share one machine and one daemon. One
wrong command can stop or wipe someone else's work.

**How to apply**: A new command that acts on shared state declares how it behaves from a worktree
and whether it is destructive, and tests both paths.

## Technical Constraints

### Technology Stack Requirements

Required to run `hm`:

- Docker Engine (daemon running) with the Compose v2 plugin (the binary embeds Compose as a
  library; the plugin is used by `hm docker-compose` and by project commands)
- git (the installation is a git checkout; `update` and `switch` move it)

Required to install or build:

- jq (used by the installer and by `generate_completion.sh`, not by the binary)
- Go (version in `go.mod`), only when no prebuilt binary is available for the platform

Optional, used by specific commands: mkcert (`ssl`), fzf (list selection), rsync and ssh
(`transfer-media`, `transfer-db`), database clients (`dbeaver`, `sequelace`, `tableplus`).

### Service Version Compatibility

Service versions MUST align with official Magento compatibility specifications:

- `data/requirements.json` is the authoritative version matrix.
- Versions are selected automatically from the detected Magento version.
- Manual overrides are allowed but validated against the matrix.

### Docker Compose Template System

Docker configuration MUST use the template-overlay pattern:

- Base template: `docker-compose/docker-compose.template.yml`
- Platform overlays: `docker-compose.dev.{mac,linux}.yml`
- Generated files: `docker-compose.yml` plus the overlays, rendered by `core.RenderCompose`
  during `hm setup`.

### Release and Installation

- Releases are tagged `X.Y.Z` (no `v` prefix); CI publishes binaries only for tags `>= 2.0`.
- The installation is a git checkout plus the `bin/hm` binary, downloaded from the release and
  verified against `checksums.txt`, or built with Go.
- `hm switch` and the installer refuse versions before 2.0.

## Quality Standards

### Code Quality Requirements

All code contributions MUST:

- Pass `gofmt` (on `./cmd`, `./internal`, `./dockergento`) and `go vet ./...`, as CI enforces.
- Follow existing patterns and reuse the existing function for shared state instead of
  re-implementing it.
- Use descriptive names; comment the why of non-obvious decisions.
- Validate user input before use.

### Testing Requirements

Changes MUST pass:

- `DOCKER_HOST=unix:///nonexistent go test ./...`
- `DOCKER_HOST=unix:///nonexistent ./tests/run.sh unit`
- The integration suites in `tests/integration` that cover the area, when Docker is available and
  only against their own throwaway projects.

Integration focus areas: routing and help, Compose generation and startup, platform overlays,
database operations, worktree environments, configuration loading and persistence.

### Documentation Requirements

All user-facing changes MUST include:

- `docs/<command>.md` for the command
- `README.md` when adding a feature
- `data/command_descriptions.json` for help and completion
- `ARCHITECTURE.md`/`architecture/*.md` when changing core patterns
- `MIGRATION.md` and `ROADMAP.md` when changing the state they track

Universal claims ("never", "every") in documentation are checked against the code first.

### Commit and Delivery Rules

- Commits are in English, follow Conventional Commits and carry no AI attribution (no
  `Co-Authored-By`, no session trailers).
- Push, pull requests, merges and releases are decided by a human.

## Governance

### Constitution Authority

This Constitution supersedes all other development practices and guidelines. When conflicts arise:

1. Constitution principles take precedence
2. Architecture documentation provides implementation guidance
3. Code comments and existing patterns serve as examples

### Amendment Process

Constitution amendments require:

1. Documented justification (what problem does this solve?)
2. Impact analysis on existing principles
3. Migration plan for affected code
4. Version bump following semantic versioning:
   - **MAJOR**: Removes or fundamentally redefines principles
   - **MINOR**: Adds new principles or materially expands guidance
   - **PATCH**: Clarifies wording, fixes typos, refines non-semantic details

### Compliance and Review

All contributions MUST be reviewed for constitutional compliance:

- Changes must demonstrate adherence to the core principles
- Breaking a principle requires explicit justification and approval
- Added complexity must be justified with clear benefits
- Use `ARCHITECTURE.md` and `CLAUDE.md` for runtime development guidance

### Continuous Improvement

Constitutional principles should evolve based on:

- Lessons learned from production issues
- User feedback and pain points
- Technical debt patterns that emerge (tracked in `ROADMAP.md`)
- New platform or technology requirements

**Version**: 2.0.0 | **Ratified**: 2026-04-01 | **Last Amended**: 2026-11-20
