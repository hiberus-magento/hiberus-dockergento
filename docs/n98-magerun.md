# hm n98-magerun

Run [n98-magerun](https://github.com/netz98/n98-magerun2) inside the php container.

## Usage

```bash
hm n98-magerun [ARGUMENTS]
```

The arguments are handed to `n98-magerun` as typed, through `bash -c` in the `phpfpm` container,
and its exit code is `hm`'s:

```bash
hm n98-magerun sys:info
```
