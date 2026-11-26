# hm purge

Remove main static content.

## Usage

```bash
hm purge
```

## Description

The `purge` command removes, inside the php container, what Magento can generate again:
`var/cache`, `generated`, `pub/static`, `var/view_preprocessed`, `var/page_cache`,
`var/generation` and `dev/tests/integration/tmp`. `var/log` is never touched.

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

## Looking first: `--dry-run`

`hm purge --dry-run` lists what is there to remove, asked of the php container with the same
patterns, and removes nothing: `generated/code`, `pub/static/frontend` and so on, one line each.
With `--json` the entries are `would_remove.files` in the envelope every `--dry-run` shares (see
[JSON output](json.md#previews-with---dry-run)). Like the removal, the listing needs the container
running.