package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

// pluginParts is what the plugin bundle is made of, and all that regenerating it replaces: a file
// somebody added beside them stays.
var pluginParts = []string{".claude-plugin", "skills", "hooks", ".mcp.json"}

// Plugin is ai-init --plugin: a Claude Code plugin in dir with the skills that come with the tool,
// its MCP server, and a hook that asks before a command hm permissions classifies as dangerous.
// The project's own AI configuration is not touched.
func (a *AITools) Plugin(dir string) int {
	dir = a.path(dir)

	if !a.pluginDirectory(dir) {
		a.Console.Say(AIError, dir+" is not empty and holds no plugin "+a.Binary+" wrote: nothing was written\n", AIDecorated)

		return 1
	}

	commands, err := a.readData("command_descriptions.json")
	if err != nil {
		a.Console.Say(AIError, "Command definitions not found: "+a.data("command_descriptions.json")+"\n", AIDecorated)

		return 1
	}

	exclusions, err := a.readData("ai-exclusions.json")
	if err != nil {
		a.Console.Say(AIError, "Exclusions not found: "+a.data("ai-exclusions.json")+"\n", AIDecorated)

		return 1
	}

	permissions, err := core.ReadAgentPermissions(commands, exclusions, a.Binary, false)
	if err != nil {
		a.Console.Say(AIError, err.Error()+"\n", AIDecorated)

		return 1
	}

	version := ""
	if a.Version != nil {
		version = a.Version()
	}

	if err := a.writePlugin(dir, version, permissions); err != nil {
		a.Console.Say(AIError, "Failed to write the plugin: "+err.Error()+"\n", AIDecorated)

		return 1
	}

	a.Console.Say(AIInfo, "Claude Code plugin written to "+dir+"\n", AIDecorated)
	a.Console.Say(AIInfo, fmt.Sprintf("  Skills, the MCP server, and a confirmation before %d dangerous command(s)\n",
		len(permissions.Permissions.Ask)), AIDecorated)
	a.Console.Stdout("\n")
	a.Console.Say(AIInfo, "Load it with: claude --plugin-dir "+dir+"\n", AIDecorated)

	return 0
}

// pluginDirectory is whether dir may be written: nothing there, an empty directory, or a plugin
// this tool wrote before.
func (a *AITools) pluginDirectory(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		_, statErr := os.Stat(dir)

		return os.IsNotExist(statErr)
	}

	if len(entries) == 0 {
		return true
	}

	var manifest core.ClaudePluginManifest

	contents, err := os.ReadFile(filepath.Join(dir, ".claude-plugin", "plugin.json"))

	return err == nil && json.Unmarshal(contents, &manifest) == nil && manifest.Name == core.ClaudePluginName
}

func (a *AITools) writePlugin(dir, version string, permissions core.AgentPermissions) error {
	for _, part := range pluginParts {
		if err := os.RemoveAll(filepath.Join(dir, part)); err != nil {
			return err
		}
	}

	if err := extractAssets(a.Tool, dir, "skills"); err != nil {
		return err
	}

	files := map[string]any{
		".claude-plugin/plugin.json": core.NewClaudePluginManifest(version),
		".mcp.json":                  core.ClaudePluginMCP(a.Binary),
		"hooks/hooks.json":           core.ClaudePluginHooks(permissions),
	}

	for name, value := range files {
		// Commands are written as a person reads them: `&&`, not `&&`
		var contents bytes.Buffer

		encoder := json.NewEncoder(&contents)
		encoder.SetEscapeHTML(false)
		encoder.SetIndent("", "  ")

		if err := encoder.Encode(value); err != nil {
			return err
		}

		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}

		if err := os.WriteFile(path, contents.Bytes(), 0o644); err != nil {
			return err
		}
	}

	return nil
}
