# Design

## Why here and not in `ai-tools`

Two reasons, both mechanical rather than a matter of taste.

**They can be verified here.** A test can extract every `hm ...` from every skill and check it
against `command_descriptions.json` — the file that already has to be edited to add a command.
The same shape as `tui_actions_test.sh`, which checks the dashboard's actions against the commands
they invoke, and which is why the dashboard has never shipped an action that does not exist. None
of the five errors above would have survived that test.

**They cannot separate from the source.** Adding a command already means passing through
`command_descriptions.json`; the skill is then in the same directory tree and the same commit. A
skill in another repository is a copy of the truth, and a copy nobody checks is a copy that drifts.

`ai-tools` remains the wider set. What moves is only the part that describes this tool.

## Installed from the installed copy, not downloaded

`hm ai-pull` downloads a tarball of each configured repository. For these skills that would be
exactly wrong: a person running 1.5 would get the skills of `main`, which describe commands their
tool does not have — the same drift in a new direction.

So the bundled skills are a repository whose directory is `$COMMAND_BIN_DIR`. Everything else is
unchanged: the same installer, the same registration, the same "custom skills are not
overwritten". A repository entry with `local: true` and no URL skips the download and nothing
else.

## Four skills, by area of work, not by command

The five that exist are one per command, which is why `varnish-controller` is seven hundred lines
about two commands and why nothing covers `hm describe`. An agent chooses a skill by what it is
trying to do, so the split is by task:

| Skill | For |
|---|---|
| `dockergento-environment` | Start, stop, look at what is running, reach a URL, read logs, diagnose |
| `dockergento-database` | Import, dump, query, snapshots, templates, anonymisation |
| `dockergento-debugging` | Xdebug, Varnish, cache and index state, static checks |
| `dockergento-agents` | Branch environments, the MCP server, permissions, the JSON contract |

Each is short on purpose. A skill is read to choose a command, not to learn shell scripting; the
five existing ones spend most of their six hundred lines on generic bash that a model does not
need and did not ask for.

## What the test checks

- Every `hm <command>` names a command that exists.
- Every option after it is declared for that command in `command_descriptions.json`, or is one of
  the global flags.
- Every service named is a service of the compose template.
- `hm bash` appears only as `hm bash` or `hm bash -r`, because that is all it accepts — the single
  error that made the existing skills unusable.

It is deliberately a whitelist: an unknown option fails the test rather than being assumed
harmless, which is the property that keeps this from happening again.
