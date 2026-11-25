# hm debug-off

Disable xdebug on `phpfpm` container.

## Usage

```bash
hm debug-off
```

## Description

Disable xdebug on `phpfpm` container.

## JSON output

With `--json`, stdout holds one envelope (see [JSON output](json.md)):

```json
{ "schema_version": 1, "command": "debug-off", "ok": true, "data": { "xdebug": false } }
```