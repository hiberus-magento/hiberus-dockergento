# hm purge

Remove main static content.

## Usage

```bash
hm purge
```

## Description

The `purge` command is used to clean static content in `./var`, `./pub`, `./generate` and `./dev`.

## JSON output

With `--json`, a successful purge prints one envelope on stdout (see [JSON output](json.md)), with
the paths it removed inside the container:

```json
{
  "schema_version": 1,
  "command": "purge",
  "ok": true,
  "data": {
    "elapsed_ms": 912,
    "project": "my-project",
    "removed": [
      "var/cache/*",
      "generated/*",
      "pub/static/*",
      "var/view_preprocessed/*",
      "var/page_cache/*",
      "var/generation/*",
      "dev/tests/integration/tmp/*"
    ]
  }
}
```

When the removal fails inside the container, its exit code is returned and nothing is printed on
stdout.