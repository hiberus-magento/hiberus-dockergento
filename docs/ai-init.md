# ai-init

Configures a project's AI tooling and installs it for the first time.

```bash
hm ai-init                                    # the wizard
hm ai-init --platforms=claude,codex --types=magento --resources=skills,agents
hm ai-init --plugin                           # only a Claude Code plugin, in ~/.hm/claude-plugin
hm ai-init --plugin=./tools/hm-plugin         # the same, somewhere else
```

The configuration goes to `config/docker/ai-properties.json`, what was installed to
`config/docker/ai-registration.json`, and each platform's skills to the directory it reads them
from — see [skills](skills.md#where-each-platform-reads-them). `hm ai-pull` installs again,
`hm ai-reset` removes what was installed, `hm ai-doctor` says what there is.

If an earlier hm installed skills where a platform no longer reads them — Codex in
`.codex/skills` — `ai-init` lists them with the command that cleans them up. It deletes nothing.

## A Claude Code plugin

`--plugin` writes a [Claude Code plugin](https://code.claude.com/docs/en/plugins-reference.md) and
does nothing else: no configuration, no installation in the project. It takes no other option.

```text
~/.hm/claude-plugin/
├── .claude-plugin/plugin.json   # name "hm", the tool's version
├── skills/                      # the skills that come with this version of the tool
├── hooks/hooks.json             # PreToolUse: ask before a dangerous hm command
└── .mcp.json                    # hm mcp, read-only, in the project Claude Code is working on
```

Load it with:

```bash
claude --plugin-dir ~/.hm/claude-plugin
```

**The hook asks before every command classified `dangerous`** — `hm down`, `hm db`, `hm exec` and
the rest — with the very rules [`hm permissions`](permissions.md) puts under `ask`: one handler
per command, each filtered with `"if": "Bash(hm down:*)"`, printing the `ask` decision. It needs
nothing installed to read its input. The `if` filter is best effort, as the hooks reference
says, so the permissions in your settings remain the thing that enforces; the hook is the
confirmation for an agent whose settings allow more.

**The MCP server** is `hm mcp`, started in `$CLAUDE_PROJECT_DIR`, which Claude Code gives a
plugin's servers. `hm` has to be on the `PATH`. For the write tools, add your own entry with
`hm mcp --config --write` (see [mcp](mcp.md)).

**It is regenerated, not merged.** Run it again after `hm update` and the skills, hook and server
match the new version: the plugin's own parts are replaced, a file you added beside them stays.
A directory that is not empty and holds no plugin hm wrote is refused, and nothing is written.

It is opt-in on purpose. A plugin is loaded into every session that enables it; installing one is
the person's decision, and `--plugin` only prepares it.
