# hm cloud-login

Execute `cloud-login` native command in container.

## Usage

```bash
hm cloud-login
```

## Description

The `cloud-login` command prints where a Magento Cloud API token is generated and runs
`magento-cloud auth:api-token-login` in the container, which asks for it.

It takes no arguments: any argument is refused as a usage error (exit 2) before anything runs. To
run another `magento-cloud` command, use [`hm cloud`](cloud.md).

With `--json`, the line saying where the token is generated goes to stderr, so stdout carries
nothing of the command's own.