# Implementation Plan: Add Magento 2.4.8 Support and Image Configuration in Requirements

**Branch**: `003-add-m248-image-config` | **Date**: 2026-04-01 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/003-add-m248-image-config/spec.md`

## Summary

Add Magento 2.4.8 as a new supported version (with all released security patches mapped), backfill missing security patch equivalences for 2.4.4–2.4.7, and refactor the docker-compose template generation system so that every service's Docker image is controlled via `data/requirements.json` instead of being partially hard-coded in the template. For 2.4.8 the stack is: PHP 8.4 (hiberusmagento), MariaDB 11.4 (official), OpenSearch 2.12 (hiberusmagento), Redis 7.2 (hiberusmagento), RabbitMQ 4.1-management (official), Varnish 7.1 (hiberusmagento).

## Technical Context

**Language/Version**: Bash 4.0+
**Primary Dependencies**: `jq` (JSON parsing), `sed` (template substitution), `docker`/`docker compose` (runtime)
**Storage**: `data/requirements.json`, `data/equivalent_versions.json` (JSON data files); `docker-compose/docker-compose.template.yml` (YAML template)
**Testing**: Manual — run `hm setup` against projects with specific versions in `composer.lock`; diff generated `docker-compose.yml` against expected output
**Target Platform**: macOS and Linux (both handled by the tool)
**Project Type**: CLI tool / Docker orchestration wrapper
**Performance Goals**: N/A — template generation is a one-time file write, no latency requirements
**Constraints**: `sed` delimiter must not conflict with `/` in image names; `|` chosen as safe alternative. No breaking changes to existing generated output for versions 2.3.x–2.4.7.
**Scale/Scope**: 10 existing version buckets + 1 new (2.4.8); 46 new equivalence mappings; 6 files modified; 1 script logic change; 1 script rewrite; 1 documentation update

## Constitution Check

| Principle | Status | Notes |
|---|---|---|
| I. Bash Implementation Consistency | PASS | All changes are in `.sh` scripts and `.json` data files. No new languages introduced. |
| II. Command Router Architecture | PASS | No new commands added. Existing routing unchanged. |
| III. Docker Abstraction Priority | PASS | Image resolution logic remains transparent to users. |
| IV. Fail-Fast Error Handling | PASS | `write_from_docker-compose_templates.sh` already uses `set -euo pipefail`. Changes must preserve this. |
| V. Platform-Specific Optimization | PASS | Changes to base template only. Mac/Linux overlays are untouched. |
| VI. Configuration Hierarchy Integrity | PASS | `requirements.json` is the authoritative source. No user-overridable values change. |
| VII. Backward Compatibility | PASS | All existing version buckets produce identical output. The image-resolution logic is additive. |

**Gate result: PASS — no violations.** No Complexity Tracking table required.

## Project Structure

### Documentation (this feature)

```text
specs/003-add-m248-image-config/
├── plan.md              ← this file
├── research.md          ← Phase 0 output
├── data-model.md        ← Phase 1 output
├── quickstart.md        ← Phase 1 output
├── contracts/
│   └── requirements-schema.md  ← Phase 1 output
└── tasks.md             ← Phase 2 output (/speckit.tasks — NOT created here)
```

### Source Code (repository root)

```text
data/
├── requirements.json          ← ADD 2.4.8 bucket + nginx/mailhog/rabbitmq/hitch to all buckets
└── equivalent_versions.json   ← ADD 46 new patch mappings

docker-compose/
└── docker-compose.template.yml ← REPLACE 4 hard-coded image: lines with <x_version> placeholders

console/tasks/
└── write_from_docker-compose_templates.sh ← UPDATE compose_regex() sed delimiter + image resolution logic

console/commands/
└── compatibility.sh ← REWRITE tabla estática → generación dinámica desde data/

README.md ← UPDATE versiones de imágenes + 2.4.8 + enlace ai-tools + mejora visual

console/commands/
└── create-project.sh ← FIX composer create-project non-empty dir (Phase 8)
```

**Structure Decision**: Single-project, data-driven CLI tool. No new directories needed — all changes are modifications to existing files within the established architecture.

## Phase 6: Command — Rewrite `compatibility.sh` Dynamic Table

**Goal**: Replace the hardcoded ASCII table in `console/commands/compatibility.sh` with a script that reads `data/requirements.json` (to obtain canonical version buckets) and `data/equivalent_versions.json` (to enumerate all mapped versions per minor), then constructs the 2.3.x and 2.4.x columns programmatically and passes them to `print_table`.

**Approach**:
1. Read all canonical bucket keys from `data/requirements.json` using `jq`.
2. Group all keys from `data/equivalent_versions.json` by their target bucket (e.g. all entries pointing to `"2.4.8"` → column cell under `2.4.8`).
3. Split buckets into two column groups: `2.3.x` and `2.4.x` (and future major lines as needed).
4. Determine the maximum row count across all columns and pad shorter columns with empty cells.
5. Build the table string and pass it to `print_table`.

**Constraints**:
- Output must be visually equivalent to the current hardcoded table for all versions 2.3.0–2.4.7 (zero regression).
- Uses only `jq`, `bash` arrays, and `printf` — no external dependencies beyond what already exists in the tool.
- `set -euo pipefail` must be present.

## Phase 8: Bug fix — `create-project` non-empty directory

**Goal**: Fix `hm create-project` so that `composer create-project` succeeds even though the container's working directory (`/var/www/html`) is not empty at the time Composer runs (it already contains docker-compose config files mounted from the host).

**Root cause**: `create_project_execute()` in `console/commands/create-project.sh` calls `exec.sh composer create-project ... "."`, which targets `/var/www/html` inside the container. By the time this runs, `docker compose up -d` has already started services and the bind mount exposes the host directory (containing docker-compose files) inside the container, making the directory non-empty. Composer refuses to create a project into a non-empty directory.

**Approach** (host pre-creation + temp dir + copy):
1. **Pre-create placeholder files on the host** before `docker compose up`:
   ```bash
   touch "$MAGENTO_DIR/composer.json"
   touch "$MAGENTO_DIR/composer.lock"
   ```
   On macOS, Docker Desktop creates missing bind-mount source paths as **empty directories** inside the container (Linux does not). These ghost directories are active kernel-level bind-mount targets — they cannot be removed from inside the container (`rm -rf` returns `EBUSY`). Pre-creating real files on the host before Docker starts prevents this: Docker sees real files and mounts them as files. This is a no-op for projects that already have these files.
2. Run `docker compose up -d`.
3. Run `composer create-project` into `/tmp/magento-new` inside the container.
4. Copy `/tmp/magento-new/.` into `$WORKDIR_PHP/` — this merges Magento project files alongside existing config files (`cp` overwrites file-with-file, which is valid).
5. Remove `/tmp/magento-new` from inside the container.

**File affected**: `console/commands/create-project.sh` (lines 38–41 — the `exec.sh composer create-project` block).

**No other files need changes**: the `docker cp` call on line 44 already targets `$WORKDIR_PHP` as source, which remains correct.