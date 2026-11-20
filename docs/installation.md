# Installing hm

hm 2 is a single binary. The installer downloads it from a GitHub release, verifies it and puts it
on your PATH. There is no git clone.

> This page is about installing the `hm` tool. `hm install`, the command that runs Magento's
> `setup:install`, is documented in [install.md](install.md).

## One line

```bash
curl -fsSL https://raw.githubusercontent.com/hiberus-magento/hiberus-dockergento/main/install.sh | bash
```

What it does:

1. Detects the system (`darwin` or `linux`) and the architecture (`amd64` or `arm64`).
2. Picks the version: the newest **final** release of 2.0 or later, read from the
   `releases/latest` redirect of github.com, which has no rate limit. The GitHub API is asked only
   for `--pre`, or when that redirect does not name a 2.x release; when the API's rate limit is
   reached it says so (set `GITHUB_TOKEN`, wait, or name the version with `--version`).
3. Downloads `hm_<os>_<arch>` and `checksums.txt` from that release and checks the SHA-256. A
   mismatch installs nothing.
4. Installs the binary (see [where](#where-it-is-installed)), replacing any previous one in a
   single rename, so a failed run never leaves half an `hm`.
5. Writes the shell completion (`hm completion zsh|bash`) to `~/.hm/completion/hm.<shell>` and adds
   **one** line to your profile that loads it. Running the installer again never adds a second
   one. The profile is never rewritten in place: the new content goes to a temporary file beside
   it with the same permissions, is checked, and replaces it in one rename. A profile that is a
   link (managed dotfiles) stays a link and the file it points to is edited. The first edit keeps
   the original as `<profile>.hm-backup`. If anything fails — a read-only profile, a link to
   nothing — the profile is left exactly as it was and the line to add (or remove) is printed.
6. Prints what it did: the version, the path and what changed.

Running it again upgrades to a newer release, or says the installed one is up to date.

## Options

Options go after `bash -s --`. Each one has an environment variable that does the same.

| Option | Variable | Meaning |
|---|---|---|
| `--version X` | `HM_VERSION=X` | Install that release (`2.0.0`, `2.1.0-rc.1`). Must be 2.0 or later. |
| `--pre` | `HM_PRE=1` | The latest may be a pre-release. |
| `--dir D` | `HM_INSTALL_DIR=D` | Install the binary in `D`. |
| `--uninstall` | | Remove what the installer installed. |
| | `GITHUB_TOKEN` | Sent to the GitHub API only, to avoid its rate limit. It is handed to curl (or wget) in a file only you can read, removed afterwards, never on the command line where `ps` would show it. |

```bash
curl -fsSL .../install.sh | bash -s -- --version 2.0.0 --dir ~/bin
curl -fsSL .../install.sh | HM_VERSION=2.0.0 bash          # the variable goes on bash, not on curl
```

`installer.sh`, the 1.x name, still works and runs `install.sh`, so old links keep working.

## Where it is installed

In this order:

1. `--dir` / `HM_INSTALL_DIR`, when given.
2. `/usr/local/bin/hm`, when it is already an `hm`: the 1.x link or an earlier hm 2. The new binary
   goes **at that same path**, so there is only ever one `hm`. If the directory is not yours, only
   that write goes through `sudo`.
3. `~/.local/bin/hm`, when an earlier hm 2 is there.
4. `/usr/local/bin`, when it is writable and has no `hm`.
5. `~/.local/bin`, created if needed. You get a warning if it is not on your PATH.

A file called `hm` that is not hm is never overwritten: the installer stops and asks you to remove
it or to choose another directory.

## Upgrading from 1.x

hm 1.x was a git clone at `~/hm`, a link `/usr/local/bin/hm → ~/hm/bin/run`, and a line
`source ~/hm/console/hm-completion.bash` in `~/.zshrc`, `~/.bashrc` or `~/.bash_profile`.

Run the one-liner. The installer:

1. Replaces the link `/usr/local/bin/hm` with the hm 2 binary, at the same path.
2. Removes the 1.x `source …/console/hm-completion.bash` line from your profiles and adds the new
   completion line.
3. **Leaves `~/hm` where it is.** It tells you that it is no longer used, whether it has local
   changes (`git status`), and the command to delete it:

```console
The 1.x installation at /Users/you/hm is no longer used: hm is now the single binary above.
It has local changes (2 changed files in git status): keep anything you need from it before deleting it.
It was left in place. To delete it: rm -rf /Users/you/hm
```

The installer refuses a 1.x version (`--version 1.5.1` exits 2 and installs nothing): 1.x is the
shell tool and has no binary. To use one, clone it in a directory of its own:

```bash
git clone -b 1.5.1 https://github.com/hiberus-magento/hiberus-dockergento.git ~/hm-1.x
~/hm-1.x/bin/run --help
```

## Updating

`hm update`, or run the installer again.

## Uninstall

```bash
curl -fsSL https://raw.githubusercontent.com/hiberus-magento/hiberus-dockergento/main/install.sh | bash -s -- --uninstall
```

It removes the binary — only if it is the hm 2 the installer put there — the completion file and
its profile line. hm's state in `~/.hm` (cache, settings) is kept; delete it with `rm -rf ~/.hm`.
A 1.x link or clone is never removed.

## Manual install

Without the installer, for example on a machine that cannot pipe into a shell:

```bash
version=2.0.0
asset=hm_$(uname -s | tr A-Z a-z)_$(uname -m | sed 's/x86_64/amd64/; s/aarch64/arm64/')
base=https://github.com/hiberus-magento/hiberus-dockergento/releases/download/$version

curl -fsSL -O "$base/$asset" -O "$base/checksums.txt"
grep " $asset\$" checksums.txt | shasum -a 256 -c -     # sha256sum -c - on Linux
chmod +x "$asset" && sudo mv "$asset" /usr/local/bin/hm

hm completion zsh > ~/.hm-completion.zsh               # optional: load it from ~/.zshrc
```

## Building from source (contributors)

With Go (the version in `go.mod`):

```bash
git clone https://github.com/hiberus-magento/hiberus-dockergento.git
cd hiberus-dockergento
go build -o bin/hm ./cmd/hm
./bin/hm --version
```

Run it as `./bin/hm` or put `bin/hm` on your PATH. Releases are built by GoReleaser from `.goreleaser.yaml`; the
tests for the installer are `tests/unit/installer_test.sh`.
