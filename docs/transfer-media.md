# hm transfer-media

This command transfers media content from external source to local environment.

## Usage

```bash
hm transfer-media [OPTIONS]
```

Refer to the [options section](#options) for an overview of available `OPTIONS` for this command.

## Description

The `transfer-media` command copies a remote store's `pub/media` into `./pub` with `rsync` over
SSH, on this machine (`rsync` must be installed). Files already here are kept, and the caches are
left behind.

Each value is asked, with the option's value, or the default, offered as the answer; under `--yes`
the offered answers are taken, and a missing SSH user stops the command (exit 2).

## Options

Each takes its value after `=` only. Any other argument is a usage error (exit 2).

| Name              | Description                                         | Example                                      |
| ----------------- | --------------------------------------------------- | -------------------------------------------- |
| `--ssh-host`      | SSH host name (default `ssh.eu-3.magento.cloud`).   | `hm transfer-media --ssh-host=<host-name>`   |
| `--ssh-user`      | SSH user.                                           | `hm transfer-media --ssh-user=<host-user>`   |
| `--ssh-path`      | Remote media path (default `/app/pub/media`).       | `hm transfer-media --ssh-path=<remote-path>` |