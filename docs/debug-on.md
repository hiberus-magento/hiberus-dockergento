# hm debug-on

Enable xdebug on `phpfpm` container.

## Usage

```bash
hm debug-on
```

## Description

Enable xdebug on `phpfpm` container and show necessary configs for your IDE debug configuration.

## JSON output

With `--json`, stdout holds one envelope (see [JSON output](json.md)) with what the readback found:

```json
{
  "schema_version": 1,
  "command": "debug-on",
  "ok": true,
  "data": {
    "host": "host.docker.internal",
    "settings": {
      "xdebug.client_host": "host.docker.internal",
      "xdebug.client_port": "9003",
      "xdebug.idekey": "PHPSTORM"
    },
    "version": "3.3.2",
    "xdebug": true
  }
}
```

`host` is `null` when no address was patched. When Xdebug is not installed or its version is not
recognised, `data` is `{"xdebug": false}`.