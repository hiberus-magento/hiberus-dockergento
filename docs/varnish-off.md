# hm varnish-off

Disable varnish service. Varnish is active by default with `Hiberus Dockergento`.

## Usage

```bash
hm varnish-off
```

It ends with what `hm magento cache:clean` does, run through that command in text mode.

Under `--json` stdout carries only the success envelope: what `bin/magento` prints while the full
page cache is switched and cleaned goes to stderr. `hm varnish-on` does the same. When the final
`cache:clean` fails, stderr carries `varnish-off`'s failure envelope (`command_failed`) and the exit
code is `cache:clean`'s own.
