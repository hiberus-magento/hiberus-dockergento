# Contract: `data/requirements.json` Schema

**Feature**: `003-add-m248-image-config`  
**Date**: 2026-04-01  
**Status**: Authoritative — implementors must satisfy this schema

---

## Overview

`data/requirements.json` is the single source of truth for Docker image versions for every supported Magento minor version. The template generation script (`console/tasks/write_from_docker-compose_templates.sh`) reads this file and applies its values to `docker-compose.template.yml` to produce the final `docker-compose.yml`.

---

## Top-Level Structure

```json
{
  "<magento-minor-version>": { /* VersionBucket */ },
  ...
}
```

**Keys**: Magento minor version strings in ascending order (`"2.3.0"`, `"2.3.3"`, …, `"2.4.8"`).  
**Values**: A `VersionBucket` object (see below).

---

## VersionBucket Object

Each version bucket MUST contain the following keys. No additional keys are permitted unless a corresponding `<key_version>` placeholder exists in `docker-compose.template.yml`.

| Key | Required | Value type | Image resolution |
|---|---|---|---|
| `php` | yes | plain tag | `hiberusmagento/php:<value>` |
| `mariadb` | yes | plain tag OR fully-qualified | contains `:` → verbatim; else → `hiberusmagento/mariadb:<value>` |
| `search` | yes | plain tag | `hiberusmagento/search:<value>` |
| `redis` | yes | plain tag | `hiberusmagento/redis:<value>` |
| `varnish` | yes | plain tag | `hiberusmagento/varnish:<value>` |
| `composer` | yes | plain tag | injected as `COMPOSER_VERSION` env var — NOT a Docker image |
| `nginx` | yes | fully-qualified | verbatim (always contains `:`) |
| `mailhog` | yes | fully-qualified | verbatim (always contains `:`) |
| `rabbitmq` | yes | fully-qualified | verbatim (always contains `:`) |
| `hitch` | yes | fully-qualified | verbatim (always contains `:`) |

### Image Resolution Rule

The script applies a single rule to every key/value pair:

```
if value contains ":"
    image = value                             (use verbatim as Docker image)
else
    image = "hiberusmagento/" + key + ":" + value   (construct hiberusmagento image)
```

This rule is applied in `compose_regex()` inside `write_from_docker-compose_templates.sh`.

### `composer` Key — Special Behaviour

The `composer` key value is substituted into `COMPOSER_VERSION: <composer_version>` (an environment variable), not into an `image:` line. The resolution rule is still applied but the result is harmless — the substitution target is the env var placeholder, not a Docker image field.

---

## Canonical Values per Version Bucket

### Shared non-versioned services (nginx, mailhog, hitch)

These values are identical across ALL version buckets (2.3.0 – 2.4.8):

```json
"nginx":   "hiberusmagento/nginx:1.18",
"mailhog": "hiberusmagento/mailhog:1",
"hitch":   "hiberusmagento/hitch:1.7"
```

### RabbitMQ per version bucket

| Bucket | `rabbitmq` value |
|---|---|
| 2.3.0 – 2.4.7 | `"hiberusmagento/rabbitmq:3.9"` |
| 2.4.8 | `"rabbitmq:4.1-management"` |

### Core services per version bucket

| Bucket | `php` | `mariadb` | `search` | `redis` | `varnish` | `composer` |
|---|---|---|---|---|---|---|
| 2.3.0 | `7.2-buster` | `10.2` | `5.6-elasticsearch` | `5` | `6` | `1` |
| 2.3.3 | `7.3-buster` | `10.2` | `6.5-elasticsearch` | `5` | `6` | `1` |
| 2.3.5 | `7.3-buster` | `10.2` | `7.17-elasticsearch` | `5` | `6` | `1` |
| 2.3.7 | `7.4-buster` | `10.3` | `7.17-elasticsearch` | `6.2` | `6` | `1` |
| 2.4.0 | `7.4-buster` | `10.4` | `7.17-elasticsearch` | `5` | `6` | `1` |
| 2.4.2 | `7.4-buster` | `10.4` | `7.17-elasticsearch` | `6.2` | `6` | `2.1` |
| 2.4.4 | `8.1-buster` | `10.4` | `1.2-opensearch` | `6.2` | `7.1` | `2.1` |
| 2.4.5 | `8.1-buster` | `10.4` | `1.2-opensearch` | `6.2` | `7.1` | `2.2` |
| 2.4.6 | `8.2-buster` | `10.6` | `2.5-opensearch` | `7` | `7.1` | `2.2` |
| 2.4.7 | `8.3-bookworm` | `10.6` | `2.12-opensearch` | `7.2` | `7.1` | `2.7` |
| **2.4.8** | `8.4-bookworm` | `mariadb:11.4` | `2.12-opensearch` | `7.2` | `7.1` | `2.9` |

---

## Validation Rules

1. **All 11 keys must be present** in every version bucket. A missing key will cause the corresponding `<key_version>` placeholder to remain unsubstituted in the generated `docker-compose.yml`.
2. **No value may contain `|`**. The sed expression uses `|` as its delimiter; a `|` in any value would break substitution.
3. **Fully-qualified values must contain `:`**. The image resolution heuristic depends on `:` being present in any value that should be used verbatim.
4. **Plain tag values must not contain `:`**. A plain tag that inadvertently contains `:` would be used verbatim instead of being prefixed with `hiberusmagento/<key>:`.
5. **`composer` value must be a plain tag** (no `:`), so it is injected as a version string, not an image reference.

---

## Example: Complete 2.4.8 Bucket

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

Resolved images produced by the script:

| Key | Resolved image / value |
|---|---|
| `php` | `hiberusmagento/php:8.4-bookworm` |
| `mariadb` | `mariadb:11.4` |
| `search` | `hiberusmagento/search:2.12-opensearch` |
| `redis` | `hiberusmagento/redis:7.2` |
| `varnish` | `hiberusmagento/varnish:7.1` |
| `composer` | `2.9` (env var only) |
| `nginx` | `hiberusmagento/nginx:1.18` |
| `mailhog` | `hiberusmagento/mailhog:1` |
| `rabbitmq` | `rabbitmq:4.1-management` |
| `hitch` | `hiberusmagento/hitch:1.7` |
