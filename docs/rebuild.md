
# hm rebuild

This command rebuilds and starting containers in detached mode.

## Usage

```bash
hm rebuild <container_name>
```

## JSON output

With `--json`, a successful rebuild prints one envelope on stdout (see [JSON output](json.md)):

```json
{
  "schema_version": 1,
  "command": "rebuild",
  "ok": true,
  "data": {
    "elapsed_ms": 61230,
    "project": "my-project",
    "services": ["phpfpm"],
    "whole_environment": false
  }
}
```

An empty `services` list, with `whole_environment: true`, means the whole environment was rebuilt.
