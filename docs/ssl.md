# hm ssl

Create certificates for `https` protocol.

## Usage

```bash
hm ssl [domain]
```

## Description

The `ssl` command creates a certificate for the domain (the project's `DOMAIN` when none is given,
`localhost` when the project has none either), signed by [mkcert](https://github.com/FiloSottile/mkcert)'s
local authority, and installs it in the `hitch` container, which terminates TLS. mkcert is installed
when it is missing: Homebrew on macOS, the release binary on Linux.

More than one domain is a usage error (exit 2). From a git worktree with no environment of its own
it is refused, because the TLS terminator is the main environment's.
