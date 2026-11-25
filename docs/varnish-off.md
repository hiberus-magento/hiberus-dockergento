# hm varnish-off

Disable varnish service. Varnish is active by default with `Hiberus Dockergento`.

## Usage

```bash
hm varnish-off
```

Under `--json` stdout carries only the success envelope: what `bin/magento` prints while the full
page cache is switched and cleaned goes to stderr. `hm varnish-on` does the same.
