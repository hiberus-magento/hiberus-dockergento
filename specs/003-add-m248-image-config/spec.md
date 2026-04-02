# Feature Specification: Add Magento 2.4.8 Support and Image Configuration in Requirements

**Feature Branch**: `003-add-m248-image-config`  
**Created**: 2026-04-01  
**Status**: Draft  
**Input**: User description: "Necesito que incluyamos en el proyecto la nuevas versiones de Magento: 2.4.8 y los parches de seguridad en forma de equivalencias. Tambien quiero que cambiemos el docker-compose template ya que actualmente en ciertos servicios está fijo el campo image y en otros siempre toma el de hiberusmagento dockerhub, quiero que esa parte del image la movamos al fichero data/requirements.json de esa forma damos más flexibilidad a las versiones de poder gestionar el image que queremos o recomendamos utilizar. Para 2.4.8: PHP 8.4 hiberusmagento, MariaDB 11.4 oficial, OpenSearch 2.12 hiberusmagento, RabbitMQ 4.1, y Valkey en lugar de Redis."

## Clarifications

### Session 2026-04-01

- Q: Which Docker image should be used for RabbitMQ 4.1 in the 2.4.8 bucket? → A: `rabbitmq:4.1-management` (official image with management UI; no Hiberus wrapper needed)
- Q: Should Valkey replace Redis for the 2.4.8 bucket? → A: No — keep Redis 7.2 for 2.4.8; Valkey introduces too much change and breaks existing workflows

## Iterations

### Iteration 2026-04-02: Dynamic compatibility table

**Change**: Añadir FR-012 para que `console/commands/compatibility.sh` genere la tabla de versiones soportadas dinámicamente desde `data/requirements.json` y `data/equivalent_versions.json`, eliminando la tabla hardcodeada.
**Scope**: Feature-wide
**Artifacts updated**: spec.md, plan.md, tasks.md, quickstart.md
**Tasks added**: T027, T028
**Tasks removed**: —
**Tasks marked complete**: —

### Iteration 2026-04-02: README update

**Change**: Actualizar `README.md` con versiones nuevas (2.4.8, PHP 8.4, MariaDB 11.4, RabbitMQ 4.1, OpenSearch 2.12), mejorar presentación visual con badges y secciones, y añadir enlace al repositorio hermano Hiberus Magento AI Tools.
**Scope**: Task-level
**Artifacts updated**: plan.md, tasks.md
**Tasks added**: T029
**Tasks removed**: —
**Tasks marked complete**: —

### Iteration 2026-04-02: Fix create-project macOS ghost directories (v2 — definitive)

**Change**: Previous iteration's fix (in-container `rm -rf` of ghost dirs) fails with EBUSY because ghost directories are active kernel-level bind-mount targets and cannot be removed from inside the container. Definitive fix: pre-create `composer.json` and `composer.lock` as real empty files on the host before `docker compose up`, so Docker never creates ghost directories.
**Scope**: Task-level
**Artifacts updated**: spec.md, plan.md, tasks.md, research.md, quickstart.md
**Tasks added**: —
**Tasks removed**: —
**Tasks marked complete**: —

### Iteration 2026-04-02: Fix create-project macOS ghost directories

**Change**: Fix `hm create-project` macOS-specific failure where Docker Desktop creates `composer.json` and `composer.lock` as empty ghost directories inside the container (because the host bind-mount source files don't exist yet at container start), causing `cp -R` to fail with "cannot overwrite directory with non-directory". Fix: add two guard lines to remove ghost dirs before the copy step.
**Scope**: Task-level
**Artifacts updated**: spec.md, plan.md, tasks.md, research.md, quickstart.md
**Tasks added**: T032
**Tasks removed**: —
**Tasks marked complete**: —

### Iteration 2026-04-02: Fix create-project non-empty dir

**Change**: Fix `hm create-project` so that `composer create-project` succeeds even though the container working directory already contains docker-compose config files — by running into a temp dir and copying results back.
**Scope**: Task-level
**Artifacts updated**: spec.md, plan.md, tasks.md, quickstart.md
**Tasks added**: T030, T031
**Tasks removed**: —
**Tasks marked complete**: —

---

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Developer sets up a Magento 2.4.8 project (Priority: P1)

A developer creates a new Magento 2.4.8 project or runs `hm setup` on an existing 2.4.8 repository. The tool automatically detects the version, resolves the correct service requirements, and generates a fully functional `docker-compose.yml` using PHP 8.4, MariaDB 11.4, OpenSearch 2.12, Redis 7.2, RabbitMQ 4.1, and Varnish 7.x.

**Why this priority**: Without this, the tool cannot support Magento 2.4.8 at all — the most current actively-supported release line.

**Independent Test**: Run `hm setup` on a project with `magento/product-community-edition 2.4.8` in `composer.lock`. Verify that `docker-compose.yml` is generated with `hiberusmagento/php:8.4-*`, `mariadb:11.4`, `hiberusmagento/search:2.12-opensearch`, `hiberusmagento/redis:7.2`, and `rabbitmq:4.1-management`.

**Acceptance Scenarios**:

1. **Given** a project with Magento 2.4.8 in `composer.lock`, **When** the developer runs `hm setup`, **Then** the tool generates a `docker-compose.yml` using the service images defined for the `2.4.8` bucket in `requirements.json`.
2. **Given** a project with a 2.4.8 security patch (e.g. `2.4.8-p1`) in `composer.lock`, **When** the developer runs `hm setup`, **Then** the tool resolves the version to the `2.4.8` bucket and generates the correct `docker-compose.yml`.
3. **Given** all four 2.4.8 security patches (`2.4.8-p1` through `2.4.8-p4`) exist in `equivalent_versions.json`, **When** any of those versions is detected, **Then** the system uses the same `2.4.8` bucket requirements without error.
4. **Given** a developer runs `hm create-project` and selects version 2.4.8, **When** the project is created, **Then** the correct service images are used.
    5. **Given** a fresh empty host directory, **When** the developer runs `hm create-project`, **Then** Docker services start successfully, Composer creates the Magento project without a "directory not empty" error, and the project files are present in `WORKDIR_PHP` after the command completes.
    6. **Given** a fresh empty host directory on macOS, **When** the developer runs `hm create-project`, **Then** no `cp: cannot overwrite directory with non-directory` error occurs and `composer.json` in the host directory is a file, not a directory.

---

### User Story 2 - Developer uses a project with a missing security patch equivalence (Priority: P2)

A developer runs `hm setup` on a project that uses a security patch version of 2.4.4, 2.4.5, 2.4.6, or 2.4.7 that is not yet present in `equivalent_versions.json`. The tool fails to resolve the version and produces an error or silently falls back to a wrong environment.

**Why this priority**: A large number of active projects use recently-released security patches that are not yet mapped, causing broken environments. All currently released patches for supported versions must be present.

**Independent Test**: Run `hm setup` on a project with `magento/product-community-edition 2.4.6-p10` in `composer.lock`. Verify it resolves to the `2.4.6` bucket and generates a valid `docker-compose.yml` without error.

**Acceptance Scenarios**:

1. **Given** a project with any of `2.4.4-p6` through `2.4.4-p17` in `composer.lock`, **When** the developer runs `hm setup`, **Then** all versions resolve to the `2.4.4` bucket without error.
2. **Given** a project with any of `2.4.5-p5` through `2.4.5-p16` in `composer.lock`, **When** the developer runs `hm setup`, **Then** all versions resolve to the `2.4.5` bucket.
3. **Given** a project with any of `2.4.6-p4` through `2.4.6-p14` in `composer.lock`, **When** the developer runs `hm setup`, **Then** all versions resolve to the `2.4.6` bucket.
4. **Given** a project with any of `2.4.7-p4` through `2.4.7-p9` in `composer.lock`, **When** the developer runs `hm setup`, **Then** all versions resolve to the `2.4.7` bucket.

---

### User Story 3 - Maintainer controls Docker images via requirements.json (Priority: P3)

A maintainer wants to configure the Docker image used by any service — including services previously hard-coded in the template (nginx, mailhog, rabbitmq, hitch) — by editing only `data/requirements.json`. For 2.4.8, this enables using the official `mariadb:11.4` image directly instead of a Hiberus-maintained wrapper that adds no value.

**Why this priority**: Removes the split between version-controlled images (in `requirements.json`) and hard-coded images (in the template), making the tool more maintainable and enabling faster adoption of new upstream versions.

**Independent Test**: Confirm that `docker-compose.yml` generated for a 2.4.8 project contains `image: mariadb:11.4` for the `db` service, and that changing the `nginx` value in the `2.4.8` requirements entry produces the corresponding image in the generated file.

**Acceptance Scenarios**:

1. **Given** a requirements entry that specifies a fully-qualified image reference (e.g. `mariadb:11.4`), **When** the developer runs `hm setup`, **Then** `docker-compose.yml` uses exactly that image reference without any `hiberusmagento/` prefix.
2. **Given** a requirements entry that specifies only a version tag (e.g. `10.6` for mariadb), **When** the developer runs `hm setup`, **Then** `docker-compose.yml` constructs the image as `hiberusmagento/mariadb:10.6` — preserving full backward compatibility for all existing version buckets.
3. **Given** that `nginx`, `mailhog`, `rabbitmq`, and `hitch` images are moved from hard-coded template values into `requirements.json` for all existing version buckets, **When** the developer runs `hm setup` on any existing project (2.3.x–2.4.7), **Then** the generated `docker-compose.yml` is identical to what was generated before this change.
4. **Given** a maintainer updates the `nginx` image value in any version bucket in `requirements.json`, **When** the developer regenerates the environment, **Then** `docker-compose.yml` reflects the updated image.

---

### User Story 4 - Maintainer adds a new Magento version and the compatibility table updates automatically (Priority: P3)

A maintainer adds a new version bucket to `data/requirements.json` and the corresponding equivalences to `data/equivalent_versions.json`. Without any changes to `console/commands/compatibility.sh`, the next run of `hm compatibility` (or equivalent) displays the new version and all its security patches in the table.

**Why this priority**: Keeps the `compatibility.sh` script consistent with the data-driven philosophy already applied to docker-compose generation (SC-005). Without this, every new Magento release requires two changes instead of one.

**Independent Test**: After adding the `2.4.8` bucket to `requirements.json` and its equivalences to `equivalent_versions.json` (as implemented in this feature), run `bash console/commands/compatibility.sh` and verify that `2.4.8` and at minimum `2.4.8-p1` through `2.4.8-p4` appear in the table without having touched the script.

**Acceptance Scenarios**:

1. **Given** a new version bucket (e.g. `2.4.8`) exists in `data/requirements.json` and its equivalences exist in `data/equivalent_versions.json`, **When** a maintainer runs `hm compatibility`, **Then** the table includes the new version and all its mapped patches without any modification to `compatibility.sh`.
2. **Given** all existing version buckets (2.3.0–2.4.7) are present in `data/requirements.json`, **When** the maintainer runs `hm compatibility` after this refactor, **Then** the table output is visually equivalent to the previous hardcoded table (regression check).

---

### Edge Cases

- What happens when a Magento version in `composer.lock` is not found in `equivalent_versions.json` (e.g. a future patch not yet added)?
- What happens when an image value in `requirements.json` contains characters special to `sed` such as forward slashes — does the substitution handle them correctly?
- What happens when a version bucket is missing a key for a service that has a placeholder in the template (e.g. `nginx` key absent from an old bucket)?
- What happens when `data/requirements.json` contains no version buckets — does `console/commands/compatibility.sh` render an empty table, print an error, or exit non-zero?
- What happens when `WORKDIR_PHP` is not empty when `composer create-project` runs? → The command must not fail; it uses a temp dir (`/tmp/magento-new`) inside the container and copies all results into `WORKDIR_PHP` after the fact.
- **macOS ghost-directory problem**: On macOS, `docker-compose.dev.mac.template.yml` bind-mounts `composer.json` and `composer.lock` as individual files. When the host files do not exist at container start time (new project), Docker Desktop creates the missing mount targets as **empty directories** inside the container. On Linux this does not happen. These ghost directories are active kernel-level bind-mount targets — they **cannot** be removed from inside the container (`rm -rf` returns `EBUSY`). The fix must happen on the **host before container start**: create real empty placeholder files with `touch "$MAGENTO_DIR/composer.json" "$MAGENTO_DIR/composer.lock"` before `docker compose up`. Docker then bind-mounts them as files and the ghost-directory problem never arises — see FR-013.

---

## Requirements *(mandatory)*

### Functional Requirements

**New Magento 2.4.8 version support:**

- **FR-001**: `data/requirements.json` MUST include a new canonical version bucket `2.4.8` with the following services: PHP 8.4 (`hiberusmagento/php:8.4-*`), MariaDB 11.4 (official `mariadb:11.4` image), OpenSearch 2.12 (`hiberusmagento/search:2.12-opensearch`), Redis 7.2 (`hiberusmagento/redis:7.2`), RabbitMQ 4.1 (official `rabbitmq:4.1-management` image), Varnish 7.x (`hiberusmagento/varnish:7.1`), and Composer 2.9.
- **FR-002**: `data/equivalent_versions.json` MUST include mappings for `2.4.8`, `2.4.8-p1`, `2.4.8-p2`, `2.4.8-p3`, and `2.4.8-p4`, all pointing to the `2.4.8` bucket.

**Missing security patch equivalences (≥ 2.4.4):**

- **FR-003**: `data/equivalent_versions.json` MUST include mappings for all released security patches of `2.4.4` from `2.4.4-p6` through `2.4.4-p17`, all pointing to the `2.4.4` bucket.
- **FR-004**: `data/equivalent_versions.json` MUST include mappings for all released security patches of `2.4.5` from `2.4.5-p5` through `2.4.5-p16`, all pointing to the `2.4.5` bucket.
- **FR-005**: `data/equivalent_versions.json` MUST include mappings for all released security patches of `2.4.6` from `2.4.6-p4` through `2.4.6-p14`, all pointing to the `2.4.6` bucket.
- **FR-006**: `data/equivalent_versions.json` MUST include mappings for all released security patches of `2.4.7` from `2.4.7-p4` through `2.4.7-p9`, all pointing to the `2.4.7` bucket.

**Image configuration in requirements.json:**

- **FR-007**: Each service value in `data/requirements.json` MUST support either a plain version tag (e.g. `10.6`) or a fully-qualified Docker image reference (e.g. `mariadb:11.4`). When the value contains a `/` or is structured as `<name>:<tag>` with a non-`hiberusmagento` registry, it MUST be used as-is in `docker-compose.yml`.
- **FR-008**: When a service value in `requirements.json` is a plain version tag, the template generation system MUST construct the image as `hiberusmagento/<service>:<tag>` — preserving full backward compatibility.
- **FR-009**: `docker-compose.template.yml` MUST be updated to replace all currently hard-coded `image:` values for `nginx`, `mailhog`, `rabbitmq`, and `hitch` with version-controlled placeholders (e.g. `<nginx_version>`, `<mailhog_version>`, `<rabbitmq_version>`, `<hitch_version>`).
- **FR-010**: All existing version buckets in `data/requirements.json` (2.3.0 through 2.4.7) MUST be updated to include entries for `nginx`, `mailhog`, `rabbitmq`, and `hitch` using their current image values (`hiberusmagento/nginx:1.18`, `hiberusmagento/mailhog:1`, `hiberusmagento/rabbitmq:3.9`, `hiberusmagento/hitch:1.7`), ensuring no regression in generated output.
- **FR-011**: The template generation script MUST use a `sed` delimiter that does not conflict with `/` characters present in fully-qualified image names (e.g. use `|` instead of `/` as the `sed` delimiter).
- **FR-012**: `console/commands/compatibility.sh` MUST generate the supported-versions table dynamically by reading the canonical version buckets from `data/requirements.json` and all entries from `data/equivalent_versions.json`, grouping security patches under their corresponding minor version. The script MUST NOT contain any hardcoded version strings in the table output.
- **FR-013**: `hm create-project` MUST run `composer create-project` into a fresh temporary directory inside the `phpfpm` container (e.g. `/tmp/magento-new`), then copy all resulting files into `WORKDIR_PHP`, so that the presence of docker-compose config files in the working directory does not cause Composer to refuse execution with a "directory not empty" error. On macOS (and as a safe default on all platforms), before starting Docker services, the script MUST create real empty placeholder files `composer.json` and `composer.lock` in `MAGENTO_DIR` if they do not already exist (using `touch`). This prevents Docker Desktop from creating ghost directories for those bind-mount paths — ghost directories are active kernel-level bind-mount targets that cannot be removed from inside the container (EBUSY).

### Key Entities

- **Version Bucket**: A canonical version key in `requirements.json` (e.g. `2.4.8`) that aggregates all service image and version requirements for a given Magento minor release.
- **Equivalent Version Mapping**: An entry in `equivalent_versions.json` that maps any specific Magento release (including security patches) to a version bucket.
- **Service Image Reference**: The Docker image identifier for a given service — either a plain version tag (resolved to `hiberusmagento/<service>:<tag>`) or a fully-qualified reference (used verbatim), stored per-version-bucket in `requirements.json`.

---

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A developer running `hm setup` on any project using Magento 2.4.4 through 2.4.8 (including all released security patches as of April 2026) completes environment generation without version resolution errors.
- **SC-002**: 100% of currently released security patch versions for 2.4.4–2.4.8 listed on the Adobe release page are present in `equivalent_versions.json`.
- **SC-003**: The `docker-compose.yml` generated for a 2.4.8 project uses `mariadb:11.4` and `rabbitmq:4.1-management` as official images, with no unnecessary Hiberus wrapper images for those services.
- **SC-004**: Existing projects using Magento versions 2.3.x–2.4.7 generate `docker-compose.yml` files with identical image references after the changes (zero regression in service images).
- **SC-005**: A maintainer can change the Docker image for any service in any version by editing only `data/requirements.json`, with no changes required in `docker-compose.template.yml` or any script.
- **SC-006**: After adding the `2.4.8` bucket to `data/requirements.json` and its equivalences to `data/equivalent_versions.json`, running `console/commands/compatibility.sh` displays `2.4.8` and all its security patches in the table with no changes to the script itself.

---

## Assumptions

- The PHP image for 2.4.8 continues to use `hiberusmagento/php:8.4-bookworm` (or equivalent bookworm-based tag), as the PHP container includes Magento-specific extensions managed by Hiberus.
- The OpenSearch image for 2.4.8 continues to use `hiberusmagento/search:2.12-opensearch`, as it bundles Magento-specific configuration.
- The Varnish image for 2.4.8 continues to use `hiberusmagento/varnish:7.1`, as it includes a Magento-optimized VCL.
- For RabbitMQ 4.1: the official `rabbitmq:4.1-management` image is used directly, consistent with the same rationale as MariaDB 11.4 (no Hiberus customisation required).
- The `redis` service continues to be used for all version buckets including 2.4.8, using `hiberusmagento/redis:7.2`. No Valkey migration is in scope for this feature.
- The `nginx` image remains `hiberusmagento/nginx:1.18` across all existing and new version buckets until a new version is explicitly needed.
- The `composer` key in `requirements.json` continues to be used as a container environment variable (`COMPOSER_VERSION`), not as a Docker image reference, and is unaffected by the image resolution logic.
- The `sed` delimiter change from `/` to `|` is safe because `|` does not appear in any current or planned image name or version tag in this project.
