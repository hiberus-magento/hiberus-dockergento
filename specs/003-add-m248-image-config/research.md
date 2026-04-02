# Research: Add Magento 2.4.8 Support and Image Configuration in Requirements

**Feature**: `003-add-m248-image-config`  
**Date**: 2026-04-01  
**Status**: Complete — all unknowns resolved

---

## 1. Magento 2.4.8 Service Version Requirements

**Decision**: Use the following stack for the `2.4.8` bucket in `requirements.json`:

| Service | Value in requirements.json | Resolved image in docker-compose.yml |
|---|---|---|
| `php` | `8.4-bookworm` | `hiberusmagento/php:8.4-bookworm` |
| `mariadb` | `mariadb:11.4` | `mariadb:11.4` (official, verbatim) |
| `search` | `2.12-opensearch` | `hiberusmagento/search:2.12-opensearch` |
| `redis` | `7.2` | `hiberusmagento/redis:7.2` |
| `varnish` | `7.1` | `hiberusmagento/varnish:7.1` |
| `composer` | `2.9` | env var `COMPOSER_VERSION=2.9` (not an image) |
| `nginx` | `hiberusmagento/nginx:1.18` | `hiberusmagento/nginx:1.18` (verbatim) |
| `mailhog` | `hiberusmagento/mailhog:1` | `hiberusmagento/mailhog:1` (verbatim) |
| `rabbitmq` | `rabbitmq:4.1-management` | `rabbitmq:4.1-management` (official, verbatim) |
| `hitch` | `hiberusmagento/hitch:1.7` | `hiberusmagento/hitch:1.7` (verbatim) |

**Rationale**:
- PHP 8.4: Latest stable PHP supported by Magento 2.4.8. Uses `bookworm` Debian base (same as 2.4.7) for consistency with existing hiberusmagento images.
- MariaDB 11.4: Official Adobe requirement for 2.4.8 on-premises. Hiberus has no customisation on this image — using official directly reduces maintenance burden.
- OpenSearch 2.12: Matches the `2.12-opensearch` tag already used for 2.4.7, which is within the OpenSearch 2.x range supported by 2.4.8.
- Redis 7.2: Continues existing hiberusmagento image, same as 2.4.7. Valkey adoption deferred.
- RabbitMQ 4.1-management: Official image with management UI. No hiberusmagento wrapper needed.
- Composer 2.9: Minimum required by Adobe for 2.4.8 (`2.9.3+`); `2.9` tag resolves to latest 2.9.x.

**Alternatives considered**:
- PHP 8.3 instead of 8.4: Both are supported by Adobe for 2.4.8. 8.4 chosen per explicit user direction.
- OpenSearch 2.19: Also supported by 2.4.8 on-premises but `2.12` is the existing latest hiberusmagento tag and sufficient.

---

## 2. Security Patch Equivalence Gaps (≥ 2.4.4)

**Decision**: Add the following mappings to `equivalent_versions.json`. All map to their parent minor version bucket.

### 2.4.4 — currently has p1–p5, needs p6–p17 (12 new entries)
`2.4.4-p6` through `2.4.4-p17` → `"2.4.4"`

### 2.4.5 — currently has p1–p4, needs p5–p16 (12 new entries)
`2.4.5-p5` through `2.4.5-p16` → `"2.4.5"`

### 2.4.6 — currently has p1–p3, needs p4–p14 (11 new entries)
`2.4.6-p4` through `2.4.6-p14` → `"2.4.6"`

### 2.4.7 — currently has p1–p3, needs p4–p9 (6 new entries)
`2.4.7-p4` through `2.4.7-p9` → `"2.4.7"`

### 2.4.8 — entirely new bucket (5 new entries)
`2.4.8`, `2.4.8-p1`, `2.4.8-p2`, `2.4.8-p3`, `2.4.8-p4` → `"2.4.8"`

**Total new entries**: 46

**Rationale**: Adobe publishes security patches that fix vulnerabilities without changing system requirements. All patches within a minor line share the same service versions. The existing pattern (all patches pointing to the minor bucket) is correct and must be extended.

---

## 3. Image Resolution Logic — Fully-Qualified vs Plain Tag

**Decision**: A service value in `requirements.json` is treated as a **fully-qualified image reference** (used verbatim) if it contains a `:` character. Otherwise it is treated as a **plain version tag** and the image is constructed as `hiberusmagento/<service>:<tag>`.

**Rationale**:
- All existing values in `requirements.json` are plain tags: `10.6`, `7.2`, `8.3-bookworm`, etc. None contain `:`.
- All newly introduced fully-qualified references contain `:`: `mariadb:11.4`, `rabbitmq:4.1-management`, `hiberusmagento/nginx:1.18`, etc.
- The `:` heuristic is unambiguous for this dataset and avoids any regex complexity.
- A value like `hiberusmagento/nginx:1.18` also contains `:`, which is correct — it should be used verbatim.

**Alternatives considered**:
- Check for `/` instead of `:` — rejected because `hiberusmagento/redis:7.2` (a possible future fully-qualified override) contains `/` but existing plain tags do not. However `:` is simpler and equally unambiguous.
- Separate `image` key — rejected by user in favour of value-direct approach.

**Bash implementation**:
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

**Why `|` as sed delimiter**: The character `|` does not appear in any Docker image name or tag used in this project (confirmed by scanning all values in `requirements.json` and all planned new values). Using `|` instead of `/` avoids escaping the `/` in registry names like `hiberusmagento/redis`.

---

## 4. Template Changes — Hard-coded Images

**Decision**: Replace these four hard-coded `image:` lines in `docker-compose.template.yml`:

| Current (hard-coded) | New placeholder |
|---|---|
| `image: hiberusmagento/nginx:1.18` | `image: <nginx_version>` |
| `image: hiberusmagento/mailhog:1` | `image: <mailhog_version>` |
| `image: hiberusmagento/rabbitmq:3.9` | `image: <rabbitmq_version>` |
| `image: hiberusmagento/hitch:1.7` | `image: <hitch_version>` |

**Rationale**: Consolidates all image decisions into `requirements.json`, satisfying SC-005. The `sed` substitution loop already handles any key in `REQUIREMENTS` — adding these four keys to every version bucket automatically feeds the new placeholders with no script changes beyond the delimiter fix.

**Backward compatibility**: Every existing version bucket (2.3.0–2.4.7) will receive the same four values they have today (`hiberusmagento/nginx:1.18`, etc.), producing identical output.

---

## 6. macOS Docker Desktop Bind-Mount Ghost-Directory Behavior

**Context**: `docker-compose.dev.mac.template.yml` defines individual file bind mounts for the `phpfpm` service:

```yaml
- {MAGENTO_DIR}/composer.json:/var/www/html/composer.json:cached
- {MAGENTO_DIR}/composer.lock:/var/www/html/composer.lock:cached
```

**Observed behavior on macOS**:
When `docker compose up` runs on a **new project** where `composer.json` and `composer.lock` do not yet exist on the host, Docker Desktop (macOS) creates the missing bind-mount source paths as **empty directories** inside the container instead of treating them as absent files.

This means `/var/www/html/composer.json` and `/var/www/html/composer.lock` exist inside the container as directories (not files) before Composer runs.

**Observed behavior on Linux**:
On Linux, missing bind-mount source paths are not created. The path simply does not exist inside the container. No ghost directories are created.

**Why in-container `rm` fails (EBUSY)**:
The ghost directories are **active kernel-level bind-mount targets**. The kernel returns `EBUSY` for any `rm` or `rmdir` call on a live mount point — from any process, including root, whether inside or outside the container. This is not a permissions problem; it is a fundamental OS constraint. No amount of in-container surgery can remove these paths while Docker is running.

**Impact on `create-project.sh`**:
After `composer create-project` writes files to `/tmp/magento-new`, the `cp -R /tmp/magento-new/. /var/www/html/` step tries to copy the file `composer.json` onto `/var/www/html/composer.json` — which is a directory. This produces:
```
cp: cannot overwrite directory '/var/www/html/./composer.json' with non-directory
```

**Correct fix — host pre-creation before container start**:
Pre-create real empty files on the **host** before `docker compose up`. Use `touch` in the host shell:
```bash
touch "$MAGENTO_DIR/composer.json"
touch "$MAGENTO_DIR/composer.lock"
```
When Docker starts, it sees real files on the host at the bind-mount source paths and mounts them as files inside the container. No ghost directories are ever created. The subsequent `cp -R` step overwrites file-with-file, which is valid. This fix is safe on both macOS and Linux (existing files are left unchanged by `touch`).

**Why not fix the template instead?**
The bind mounts in `docker-compose.dev.mac.template.yml` serve a legitimate purpose (performance optimisation on macOS via the `:cached` flag). Removing them would degrade I/O performance. Host pre-creation is the least-invasive, permanent fix.

**Manual recovery if containers are already up with ghost dirs**:
```bash
docker compose down
touch composer.json composer.lock
hm create-project
```

---

## 5. `composer` Key — Excluded from Image Resolution**Decision**: The `composer` key continues to be substituted as-is (its value is inserted into `COMPOSER_VERSION: <composer_version>` in the template, not into an `image:` line). The image resolution logic has no effect on it.

**Rationale**: The `composer` placeholder is an environment variable, not a Docker image tag. The `<composer_version>` placeholder is replaced by the value `2.9` (for 2.4.8), which is correct for `COMPOSER_VERSION: 2.9`. The `:` heuristic correctly classifies `2.9` as a plain tag, so even if the logic tried to construct an image it would produce `hiberusmagento/composer:2.9` — but since there is no `image: <composer_version>` line in the template, the substitution is harmless.
