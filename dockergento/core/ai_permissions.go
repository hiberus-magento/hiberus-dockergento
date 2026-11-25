package core

import (
	"fmt"
	"strings"
)

//
// The permissions an agent is given, in the configuration of each agent that is not Claude Code.
// They are read back from the Claude Code rules ReadAgentPermissions builds, so that every format
// carries the one classification and none can drift from it.
//

// AgentRules is the permissions as commands and paths, whatever the agent: the commands allowed
// and asked about ("hm describe"), and the paths denied — a directory ends in a slash.
type AgentRules struct {
	Allow []string
	Ask   []string
	Deny  []string
}

// ReadAgentRules takes the commands out of `Bash(<command>:*)` and the paths out of `Read(./<path>)`.
// A rule of any other shape is not one ReadAgentPermissions writes, and is left out.
func ReadAgentRules(permissions AgentPermissions) AgentRules {
	commands := func(rules []string) []string {
		found := []string{}

		for _, rule := range rules {
			if command, ok := bashRuleCommand(rule); ok {
				found = append(found, command)
			}
		}

		return found
	}

	rules := AgentRules{Allow: commands(permissions.Permissions.Allow), Ask: commands(permissions.Permissions.Ask), Deny: []string{}}

	for _, rule := range permissions.Permissions.Deny {
		path, ok := strings.CutPrefix(rule, "Read(./")
		if !ok {
			continue
		}

		if path, ok = strings.CutSuffix(path, ")"); !ok || path == "" {
			continue
		}

		if directory, isDirectory := strings.CutSuffix(path, "/**"); isDirectory {
			path = directory + "/"
		}

		rules.Deny = append(rules.Deny, path)
	}

	return rules
}

// bashRuleCommand is the command of a `Bash(<command>:*)` rule.
func bashRuleCommand(rule string) (string, bool) {
	command, ok := strings.CutPrefix(rule, "Bash(")
	if !ok {
		return "", false
	}

	command, ok = strings.CutSuffix(command, ":*)")

	return command, ok && command != ""
}

// CodexRulesFile is where Codex reads a project's rules, when the project is trusted.
const CodexRulesFile = ".codex/rules/hm.rules"

// CodexRules is the permissions as a Codex rules file: one prefix_rule() per command, allow or
// prompt (https://learn.chatgpt.com/docs/agent-configuration/rules). Codex has no rule for a path
// an agent must not read, so those are written down as comments rather than dropped.
func CodexRules(permissions AgentPermissions, binary string) string {
	rules := ReadAgentRules(permissions)

	var out strings.Builder

	out.WriteString("# The commands of " + binary + " an agent may run, from " + binary + " permissions --format=codex.\n")
	out.WriteString("# Save as " + CodexRulesFile + " (read when the project is trusted) or ~/.codex/rules/hm.rules.\n")
	out.WriteString("# In Codex, allow runs the command outside the sandbox without prompting; prompt asks first.\n")

	if len(rules.Deny) > 0 {
		out.WriteString("#\n# Codex has no rule for paths. These hold secrets, customer data, or what is generated\n")
		out.WriteString("# rather than written, and an agent should not read them:\n")

		for _, path := range rules.Deny {
			out.WriteString("#   " + path + "\n")
		}
	}

	write := func(commands []string, decision, justification string) {
		for _, command := range commands {
			words := strings.Fields(command)
			quoted := make([]string, len(words))

			for i, word := range words {
				quoted[i] = fmt.Sprintf("%q", word)
			}

			fmt.Fprintf(&out, "prefix_rule(pattern = [%s], decision = %q, justification = %q)\n",
				strings.Join(quoted, ", "), decision, command+" "+justification)
		}
	}

	out.WriteString("\n")
	write(rules.Allow, "allow", "changes nothing, or changes things recoverably")
	write(rules.Ask, "prompt", "destroys data or reaches beyond the project")

	return out.String()
}

// OpenCodeRule is one of OpenCode's permissions.
type OpenCodeRule struct {
	Action   string `json:"action"`
	Resource string `json:"resource"`
	Effect   string `json:"effect"`
}

// OpenCodeConfig is the part of an opencode.json the permissions are.
type OpenCodeConfig struct {
	Schema      string         `json:"$schema"`
	Permissions []OpenCodeRule `json:"permissions"`
}

// OpenCodePermissions is the permissions as OpenCode's ordered list, where the last rule that
// matches wins and a shell pattern ending in " *" also matches the bare command
// (https://opencode.ai/docs/permissions). The commands never overlap — "hm db *" does not match
// "hm debug-on" — so their order does not matter, and the paths come last because a denial must
// not be undone by anything after it.
func OpenCodePermissions(permissions AgentPermissions) OpenCodeConfig {
	rules := ReadAgentRules(permissions)
	config := OpenCodeConfig{Schema: "https://opencode.ai/config.json", Permissions: []OpenCodeRule{}}

	for _, command := range rules.Allow {
		config.Permissions = append(config.Permissions, OpenCodeRule{"shell", command + " *", "allow"})
	}

	for _, command := range rules.Ask {
		config.Permissions = append(config.Permissions, OpenCodeRule{"shell", command + " *", "ask"})
	}

	for _, path := range rules.Deny {
		if strings.HasSuffix(path, "/") {
			path += "*"
		}

		config.Permissions = append(config.Permissions, OpenCodeRule{"read", path, "deny"})
	}

	return config
}
