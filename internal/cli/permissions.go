package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

// permissionsFormats is the agents whose configuration permissions prints, the default first.
var permissionsFormats = []string{"claude", "codex", "opencode"}

// permissions prints the permissions an agent using this tool should be given, for its settings
// file. Nothing is written: the configuration file is the person's, and may have rules that matter
// to them.
//
// Under --json the configuration itself is printed, not wrapped in the contract's envelope: it is
// what gets pasted into the agent's settings, and that is how the shell implementation printed it.
// Codex's is not JSON, so under --json it comes as the file it goes in and what that file holds.
func permissions(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	strict, format := false, "claude"
	usage := binaryName() + " permissions [--strict] [--format=" + strings.Join(permissionsFormats, "|") + "]"

	for _, argument := range args {
		value, isFormat := strings.CutPrefix(argument, "--format=")

		switch {
		case argument == "--strict":
			strict = true
		case isFormat && known(value, permissionsFormats):
			format = value
		case isFormat:
			return failure(stderr, jsonOutput, "permissions", exitUsage, "invalid_argument",
				fmt.Sprintf("Unknown format: %q", value), usage)
		default:
			return failure(stderr, jsonOutput, "permissions", exitUsage, "invalid_argument",
				fmt.Sprintf("Unknown option: %s", argument), usage)
		}
	}

	// Codex runs an allowed command outside its sandbox without asking, so only what changes
	// nothing may be allowed there: that is the strict classification, with or without --strict
	configuration, err := newEngine(stdout, stderr, jsonOutput).AgentPermissions(strict || format == "codex")
	if err != nil {
		return report(stderr, jsonOutput, "permissions", err)
	}

	var body []byte

	switch format {
	case "codex":
		rules := core.CodexRules(configuration, binaryName())
		if !jsonOutput {
			body = []byte(strings.TrimSuffix(rules, "\n"))

			break
		}

		body, err = json.MarshalIndent(struct {
			File  string `json:"file"`
			Rules string `json:"rules"`
		}{core.CodexRulesFile, rules}, "", "  ")
	case "opencode":
		body, err = json.MarshalIndent(core.OpenCodePermissions(configuration), "", "  ")
	default:
		body, err = json.MarshalIndent(configuration, "", "  ")
	}

	if err != nil {
		return report(stderr, jsonOutput, "permissions", err)
	}

	if jsonOutput {
		fmt.Fprintf(stdout, "%s\n", body)

		return exitOK
	}

	allowed, asked, denied := len(configuration.Permissions.Allow), len(configuration.Permissions.Ask),
		len(configuration.Permissions.Deny)

	fmt.Fprintf(stdout, "\n%s\n\n", header("Permissions for an agent using "+binaryName()))

	if strict {
		fmt.Fprint(stdout, "  Strict: only commands with no side effects are allowed.\n\n")
	} else {
		fmt.Fprintf(stdout, "  %d command(s) allowed, %d asking for confirmation.\n\n", allowed, asked)
	}

	if format == "codex" {
		fmt.Fprintf(stdout, "  %d path(s) an agent should not read, as comments: Codex has no rule for\n", denied)
		fmt.Fprint(stdout, "  paths. See docs/ai-context.md.\n\n")
	} else {
		fmt.Fprintf(stdout, "  %d path(s) denied outright: secrets, customer data, and what is\n", denied)
		fmt.Fprint(stdout, "  generated rather than written. See docs/ai-context.md.\n\n")
	}

	fmt.Fprintf(stdout, "%s\n\n", body)

	switch format {
	case "codex":
		fmt.Fprint(stdout, "  Save this as "+core.CodexRulesFile+", or in ~/.codex/rules/. Nothing was\n")
		fmt.Fprint(stdout, "  written for you: Codex reads a project's rules only when you trust it.\n\n")
	case "opencode":
		fmt.Fprint(stdout, "  Copy this into your opencode.json. Nothing was written for you: your\n")
		fmt.Fprint(stdout, "  configuration file is yours, and may have rules that matter to you.\n\n")
	default:
		fmt.Fprint(stdout, "  Copy this into your agent's settings. Nothing was written for you: your\n")
		fmt.Fprint(stdout, "  configuration file is yours, and may have rules that matter to you.\n\n")
	}

	return exitOK
}

func known(value string, among []string) bool {
	for _, candidate := range among {
		if value == candidate {
			return true
		}
	}

	return false
}
