# Data Model: Add Magento 2.4.8 Support and Image Configuration in Requirements

**Feature**: `003-add-m248-image-config`  
**Date**: 2026-04-01

---

## Entities

### 1. Version Bucket (`data/requirements.json`)

A JSON object keyed by Magento minor version string. Each value is a service map.

**Key**: Magento minor version string (e.g. `"2.4.8"`)

**Value shape** (updated schema):

```json
{
  "php":       "<plain-tag>",
  "mariadb":   "<plain-tag> | <fully-qualified-image>",
  "search":    "<plain-tag>",
  "redis":     "<plain-tag>",
  "varnish":   "<plain-tag>",
  "composer":  "<plain-tag>",
  "nginx":     "<fully-qualified-image>",
  "mailhog":   "<fully-qualified-image>",
  "rabbitmq":  "<plain-tag> | <fully-qualified-image>",
  "hitch":     "<fully-qualified-image>"
}
```

**Field rules**:

| Field | Type | Image resolution rule |
|---|---|---|
| `php` | plain tag | → `hiberusmagento/php:<value>` |
| `mariadb` | plain tag OR fully-qualified | contains `:` → verbatim; else → `hiberusmagento/mariadb:<value>` |
| `search` | plain tag | → `hiberusmagento/search:<value>` |
| `redis` | plain tag | → `hiberusmagento/redis:<value>` |
| `varnish` | plain tag | → `hiberusmagento/varnish:<value>` |
| `composer` | plain tag | → injected as `COMPOSER_VERSION` env var, not an image |
| `nginx` | fully-qualified | contains `:` → verbatim |
| `mailhog` | fully-qualified | contains `:` → verbatim |
| `rabbitmq` | fully-qualified | contains `:` → verbatim |
| `hitch` | fully-qualified | contains `:` → verbatim |

**Detection rule**: If `value` contains `:`, use verbatim as Docker image. Otherwise prepend `hiberusmagento/<key>:`.

**New 2.4.8 bucket** (complete):

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

**New fields added to all existing buckets** (2.3.0 – 2.4.7):

```json
"nginx":    "hiberusmagento/nginx:1.18",
"mailhog":  "hiberusmagento/mailhog:1",
"rabbitmq": "hiberusmagento/rabbitmq:3.9",
"hitch":    "hiberusmagento/hitch:1.7"
```

---

### 2. Equivalent Version Mapping (`data/equivalent_versions.json`)

A flat JSON object mapping any released Magento version string to the canonical version bucket key in `requirements.json`.

**Key**: Any Magento version string (e.g. `"2.4.8-p2"`)  
**Value**: Canonical bucket key (e.g. `"2.4.8"`)

**New entries** (46 total):

| Range | Maps to |
|---|---|
| `2.4.4-p6` … `2.4.4-p17` | `"2.4.4"` |
| `2.4.5-p5` … `2.4.5-p16` | `"2.4.5"` |
| `2.4.6-p4` … `2.4.6-p14` | `"2.4.6"` |
| `2.4.7-p4` … `2.4.7-p9` | `"2.4.7"` |
| `2.4.8`, `2.4.8-p1` … `2.4.8-p4` | `"2.4.8"` |

---

### 3. Docker Compose Template (`docker-compose/docker-compose.template.yml`)

A YAML template file with `<key_version>` placeholders. The `sed` substitution loop replaces each placeholder with its resolved Docker image value from the active version bucket.

**Updated placeholders** (replacing formerly hard-coded `image:` lines):

| Service block | Old value (hard-coded) | New placeholder |
|---|---|---|
| `nginx` | `hiberusmagento/nginx:1.18` | `<nginx_version>` |
| `mailhog` | `hiberusmagento/mailhog:1` | `<mailhog_version>` |
| `rabbitmq` | `hiberusmagento/rabbitmq:3.9` | `<rabbitmq_version>` |
| `hitch` | `hiberusmagento/hitch:1.7` | `<hitch_version>` |

**Existing placeholders** (unchanged):

| Service block | Placeholder | Example resolved value |
|---|---|---|
| `phpfpm` | `<php_version>` | `hiberusmagento/php:8.4-bookworm` |
| `db` | `<mariadb_version>` | `mariadb:11.4` |
| `search` | `<search_version>` | `hiberusmagento/search:2.12-opensearch` |
| `redis` | `<redis_version>` | `hiberusmagento/redis:7.2` |
| `varnish` | `<varnish_version>` | `hiberusmagento/varnish:7.1` |
| `phpfpm` env | `<composer_version>` | `2.9` (env var, not image) |

---

### 4. Template Generation Script (`console/tasks/write_from_docker-compose_templates.sh`)

**`compose_regex()` — updated logic**:

```
Input:  $REQUIREMENTS (JSON object: service → value)
Output: $regex (multi-substitution sed expression using | delimiter)

For each key/value pair in REQUIREMENTS:
  if value contains ":"
    image = value                          (use verbatim)
  else
    image = "hiberusmagento/" + key + ":" + value   (construct hiberusmagento image)
  append "s|<key_version>|image|g; " to regex
```

**State transitions**:

```
composer.lock detected
       ↓
get_equivalent_version("2.4.8-p1")
       ↓
equivalent_versions.json lookup → "2.4.8"
       ↓
requirements.json lookup → REQUIREMENTS object (10 keys)
       ↓
compose_regex() builds sed expression (| delimiter, verbatim/constructed image)
       ↓
sed applied to docker-compose.template.yml → docker-compose.yml
       ↓
set_settings() substitutes {YML_VERSION}, {MAGENTO_DIR}, {FILES_IN_GIT}
```
