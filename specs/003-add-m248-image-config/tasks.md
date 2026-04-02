# Tasks: Add Magento 2.4.8 Support and Image Configuration in Requirements

**Input**: Design documents from `/specs/003-add-m248-image-config/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/requirements-schema.md

**Tests**: Manual — run `hm setup` against projects with specific versions in `composer.lock`; diff generated `docker-compose.yml` against expected output.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- Data files: `data/`
- Docker Compose template: `docker-compose/`
- Task scripts: `console/tasks/`

---

## Phase 1: Data — Equivalent Version Mappings

**Purpose**: Add all missing security patch equivalences and the new 2.4.8 bucket mappings to `data/equivalent_versions.json`.

**[P]** tasks in this phase are all edits to the same file; they can be batched but have no inter-task code dependencies.

- [x] T001 [P] [US2] Add `2.4.4-p6` through `2.4.4-p17` (12 entries) to `data/equivalent_versions.json`, all mapping to `"2.4.4"`
- [x] T002 [P] [US2] Add `2.4.5-p5` through `2.4.5-p16` (12 entries) to `data/equivalent_versions.json`, all mapping to `"2.4.5"`
- [x] T003 [P] [US2] Add `2.4.6-p4` through `2.4.6-p14` (11 entries) to `data/equivalent_versions.json`, all mapping to `"2.4.6"`
- [x] T004 [P] [US2] Add `2.4.7-p4` through `2.4.7-p9` (6 entries) to `data/equivalent_versions.json`, all mapping to `"2.4.7"`
- [x] T005 [P] [US1] Add `2.4.8`, `2.4.8-p1`, `2.4.8-p2`, `2.4.8-p3`, `2.4.8-p4` (5 entries) to `data/equivalent_versions.json`, all mapping to `"2.4.8"`

**Checkpoint**: `data/equivalent_versions.json` now contains all 46 new entries. Run `jq 'keys | length' data/equivalent_versions.json` — expected value: 94 (48 existing + 46 new).

---

## Phase 2: Data — Requirements Version Buckets

**Purpose**: Add the `nginx`, `mailhog`, `rabbitmq`, `hitch` keys to all existing buckets and add the new `2.4.8` bucket to `data/requirements.json`.

- [x] T006 [P] [US3] Add `"nginx": "hiberusmagento/nginx:1.18"`, `"mailhog": "hiberusmagento/mailhog:1"`, `"rabbitmq": "hiberusmagento/rabbitmq:3.9"`, `"hitch": "hiberusmagento/hitch:1.7"` to the `2.3.0` bucket in `data/requirements.json`
- [x] T007 [P] [US3] Add `"nginx": "hiberusmagento/nginx:1.18"`, `"mailhog": "hiberusmagento/mailhog:1"`, `"rabbitmq": "hiberusmagento/rabbitmq:3.9"`, `"hitch": "hiberusmagento/hitch:1.7"` to the `2.3.3` bucket in `data/requirements.json`
- [x] T008 [P] [US3] Add `"nginx": "hiberusmagento/nginx:1.18"`, `"mailhog": "hiberusmagento/mailhog:1"`, `"rabbitmq": "hiberusmagento/rabbitmq:3.9"`, `"hitch": "hiberusmagento/hitch:1.7"` to the `2.3.5` bucket in `data/requirements.json`
- [x] T009 [P] [US3] Add `"nginx": "hiberusmagento/nginx:1.18"`, `"mailhog": "hiberusmagento/mailhog:1"`, `"rabbitmq": "hiberusmagento/rabbitmq:3.9"`, `"hitch": "hiberusmagento/hitch:1.7"` to the `2.3.7` bucket in `data/requirements.json`
- [x] T010 [P] [US3] Add `"nginx": "hiberusmagento/nginx:1.18"`, `"mailhog": "hiberusmagento/mailhog:1"`, `"rabbitmq": "hiberusmagento/rabbitmq:3.9"`, `"hitch": "hiberusmagento/hitch:1.7"` to the `2.4.0` bucket in `data/requirements.json`
- [x] T011 [P] [US3] Add `"nginx": "hiberusmagento/nginx:1.18"`, `"mailhog": "hiberusmagento/mailhog:1"`, `"rabbitmq": "hiberusmagento/rabbitmq:3.9"`, `"hitch": "hiberusmagento/hitch:1.7"` to the `2.4.2` bucket in `data/requirements.json`
- [x] T012 [P] [US3] Add `"nginx": "hiberusmagento/nginx:1.18"`, `"mailhog": "hiberusmagento/mailhog:1"`, `"rabbitmq": "hiberusmagento/rabbitmq:3.9"`, `"hitch": "hiberusmagento/hitch:1.7"` to the `2.4.4` bucket in `data/requirements.json`
- [x] T013 [P] [US3] Add `"nginx": "hiberusmagento/nginx:1.18"`, `"mailhog": "hiberusmagento/mailhog:1"`, `"rabbitmq": "hiberusmagento/rabbitmq:3.9"`, `"hitch": "hiberusmagento/hitch:1.7"` to the `2.4.5` bucket in `data/requirements.json`
- [x] T014 [P] [US3] Add `"nginx": "hiberusmagento/nginx:1.18"`, `"mailhog": "hiberusmagento/mailhog:1"`, `"rabbitmq": "hiberusmagento/rabbitmq:3.9"`, `"hitch": "hiberusmagento/hitch:1.7"` to the `2.4.6` bucket in `data/requirements.json`
- [x] T015 [P] [US3] Add `"nginx": "hiberusmagento/nginx:1.18"`, `"mailhog": "hiberusmagento/mailhog:1"`, `"rabbitmq": "hiberusmagento/rabbitmq:3.9"`, `"hitch": "hiberusmagento/hitch:1.7"` to the `2.4.7` bucket in `data/requirements.json`
- [x] T016 [US1] Add the complete `2.4.8` bucket to `data/requirements.json` (after all existing buckets):
  ```json
  "2.4.8": {
      "php":      "8.4-bookworm",
      "mariadb":  "mariadb:11.4",
      "search":   "2.12-opensearch",
      "redis":    "7.2",
      "varnish":  "7.1",
      "composer": "2.9",
      "nginx":    "hiberusmagento/nginx:1.18",
      "mailhog":  "hiberusmagento/mailhog:1",
      "rabbitmq": "rabbitmq:4.1-management",
      "hitch":    "hiberusmagento/hitch:1.7"
  }
  ```

**Checkpoint**: `data/requirements.json` now has 11 version buckets. Validate with `jq 'keys | length' data/requirements.json` — expected: 11. Validate each bucket has 10 keys: `jq 'to_entries | map(.value | keys | length)' data/requirements.json` — every element must be 10.

---

## Phase 3: Template — Replace Hard-coded Image Lines

**Purpose**: Replace the four hard-coded `image:` values in `docker-compose/docker-compose.template.yml` with `<key_version>` placeholders so they are driven by `data/requirements.json`.

- [x] T017 [P] [US3] In `docker-compose/docker-compose.template.yml`, replace `image: hiberusmagento/nginx:1.18` (line 20) with `image: <nginx_version>`
- [x] T018 [P] [US3] In `docker-compose/docker-compose.template.yml`, replace `image: hiberusmagento/mailhog:1` (line 66) with `image: <mailhog_version>`
- [x] T019 [P] [US3] In `docker-compose/docker-compose.template.yml`, replace `image: hiberusmagento/rabbitmq:3.9` (line 71) with `image: <rabbitmq_version>`
- [x] T020 [P] [US3] In `docker-compose/docker-compose.template.yml`, replace `image: hiberusmagento/hitch:1.7` (line 82) with `image: <hitch_version>`

**Checkpoint**: Verify the template contains no remaining hard-coded `hiberusmagento/nginx`, `hiberusmagento/mailhog`, `hiberusmagento/rabbitmq`, or `hiberusmagento/hitch` image lines. Run `grep -n 'image:' docker-compose/docker-compose.template.yml` — all `image:` lines should now use `<..._version>` placeholders or `hiberusmagento/php:`, `hiberusmagento/mariadb:`, etc. (the already-templated ones). After this phase all eight service image lines use placeholders.

---

## Phase 4: Script — Update `compose_regex()` Logic

**Purpose**: Update `console/tasks/write_from_docker-compose_templates.sh` to (a) change the `sed` delimiter from `/` to `|` and (b) implement the fully-qualified vs plain-tag image resolution rule.

- [x] T021 [US3] In `console/tasks/write_from_docker-compose_templates.sh`, rewrite `compose_regex()` to implement the `:` heuristic and use `|` as the `sed` delimiter:

  ```bash
  compose_regex() {
      local services
      services=$(echo "$REQUIREMENTS" | jq -r 'keys | join(" ")')
      for index in $services; do
          value=$(echo "$REQUIREMENTS" | jq -r '.["'"$index"'"]')
          if [[ "$value" == *":"* ]]; then
              image="$value"
          else
              image="hiberusmagento/${index}:${value}"
          fi
          regex+="s|<${index}_version>|${image}|g; "
      done
  }
  ```

  Also update the `sed` call in `write_docker_compose()` if the delimiter change requires any adjustment (it does not — `sed` accepts the expression as-is).

**Checkpoint**: The function now uses `|` as delimiter throughout. The `jq` call uses array-style key access (`.["key"]`) to handle keys with special characters safely. Confirm `set -euo pipefail` is still present at the top of the file.

---

## Phase 5: Validation

**Purpose**: Manual end-to-end verification of all three user stories and regression check for existing versions.

- [x] T022 [US1] Verify Magento 2.4.8 setup: simulate `REQUIREMENTS` for `2.4.8` bucket, run `compose_regex()` manually or via `hm setup` on a test project, confirm `docker-compose.yml` contains `image: hiberusmagento/php:8.4-bookworm`, `image: mariadb:11.4`, `image: hiberusmagento/search:2.12-opensearch`, `image: hiberusmagento/redis:7.2`, `image: rabbitmq:4.1-management`, `image: hiberusmagento/varnish:7.1`, `image: hiberusmagento/nginx:1.18`, `image: hiberusmagento/mailhog:1`, `image: hiberusmagento/hitch:1.7`
- [x] T023 [US2] Verify security patch resolution: confirm `jq '."2.4.6-p10"' data/equivalent_versions.json` returns `"2.4.6"` and `jq '."2.4.8-p3"' data/equivalent_versions.json` returns `"2.4.8"`
- [x] T024 [US3] Regression check: simulate `REQUIREMENTS` for `2.4.7` bucket, confirm generated `docker-compose.yml` is identical to pre-change output — specifically `image: hiberusmagento/rabbitmq:3.9`, `image: hiberusmagento/nginx:1.18`, `image: hiberusmagento/mailhog:1`, `image: hiberusmagento/hitch:1.7`
- [x] T025 [US3] Regression check: simulate `REQUIREMENTS` for `2.4.6` bucket and confirm `image: hiberusmagento/mariadb:10.6` (plain-tag path, no `:` in value)
- [x] T026 Edge case: verify `jq` is valid on both data files — `jq '.' data/requirements.json` and `jq '.' data/equivalent_versions.json` must both exit 0

---

## Phase 6: Command — Rewrite `compatibility.sh` Dynamic Table

**Purpose**: Replace the hardcoded ASCII table in `console/commands/compatibility.sh` with a data-driven implementation that reads version buckets from `data/requirements.json` and all mapped versions from `data/equivalent_versions.json`, producing a visually equivalent table that automatically includes new versions as they are added to the data files.

**Depends on**: Phases 1–2 complete (data files must contain all version buckets and equivalences).

- [x] T027 [US4] Rewrite `console/commands/compatibility.sh` to generate the supported-versions table dynamically:
  - Read canonical bucket keys from `data/requirements.json` using `jq keys[]`
  - Group all entries from `data/equivalent_versions.json` by their target bucket value to enumerate versions per minor
  - Split buckets into two column groups: `2.3.x` (buckets matching `2.3.*`) and `2.4.x` (buckets matching `2.4.*`)
  - Determine the maximum row count across all columns and pad shorter columns with empty cells
  - Build the table string and pass it to `print_table` (sourced from `$COMPONENTS_DIR/print_message.sh`)
  - Ensure `set -euo pipefail` is present; use only `jq` and standard Bash — no new dependencies
  - Output must be visually equivalent to the current hardcoded table for versions 2.3.0–2.4.7

- [x] T028 [US4] Validation — dynamic compatibility table:
  - Run `bash console/commands/compatibility.sh` (or `hm compatibility`) and verify the table includes `2.4.8` and its security patches (`2.4.8-p1` through `2.4.8-p4`)
  - Verify all existing 2.3.x and 2.4.0–2.4.7 versions still appear correctly (regression check)
  - Confirm the script contains no hardcoded version strings in the table-building logic

**Checkpoint**: `console/commands/compatibility.sh` contains no hardcoded version table. Running the script produces a table that includes `2.4.8` and its patches. Diff the table columns against the previous hardcoded output for versions 2.3.0–2.4.7 — they must be identical.

---

## Phase 7: Documentation — Update README.md

**Purpose**: Update the project README to reflect the new versions introduced in this feature, improve visual presentation, and add a link to the Hiberus Magento AI Tools sibling repository.

**Depends on**: Phases 1–2 (data files must be complete to ensure version lists are accurate).

- [x] T029 [US1] Update `README.md`:
  - **Docker images section**: add PHP `8.4`, MariaDB `11.4` (official, no Hiberus wrapper), OpenSearch `2.12`, RabbitMQ `4.1`, Redis `7.2` to their respective image lists; retain all existing versions.
  - **Magento compatible versions section**: add `2.4.8` to the `2.4.x` list.
  - **Badges**: add badge row in the header for licence (GPLv3), DockerHub link, and latest supported Magento version.
  - **Visual improvements**: add section emoji/icons, improve spacing and readability; convert the flat commands list to a more scannable format if appropriate.
  - **Ecosystem / Related Projects section**: add a new section (before "Thanks to") with a link and description of [Hiberus Magento AI Tools](https://github.com/hiberus-magento/ai-tools) — "AI-powered skills and agents for Magento 2 that extend AI coding assistants (Claude, Copilot, Cursor, Gemini, etc.) with expert Magento knowledge. Dockergento integrates with ai-tools to provide an agile, AI-assisted development workflow."

---

## Phase 8: Bug fix — `create-project` non-empty directory

**Purpose**: Fix `hm create-project` so that `composer create-project` succeeds even when the container's working directory (`/var/www/html`) already contains files from the docker-compose setup step. Use a temp dir inside the container as the Composer target, then copy results into `WORKDIR_PHP`.

**Depends on**: No dependencies on Phases 1–7 — this is an isolated bug fix to `create-project.sh`.

- [x] T030 [US1] Fix `console/commands/create-project.sh`: replace the `exec.sh composer create-project ... "."` block with the following pattern:
  1. Pre-create placeholder files on the host **before** `$DOCKER_COMPOSE up -d` so Docker mounts them as files (not ghost directories on macOS):
     ```bash
     # Pre-create placeholder files so Docker Desktop mounts them as files, not ghost directories
     touch "$MAGENTO_DIR/composer.json"
     touch "$MAGENTO_DIR/composer.lock"
     ```
  2. Run `$DOCKER_COMPOSE up -d` (existing call — unchanged, just placed after the touch lines).
  3. Run `composer create-project` into `/tmp/magento-new` inside the container:
     ```bash
     "$COMMANDS_DIR"/exec.sh composer create-project \
         --no-install \
         --repository=https://repo.magento.com/ \
         magento/project-"$MAGENTO_EDITION"-edition="$MAGENTO_VERSION" /tmp/magento-new
     ```
  4. Copy all files from the temp dir into `WORKDIR_PHP`:
     ```bash
     "$COMMANDS_DIR"/exec.sh sh -c "cp -R /tmp/magento-new/. $WORKDIR_PHP/"
     ```
  5. Remove the temp dir:
     ```bash
     "$COMMANDS_DIR"/exec.sh rm -rf /tmp/magento-new
     ```
  Ensure `set -euo pipefail` remains at the top of the file. Remove any `exec.sh sh -c "[ -d ... ] && rm -rf ..."` ghost-dir cleanup lines that may be present from a previous iteration — these cannot work (EBUSY) and must be deleted.

- [x] T031 [US1] Validation — run `hm create-project` on a fresh empty directory selecting Magento 2.4.8-p4 and confirm:
  - No "Project directory is not empty" error from Composer
  - All 9 Docker services start successfully
  - `composer.json` and `composer.lock` are present in the host directory after the command completes
  - Containers remain running after the command

- [ ] T032 [US1] Validation (macOS) — run `hm create-project` on a fresh empty directory on macOS (Docker Desktop) and confirm:
  - No `cp: cannot overwrite directory with non-directory` error
  - `composer.json` in the host directory is a **file**, not a directory
  - All 9 containers remain running after the command

**Checkpoint**: `hm create-project` completes end-to-end without the "directory not empty" error. The `composer.json` written by Composer is available on the host via `docker cp`. The three-step pattern (create → copy → cleanup) leaves no orphaned temp dirs in the container.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Equivalent versions)**: No dependencies — start immediately
- **Phase 2 (Requirements buckets)**: No dependencies — start immediately; can run in parallel with Phase 1
- **Phase 3 (Template placeholders)**: No dependencies — start immediately; can run in parallel with Phases 1–2
- **Phase 4 (Script logic)**: Depends on Phase 3 completing (template must have placeholders before testing the script path); can be prepared in parallel but only validated after Phase 3
- **Phase 5 (Validation)**: Depends on all of Phases 1–4 being complete
- **Phase 6 (Dynamic compatibility table)**: Depends on Phases 1–2 (data files must be complete); independent of Phases 3–5
- **Phase 7 (README update)**: Depends on Phases 1–2 (needs final version/image lists to be accurate); independent of Phases 3–6
- **Phase 8 (Bug fix: create-project)**: No dependencies on Phases 1–7 — isolated fix to `create-project.sh`

### Parallel Opportunities

**Phases 1, 2, 3** can all begin simultaneously — they touch separate files:
- Phase 1 → `data/equivalent_versions.json`
- Phase 2 → `data/requirements.json`
- Phase 3 → `docker-compose/docker-compose.template.yml`
- Phase 4 → `console/tasks/write_from_docker-compose_templates.sh`

Within Phase 2, T006–T016 all edit the same file but have no logical dependencies on each other and can be applied as a single batched edit.

---

## Notes

- The `sed` delimiter change (` /` → `|`) is the only script behaviour change. All other substitution mechanics are preserved.
- The `:` heuristic is unambiguous: no existing plain-tag value in `requirements.json` contains `:`. See research.md §3.
- The `composer` key value (`2.9`, `2.7`, etc.) does not contain `:`, so it is processed by the plain-tag branch, producing `hiberusmagento/composer:2.9` — but since no `image: <composer_version>` line exists in the template, this substitution is a no-op and harmless.
- All changes are additive or substitutive with no removal of existing data; backward compatibility is guaranteed for 2.3.x–2.4.7.
- `set -euo pipefail` must remain at the top of `write_from_docker-compose_templates.sh` (Constitutional requirement IV).
