# hm grunt

Create theme grunt config and to launch `grunt exec` and `grunt watch` command on container.

## Usage

```bash
hm grunt <Vendor/theme> <locale_LOCALE>
```

## Description

The `grunt` serves to use grunt with Magento static files.

```bash
hm grunt Hiberus/theme es_ES
```

The theme must be shaped `Vendor/theme` (letters, digits and underscores, plus hyphens in the
theme name) and the locale `xx_YY`, or `xx_Script_YY` for the locales that name a script, such as
`zh_Hans_CN`. Anything else is a usage error (exit 2) before anything runs in the container.