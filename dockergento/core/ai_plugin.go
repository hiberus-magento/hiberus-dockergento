package core

import (
	"encoding/json"
	"strings"
)

//
// The files of the Claude Code plugin ai-init --plugin writes, in the layout and shapes of
// https://code.claude.com/docs/en/plugins-reference.md and https://code.claude.com/docs/en/hooks.md.
//

// ClaudePluginName is the plugin's name, which Claude Code namespaces its skills under.
const ClaudePluginName = "hm"

// ClaudePluginManifest is .claude-plugin/plugin.json. A version is left out when there is none to
// give: Claude Code pins a plugin to the one it names.
type ClaudePluginManifest struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Version     string `json:"version,omitempty"`
	Description string `json:"description"`
	Author      struct {
		Name string `json:"name"`
	} `json:"author"`
	Homepage string `json:"homepage"`
}

// NewClaudePluginManifest is the manifest of the plugin for this version of the tool.
func NewClaudePluginManifest(version string) ClaudePluginManifest {
	manifest := ClaudePluginManifest{
		Name:        ClaudePluginName,
		DisplayName: "Hiberus Dockergento",
		Version:     version,
		Description: "Skills, the MCP server and a confirmation before dangerous commands, for Magento 2 projects run with hm",
		Homepage:    "https://github.com/hiberus-magento/hiberus-dockergento",
	}
	manifest.Author.Name = "hiberus-magento"

	return manifest
}

// ClaudePluginMCP is .mcp.json: the tool's own MCP server, read-only, started in the project
// Claude Code is working on. Claude Code exports CLAUDE_PROJECT_DIR to a plugin's stdio servers;
// the server answers for the directory it runs in, hence the cd.
func ClaudePluginMCP(binary string) map[string]any {
	return map[string]any{
		"mcpServers": map[string]any{
			ClaudePluginName: map[string]any{
				"command": "sh",
				"args":    []string{"-c", `cd "$CLAUDE_PROJECT_DIR" && exec ` + binary + " mcp"},
			},
		},
	}
}

// ClaudePluginHooks is hooks/hooks.json: before Bash runs one of the commands hm permissions asks
// about — the dangerous ones — the person is asked. A handler holds exactly one permission rule in
// `if`, so there is one per command, each with the very rule permissions prints; the handler only
// prints the decision, so it needs nothing installed to read its input.
func ClaudePluginHooks(permissions AgentPermissions) map[string]any {
	handlers := []map[string]any{}

	for _, rule := range permissions.Permissions.Ask {
		command, ok := bashRuleCommand(rule)
		if !ok {
			continue
		}

		decision, _ := json.Marshal(map[string]any{
			"hookSpecificOutput": map[string]string{
				"hookEventName":            "PreToolUse",
				"permissionDecision":       "ask",
				"permissionDecisionReason": command + " is dangerous: it destroys data or reaches beyond the project.",
			},
		})

		handlers = append(handlers, map[string]any{
			"type":    "command",
			"if":      rule,
			"command": "printf '%s\\n' " + shellQuote(string(decision)),
		})
	}

	return map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []map[string]any{{"matcher": "Bash", "hooks": handlers}},
		},
	}
}

// shellQuote is text as one single-quoted shell word.
func shellQuote(text string) string {
	return "'" + strings.ReplaceAll(text, "'", `'\''`) + "'"
}
