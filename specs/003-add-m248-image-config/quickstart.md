# Quickstart: Add Magento 2.4.8 Support and Image Configuration in Requirements

**Feature**: `003-add-m248-image-config`  
**Date**: 2026-04-01  
**Branch**: `003-add-m248-image-config`

---

## What You Are Implementing

Four changes across four files:

| # | File | Change |
|---|---|---|
| 1 | `data/requirements.json` | Add `nginx`, `mailhog`, `rabbitmq`, `hitch` keys to all 10 existing buckets; add new `2.4.8` bucket (10 keys) |
| 2 | `data/equivalent_versions.json` | Add 46 new patch-to-bucket mappings (2.4.4-p6…p17, 2.4.5-p5…p16, 2.4.6-p4…p14, 2.4.7-p4…p9, 2.4.8…2.4.8-p4) |
| 3 | `docker-compose/docker-compose.template.yml` | Replace 4 hard-coded `image:` lines with `<nginx_version>`, `<mailhog_version>`, `<rabbitmq_version>`, `<hitch_version>` placeholders |
| 4 | `console/tasks/write_from_docker-compose_templates.sh` | Update `compose_regex()`: change `sed` delimiter from `/` to `|`; add `:` heuristic for verbatim vs constructed images |

---

## Step-by-Step Implementation

### Step 1 — `console/tasks/write_from_docker-compose_templates.sh`

Replace the `compose_regex()` function body. The current implementation:

```bash
compose_regex() {
    local services=$(echo "$REQUIREMENTS" | jq -r 'keys|join(" ")')

    for index in $services; do
        value=$(echo "$REQUIREMENTS" | jq -r '.'"$index"'')
        regex+="s/<${index}_version>/${value}/g; "
    done
}
```

New implementation:

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

Key differences:
- `jq` key access changed from `.key` to `.["key"]` (handles hyphenated keys safely)
- `sed` delimiter changed from `/` to `|`
- Added `if/else` block: values containing `:` are used verbatim; others are prefixed with `hiberusmagento/<key>:`

> **Note**: The existing sed call on line 25 (`sed "$regex" ...`) does not specify a delimiter — it uses the expression built by `compose_regex()`, which now uses `|`. No change needed to the `sed` invocation itself.

---

### Step 2 — `docker-compose/docker-compose.template.yml`

Replace four hard-coded `image:` lines with placeholders:

| Line | Old value | New value |
|---|---|---|
| 20 | `image: hiberusmagento/nginx:1.18` | `image: <nginx_version>` |
| 66 | `image: hiberusmagento/mailhog:1` | `image: <mailhog_version>` |
| 71 | `image: hiberusmagento/rabbitmq:3.9` | `image: <rabbitmq_version>` |
| 82 | `image: hiberusmagento/hitch:1.7` | `image: <hitch_version>` |

No other changes to this file.

---

### Step 3 — `data/requirements.json`

Two sub-tasks:

**3a. Add 4 new keys to all 10 existing buckets** (2.3.0 through 2.4.7):

```json
"nginx":    "hiberusmagento/nginx:1.18",
"mailhog":  "hiberusmagento/mailhog:1",
"rabbitmq": "hiberusmagento/rabbitmq:3.9",
"hitch":    "hiberusmagento/hitch:1.7"
```

Add these four keys to every existing bucket. Order within the object does not matter — `jq` sorts by key during iteration.

**3b. Add the new `2.4.8` bucket** after `2.4.7`:

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

> Note: `rabbitmq` for 2.4.8 is `"rabbitmq:4.1-management"` (official image), not the hiberusmagento one used in older buckets.

---

### Step 4 — `data/equivalent_versions.json`

Add 46 new entries after the existing last entry (`"2.4.7-p3"`). Maintain the existing sort order (ascending by version):

```json
    "2.4.4-p6":  "2.4.4",
    "2.4.4-p7":  "2.4.4",
    "2.4.4-p8":  "2.4.4",
    "2.4.4-p9":  "2.4.4",
    "2.4.4-p10": "2.4.4",
    "2.4.4-p11": "2.4.4",
    "2.4.4-p12": "2.4.4",
    "2.4.4-p13": "2.4.4",
    "2.4.4-p14": "2.4.4",
    "2.4.4-p15": "2.4.4",
    "2.4.4-p16": "2.4.4",
    "2.4.4-p17": "2.4.4",
    "2.4.5-p5":  "2.4.5",
    "2.4.5-p6":  "2.4.5",
    "2.4.5-p7":  "2.4.5",
    "2.4.5-p8":  "2.4.5",
    "2.4.5-p9":  "2.4.5",
    "2.4.5-p10": "2.4.5",
    "2.4.5-p11": "2.4.5",
    "2.4.5-p12": "2.4.5",
    "2.4.5-p13": "2.4.5",
    "2.4.5-p14": "2.4.5",
    "2.4.5-p15": "2.4.5",
    "2.4.5-p16": "2.4.5",
    "2.4.6-p4":  "2.4.6",
    "2.4.6-p5":  "2.4.6",
    "2.4.6-p6":  "2.4.6",
    "2.4.6-p7":  "2.4.6",
    "2.4.6-p8":  "2.4.6",
    "2.4.6-p9":  "2.4.6",
    "2.4.6-p10": "2.4.6",
    "2.4.6-p11": "2.4.6",
    "2.4.6-p12": "2.4.6",
    "2.4.6-p13": "2.4.6",
    "2.4.6-p14": "2.4.6",
    "2.4.7-p4":  "2.4.7",
    "2.4.7-p5":  "2.4.7",
    "2.4.7-p6":  "2.4.7",
    "2.4.7-p7":  "2.4.7",
    "2.4.7-p8":  "2.4.7",
    "2.4.7-p9":  "2.4.7",
    "2.4.8":     "2.4.8",
    "2.4.8-p1":  "2.4.8",
    "2.4.8-p2":  "2.4.8",
    "2.4.8-p3":  "2.4.8",
    "2.4.8-p4":  "2.4.8"
```

---

### Step 5 — `console/commands/compatibility.sh`

Replace the hardcoded table with a dynamic implementation that reads from the data files.

**Logic overview**:

1. Source `$COMPONENTS_DIR/print_message.sh` (already done by the existing script).
2. Read canonical bucket keys from `data/requirements.json`:
   ```bash
   buckets=$(jq -r 'keys[]' data/requirements.json)
   ```
3. For each bucket, collect all keys from `data/equivalent_versions.json` that map to it:
   ```bash
   versions=$(jq -r --arg b "$bucket" 'to_entries | map(select(.value == $b)) | .[].key' data/equivalent_versions.json)
   ```
4. Split buckets into `2.3.x` and `2.4.x` groups (filter by prefix).
5. Determine max column height; pad shorter columns with empty strings.
6. Build the table string following the existing format and pass to `print_table`.

**Verification command**:

```bash
bash console/commands/compatibility.sh
# Expected: table includes 2.4.8, 2.4.8-p1 through 2.4.8-p4 without any hardcoded strings in the script
```

**Regression check**:

```bash
# Capture output and diff against the known-good hardcoded table
bash console/commands/compatibility.sh > /tmp/dynamic_table.txt
# Compare 2.3.x and 2.4.0–2.4.7 cells against the original — must be identical
```

---

## Verification

### Manual test (primary)

Run `hm setup` (or the equivalent version detection + template generation path) for projects with versions spanning multiple buckets. Compare generated `docker-compose.yml` against expected images.

**Quick smoke test** — generate the docker-compose for a specific version without a full project:

```bash
# From the repo root, simulate what write_from_docker-compose_templates.sh does:
REQUIREMENTS=$(jq '.["2.4.7"]' data/requirements.json)

# Run the updated compose_regex logic manually and apply to template:
regex=""
for index in $(echo "$REQUIREMENTS" | jq -r 'keys | join(" ")'); do
    value=$(echo "$REQUIREMENTS" | jq -r '.["'"$index"'"]')
    if [[ "$value" == *":"* ]]; then
        image="$value"
    else
        image="hiberusmagento/${index}:${value}"
    fi
    regex+="s|<${index}_version>|${image}|g; "
done
sed "$regex" docker-compose/docker-compose.template.yml
```

Expected output for 2.4.7 should be byte-for-byte identical to what the current script produces (backward compatibility check).

### Backward compatibility check

For each existing bucket (2.3.0 – 2.4.7), the generated `docker-compose.yml` must have the same `image:` values as today. The four previously hard-coded lines:

```yaml
image: hiberusmagento/nginx:1.18
image: hiberusmagento/mailhog:1
image: hiberusmagento/rabbitmq:3.9
image: hiberusmagento/hitch:1.7
```

…must appear unchanged in output for all existing buckets.

### New 2.4.8 check

For the `2.4.8` bucket, verify these specific lines appear in the generated output:

```yaml
image: hiberusmagento/php:8.4-bookworm
image: mariadb:11.4
image: hiberusmagento/search:2.12-opensearch
image: hiberusmagento/redis:7.2
image: hiberusmagento/varnish:7.1
image: hiberusmagento/nginx:1.18
image: hiberusmagento/mailhog:1
image: rabbitmq:4.1-management
image: hiberusmagento/hitch:1.7
```

And verify the `phpfpm` environment block contains:

```yaml
COMPOSER_VERSION: 2.9
```

### Equivalence lookup check

```bash
jq '.["2.4.8-p2"]' data/equivalent_versions.json   # → "2.4.8"
jq '.["2.4.4-p17"]' data/equivalent_versions.json  # → "2.4.4"
jq '.["2.4.7-p9"]' data/equivalent_versions.json   # → "2.4.7"
```

---

## Common Mistakes

- **Wrong jq accessor**: Using `.key` instead of `.["key"]` works for simple keys but fails for keys with special characters. Use `.["key"]` for safety in the loop.
- **Forgetting `rabbitmq` differs for 2.4.8**: Older buckets use `hiberusmagento/rabbitmq:3.9`; 2.4.8 uses `rabbitmq:4.1-management` (official).
- **Off-by-one on patch counts**: 2.4.4 goes up to p17 (12 new entries from p6), 2.4.5 to p16 (12 new from p5), 2.4.6 to p14 (11 new from p4), 2.4.7 to p9 (6 new from p4). Recount if unsure.
- **`|` in values**: Any value containing `|` will break the sed expression. No current or planned values contain `|` — verify before adding future entries.
- **New major version column**: If a `2.5.x` bucket is ever added, the column-splitting logic in `compatibility.sh` (which groups by `2.3.*` vs `2.4.*`) will need updating to add a third column group. This is by design — the script is data-driven for rows but the column grouping logic must be adjusted for new major lines.
- **`hm create-project` temp dir cleanup**: If the command is interrupted after the `composer create-project` step but before the `cp` or `rm` steps, a `/tmp/magento-new` directory may be left inside the container. On the next run this will cause a second "directory not empty" error for the temp dir itself. If this happens, exec into the container and run `rm -rf /tmp/magento-new` before retrying.
- **macOS ghost-directory error** (`cp: cannot overwrite directory with non-directory`): On macOS, Docker Desktop creates missing bind-mount source paths as empty directories if the host file doesn't exist at container start time. These ghost directories are **active kernel-level bind-mount targets** — `rm -rf` from inside the container fails with `EBUSY` (not a permissions problem; it is a hard OS constraint). Recovery: `docker compose down`, then `touch composer.json composer.lock` in the project directory, then retry `hm create-project`. If you see this error with the updated script, it means the `touch` pre-creation step did not run before `docker compose up` (e.g. an older version of `create-project.sh` is installed).

---

## Step 6 — Test `hm create-project` on a fresh empty directory

**Purpose**: Validate that the Phase 8 fix resolves the "directory not empty" error and that `hm create-project` completes end-to-end.

**Preconditions**:
- `hm` CLI is installed and authenticated with `repo.magento.com` (auth.json in place).
- T030 has been implemented in `console/commands/create-project.sh`.

**Steps**:

```bash
# 1. Create a fresh empty directory
mkdir /tmp/test-create-project && cd /tmp/test-create-project

# 2. Run create-project (select Magento 2.4.8 or 2.4.8-p4 when prompted)
hm create-project

# 3. Verify no "directory not empty" error was printed during the Composer step

# 4. Confirm composer.json is present on the host and is a FILE (not a directory)
[ -f composer.json ] && echo "OK: file" || echo "ERROR: not a file or missing"

# 5. Confirm composer.lock is present
ls composer.lock      # must exist

# 6. Confirm all 9 containers are running
docker compose ps     # all services should show "running"

# 7. Cleanup
cd .. && rm -rf /tmp/test-create-project
```

**Verification of the fix**: The `touch "$MAGENTO_DIR/composer.json"` and `touch "$MAGENTO_DIR/composer.lock"` lines in `create-project.sh` must run **before** `$DOCKER_COMPOSE up -d`. If they are missing or placed after the `up` call, the ghost-directory problem will reappear on macOS.

**Expected result**: The command runs to completion. Composer's output shows "Creating a magento/project-community-edition project" followed by success — no error about the directory being non-empty. On macOS, `composer.json` is a **file** (not a directory) in the host directory. All 9 containers remain running.
