package core

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// AgentPermissions is the configuration an agent's settings take: what it may run without asking,
// what it has to ask about, and what it must never read. The field order is the order printed.
type AgentPermissions struct {
	Permissions struct {
		Allow []string `json:"allow"`
		Ask   []string `json:"ask"`
		Deny  []string `json:"deny"`
	} `json:"permissions"`
}

// ReadAgentPermissions builds the permissions from command_descriptions.json and ai-exclusions.json.
//
// A command whose safety is "safe" — or "caution", unless strict — is allowed; every other one is
// asked about. An excluded path is denied, as a directory unless its name has a dot in it. Keys
// starting with an underscore are the file's own notes, not commands. Every list is sorted, as jq's
// sort did for the shell implementation.
func ReadAgentPermissions(commands, exclusions []byte, binary string, strict bool) (AgentPermissions, error) {
	// Raw first: the file's own notes, the keys starting with an underscore, are not shaped like
	// a command (some are lists), so they are set aside before a command is read out of anything
	var entries map[string]json.RawMessage
	if err := json.Unmarshal(commands, &entries); err != nil {
		return AgentPermissions{}, fmt.Errorf("reading command_descriptions.json: %w", err)
	}

	type command struct {
		Safety string `json:"safety"`
	}

	described := map[string]command{}

	for name, raw := range entries {
		if strings.HasPrefix(name, "_") {
			continue
		}

		var one command
		if err := json.Unmarshal(raw, &one); err != nil {
			return AgentPermissions{}, fmt.Errorf("reading command_descriptions.json, %s: %w", name, err)
		}

		described[name] = one
	}

	var excluded struct {
		Exclusions []struct {
			Path string `json:"path"`
		} `json:"exclusions"`
	}
	if err := json.Unmarshal(exclusions, &excluded); err != nil {
		return AgentPermissions{}, fmt.Errorf("reading ai-exclusions.json: %w", err)
	}

	allowed := map[string]bool{"safe": true, "caution": !strict}

	permissions := AgentPermissions{}
	permissions.Permissions.Allow = []string{}
	permissions.Permissions.Ask = []string{}
	permissions.Permissions.Deny = []string{}

	for name, one := range described {
		rule := "Bash(" + binary + " " + name + ":*)"

		if allowed[one.Safety] {
			permissions.Permissions.Allow = append(permissions.Permissions.Allow, rule)
		} else {
			permissions.Permissions.Ask = append(permissions.Permissions.Ask, rule)
		}
	}

	for _, exclusion := range excluded.Exclusions {
		path := "./" + exclusion.Path
		if !strings.Contains(exclusion.Path, ".") {
			path += "/**"
		}

		permissions.Permissions.Deny = append(permissions.Permissions.Deny, "Read("+path+")")
	}

	sort.Strings(permissions.Permissions.Allow)
	sort.Strings(permissions.Permissions.Ask)
	sort.Strings(permissions.Permissions.Deny)

	return permissions, nil
}
