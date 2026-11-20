# hm completion

Print the shell completion script, for bash or zsh. The binary carries the list of commands, so the
completion always matches the `hm` you are running.

## Usage

```bash
hm completion bash
hm completion zsh
```

## Enable it

bash, in `~/.bashrc`:

```bash
source <(hm completion bash)
```

zsh, in `~/.zshrc`, after `compinit`:

```zsh
source <(hm completion zsh)
```

Or write it once to a directory on `$fpath` as `_hm` (`hm completion zsh > ~/.zsh/completions/_hm`).

On Linux the two macOS-only commands, `copy-to-container` and `copy-from-container`, are left out.

`generate_completion.sh` still writes `console/hm-completion.bash` from a checkout, for the shell
profiles that already source that path; its content is the same as `hm completion bash`.
